package rest

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/application/workflows"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/temporal"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	"go.temporal.io/sdk/client"
)

// ---------------------------------------------------------------------------
// Escrow key registry endpoints — issue #429, ADR-0009.
//
// Scope. A request with X-Tenant-ID acts for that operator: it sees its own
// parties and the platform's, and changes only its own. A request without it
// acts for the platform, which requires the escrow:platform-keys:admin
// permission even to read, because the platform view is staff-only.
//
// Permission. Every change requires escrow:keys:admin (operator) or
// escrow:platform-keys:admin (platform). The legacy shared token has neither,
// so it can read an operator's registry but change nothing.
//
// Material. Import request bodies are capped, never logged, and never echoed:
// no response or error carries key material or a secret reference.
// ---------------------------------------------------------------------------

// maxKeyImportBodyBytes bounds an import request; a large armored key is a few KB.
const maxKeyImportBodyBytes = 64 << 10

// EscrowKeyOwnerResponse names who owns a party.
type EscrowKeyOwnerResponse struct {
	Kind     string `json:"kind"`
	Operator string `json:"operator,omitempty"`
}

// EscrowPartyResponse is the API shape of a party.
type EscrowPartyResponse struct {
	ID         string                 `json:"id"`
	Owner      EscrowKeyOwnerResponse `json:"owner"`
	Name       string                 `json:"name"`
	Kind       string                 `json:"kind"`
	Side       string                 `json:"side"`
	Purposes   []string               `json:"purposes"`
	Manageable bool                   `json:"manageable"`
	CreatedAt  time.Time              `json:"createdAt"`
	CreatedBy  string                 `json:"createdBy,omitempty"`
}

// EscrowKeyVersionResponse is the API shape of a key version. It carries the
// public key and public metadata only.
type EscrowKeyVersionResponse struct {
	ID               string     `json:"id"`
	PartyID          string     `json:"partyId"`
	Owner            string     `json:"owner"`
	Purpose          string     `json:"purpose"`
	Material         string     `json:"material"`
	Version          int        `json:"version"`
	Fingerprint      string     `json:"fingerprint"`
	HasPublicKey     bool       `json:"hasPublicKey"`
	State            string     `json:"state"`
	NotBefore        *time.Time `json:"notBefore,omitempty"`
	NotAfter         *time.Time `json:"notAfter,omitempty"`
	KeyExpiresAt     *time.Time `json:"keyExpiresAt,omitempty"`
	LastProbeAt      *time.Time `json:"lastProbeAt,omitempty"`
	LastProbeOK      bool       `json:"lastProbeOk"`
	ActivatedAt      *time.Time `json:"activatedAt,omitempty"`
	DeactivatedAt    *time.Time `json:"deactivatedAt,omitempty"`
	RevokedAt        *time.Time `json:"revokedAt,omitempty"`
	RevocationReason string     `json:"revocationReason,omitempty"`
	Compromised      bool       `json:"compromised"`
	DestroyedAt      *time.Time `json:"destroyedAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	CreatedBy        string     `json:"createdBy,omitempty"`
	ProbeWorkflowID  string     `json:"probeWorkflowId,omitempty"`
}

// EscrowArrangementResponse is the API shape of one arrangement revision.
type EscrowArrangementResponse struct {
	ID               string    `json:"id"`
	Level            string    `json:"level"`
	Operator         string    `json:"operator,omitempty"`
	TLD              string    `json:"tld,omitempty"`
	DepositorPartyID *string   `json:"depositorPartyId,omitempty"`
	ReceiverPartyID  *string   `json:"receiverPartyId,omitempty"`
	Revision         int       `json:"revision"`
	CreatedAt        time.Time `json:"createdAt"`
	CreatedBy        string    `json:"createdBy,omitempty"`
}

// EscrowKeyAuditEventResponse is the API shape of one audit entry.
type EscrowKeyAuditEventResponse struct {
	ID            string    `json:"id"`
	Action        string    `json:"action"`
	Subject       string    `json:"subject"`
	SubjectID     string    `json:"subjectId"`
	Purpose       string    `json:"purpose,omitempty"`
	Version       int       `json:"version,omitempty"`
	Fingerprint   string    `json:"fingerprint,omitempty"`
	StateBefore   string    `json:"stateBefore,omitempty"`
	StateAfter    string    `json:"stateAfter,omitempty"`
	Reason        string    `json:"reason,omitempty"`
	Compromised   bool      `json:"compromised,omitempty"`
	Actor         string    `json:"actor,omitempty"`
	CorrelationID string    `json:"correlationId,omitempty"`
	At            time.Time `json:"at"`
}

type createEscrowPartyRequest struct {
	Name string `json:"name" binding:"required"`
	Kind string `json:"kind" binding:"required"`
	Side string `json:"side" binding:"required"`
}

type importPrivateKeyRequest struct {
	Purpose           string     `json:"purpose" binding:"required"`
	ArmoredPrivateKey string     `json:"armoredPrivateKey" binding:"required"`
	Passphrase        string     `json:"passphrase"` //nolint:gosec // G117: request field; forwarded to the key store and never echoed
	NotBefore         *time.Time `json:"notBefore"`
	NotAfter          *time.Time `json:"notAfter"`
}

type addPublicKeyRequest struct {
	Purpose          string     `json:"purpose" binding:"required"`
	ArmoredPublicKey string     `json:"armoredPublicKey" binding:"required"`
	NotBefore        *time.Time `json:"notBefore"`
	NotAfter         *time.Time `json:"notAfter"`
}

type generateKeyRequest struct {
	Purpose string `json:"purpose" binding:"required"`
}

type activateKeyRequest struct {
	ConfirmReplace bool `json:"confirmReplace"`
}

type revokeKeyRequest struct {
	Reason      string `json:"reason" binding:"required"`
	Compromised bool   `json:"compromised"`
}

type destroyKeyRequest struct {
	ConfirmFingerprint string `json:"confirmFingerprint" binding:"required"`
}

type setArrangementRequest struct {
	DepositorPartyID *string `json:"depositorPartyId"`
	ReceiverPartyID  *string `json:"receiverPartyId"`
}

// ---------- mapping ----------

func toPartyResponse(p *entities.EscrowParty, scope entities.EscrowKeyScope) EscrowPartyResponse {
	purposes := make([]string, 0, len(p.Purposes()))
	for _, pu := range p.Purposes() {
		if policy, err := entities.EscrowKeyPurposePolicyFor(pu); err == nil && policy.Supported {
			purposes = append(purposes, string(pu))
		}
	}
	return EscrowPartyResponse{
		ID: p.ID.String(), Owner: EscrowKeyOwnerResponse{Kind: string(p.Owner.Kind), Operator: p.Owner.Operator.String()},
		Name: p.Name, Kind: string(p.Kind), Side: string(p.Side), Purposes: purposes, Manageable: scope.CanManage(p.Owner),
		CreatedAt: p.CreatedAt, CreatedBy: p.CreatedBy,
	}
}

func toKeyVersionResponse(v *entities.EscrowKeyVersion) EscrowKeyVersionResponse {
	return EscrowKeyVersionResponse{
		ID: v.ID.String(), PartyID: v.PartyID.String(), Owner: v.Owner.String(), Purpose: string(v.Purpose),
		Material: string(v.Policy().Material), Version: v.Version, Fingerprint: v.Fingerprint, HasPublicKey: v.ArmoredPublicKey != "",
		State: string(v.State), NotBefore: v.NotBefore, NotAfter: v.NotAfter, KeyExpiresAt: v.KeyExpiresAt,
		LastProbeAt: v.LastProbeAt, LastProbeOK: v.LastProbeOK, ActivatedAt: v.ActivatedAt, DeactivatedAt: v.DeactivatedAt,
		RevokedAt: v.RevokedAt, RevocationReason: v.RevocationReason, Compromised: v.Compromised, DestroyedAt: v.DestroyedAt,
		CreatedAt: v.CreatedAt, CreatedBy: v.CreatedBy,
	}
}

func uuidPtrString(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}

func toArrangementResponse(a *entities.EscrowArrangement) EscrowArrangementResponse {
	return EscrowArrangementResponse{
		ID: a.ID.String(), Level: string(a.Level), Operator: a.Operator.String(), TLD: a.TLD,
		DepositorPartyID: uuidPtrString(a.DepositorPartyID), ReceiverPartyID: uuidPtrString(a.ReceiverPartyID),
		Revision: a.Revision, CreatedAt: a.CreatedAt, CreatedBy: a.CreatedBy,
	}
}

func toAuditResponse(e *entities.EscrowKeyAuditEvent) EscrowKeyAuditEventResponse {
	return EscrowKeyAuditEventResponse{
		ID: e.ID.String(), Action: string(e.Action), Subject: string(e.Subject), SubjectID: e.SubjectID.String(),
		Purpose: string(e.Purpose), Version: e.Version, Fingerprint: e.Fingerprint, StateBefore: e.StateBefore, StateAfter: e.StateAfter,
		Reason: e.Reason, Compromised: e.Compromised, Actor: e.Actor, CorrelationID: e.CorrelationID, At: e.At,
	}
}

// ---------- scope, permission, errors ----------

// escrowKeyScope derives the key-registry scope for a request (see the file
// comment). It writes the error response and returns false when it cannot.
func escrowKeyScope(ctx *gin.Context) (entities.EscrowKeyScope, bool) {
	op, err := OperatorScopeFromRequest(ctx)
	switch {
	case err == nil:
		return entities.OperatorEscrowKeyScope(op), true
	case !errors.Is(err, ErrMissingOperatorScope):
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return entities.EscrowKeyScope{}, false
	case hasAuthScope(ctx, ScopeEscrowPlatformKeysAdmin):
		// This is where the platform scope is granted: the principal's token
		// carries the platform permission (ADR-0006: a claim, not a header).
		return entities.PlatformEscrowKeyScope(entities.NewPlatformScope()), true
	default:
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error() + ", or use a token with the " + ScopeEscrowPlatformKeysAdmin + " permission for the platform registry"})
		return entities.EscrowKeyScope{}, false
	}
}

// requireKeyManagement checks the permission a change needs in this scope.
func requireKeyManagement(ctx *gin.Context, scope entities.EscrowKeyScope) bool {
	if scope.IsPlatform() {
		return requireAuthScope(ctx, ScopeEscrowPlatformKeysAdmin)
	}
	return requireAuthScope(ctx, ScopeEscrowKeysAdmin)
}

func escrowKeyError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, entities.ErrEscrowPartyNotFound), errors.Is(err, entities.ErrEscrowKeyVersionNotFound),
		errors.Is(err, entities.ErrEscrowArrangementNotFound), errors.Is(err, entities.ErrTLDNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"error": fixedEscrowKeyMessage(err)})
	case errors.Is(err, entities.ErrEscrowKeyOwnerMismatch):
		ctx.JSON(http.StatusForbidden, gin.H{"error": "this party belongs to another owner and can only be used, not changed, from this scope"})
	case errors.Is(err, entities.ErrEscrowKeyInvalidTransition), errors.Is(err, entities.ErrEscrowKeyVersionConflict),
		errors.Is(err, entities.ErrEscrowArrangementConflict), errors.Is(err, entities.ErrEscrowKeyDuplicateFingerprint),
		errors.Is(err, entities.ErrEscrowKeyReplaceNotConfirmed), errors.Is(err, entities.ErrEscrowKeyNotProbed):
		ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, entities.ErrEscrowKeyStoreNotConfigured), errors.Is(err, entities.ErrEscrowKeyStoreUnavailable):
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": fixedEscrowKeyMessage(err)})
	case errors.Is(err, entities.ErrEscrowKeyMaterialUnreadable):
		// A rejection carries a reason about the shape of what was submitted,
		// never its contents. Without one, stay generic: the private-key path
		// must not say whether the block or the passphrase was wrong.
		var rejected *entities.EscrowKeyMaterialRejection
		if errors.As(err, &rejected) {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": rejected.Reason})
			return
		}
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "the key material could not be read: expected exactly one ASCII-armored OpenPGP key of the right kind, unlockable with the given passphrase"})
	case errors.Is(err, entities.ErrInvalidEscrowParty), errors.Is(err, entities.ErrInvalidEscrowKeyVersion),
		errors.Is(err, entities.ErrInvalidEscrowArrangement), errors.Is(err, entities.ErrEscrowPartyRoleNotSupported),
		errors.Is(err, entities.ErrEscrowKeyPurposeNotAllowed), errors.Is(err, entities.ErrEscrowKeyPurposeNotSupported),
		errors.Is(err, entities.ErrEscrowKeyMaterialNotApplicable), errors.Is(err, entities.ErrInvalidEscrowKeyOwner):
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "the escrow key registry request failed"})
	}
}

// fixedEscrowKeyMessage returns the sentinel's own text, not a wrapped chain
// that could name a secret or an internal id.
func fixedEscrowKeyMessage(err error) string {
	for _, s := range []error{entities.ErrEscrowPartyNotFound, entities.ErrEscrowKeyVersionNotFound, entities.ErrEscrowArrangementNotFound,
		entities.ErrTLDNotFound, entities.ErrEscrowKeyStoreNotConfigured, entities.ErrEscrowKeyStoreUnavailable} {
		if errors.Is(err, s) {
			return s.Error()
		}
	}
	return "not found"
}

func pathUUID(ctx *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(ctx.Param(name))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + name})
		return uuid.Nil, false
	}
	return id, true
}

func optionalUUID(raw *string) (*uuid.UUID, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	id, err := uuid.Parse(strings.TrimSpace(*raw))
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// ---------- probe starter ----------

// TemporalEscrowKeyProbeStarter starts EscrowKeyProbeWorkflow on the heavy-batch
// queue, where the escrow activities that use the keys run.
type TemporalEscrowKeyProbeStarter struct{}

var _ interfaces.EscrowKeyProbeStarter = TemporalEscrowKeyProbeStarter{}

// StartEscrowKeyProbe implements interfaces.EscrowKeyProbeStarter.
func (TemporalEscrowKeyProbeStarter) StartEscrowKeyProbe(ctx context.Context, owner entities.EscrowKeyOwner, versionID uuid.UUID, requestedBy string) (string, error) {
	cfg := temporal.NewClientConfigFromEnv(temporal.QueueHeavyBatch)
	cli, err := temporal.GetTemporalClient(cfg)
	if err != nil {
		return "", err
	}
	defer cli.Close()
	wfID := "escrow-key-probe-" + versionID.String() + "-" + time.Now().UTC().Format("20060102-150405")
	we, err := cli.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: wfID, TaskQueue: cfg.WorkerQueue},
		workflows.EscrowKeyProbeWorkflow, workflows.EscrowKeyProbeParams{
			Owner: activities.OwnerRefFor(owner), VersionID: versionID.String(), RequestedBy: requestedBy,
		})
	if err != nil {
		return "", err
	}
	return we.GetID(), nil
}

// ---------- handlers: parties ----------

func (c *EscrowController) registerKeyRoutes(grp *gin.RouterGroup) {
	grp.GET("/parties", c.ListParties)
	grp.POST("/parties", c.CreateParty)
	grp.GET("/parties/:id", c.GetParty)
	grp.GET("/parties/:id/audit", c.GetPartyAudit)
	grp.POST("/parties/:id/versions/private", c.ImportPrivateKey)
	grp.POST("/parties/:id/versions/public", c.AddPublicKey)
	grp.POST("/parties/:id/versions/generate", c.GenerateKey)

	grp.GET("/key-versions/:id", c.GetKeyVersion)
	grp.GET("/key-versions/:id/public-key", c.GetPublicKey)
	grp.POST("/key-versions/:id/probe", c.ProbeKeyVersion)
	grp.POST("/key-versions/:id/activate", c.ActivateKeyVersion)
	grp.POST("/key-versions/:id/deactivate", c.DeactivateKeyVersion)
	grp.POST("/key-versions/:id/revoke", c.RevokeKeyVersion)
	grp.POST("/key-versions/:id/destroy", c.DestroyKeyVersion)

	grp.GET("/arrangements/default", c.GetDefaultArrangement)
	grp.PUT("/arrangements/default", c.SetDefaultArrangement)
	grp.DELETE("/arrangements/default", c.RemoveDefaultArrangement)
	grp.GET("/arrangements/tlds", c.ListTLDArrangements)
	grp.GET("/arrangements/tlds/:tld", c.GetTLDArrangement)
	grp.PUT("/arrangements/tlds/:tld", c.SetTLDArrangement)
	grp.DELETE("/arrangements/tlds/:tld", c.RemoveTLDArrangement)
	grp.GET("/arrangements/tlds/:tld/effective", c.GetEffectiveArrangement)
}

// ListParties lists the parties visible in the request scope.
// @Summary List escrow parties
// @Tags Escrow keys
// @Produce json
// @Param X-Tenant-ID header string false "Operator scope; omit for the platform registry"
// @Success 200 {object} map[string]interface{}
// @Router /escrow/parties [get]
func (c *EscrowController) ListParties(ctx *gin.Context) {
	scope, ok := escrowKeyScope(ctx)
	if !ok {
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
	parties, next, err := c.deps.KeyRegistry.ListParties(ctx.Request.Context(), scope, queries.ListItemsQuery{PageSize: pageSize, PageCursor: cursor})
	if err != nil {
		escrowKeyError(ctx, err)
		return
	}
	items := make([]EscrowPartyResponse, len(parties))
	for i, p := range parties {
		items[i] = toPartyResponse(p, scope)
	}
	resp := gin.H{"items": items, "count": len(items)}
	if next != "" {
		resp["nextCursor"] = base64.URLEncoding.EncodeToString([]byte(next))
	}
	ctx.JSON(http.StatusOK, resp)
}

// CreateParty registers a party owned by the request scope.
// @Summary Register an escrow party
// @Tags Escrow keys
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string false "Operator scope; omit for the platform registry"
// @Param body body createEscrowPartyRequest true "Party"
// @Success 201 {object} EscrowPartyResponse
// @Router /escrow/parties [post]
func (c *EscrowController) CreateParty(ctx *gin.Context) {
	scope, ok := escrowKeyScope(ctx)
	if !ok || !requireKeyManagement(ctx, scope) {
		return
	}
	var req createEscrowPartyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "name, kind and side are required"})
		return
	}
	p, err := c.deps.KeyRegistry.CreateParty(ctx.Request.Context(), scope, commands.CreateEscrowPartyCommand{Name: req.Name, Kind: req.Kind, Side: req.Side})
	if err != nil {
		escrowKeyError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, toPartyResponse(p, scope))
}

// GetParty returns a party with its key versions and the arrangements using it.
// @Summary Get an escrow party
// @Tags Escrow keys
// @Produce json
// @Param id path string true "Party ID"
// @Success 200 {object} map[string]interface{}
// @Router /escrow/parties/{id} [get]
func (c *EscrowController) GetParty(ctx *gin.Context) {
	scope, ok := escrowKeyScope(ctx)
	if !ok {
		return
	}
	id, ok := pathUUID(ctx, "id")
	if !ok {
		return
	}
	d, err := c.deps.KeyRegistry.GetParty(ctx.Request.Context(), scope, id)
	if err != nil {
		escrowKeyError(ctx, err)
		return
	}
	versions := make([]EscrowKeyVersionResponse, len(d.Versions))
	for i, v := range d.Versions {
		versions[i] = toKeyVersionResponse(v)
	}
	usedBy := make([]EscrowArrangementResponse, len(d.UsedBy))
	for i, a := range d.UsedBy {
		usedBy[i] = toArrangementResponse(a)
	}
	ctx.JSON(http.StatusOK, gin.H{"party": toPartyResponse(d.Party, scope), "versions": versions, "usedBy": usedBy})
}

// GetPartyAudit returns a party's audit trail.
// @Summary Get an escrow party's audit trail
// @Tags Escrow keys
// @Produce json
// @Param id path string true "Party ID"
// @Success 200 {object} map[string]interface{}
// @Router /escrow/parties/{id}/audit [get]
func (c *EscrowController) GetPartyAudit(ctx *gin.Context) {
	scope, ok := escrowKeyScope(ctx)
	if !ok {
		return
	}
	id, ok := pathUUID(ctx, "id")
	if !ok {
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
	events, next, err := c.deps.KeyRegistry.PartyAudit(ctx.Request.Context(), scope, id, queries.ListItemsQuery{PageSize: pageSize, PageCursor: cursor})
	if err != nil {
		escrowKeyError(ctx, err)
		return
	}
	items := make([]EscrowKeyAuditEventResponse, len(events))
	for i, e := range events {
		items[i] = toAuditResponse(e)
	}
	resp := gin.H{"items": items, "count": len(items)}
	if next != "" {
		resp["nextCursor"] = base64.URLEncoding.EncodeToString([]byte(next))
	}
	ctx.JSON(http.StatusOK, resp)
}

// ---------- handlers: adding versions ----------

func (c *EscrowController) versionCreated(ctx *gin.Context, out *services.EscrowKeyVersionCreated, err error) {
	if err != nil {
		escrowKeyError(ctx, err)
		return
	}
	resp := toKeyVersionResponse(out.Version)
	resp.ProbeWorkflowID = out.ProbeWorkflowID
	ctx.JSON(http.StatusCreated, resp)
}

// ImportPrivateKey imports an OpenPGP private key as a new STAGED version and
// starts its probe.
// @Summary Import a private key version
// @Tags Escrow keys
// @Accept json
// @Produce json
// @Param id path string true "Party ID"
// @Param body body importPrivateKeyRequest true "Key"
// @Success 201 {object} EscrowKeyVersionResponse
// @Router /escrow/parties/{id}/versions/private [post]
func (c *EscrowController) ImportPrivateKey(ctx *gin.Context) {
	scope, ok := escrowKeyScope(ctx)
	if !ok || !requireKeyManagement(ctx, scope) {
		return
	}
	id, ok := pathUUID(ctx, "id")
	if !ok {
		return
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxKeyImportBodyBytes)
	var req importPrivateKeyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		// Never include the decoder's error: it can quote the body.
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "purpose and armoredPrivateKey are required, and the request must be under 64 KB"})
		return
	}
	out, err := c.deps.KeyRegistry.ImportPrivateKey(ctx.Request.Context(), scope, id, commands.ImportEscrowPrivateKeyCommand{
		Purpose: req.Purpose, ArmoredPrivateKey: req.ArmoredPrivateKey, Passphrase: req.Passphrase, NotBefore: req.NotBefore, NotAfter: req.NotAfter,
	})
	c.versionCreated(ctx, out, err)
}

// AddPublicKey adds a counterparty public key as a new STAGED version.
// @Summary Add a public key version
// @Tags Escrow keys
// @Accept json
// @Produce json
// @Param id path string true "Party ID"
// @Param body body addPublicKeyRequest true "Key"
// @Success 201 {object} EscrowKeyVersionResponse
// @Router /escrow/parties/{id}/versions/public [post]
func (c *EscrowController) AddPublicKey(ctx *gin.Context) {
	scope, ok := escrowKeyScope(ctx)
	if !ok || !requireKeyManagement(ctx, scope) {
		return
	}
	id, ok := pathUUID(ctx, "id")
	if !ok {
		return
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxKeyImportBodyBytes)
	var req addPublicKeyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "purpose and armoredPublicKey are required, and the request must be under 64 KB"})
		return
	}
	out, err := c.deps.KeyRegistry.AddPublicKey(ctx.Request.Context(), scope, id, commands.AddEscrowPublicKeyCommand{
		Purpose: req.Purpose, ArmoredPublicKey: req.ArmoredPublicKey, NotBefore: req.NotBefore, NotAfter: req.NotAfter,
	})
	c.versionCreated(ctx, out, err)
}

// GenerateKey has the service generate a symmetric key version.
// @Summary Generate a symmetric key version
// @Tags Escrow keys
// @Accept json
// @Produce json
// @Param id path string true "Party ID"
// @Param body body generateKeyRequest true "Purpose"
// @Success 201 {object} EscrowKeyVersionResponse
// @Router /escrow/parties/{id}/versions/generate [post]
func (c *EscrowController) GenerateKey(ctx *gin.Context) {
	scope, ok := escrowKeyScope(ctx)
	if !ok || !requireKeyManagement(ctx, scope) {
		return
	}
	id, ok := pathUUID(ctx, "id")
	if !ok {
		return
	}
	var req generateKeyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "purpose is required"})
		return
	}
	out, err := c.deps.KeyRegistry.GenerateSymmetricKey(ctx.Request.Context(), scope, id, commands.GenerateEscrowSymmetricKeyCommand{Purpose: req.Purpose})
	c.versionCreated(ctx, out, err)
}

// ---------- handlers: lifecycle ----------

// GetKeyVersion returns one version.
// @Summary Get a key version
// @Tags Escrow keys
// @Produce json
// @Param id path string true "Key version ID"
// @Success 200 {object} EscrowKeyVersionResponse
// @Router /escrow/key-versions/{id} [get]
func (c *EscrowController) GetKeyVersion(ctx *gin.Context) {
	scope, ok := escrowKeyScope(ctx)
	if !ok {
		return
	}
	id, ok := pathUUID(ctx, "id")
	if !ok {
		return
	}
	v, err := c.deps.KeyRegistry.GetVersion(ctx.Request.Context(), scope, id)
	if err != nil {
		escrowKeyError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, toKeyVersionResponse(v))
}

// GetPublicKey returns a version's armored public key, e.g. to hand to a registry.
// @Summary Download a key version's public key
// @Tags Escrow keys
// @Produce plain
// @Param id path string true "Key version ID"
// @Success 200 {string} string
// @Router /escrow/key-versions/{id}/public-key [get]
func (c *EscrowController) GetPublicKey(ctx *gin.Context) {
	scope, ok := escrowKeyScope(ctx)
	if !ok {
		return
	}
	id, ok := pathUUID(ctx, "id")
	if !ok {
		return
	}
	v, err := c.deps.KeyRegistry.GetVersion(ctx.Request.Context(), scope, id)
	if err != nil {
		escrowKeyError(ctx, err)
		return
	}
	if v.ArmoredPublicKey == "" {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "this key version has no public key"})
		return
	}
	ctx.Header("Content-Disposition", `attachment; filename="`+v.Fingerprint+`.asc"`)
	ctx.Data(http.StatusOK, "application/pgp-keys", []byte(v.ArmoredPublicKey))
}

// ProbeKeyVersion starts a probe of a version.
// @Summary Probe a key version on a worker
// @Tags Escrow keys
// @Produce json
// @Param id path string true "Key version ID"
// @Success 202 {object} map[string]interface{}
// @Router /escrow/key-versions/{id}/probe [post]
func (c *EscrowController) ProbeKeyVersion(ctx *gin.Context) {
	scope, ok := escrowKeyScope(ctx)
	if !ok || !requireKeyManagement(ctx, scope) {
		return
	}
	id, ok := pathUUID(ctx, "id")
	if !ok {
		return
	}
	wfID, err := c.deps.KeyRegistry.Probe(ctx.Request.Context(), scope, id)
	if err != nil {
		escrowKeyError(ctx, err)
		return
	}
	ctx.JSON(http.StatusAccepted, gin.H{"workflowId": wfID})
}

func (c *EscrowController) lifecycle(ctx *gin.Context, bind interface{}, do func(context.Context, entities.EscrowKeyScope, uuid.UUID) (*entities.EscrowKeyVersion, error)) {
	scope, ok := escrowKeyScope(ctx)
	if !ok || !requireKeyManagement(ctx, scope) {
		return
	}
	id, ok := pathUUID(ctx, "id")
	if !ok {
		return
	}
	if bind != nil {
		if err := ctx.ShouldBindJSON(bind); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
	}
	v, err := do(ctx.Request.Context(), scope, id)
	if err != nil {
		escrowKeyError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, toKeyVersionResponse(v))
}

// ActivateKeyVersion activates a version.
// @Summary Activate a key version
// @Tags Escrow keys
// @Accept json
// @Produce json
// @Param id path string true "Key version ID"
// @Param body body activateKeyRequest false "Confirm replacing the active version"
// @Success 200 {object} EscrowKeyVersionResponse
// @Router /escrow/key-versions/{id}/activate [post]
func (c *EscrowController) ActivateKeyVersion(ctx *gin.Context) {
	var req activateKeyRequest
	var bind interface{}
	if ctx.Request.ContentLength > 0 {
		bind = &req
	}
	c.lifecycle(ctx, bind, func(rc context.Context, scope entities.EscrowKeyScope, id uuid.UUID) (*entities.EscrowKeyVersion, error) {
		return c.deps.KeyRegistry.Activate(rc, scope, id, commands.ActivateEscrowKeyVersionCommand{ConfirmReplace: req.ConfirmReplace})
	})
}

// DeactivateKeyVersion stops using a version for new work.
// @Summary Deactivate a key version
// @Tags Escrow keys
// @Produce json
// @Param id path string true "Key version ID"
// @Success 200 {object} EscrowKeyVersionResponse
// @Router /escrow/key-versions/{id}/deactivate [post]
func (c *EscrowController) DeactivateKeyVersion(ctx *gin.Context) {
	c.lifecycle(ctx, nil, c.deps.KeyRegistry.Deactivate)
}

// RevokeKeyVersion withdraws a version from every use.
// @Summary Revoke a key version
// @Tags Escrow keys
// @Accept json
// @Produce json
// @Param id path string true "Key version ID"
// @Param body body revokeKeyRequest true "Reason"
// @Success 200 {object} EscrowKeyVersionResponse
// @Router /escrow/key-versions/{id}/revoke [post]
func (c *EscrowController) RevokeKeyVersion(ctx *gin.Context) {
	var req revokeKeyRequest
	c.lifecycle(ctx, &req, func(rc context.Context, scope entities.EscrowKeyScope, id uuid.UUID) (*entities.EscrowKeyVersion, error) {
		return c.deps.KeyRegistry.Revoke(rc, scope, id, commands.RevokeEscrowKeyVersionCommand{Reason: req.Reason, Compromised: req.Compromised})
	})
}

// DestroyKeyVersion destroys a version's material.
// @Summary Destroy a key version's material
// @Tags Escrow keys
// @Accept json
// @Produce json
// @Param id path string true "Key version ID"
// @Param body body destroyKeyRequest true "Fingerprint confirmation"
// @Success 200 {object} EscrowKeyVersionResponse
// @Router /escrow/key-versions/{id}/destroy [post]
func (c *EscrowController) DestroyKeyVersion(ctx *gin.Context) {
	var req destroyKeyRequest
	c.lifecycle(ctx, &req, func(rc context.Context, scope entities.EscrowKeyScope, id uuid.UUID) (*entities.EscrowKeyVersion, error) {
		return c.deps.KeyRegistry.Destroy(rc, scope, id, commands.DestroyEscrowKeyVersionCommand{ConfirmFingerprint: req.ConfirmFingerprint})
	})
}

// ---------- handlers: arrangements ----------

func (c *EscrowController) arrangementTLD(ctx *gin.Context) (string, bool) {
	tld, err := entities.NormalizeEscrowTLD(ctx.Param("tld"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid tld"})
		return "", false
	}
	return tld, true
}

func (c *EscrowController) getArrangement(ctx *gin.Context, scope entities.EscrowKeyScope, tld string) {
	a, err := c.deps.KeyRegistry.GetArrangement(ctx.Request.Context(), scope, tld)
	if err != nil {
		escrowKeyError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, toArrangementResponse(a))
}

func (c *EscrowController) setArrangement(ctx *gin.Context, scope entities.EscrowKeyScope, tld string) {
	if !requireKeyManagement(ctx, scope) {
		return
	}
	var req setArrangementRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	depositor, err := optionalUUID(req.DepositorPartyID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid depositorPartyId"})
		return
	}
	receiver, err := optionalUUID(req.ReceiverPartyID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid receiverPartyId"})
		return
	}
	a, err := c.deps.KeyRegistry.SetArrangement(ctx.Request.Context(), scope, tld, commands.SetEscrowArrangementCommand{DepositorPartyID: depositor, ReceiverPartyID: receiver})
	if err != nil {
		escrowKeyError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, toArrangementResponse(a))
}

func (c *EscrowController) removeArrangement(ctx *gin.Context, scope entities.EscrowKeyScope, tld string) {
	if !requireKeyManagement(ctx, scope) {
		return
	}
	if err := c.deps.KeyRegistry.RemoveArrangement(ctx.Request.Context(), scope, tld); err != nil {
		escrowKeyError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// GetDefaultArrangement returns the scope's default arrangement: the operator
// default with X-Tenant-ID, the platform default without.
// @Summary Get the default escrow arrangement
// @Tags Escrow keys
// @Produce json
// @Success 200 {object} EscrowArrangementResponse
// @Router /escrow/arrangements/default [get]
func (c *EscrowController) GetDefaultArrangement(ctx *gin.Context) {
	if scope, ok := escrowKeyScope(ctx); ok {
		c.getArrangement(ctx, scope, "")
	}
}

// SetDefaultArrangement writes the next revision of the scope's default.
// @Summary Set the default escrow arrangement
// @Tags Escrow keys
// @Accept json
// @Produce json
// @Param body body setArrangementRequest true "Parties"
// @Success 200 {object} EscrowArrangementResponse
// @Router /escrow/arrangements/default [put]
func (c *EscrowController) SetDefaultArrangement(ctx *gin.Context) {
	if scope, ok := escrowKeyScope(ctx); ok {
		c.setArrangement(ctx, scope, "")
	}
}

// RemoveDefaultArrangement removes the scope's default.
// @Summary Remove the default escrow arrangement
// @Tags Escrow keys
// @Success 204
// @Router /escrow/arrangements/default [delete]
func (c *EscrowController) RemoveDefaultArrangement(ctx *gin.Context) {
	if scope, ok := escrowKeyScope(ctx); ok {
		c.removeArrangement(ctx, scope, "")
	}
}

func operatorKeyScope(ctx *gin.Context) (entities.EscrowKeyScope, bool) {
	op, err := OperatorScopeFromRequest(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return entities.EscrowKeyScope{}, false
	}
	return entities.OperatorEscrowKeyScope(op), true
}

// ListTLDArrangements lists the operator's TLD overrides.
// @Summary List TLD escrow arrangement overrides
// @Tags Escrow keys
// @Produce json
// @Param X-Tenant-ID header string true "Operator scope"
// @Success 200 {object} map[string]interface{}
// @Router /escrow/arrangements/tlds [get]
func (c *EscrowController) ListTLDArrangements(ctx *gin.Context) {
	scope, ok := operatorKeyScope(ctx)
	if !ok {
		return
	}
	rows, err := c.deps.KeyRegistry.ListTLDOverrides(ctx.Request.Context(), scope.Operator())
	if err != nil {
		escrowKeyError(ctx, err)
		return
	}
	items := make([]EscrowArrangementResponse, len(rows))
	for i, a := range rows {
		items[i] = toArrangementResponse(a)
	}
	ctx.JSON(http.StatusOK, gin.H{"items": items, "count": len(items)})
}

// GetTLDArrangement returns a TLD override.
// @Summary Get a TLD escrow arrangement override
// @Tags Escrow keys
// @Produce json
// @Param X-Tenant-ID header string true "Operator scope"
// @Param tld path string true "TLD"
// @Success 200 {object} EscrowArrangementResponse
// @Router /escrow/arrangements/tlds/{tld} [get]
func (c *EscrowController) GetTLDArrangement(ctx *gin.Context) {
	scope, ok := operatorKeyScope(ctx)
	if !ok {
		return
	}
	if tld, ok := c.arrangementTLD(ctx); ok {
		c.getArrangement(ctx, scope, tld)
	}
}

// SetTLDArrangement writes the next revision of a TLD override.
// @Summary Set a TLD escrow arrangement override
// @Tags Escrow keys
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "Operator scope"
// @Param tld path string true "TLD"
// @Param body body setArrangementRequest true "Parties"
// @Success 200 {object} EscrowArrangementResponse
// @Router /escrow/arrangements/tlds/{tld} [put]
func (c *EscrowController) SetTLDArrangement(ctx *gin.Context) {
	scope, ok := operatorKeyScope(ctx)
	if !ok {
		return
	}
	if tld, ok := c.arrangementTLD(ctx); ok {
		c.setArrangement(ctx, scope, tld)
	}
}

// RemoveTLDArrangement removes a TLD override so the TLD inherits again.
// @Summary Remove a TLD escrow arrangement override
// @Tags Escrow keys
// @Param X-Tenant-ID header string true "Operator scope"
// @Param tld path string true "TLD"
// @Success 204
// @Router /escrow/arrangements/tlds/{tld} [delete]
func (c *EscrowController) RemoveTLDArrangement(ctx *gin.Context) {
	scope, ok := operatorKeyScope(ctx)
	if !ok {
		return
	}
	if tld, ok := c.arrangementTLD(ctx); ok {
		c.removeArrangement(ctx, scope, tld)
	}
}

// GetEffectiveArrangement resolves who deposits for a TLD and who receives,
// with the level each side is inherited from.
// @Summary Get a TLD's effective escrow arrangement
// @Tags Escrow keys
// @Produce json
// @Param X-Tenant-ID header string true "Operator scope"
// @Param tld path string true "TLD"
// @Success 200 {object} entities.EffectiveEscrowArrangement
// @Router /escrow/arrangements/tlds/{tld}/effective [get]
func (c *EscrowController) GetEffectiveArrangement(ctx *gin.Context) {
	scope, ok := operatorKeyScope(ctx)
	if !ok {
		return
	}
	tld, ok := c.arrangementTLD(ctx)
	if !ok {
		return
	}
	eff, err := c.deps.KeyRegistry.EffectiveArrangement(ctx.Request.Context(), scope.Operator(), tld)
	if err != nil {
		escrowKeyError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, eff)
}
