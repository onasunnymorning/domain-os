package rest

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/internal/application/rdesanitize"
	"github.com/onasunnymorning/domain-os/internal/application/workflows"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/temporal"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	"go.temporal.io/sdk/client"
)

// startEscrowSanitizationRequest launches a derivative run. The TLD is
// deliberately absent: it comes from the bound source deposit, so a derivative
// cannot be attributed to a TLD the source did not belong to.
type startEscrowSanitizationRequest struct {
	SourceValidationRunID string `json:"sourceValidationRunId" binding:"required"`
	// SyntheticSuffix overrides the configured default for this run.
	SyntheticSuffix string `json:"syntheticSuffix"`
}

// EscrowSanitizationRunResponse is the API shape of a sanitisation run.
//
// It carries object keys, checksums, counts and reason codes — never a value
// from the deposit.
type EscrowSanitizationRunResponse struct {
	ID       string `json:"id"`
	TLD      string `json:"tld"`
	TenantID string `json:"tenantId"`
	// Label is always "sanitized-pseudonymized": the derivative is not
	// anonymous and must not be described as such.
	Label string `json:"label"`

	SourceValidationRunID string `json:"sourceValidationRunId"`
	SourceDepositID       string `json:"sourceDepositId"`
	SourceArtifactSHA256  string `json:"sourceArtifactSha256"`

	PolicyVersion   string `json:"policyVersion"`
	WorkflowVersion string `json:"workflowVersion"`
	SyntheticSuffix string `json:"syntheticSuffix"`
	WorkflowID      string `json:"workflowId"`
	RunID           string `json:"runId,omitempty"`

	Outcome      string                   `json:"outcome"`
	StageReached string                   `json:"stageReached,omitempty"`
	Findings     []entities.EscrowFinding `json:"findings"`
	// FindingTally is exact where Findings is truncated: one row per distinct
	// thing a code fired on, with every occurrence counted.
	FindingTally []entities.EscrowFindingTally `json:"findingTally"`

	DerivativeObjectKey string                            `json:"derivativeObjectKey,omitempty"`
	DerivativeSHA256    string                            `json:"derivativeSha256,omitempty"`
	DerivativeBytes     int64                             `json:"derivativeBytes,omitempty"`
	ManifestObjectKey   string                            `json:"manifestObjectKey,omitempty"`
	Counts              entities.EscrowSanitizationCounts `json:"counts"`

	StartedAt   time.Time  `json:"startedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

func toSanitizationRunResponse(r *entities.EscrowSanitizationRun) EscrowSanitizationRunResponse {
	findings := r.Findings
	if findings == nil {
		findings = []entities.EscrowFinding{}
	}
	tally := r.FindingTally
	if tally == nil {
		tally = []entities.EscrowFindingTally{}
	}
	return EscrowSanitizationRunResponse{
		ID: r.ID.String(), TLD: r.TLD, TenantID: r.TenantID.String(), Label: entities.EscrowDerivativeLabel,
		SourceValidationRunID: r.SourceValidationRunID.String(), SourceDepositID: r.SourceDepositID.String(),
		SourceArtifactSHA256: r.SourceArtifactSHA256,
		PolicyVersion:        r.PolicyVersion, WorkflowVersion: r.WorkflowVersion, SyntheticSuffix: r.SyntheticSuffix,
		WorkflowID: r.WorkflowID, RunID: r.RunID,
		Outcome: string(r.Outcome), StageReached: r.StageReached, Findings: findings, FindingTally: tally,
		DerivativeObjectKey: r.DerivativeObjectKey, DerivativeSHA256: r.DerivativeSHA256,
		DerivativeBytes: r.DerivativeBytes, ManifestObjectKey: r.ManifestObjectKey, Counts: r.Counts,
		StartedAt: r.StartedAt, CompletedAt: r.CompletedAt,
	}
}

// StartSanitization launches a derivative run against an accepted validation run.
// @Summary Produce a sanitized-pseudonymized derivative of an accepted deposit
// @Tags Escrow
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "Operator scope"
// @Param body body startEscrowSanitizationRequest true "Source validation run"
// @Success 202 {object} startEscrowImportResponse
// @Router /escrow/sanitizations [post]
func (c *EscrowController) StartSanitization(ctx *gin.Context) {
	scope, err := OperatorScopeFromRequest(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var req startEscrowSanitizationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "sourceValidationRunId is required"})
		return
	}
	sourceID, err := uuid.Parse(strings.TrimSpace(req.SourceValidationRunID))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid sourceValidationRunId"})
		return
	}
	// Resolve the source in the caller's scope: another tenant's run is a 404,
	// not a 403, so run ids cannot be enumerated across tenants.
	source, err := c.deps.Runs.GetByID(ctx.Request.Context(), scope, sourceID)
	if err != nil {
		if errors.Is(err, entities.ErrEscrowValidationRunNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "validation run not found"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed"})
		return
	}
	if !entities.EscrowValidationRunIsSanitizableSource(source) {
		ctx.JSON(http.StatusConflict, gin.H{"error": "the source validation run did not pass; only an accepted deposit can be sanitized"})
		return
	}
	if suffix := strings.TrimSpace(req.SyntheticSuffix); suffix != "" {
		normalised, err := entities.NormalizeEscrowTLD(suffix)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid syntheticSuffix"})
			return
		}
		if normalised == source.TLD {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "syntheticSuffix must differ from the source TLD"})
			return
		}
	}

	requestedBy := scope.String()
	if uid, ok := appcontext.UserID(ctx.Request.Context()); ok && strings.TrimSpace(uid) != "" {
		requestedBy = uid
	}

	cfg := temporal.NewClientConfigFromEnv(temporal.QueueHeavyBatch)
	cli, err := temporal.GetTemporalClient(cfg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	// The source run id is in the workflow id: a second launch for the same
	// source collides rather than racing to produce two derivatives.
	wfID := "escrow-sanitize-" + source.TLD + "-" + source.ID.String()
	we, err := cli.ExecuteWorkflow(ctx.Request.Context(), client.StartWorkflowOptions{ID: wfID, TaskQueue: cfg.WorkerQueue},
		workflows.EscrowSanitizeWorkflow, workflows.EscrowSanitizeParams{
			Scope: scope.String(), SourceValidationRunID: source.ID.String(),
			SyntheticSuffix: req.SyntheticSuffix, RequestedBy: requestedBy,
		})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusAccepted, startEscrowImportResponse{
		WorkflowID: we.GetID(), RunID: we.GetRunID(), Status: "started",
		URL: buildTemporalUILink(cfg.Namespace, we.GetID(), we.GetRunID()),
	})
}

// ListSanitizations lists derivative runs for the caller's scope, newest first.
// @Summary List escrow sanitization runs
// @Tags Escrow
// @Produce json
// @Param X-Tenant-ID header string true "Operator scope"
// @Param tld query string false "Filter by TLD"
// @Param outcome query string false "Filter by outcome (RUNNING, PASS, QUARANTINED, ERROR)"
// @Param sourceValidationRunId query string false "Filter by source validation run"
// @Param pagesize query int false "Page size"
// @Param cursor query string false "Cursor from a previous page"
// @Success 200 {object} map[string]interface{}
// @Router /escrow/sanitizations [get]
func (c *EscrowController) ListSanitizations(ctx *gin.Context) {
	scope, err := OperatorScopeFromRequest(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	pageSize, err := GetPageSize(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cursor, err := GetAndDecodeCursor(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	filter := queries.ListEscrowSanitizationRunsFilter{
		OutcomeEquals: strings.ToUpper(strings.TrimSpace(ctx.Query("outcome"))),
	}
	if raw := strings.TrimSpace(ctx.Query("tld")); raw != "" {
		tld, err := entities.NormalizeEscrowTLD(raw)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid tld"})
			return
		}
		filter.TLDEquals = tld
	}
	if raw := strings.TrimSpace(ctx.Query("sourceValidationRunId")); raw != "" {
		if _, err := uuid.Parse(raw); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid sourceValidationRunId"})
			return
		}
		filter.SourceValidationRunIDEquals = raw
	}

	runs, next, err := c.deps.Sanitizations.List(ctx.Request.Context(), scope,
		queries.ListItemsQuery{PageSize: pageSize, PageCursor: cursor, Filter: filter})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "listing failed"})
		return
	}
	items := make([]EscrowSanitizationRunResponse, len(runs))
	for i, r := range runs {
		items[i] = toSanitizationRunResponse(r)
	}
	resp := gin.H{"items": items, "count": len(items)}
	if next != "" {
		resp["nextCursor"] = base64.URLEncoding.EncodeToString([]byte(next))
	}
	ctx.JSON(http.StatusOK, resp)
}

// GetSanitization returns one derivative run.
// @Summary Get an escrow sanitization run
// @Tags Escrow
// @Produce json
// @Param X-Tenant-ID header string true "Operator scope"
// @Param id path string true "Sanitization run ID"
// @Success 200 {object} EscrowSanitizationRunResponse
// @Router /escrow/sanitizations/{id} [get]
func (c *EscrowController) GetSanitization(ctx *gin.Context) {
	scope, err := OperatorScopeFromRequest(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	run, err := c.deps.Sanitizations.GetByID(ctx.Request.Context(), scope, id)
	if err != nil {
		if errors.Is(err, entities.ErrEscrowSanitizationRunNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "sanitization run not found"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"run": toSanitizationRunResponse(run), "policyVersion": rdesanitize.PolicyVersion})
}
