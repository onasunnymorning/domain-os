package rest

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// LifecyclePoliciesController manages the lifecycle policies (ADR 0023):
// the named timing a TLD phase follows. Changing one needs the
// registry:policy:admin permission: a policy is platform-wide, so tenant
// membership cannot scope a change to it. Reading the catalogue needs only
// a valid token: whoever edits a phase picks from it (the phase drawer and
// the create wizard), and naming a policy on a phase is a phase edit
// (PhaseController.SetLifecyclePolicy).
type LifecyclePoliciesController struct {
	svc interfaces.LifecyclePolicyService
}

// NewLifecyclePoliciesController registers the routes under
// /lifecycle-policies.
func NewLifecyclePoliciesController(e *gin.Engine, svc interfaces.LifecyclePolicyService, handler gin.HandlerFunc) *LifecyclePoliciesController {
	ctrl := &LifecyclePoliciesController{svc: svc}
	permitted := func(ctx *gin.Context) {
		if requireAuthScope(ctx, ScopeRegistryPolicyAdmin) {
			ctx.Next()
		}
	}
	p := e.Group("/lifecycle-policies", handler)
	{
		p.GET("", ctrl.ListPolicies)
		p.GET(":name", ctrl.GetPolicy)
		p.PUT(":name", permitted, ctrl.SavePolicy)
		p.DELETE(":name", permitted, ctrl.DeletePolicy)
	}
	return ctrl
}

// ListPolicies godoc
// @Summary List lifecycle policies
// @Description Every lifecycle policy, gtld-standard included, each with the phases that name it (usedBy) and its settings resolved. Any authenticated caller may read the catalogue; changing it needs registry:policy:admin.
// @Tags Lifecycle policies
// @Produce json
// @Success 200 {array} entities.LifecyclePolicy
// @Router /lifecycle-policies [get]
func (ctrl *LifecyclePoliciesController) ListPolicies(ctx *gin.Context) {
	ps, err := ctrl.svc.ListPolicies(ctx.Request.Context())
	if err != nil {
		lifecyclePolicyError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, ps)
}

// GetPolicy godoc
// @Summary Get a lifecycle policy
// @Description The policy as stored (a null setting is the default), resolved, and the phases that name it.
// @Tags Lifecycle policies
// @Produce json
// @Param name path string true "Policy name"
// @Success 200 {object} entities.LifecyclePolicy
// @Failure 404
// @Router /lifecycle-policies/{name} [get]
func (ctrl *LifecyclePoliciesController) GetPolicy(ctx *gin.Context) {
	p, err := ctrl.svc.GetPolicy(ctx.Request.Context(), ctx.Param("name"))
	if err != nil {
		lifecyclePolicyError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, p)
}

// SavePolicy godoc
// @Summary Create or replace a lifecycle policy
// @Description Durations are whole days, each 0 to its bound (grace periods and the transfer lock 365, pending transfer and pending restore 30, redemption 365, pending delete 90); maxHorizon is 1 to 10 years. A setting left null is the policy's default. The change reaches every phase that names the policy at once. With If-None-Match: * the save is create-only (412 if the name is taken); with If-Match: "<updatedAt>" it is refused (412) if the policy changed since it was read.
// @Tags Lifecycle policies
// @Accept json
// @Produce json
// @Param name path string true "Policy name: 1 to 32 lower-case letters, digits and hyphens, starting with a letter"
// @Param policy body commands.SaveLifecyclePolicyCommand true "The policy"
// @Success 200 {object} entities.LifecyclePolicy
// @Failure 400
// @Failure 412
// @Router /lifecycle-policies/{name} [put]
func (ctrl *LifecyclePoliciesController) SavePolicy(ctx *gin.Context) {
	var cmd commands.SaveLifecyclePolicyCommand
	var err error
	if err = ctx.ShouldBindJSON(&cmd); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cmd.CreateOnly = createOnly(ctx)
	if cmd.ExpectedUpdatedAt, err = expectedVersion(ctx); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p, err := ctrl.svc.SavePolicy(ctx.Request.Context(), ctx.Param("name"), cmd)
	if err != nil {
		lifecyclePolicyError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, p)
}

// DeletePolicy godoc
// @Summary Delete a lifecycle policy
// @Description A policy no phase names. gtld-standard cannot be deleted.
// @Tags Lifecycle policies
// @Param name path string true "Policy name"
// @Success 204
// @Failure 404
// @Failure 409
// @Router /lifecycle-policies/{name} [delete]
func (ctrl *LifecyclePoliciesController) DeletePolicy(ctx *gin.Context) {
	if err := ctrl.svc.DeletePolicy(ctx.Request.Context(), ctx.Param("name")); err != nil {
		lifecyclePolicyError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func lifecyclePolicyError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, entities.ErrLifecyclePolicyNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, entities.ErrLifecyclePolicyBuiltIn), errors.Is(err, entities.ErrLifecyclePolicyInUse):
		ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, entities.ErrLifecyclePolicyExists), errors.Is(err, entities.ErrLifecyclePolicyChanged):
		ctx.JSON(http.StatusPreconditionFailed, gin.H{"error": err.Error()})
	case errors.Is(err, entities.ErrInvalidLifecyclePolicy):
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "the lifecycle policy request failed"})
	}
}
