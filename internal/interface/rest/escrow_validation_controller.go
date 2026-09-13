package rest

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"encoding/base64"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/application/workflows"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/temporal"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
	"go.temporal.io/sdk/client"
)

// ---------------------------------------------------------------------------
// Escrow validation (EVE) endpoints — issue #412.
//
// Tenancy (ADR-0006): the operator scope is derived once per request by
// OperatorScopeFromRequest; the TLD is resolved with the scoped repository
// lookup so a tenant cannot validate, list or register keys for a TLD it
// does not operate — and cannot tell whether such a TLD exists.
// ---------------------------------------------------------------------------

// EscrowValidationDeps are the repositories the validation endpoints need.
type EscrowValidationDeps struct {
	TLDs     repositories.TLDRepository
	Deposits repositories.EscrowDepositRepository
	Runs     repositories.EscrowValidationRunRepository
	// Sanitizations is the derivative record set (issue #415).
	Sanitizations repositories.EscrowSanitizationRunRepository
	// KeyRegistry manages escrow parties, keys and arrangements (issue #429).
	// Its routes are registered only when it is set.
	KeyRegistry *services.EscrowKeyService
}

type startEscrowValidationRequest struct {
	TLD string `json:"tld" binding:"required"`
	// Profile selects the artifact shape: "ryde+sig" (default) or "xml" for an
	// unsigned .xml / .xml.gz deposit.
	Profile            string `json:"profile"`
	ArtifactObjectKey  string `json:"artifactObjectKey" binding:"required"`
	SignatureObjectKey string `json:"signatureObjectKey"`
	IntakeRef          string `json:"intakeRef"`
}

// EscrowDepositResponse is the API shape of an escrow deposit record.
type EscrowDepositResponse struct {
	ID                 string    `json:"id"`
	TLD                string    `json:"tld"`
	Profile            string    `json:"profile"`
	ReceivedAt         time.Time `json:"receivedAt"`
	SubmittedBy        string    `json:"submittedBy"`
	IntakeRef          string    `json:"intakeRef,omitempty"`
	ArtifactObjectKey  string    `json:"artifactObjectKey"`
	ArtifactSHA256     string    `json:"artifactSha256"`
	ArtifactBytes      int64     `json:"artifactBytes"`
	SignatureObjectKey string    `json:"signatureObjectKey,omitempty"`
	SignatureSHA256    string    `json:"signatureSha256,omitempty"`
	SignatureBytes     int64     `json:"signatureBytes,omitempty"`
	CreatedAt          time.Time `json:"createdAt"`
}

// EscrowValidationRunResponse is the API shape of a validation run.
type EscrowValidationRunResponse struct {
	ID                       string                         `json:"id"`
	DepositID                string                         `json:"depositId"`
	TLD                      string                         `json:"tld"`
	WorkflowID               string                         `json:"workflowId"`
	RunID                    string                         `json:"runId,omitempty"`
	Profile                  string                         `json:"profile"`
	Outcome                  string                         `json:"outcome"`
	Verified                 bool                           `json:"verified"`
	StageReached             string                         `json:"stageReached,omitempty"`
	Findings                 []entities.EscrowFinding       `json:"findings"`
	SigningKeyFingerprint    string                         `json:"signingKeyFingerprint,omitempty"`
	DecryptionKeyFingerprint string                         `json:"decryptionKeyFingerprint,omitempty"`
	Keys                     *entities.EscrowRunKeyEvidence `json:"keys,omitempty"`
	PlaintextSHA256          string                         `json:"plaintextSha256,omitempty"`
	RDEDepositID             string                         `json:"rdeDepositId,omitempty"`
	RDEKind                  string                         `json:"rdeKind,omitempty"`
	RDEResend                int                            `json:"rdeResend"`
	RDEWatermark             *time.Time                     `json:"rdeWatermark,omitempty"`
	FindingTally             []entities.EscrowFindingTally  `json:"findingTally"`
	SummaryObjectKey         string                         `json:"summaryObjectKey,omitempty"`
	ReportObjectKey          string                         `json:"reportObjectKey,omitempty"`
	NotificationObjectKey    string                         `json:"notificationObjectKey,omitempty"`
	NotificationStatus       string                         `json:"notificationStatus,omitempty"`
	StartedAt                time.Time                      `json:"startedAt"`
	CompletedAt              *time.Time                     `json:"completedAt,omitempty"`
}

func toDepositResponse(d *entities.EscrowDeposit) EscrowDepositResponse {
	return EscrowDepositResponse{
		ID: d.ID.String(), TLD: d.TLD, Profile: d.Profile, ReceivedAt: d.ReceivedAt,
		SubmittedBy: d.SubmittedBy, IntakeRef: d.IntakeRef,
		ArtifactObjectKey: d.ArtifactObjectKey, ArtifactSHA256: d.ArtifactSHA256, ArtifactBytes: d.ArtifactBytes,
		SignatureObjectKey: d.SignatureObjectKey, SignatureSHA256: d.SignatureSHA256, SignatureBytes: d.SignatureBytes,
		CreatedAt: d.CreatedAt,
	}
}

func toRunResponse(r *entities.EscrowValidationRun) EscrowValidationRunResponse {
	findings := r.Findings
	if findings == nil {
		findings = []entities.EscrowFinding{}
	}
	tally := r.FindingTally
	if tally == nil {
		tally = []entities.EscrowFindingTally{}
	}
	var keys *entities.EscrowRunKeyEvidence
	if !r.Keys.IsZero() {
		k := r.Keys
		keys = &k
	}
	return EscrowValidationRunResponse{
		Keys: keys,
		ID:   r.ID.String(), DepositID: r.DepositID.String(), TLD: r.TLD, WorkflowID: r.WorkflowID, RunID: r.RunID,
		Profile: r.Profile, Outcome: string(r.Outcome), Verified: r.Verified(), StageReached: r.StageReached, Findings: findings,
		SigningKeyFingerprint: r.SigningKeyFingerprint, DecryptionKeyFingerprint: r.DecryptionKeyFingerprint, PlaintextSHA256: r.PlaintextSHA256,
		RDEDepositID: r.RDEDepositID, RDEKind: r.RDEKind, RDEResend: r.RDEResend, RDEWatermark: r.RDEWatermark,
		FindingTally:     tally,
		SummaryObjectKey: r.SummaryObjectKey,
		ReportObjectKey:  r.ReportObjectKey, NotificationObjectKey: r.NotificationObjectKey, NotificationStatus: string(r.NotificationStatus),
		StartedAt: r.StartedAt, CompletedAt: r.CompletedAt,
	}
}

// scopedTLD resolves the request scope and checks that the TLD is operated by
// it. A TLD operated by someone else is reported as not found.
func (c *EscrowController) scopedTLD(ctx *gin.Context, rawTLD string) (entities.OperatorID, string, bool) {
	scope, err := OperatorScopeFromRequest(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return "", "", false
	}
	tld, err := entities.NormalizeEscrowTLD(rawTLD)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid tld"})
		return "", "", false
	}
	if _, err := c.deps.TLDs.GetByNameForOperator(ctx.Request.Context(), scope, tld); err != nil {
		if errors.Is(err, entities.ErrTLDNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "tld not found"})
		} else {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "tld lookup failed"})
		}
		return "", "", false
	}
	return scope, tld, true
}

// StartValidation launches the EscrowValidationWorkflow for a .ryde/.sig pair.
// @Summary Validate an RDE deposit (signed .ryde+.sig, or unsigned .xml/.xml.gz)
// @Tags Escrow
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "Operator scope"
// @Param body body startEscrowValidationRequest true "Deposit artifact set"
// @Success 202 {object} startEscrowImportResponse
// @Router /escrow/validations [post]
func (c *EscrowController) StartValidation(ctx *gin.Context) {
	var req startEscrowValidationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "tld and artifactObjectKey are required"})
		return
	}
	profile := req.Profile
	if profile == "" {
		profile = entities.EscrowProfileRydeSig
	}
	if !entities.IsEscrowProfile(profile) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "unknown profile: expected \"ryde+sig\" or \"xml\""})
		return
	}
	switch {
	case entities.EscrowProfileIsSigned(profile) && strings.TrimSpace(req.SignatureObjectKey) == "":
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "signatureObjectKey is required for the ryde+sig profile"})
		return
	case !entities.EscrowProfileIsSigned(profile) && strings.TrimSpace(req.SignatureObjectKey) != "":
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "signatureObjectKey must be omitted for an unsigned profile"})
		return
	}
	scope, tld, ok := c.scopedTLD(ctx, req.TLD)
	if !ok {
		return
	}
	submittedBy := scope.String()
	if uid, ok := appcontext.UserID(ctx.Request.Context()); ok && strings.TrimSpace(uid) != "" {
		submittedBy = uid
	}

	cfg := temporal.NewClientConfigFromEnv(temporal.QueueHeavyBatch)
	cli, err := temporal.GetTemporalClient(cfg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	now := time.Now().UTC()
	wfID := "escrow-validation-" + tld + "-" + now.Format("20060102-150405")
	we, err := cli.ExecuteWorkflow(ctx.Request.Context(), client.StartWorkflowOptions{ID: wfID, TaskQueue: cfg.WorkerQueue},
		workflows.EscrowValidationWorkflow, workflows.EscrowValidationParams{
			Scope: scope.String(), TLD: tld, Profile: profile,
			ArtifactObjectKey: req.ArtifactObjectKey, SignatureObjectKey: req.SignatureObjectKey,
			SubmittedBy: submittedBy, IntakeRef: req.IntakeRef, ReceivedAt: now,
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

// ListValidations lists validation runs for the caller's scope, newest first.
// @Summary List escrow validation runs
// @Tags Escrow
// @Produce json
// @Param X-Tenant-ID header string true "Operator scope"
// @Param tld query string false "Filter by TLD"
// @Param outcome query string false "Filter by outcome (RUNNING, PASS, FAIL, ERROR)"
// @Param keyVersionId query string false "Only runs that verified or decrypted with this key version"
// @Param pagesize query int false "Page size"
// @Param cursor query string false "Cursor from a previous page"
// @Success 200 {object} map[string]interface{}
// @Router /escrow/validations [get]
func (c *EscrowController) ListValidations(ctx *gin.Context) {
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
	filter := queries.ListEscrowValidationRunsFilter{OutcomeEquals: strings.ToUpper(strings.TrimSpace(ctx.Query("outcome")))}
	if raw := strings.TrimSpace(ctx.Query("tld")); raw != "" {
		tld, err := entities.NormalizeEscrowTLD(raw)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid tld"})
			return
		}
		filter.TLDEquals = tld
	}
	if raw := strings.TrimSpace(ctx.Query("keyVersionId")); raw != "" {
		if _, err := uuid.Parse(raw); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid keyVersionId"})
			return
		}
		filter.KeyVersionIDEquals = raw
	}
	runs, next, err := c.deps.Runs.List(ctx.Request.Context(), scope, queries.ListItemsQuery{PageSize: pageSize, PageCursor: cursor, Filter: filter})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "listing failed"})
		return
	}
	items := make([]EscrowValidationRunResponse, len(runs))
	for i, r := range runs {
		items[i] = toRunResponse(r)
	}
	resp := gin.H{"items": items, "count": len(items)}
	if next != "" {
		resp["nextCursor"] = base64.URLEncoding.EncodeToString([]byte(next))
	}
	ctx.JSON(http.StatusOK, resp)
}

// GetValidation returns one run with its deposit.
// @Summary Get an escrow validation run
// @Tags Escrow
// @Produce json
// @Param X-Tenant-ID header string true "Operator scope"
// @Param id path string true "Run ID"
// @Success 200 {object} map[string]interface{}
// @Router /escrow/validations/{id} [get]
func (c *EscrowController) GetValidation(ctx *gin.Context) {
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
	run, err := c.deps.Runs.GetByID(ctx.Request.Context(), scope, id)
	if err != nil {
		if errors.Is(err, entities.ErrEscrowValidationRunNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "validation run not found"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed"})
		return
	}
	resp := gin.H{"run": toRunResponse(run)}
	if dep, err := c.deps.Deposits.GetByID(ctx.Request.Context(), scope, run.DepositID); err == nil {
		resp["deposit"] = toDepositResponse(dep)
	}
	ctx.JSON(http.StatusOK, resp)
}

// GetDeposit returns one deposit with its runs.
// @Summary Get an escrow deposit record
// @Tags Escrow
// @Produce json
// @Param X-Tenant-ID header string true "Operator scope"
// @Param id path string true "Deposit ID"
// @Success 200 {object} map[string]interface{}
// @Router /escrow/deposits/{id} [get]
func (c *EscrowController) GetDeposit(ctx *gin.Context) {
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
	dep, err := c.deps.Deposits.GetByID(ctx.Request.Context(), scope, id)
	if err != nil {
		if errors.Is(err, entities.ErrEscrowDepositNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "deposit not found"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed"})
		return
	}
	runs, err := c.deps.Runs.ListByDeposit(ctx.Request.Context(), scope, id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed"})
		return
	}
	items := make([]EscrowValidationRunResponse, len(runs))
	for i, r := range runs {
		items[i] = toRunResponse(r)
	}
	ctx.JSON(http.StatusOK, gin.H{"deposit": toDepositResponse(dep), "runs": items})
}
