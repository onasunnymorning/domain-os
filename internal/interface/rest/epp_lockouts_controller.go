package rest

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// EPPLockoutsController lists and lifts EPP logins locked out after failed
// passwords (docs/epp/POLICY_REFERENCE.md § Authentication). Both endpoints
// need the epp:credentials:admin permission.
type EPPLockoutsController struct {
	svc interfaces.EPPLockoutService
}

// NewEPPLockoutsController registers GET /epp/lockouts and DELETE
// /registrars/:clid/epp/logins/:login/lockouts.
func NewEPPLockoutsController(e *gin.Engine, svc interfaces.EPPLockoutService, handler gin.HandlerFunc) *EPPLockoutsController {
	ctrl := &EPPLockoutsController{svc: svc}
	permitted := func(ctx *gin.Context) {
		if requireAuthScope(ctx, ScopeEPPCredentialsAdmin) {
			ctx.Next()
		}
	}
	e.GET("/epp/lockouts", handler, permitted, ctrl.List)
	e.DELETE("/registrars/:clid/epp/logins/:login/lockouts", handler, permitted, ctrl.Lift)
	return ctrl
}

// List godoc
// @Summary List EPP logins locked out after failed passwords
// @Description Each locked (login, source address) pair, its registrar, and when the lockout ends.
// @Tags EPP
// @Produce json
// @Param clid query string false "Only this registrar's logins"
// @Success 200 {array} entities.EPPLoginLockout
// @Router /epp/lockouts [get]
func (ctrl *EPPLockoutsController) List(ctx *gin.Context) {
	out, err := ctrl.svc.List(ctx.Request.Context(), ctx.Query("clid"))
	if err != nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "the lockouts cannot be read"})
		return
	}
	ctx.JSON(http.StatusOK, out)
}

// Lift godoc
// @Summary Lift an EPP login's lockouts
// @Description Clears the login's lockouts, failure counts and strikes from every source address at once. Recorded as an epp.login.lockout_lifted event with the operator as actor.
// @Tags EPP
// @Produce json
// @Param clid path string true "Registrar ClID"
// @Param login path string true "Login ID"
// @Success 200 {array} entities.EPPLoginLockout "The lockouts lifted"
// @Failure 404
// @Router /registrars/{clid}/epp/logins/{login}/lockouts [delete]
func (ctrl *EPPLockoutsController) Lift(ctx *gin.Context) {
	lifted, err := ctrl.svc.Lift(ctx.Request.Context(), ctx.Param("clid"), ctx.Param("login"))
	switch {
	case err == nil:
		ctx.JSON(http.StatusOK, lifted)
	case errors.Is(err, entities.ErrEPPLockoutNotFound), errors.Is(err, entities.ErrEPPLoginNotFound),
		errors.Is(err, entities.ErrRegistrarNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "the lockouts were not lifted"})
	}
}
