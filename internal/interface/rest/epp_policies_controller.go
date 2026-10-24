package rest

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// EPPPoliciesController manages the EPP policies, their assignment to
// registrars, and registrar families (ADR 0019,
// docs/epp/POLICY_REFERENCE.md § Policies). Every endpoint needs the
// epp:policy:admin permission. Changes reach running EPP servers within
// their credential refresh interval.
type EPPPoliciesController struct {
	svc interfaces.EPPPolicyService
}

// NewEPPPoliciesController registers the routes under /epp/policies,
// /epp/families and /registrars/:clid/epp/policy.
func NewEPPPoliciesController(e *gin.Engine, svc interfaces.EPPPolicyService, handler gin.HandlerFunc) *EPPPoliciesController {
	ctrl := &EPPPoliciesController{svc: svc}
	permitted := func(ctx *gin.Context) {
		if requireAuthScope(ctx, ScopeEPPPolicyAdmin) {
			ctx.Next()
		}
	}
	p := e.Group("/epp/policies", handler, permitted)
	{
		p.GET("", ctrl.ListPolicies)
		p.GET(":name", ctrl.GetPolicy)
		p.PUT(":name", ctrl.SavePolicy)
		p.DELETE(":name", ctrl.DeletePolicy)
	}
	f := e.Group("/epp/families", handler, permitted)
	{
		f.GET("", ctrl.ListFamilies)
		f.GET(":name", ctrl.GetFamily)
		f.PUT(":name", ctrl.SaveFamily)
		f.DELETE(":name", ctrl.DeleteFamily)
	}
	r := e.Group("/registrars/:clid/epp/policy", handler, permitted)
	{
		r.GET("", ctrl.GetRegistrarPolicy)
		r.PUT("", ctrl.AssignPolicy)
	}
	e.GET("/registrars/:clid/epp/create-outcomes", handler, permitted, ctrl.CreateOutcomes)
	return ctrl
}

// ListPolicies godoc
// @Summary List EPP policies
// @Description Every EPP policy, the built-in "default" and "probation" included.
// @Tags EPP
// @Produce json
// @Success 200 {array} entities.EPPPolicy
// @Failure 403
// @Router /epp/policies [get]
func (ctrl *EPPPoliciesController) ListPolicies(ctx *gin.Context) {
	ps, err := ctrl.svc.ListPolicies(ctx.Request.Context())
	if err != nil {
		eppPolicyError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, ps)
}

// GetPolicy godoc
// @Summary Get an EPP policy
// @Tags EPP
// @Produce json
// @Param name path string true "Policy name"
// @Success 200 {object} entities.EPPPolicy
// @Failure 404
// @Router /epp/policies/{name} [get]
func (ctrl *EPPPoliciesController) GetPolicy(ctx *gin.Context) {
	p, err := ctrl.svc.GetPolicy(ctx.Request.Context(), ctx.Param("name"))
	if err != nil {
		eppPolicyError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, p)
}

// SavePolicy godoc
// @Summary Create or replace an EPP policy
// @Description Every limit must be given: at least 1, except creates_per_day (0 is no cap), and each check burst at least check_names. The built-in policies can be changed this way too.
// @Tags EPP
// @Accept json
// @Produce json
// @Param name path string true "Policy name: lower-case letters, digits and hyphens"
// @Param policy body commands.SaveEPPPolicyCommand true "The policy"
// @Success 200 {object} entities.EPPPolicy
// @Failure 400
// @Router /epp/policies/{name} [put]
func (ctrl *EPPPoliciesController) SavePolicy(ctx *gin.Context) {
	var cmd commands.SaveEPPPolicyCommand
	if err := ctx.ShouldBindJSON(&cmd); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p, err := ctrl.svc.SavePolicy(ctx.Request.Context(), ctx.Param("name"), cmd)
	if err != nil {
		eppPolicyError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, p)
}

// DeletePolicy godoc
// @Summary Delete an EPP policy
// @Description A custom policy no registrar or family is assigned. The built-in policies cannot be deleted.
// @Tags EPP
// @Param name path string true "Policy name"
// @Success 204
// @Failure 404
// @Failure 409
// @Router /epp/policies/{name} [delete]
func (ctrl *EPPPoliciesController) DeletePolicy(ctx *gin.Context) {
	if err := ctrl.svc.DeletePolicy(ctx.Request.Context(), ctx.Param("name")); err != nil {
		eppPolicyError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// GetRegistrarPolicy godoc
// @Summary What EPP policy applies to a registrar
// @Description The registrar's policy, and its family with the family's policy, if it is in one.
// @Tags EPP
// @Produce json
// @Param clid path string true "Registrar ClID"
// @Success 200 {object} entities.EPPRegistrarPolicy
// @Failure 404
// @Router /registrars/{clid}/epp/policy [get]
func (ctrl *EPPPoliciesController) GetRegistrarPolicy(ctx *gin.Context) {
	rp, err := ctrl.svc.RegistrarPolicy(ctx.Request.Context(), ctx.Param("clid"))
	if err != nil {
		eppPolicyError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, rp)
}

// AssignPolicy godoc
// @Summary Assign a registrar its EPP policy
// @Description "default", or an empty policy, removes the assignment.
// @Tags EPP
// @Accept json
// @Produce json
// @Param clid path string true "Registrar ClID"
// @Param assignment body commands.AssignEPPPolicyCommand true "The policy"
// @Success 200 {object} entities.EPPRegistrarPolicy
// @Failure 404
// @Router /registrars/{clid}/epp/policy [put]
func (ctrl *EPPPoliciesController) AssignPolicy(ctx *gin.Context) {
	var cmd commands.AssignEPPPolicyCommand
	if err := ctx.ShouldBindJSON(&cmd); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rp, err := ctrl.svc.AssignPolicy(ctx.Request.Context(), ctx.Param("clid"), cmd)
	if err != nil {
		eppPolicyError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, rp)
}

// CreateOutcomes godoc
// @Summary A registrar's daily domain create outcomes
// @Description For each UTC day: domain creates that created a domain, and those refused because the name is registered (2302), the basis for a fee on failed creates. Days with neither are left out. from and to default to the last 30 days, and may be at most 366 days apart.
// @Tags EPP
// @Produce json
// @Param clid path string true "Registrar ClID"
// @Param from query string false "First UTC day, YYYY-MM-DD"
// @Param to query string false "Last UTC day, YYYY-MM-DD"
// @Success 200 {array} entities.EPPCreateOutcomes
// @Failure 400
// @Failure 404
// @Router /registrars/{clid}/epp/create-outcomes [get]
func (ctrl *EPPPoliciesController) CreateOutcomes(ctx *gin.Context) {
	to := time.Now().UTC()
	from := to.AddDate(0, 0, -29)
	for _, q := range []struct {
		name string
		into *time.Time
	}{{"from", &from}, {"to", &to}} {
		if v := ctx.Query(q.name); v != "" {
			t, err := time.Parse(time.DateOnly, v)
			if err != nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": q.name + " must be a date, YYYY-MM-DD"})
				return
			}
			*q.into = t
		}
	}
	out, err := ctrl.svc.CreateOutcomes(ctx.Request.Context(), ctx.Param("clid"), from, to)
	if err != nil {
		eppPolicyError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, out)
}

// ListFamilies godoc
// @Summary List registrar families
// @Tags EPP
// @Produce json
// @Success 200 {array} entities.RegistrarFamily
// @Router /epp/families [get]
func (ctrl *EPPPoliciesController) ListFamilies(ctx *gin.Context) {
	fs, err := ctrl.svc.ListFamilies(ctx.Request.Context())
	if err != nil {
		eppPolicyError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, fs)
}

// GetFamily godoc
// @Summary Get a registrar family
// @Tags EPP
// @Produce json
// @Param name path string true "Family name"
// @Success 200 {object} entities.RegistrarFamily
// @Failure 404
// @Router /epp/families/{name} [get]
func (ctrl *EPPPoliciesController) GetFamily(ctx *gin.Context) {
	f, err := ctrl.svc.GetFamily(ctx.Request.Context(), ctx.Param("name"))
	if err != nil {
		eppPolicyError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, f)
}

// SaveFamily godoc
// @Summary Create or replace a registrar family
// @Description Its members are replaced. A registrar is in at most one family.
// @Tags EPP
// @Accept json
// @Produce json
// @Param name path string true "Family name: lower-case letters, digits and hyphens"
// @Param family body commands.SaveRegistrarFamilyCommand true "The family"
// @Success 200 {object} entities.RegistrarFamily
// @Failure 400
// @Failure 404
// @Failure 409
// @Router /epp/families/{name} [put]
func (ctrl *EPPPoliciesController) SaveFamily(ctx *gin.Context) {
	var cmd commands.SaveRegistrarFamilyCommand
	if err := ctx.ShouldBindJSON(&cmd); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	f, err := ctrl.svc.SaveFamily(ctx.Request.Context(), ctx.Param("name"), cmd)
	if err != nil {
		eppPolicyError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, f)
}

// DeleteFamily godoc
// @Summary Delete a registrar family
// @Tags EPP
// @Param name path string true "Family name"
// @Success 204
// @Failure 404
// @Router /epp/families/{name} [delete]
func (ctrl *EPPPoliciesController) DeleteFamily(ctx *gin.Context) {
	if err := ctrl.svc.DeleteFamily(ctx.Request.Context(), ctx.Param("name")); err != nil {
		eppPolicyError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func eppPolicyError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, entities.ErrEPPPolicyNotFound), errors.Is(err, entities.ErrRegistrarFamilyNotFound),
		errors.Is(err, entities.ErrRegistrarNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, entities.ErrEPPPolicyBuiltIn), errors.Is(err, entities.ErrEPPPolicyInUse),
		errors.Is(err, entities.ErrRegistrarInFamily):
		ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, entities.ErrInvalidEPPPolicy), errors.Is(err, entities.ErrInvalidRegistrarFamily):
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "the EPP policy request failed"})
	}
}
