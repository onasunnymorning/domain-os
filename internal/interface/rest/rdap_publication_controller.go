package rest

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// RDAPPublicationController manages the RDAP publication policies and their
// assignment to TLDs (ADR 0021, docs/rdap/PUBLICATION_REFERENCE.md). Every
// endpoint needs the rdap:policy:admin permission. Changes reach running
// RDAP servers within RDAP_POLICY_REFRESH_INTERVAL.
type RDAPPublicationController struct {
	svc interfaces.RDAPPublicationPolicyService
}

// NewRDAPPublicationController registers the routes under
// /rdap/publication-policies and /tlds/:tldName/rdap/publication-policy.
func NewRDAPPublicationController(e *gin.Engine, svc interfaces.RDAPPublicationPolicyService, handler gin.HandlerFunc) *RDAPPublicationController {
	ctrl := &RDAPPublicationController{svc: svc}
	permitted := func(ctx *gin.Context) {
		if requireAuthScope(ctx, ScopeRDAPPolicyAdmin) {
			ctx.Next()
		}
	}
	p := e.Group("/rdap/publication-policies", handler, permitted)
	{
		p.GET("", ctrl.ListPolicies)
		p.GET(":name", ctrl.GetPolicy)
		p.PUT(":name", ctrl.SavePolicy)
		p.DELETE(":name", ctrl.DeletePolicy)
	}
	t := e.Group("/tlds/:tldName/rdap/publication-policy", handler, permitted)
	{
		t.GET("", ctrl.GetTLDPolicy)
		t.PUT("", ctrl.AssignPolicy)
	}
	return ctrl
}

// ListPolicies godoc
// @Summary List RDAP publication policies
// @Description Every publication policy, the built-in "gtld-rdp-2024" and "cctld-minimal" included.
// @Tags RDAP
// @Produce json
// @Success 200 {array} entities.RDAPPublicationPolicy
// @Failure 403
// @Router /rdap/publication-policies [get]
func (ctrl *RDAPPublicationController) ListPolicies(ctx *gin.Context) {
	ps, err := ctrl.svc.ListPolicies(ctx.Request.Context())
	if err != nil {
		rdapPublicationError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, ps)
}

// GetPolicy godoc
// @Summary Get an RDAP publication policy
// @Tags RDAP
// @Produce json
// @Param name path string true "Policy name"
// @Success 200 {object} entities.RDAPPublicationPolicy
// @Failure 404
// @Router /rdap/publication-policies/{name} [get]
func (ctrl *RDAPPublicationController) GetPolicy(ctx *gin.Context) {
	p, err := ctrl.svc.GetPolicy(ctx.Request.Context(), ctx.Param("name"))
	if err != nil {
		rdapPublicationError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, p)
}

// SavePolicy godoc
// @Summary Create or replace an RDAP publication policy
// @Description Each role listed must give every field a rule (publish, publish-if-disclosed or withhold, with a method for withhold); a role left out is not published. The built-in policies can be changed this way too.
// @Tags RDAP
// @Accept json
// @Produce json
// @Param name path string true "Policy name: lower-case letters, digits and hyphens"
// @Param policy body commands.SaveRDAPPublicationPolicyCommand true "The policy"
// @Success 200 {object} entities.RDAPPublicationPolicy
// @Failure 400
// @Router /rdap/publication-policies/{name} [put]
func (ctrl *RDAPPublicationController) SavePolicy(ctx *gin.Context) {
	var cmd commands.SaveRDAPPublicationPolicyCommand
	if err := ctx.ShouldBindJSON(&cmd); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p, err := ctrl.svc.SavePolicy(ctx.Request.Context(), ctx.Param("name"), cmd)
	if err != nil {
		rdapPublicationError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, p)
}

// DeletePolicy godoc
// @Summary Delete an RDAP publication policy
// @Description A custom policy no TLD is assigned. The built-in policies cannot be deleted.
// @Tags RDAP
// @Param name path string true "Policy name"
// @Success 204
// @Failure 404
// @Failure 409
// @Router /rdap/publication-policies/{name} [delete]
func (ctrl *RDAPPublicationController) DeletePolicy(ctx *gin.Context) {
	if err := ctrl.svc.DeletePolicy(ctx.Request.Context(), ctx.Param("name")); err != nil {
		rdapPublicationError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// GetTLDPolicy godoc
// @Summary What RDAP publication policy applies to a TLD
// @Tags RDAP
// @Produce json
// @Param tldName path string true "TLD"
// @Success 200 {object} entities.RDAPTLDPublication
// @Failure 404
// @Router /tlds/{tldName}/rdap/publication-policy [get]
func (ctrl *RDAPPublicationController) GetTLDPolicy(ctx *gin.Context) {
	tp, err := ctrl.svc.TLDPolicy(ctx.Request.Context(), ctx.Param("tldName"))
	if err != nil {
		rdapPublicationError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, tp)
}

// AssignPolicy godoc
// @Summary Assign a TLD its RDAP publication policy
// @Description An empty policy removes the assignment, so the TLD has the built-in for its type.
// @Tags RDAP
// @Accept json
// @Produce json
// @Param tldName path string true "TLD"
// @Param assignment body commands.AssignRDAPPublicationPolicyCommand true "The policy"
// @Success 200 {object} entities.RDAPTLDPublication
// @Failure 404
// @Router /tlds/{tldName}/rdap/publication-policy [put]
func (ctrl *RDAPPublicationController) AssignPolicy(ctx *gin.Context) {
	var cmd commands.AssignRDAPPublicationPolicyCommand
	if err := ctx.ShouldBindJSON(&cmd); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tp, err := ctrl.svc.AssignPolicy(ctx.Request.Context(), ctx.Param("tldName"), cmd)
	if err != nil {
		rdapPublicationError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, tp)
}

func rdapPublicationError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, entities.ErrRDAPPublicationPolicyNotFound), errors.Is(err, entities.ErrTLDNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, entities.ErrRDAPPublicationPolicyBuiltIn), errors.Is(err, entities.ErrRDAPPublicationPolicyInUse):
		ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, entities.ErrInvalidRDAPPublicationPolicy):
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "the RDAP publication policy request failed"})
	}
}
