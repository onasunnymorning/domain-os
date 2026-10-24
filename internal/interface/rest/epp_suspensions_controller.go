package rest

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// EPPSuspensionsController lists and lifts EPP logins suspended for
// creating registered names past the failed-create budget
// (docs/epp/POLICY_REFERENCE.md § Failed creates). Both endpoints need the
// epp:policy:admin permission.
type EPPSuspensionsController struct {
	svc interfaces.EPPSuspensionService
}

// NewEPPSuspensionsController registers GET /epp/suspensions and DELETE
// /registrars/:clid/epp/logins/:login/suspension.
func NewEPPSuspensionsController(e *gin.Engine, svc interfaces.EPPSuspensionService, handler gin.HandlerFunc) *EPPSuspensionsController {
	ctrl := &EPPSuspensionsController{svc: svc}
	permitted := func(ctx *gin.Context) {
		if requireAuthScope(ctx, ScopeEPPPolicyAdmin) {
			ctx.Next()
		}
	}
	e.GET("/epp/suspensions", handler, permitted, ctrl.List)
	e.DELETE("/registrars/:clid/epp/logins/:login/suspension", handler, permitted, ctrl.Lift)
	return ctrl
}

// List godoc
// @Summary List EPP logins suspended for failed creates
// @Description Each suspended login, its registrar, whether its registrar's or its family's strikes suspended it, and when the suspension ends.
// @Tags EPP
// @Produce json
// @Param clid query string false "Only this registrar's logins"
// @Success 200 {array} entities.EPPLoginSuspension
// @Router /epp/suspensions [get]
func (ctrl *EPPSuspensionsController) List(ctx *gin.Context) {
	out, err := ctrl.svc.List(ctx.Request.Context(), ctx.Query("clid"))
	if err != nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "the suspensions cannot be read"})
		return
	}
	ctx.JSON(http.StatusOK, out)
}

// Lift godoc
// @Summary Lift an EPP login's suspension for failed creates
// @Description Ends the suspension at once and clears the strikes that caused it, so the login's next create over budget starts escalation afresh. Recorded as an epp.login.suspension_lifted event with the operator as actor.
// @Tags EPP
// @Produce json
// @Param clid path string true "Registrar ClID"
// @Param login path string true "Login ID"
// @Success 200 {object} entities.EPPLoginSuspension "The suspension that was lifted"
// @Failure 404
// @Router /registrars/{clid}/epp/logins/{login}/suspension [delete]
func (ctrl *EPPSuspensionsController) Lift(ctx *gin.Context) {
	lifted, err := ctrl.svc.Lift(ctx.Request.Context(), ctx.Param("clid"), ctx.Param("login"))
	switch {
	case err == nil:
		ctx.JSON(http.StatusOK, lifted)
	case errors.Is(err, entities.ErrEPPSuspensionNotFound), errors.Is(err, entities.ErrEPPLoginNotFound),
		errors.Is(err, entities.ErrRegistrarNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "the suspension was not lifted"})
	}
}
