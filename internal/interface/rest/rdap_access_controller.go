package rest

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// RDAPAccessController manages the RDAP access policies and the networks
// they are assigned to (ADR 0022, docs/rdap/POLICY_REFERENCE.md § Access
// policies). Every endpoint needs the rdap:policy:admin permission. Changes
// reach running RDAP servers within RDAP_POLICY_REFRESH_INTERVAL.
type RDAPAccessController struct {
	svc interfaces.RDAPAccessPolicyService
}

// NewRDAPAccessController registers the routes under /rdap/access-policies
// and /rdap/access-networks.
func NewRDAPAccessController(e *gin.Engine, svc interfaces.RDAPAccessPolicyService, handler gin.HandlerFunc) *RDAPAccessController {
	ctrl := &RDAPAccessController{svc: svc}
	permitted := func(ctx *gin.Context) {
		if requireAuthScope(ctx, ScopeRDAPPolicyAdmin) {
			ctx.Next()
		}
	}
	p := e.Group("/rdap/access-policies", handler, permitted)
	{
		p.GET("", ctrl.ListPolicies)
		p.GET(":name", ctrl.GetPolicy)
		p.PUT(":name", ctrl.SavePolicy)
		p.DELETE(":name", ctrl.DeletePolicy)
	}
	n := e.Group("/rdap/access-networks", handler, permitted)
	{
		n.GET("", ctrl.ListNetworks)
		n.PUT("", ctrl.AssignNetwork)
	}
	return ctrl
}

// ListPolicies godoc
// @Summary List RDAP access policies
// @Description Every access policy, the built-in "anonymous" and "monitoring" included.
// @Tags RDAP
// @Produce json
// @Success 200 {array} entities.RDAPAccessPolicy
// @Failure 403
// @Router /rdap/access-policies [get]
func (ctrl *RDAPAccessController) ListPolicies(ctx *gin.Context) {
	ps, err := ctrl.svc.ListPolicies(ctx.Request.Context())
	if err != nil {
		rdapAccessError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, ps)
}

// GetPolicy godoc
// @Summary Get an RDAP access policy
// @Tags RDAP
// @Produce json
// @Param name path string true "Policy name"
// @Success 200 {object} entities.RDAPAccessPolicy
// @Failure 404
// @Router /rdap/access-policies/{name} [get]
func (ctrl *RDAPAccessController) GetPolicy(ctx *gin.Context) {
	p, err := ctrl.svc.GetPolicy(ctx.Request.Context(), ctx.Param("name"))
	if err != nil {
		rdapAccessError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, p)
}

// SavePolicy godoc
// @Summary Create or replace an RDAP access policy
// @Description perAddress needs a rate and burst of at least 1; perAggregate both or neither (no aggregate limit); notFoundCost at least 1 and within every burst. The built-in policies can be changed this way too.
// @Tags RDAP
// @Accept json
// @Produce json
// @Param name path string true "Policy name: lower-case letters, digits and hyphens"
// @Param policy body commands.SaveRDAPAccessPolicyCommand true "The policy"
// @Success 200 {object} entities.RDAPAccessPolicy
// @Failure 400
// @Router /rdap/access-policies/{name} [put]
func (ctrl *RDAPAccessController) SavePolicy(ctx *gin.Context) {
	var cmd commands.SaveRDAPAccessPolicyCommand
	if err := ctx.ShouldBindJSON(&cmd); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p, err := ctrl.svc.SavePolicy(ctx.Request.Context(), ctx.Param("name"), cmd)
	if err != nil {
		rdapAccessError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, p)
}

// DeletePolicy godoc
// @Summary Delete an RDAP access policy
// @Description A custom policy no network is assigned. The built-in policies cannot be deleted.
// @Tags RDAP
// @Param name path string true "Policy name"
// @Success 204
// @Failure 404
// @Failure 409
// @Router /rdap/access-policies/{name} [delete]
func (ctrl *RDAPAccessController) DeletePolicy(ctx *gin.Context) {
	if err := ctrl.svc.DeletePolicy(ctx.Request.Context(), ctx.Param("name")); err != nil {
		rdapAccessError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// ListNetworks godoc
// @Summary List the networks assigned RDAP access policies
// @Description Most specific first: that is the order a client's network is found in.
// @Tags RDAP
// @Produce json
// @Success 200 {array} entities.RDAPAccessNetwork
// @Router /rdap/access-networks [get]
func (ctrl *RDAPAccessController) ListNetworks(ctx *gin.Context) {
	ns, err := ctrl.svc.ListNetworks(ctx.Request.Context())
	if err != nil {
		rdapAccessError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, ns)
}

// AssignNetwork godoc
// @Summary Assign a network an RDAP access policy
// @Description network is a CIDR or a single address. policy is required: an empty one removes the assignment, so the network's clients are anonymous; a missing or null one is refused.
// @Tags RDAP
// @Accept json
// @Produce json
// @Param assignment body commands.AssignRDAPAccessNetworkCommand true "The network and policy"
// @Success 200 {object} entities.RDAPAccessNetwork
// @Failure 400
// @Failure 404
// @Router /rdap/access-networks [put]
func (ctrl *RDAPAccessController) AssignNetwork(ctx *gin.Context) {
	// policy must be present and a string: an empty one removes the
	// assignment, which would drop a monitoring network to anonymous limits,
	// so a body that merely misspells or omits it must not.
	var body struct {
		Network string  `json:"network" binding:"required"`
		Policy  *string `json:"policy"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Policy == nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": `the body needs a "policy" member: a policy name, or "" to remove the assignment`})
		return
	}
	n, err := ctrl.svc.AssignNetwork(ctx.Request.Context(), commands.AssignRDAPAccessNetworkCommand{Network: body.Network, Policy: *body.Policy})
	if err != nil {
		rdapAccessError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, n)
}

func rdapAccessError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, entities.ErrRDAPAccessPolicyNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, entities.ErrRDAPAccessPolicyBuiltIn), errors.Is(err, entities.ErrRDAPAccessPolicyInUse):
		ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, entities.ErrInvalidRDAPAccessPolicy), errors.Is(err, entities.ErrInvalidRDAPAccessNetwork):
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "the RDAP access policy request failed"})
	}
}
