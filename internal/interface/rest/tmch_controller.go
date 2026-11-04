package rest

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// TMCHController reports on the platform's TMCH data (ADR 0024 §4, roadmap
// 9a) and runs a refresh on request. Reading is open to any authenticated
// caller; refreshing is a platform action under registry:policy:admin.
type TMCHController struct {
	svc interfaces.TMCHService
}

// NewTMCHController registers the TMCH routes.
func NewTMCHController(e *gin.Engine, svc interfaces.TMCHService, handler gin.HandlerFunc) *TMCHController {
	ctrl := &TMCHController{svc: svc}
	permitted := func(ctx *gin.Context) {
		if requireAuthScope(ctx, ScopeRegistryPolicyAdmin) {
			ctx.Next()
		}
	}
	g := e.Group("/tmch", handler)
	{
		g.GET("", ctrl.Status)
		g.POST("/refresh", permitted, ctrl.Refresh)
	}
	return ctrl
}

// tmchStatusResponse is the status with whether a feed is configured.
type tmchStatusResponse struct {
	Configured bool `json:"configured"`
	entities.TMCHStatus
}

// Status godoc
// @Summary The freshness of the TMCH data sets
// @Description When the Clearinghouse generated each stored set (the CA bundle, the SMD revocation list, the DNL list), when this registry fetched it, and how many entries it holds. A set never fetched is null.
// @Tags TMCH
// @Produce json
// @Success 200 {object} tmchStatusResponse
// @Router /tmch [get]
func (ctrl *TMCHController) Status(ctx *gin.Context) {
	status, err := ctrl.svc.Status(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, tmchStatusResponse{Configured: ctrl.svc.Configured(), TMCHStatus: *status})
}

// Refresh godoc
// @Summary Refresh the TMCH data sets now
// @Description Fetches the three sets from the Clearinghouse and stores each newer generation, as the refresh-tmch schedule does. Needs the registry:policy:admin permission. 409 when the installation has no TMCH feed configured; 502 when a set could not be fetched or stored (the result says which).
// @Tags TMCH
// @Produce json
// @Success 200 {object} queries.TMCHRefreshResult
// @Failure 409 {object} map[string]string
// @Failure 502 {object} map[string]interface{}
// @Router /tmch/refresh [post]
func (ctrl *TMCHController) Refresh(ctx *gin.Context) {
	result, err := ctrl.svc.Refresh(ctx.Request.Context())
	switch {
	case errors.Is(err, entities.ErrTMCHNotConfigured):
		ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case err != nil:
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "result": result})
	default:
		ctx.JSON(http.StatusOK, result)
	}
}
