package rest

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
)

// FXController is the controller for FX
type FXController struct {
	fxService interfaces.FXService
}

// NewFXController returns a new FXController
func NewFXController(e *gin.Engine, fxService interfaces.FXService) *FXController {
	ctrl := &FXController{
		fxService: fxService,
	}

	e.GET("/fx/:baseCurrency", ctrl.ListByBaseCurrency)
	e.GET("/fx/:baseCurrency/:targetCurrency", ctrl.GetByBaseAndTargetCurrency)

	return ctrl
}

// ListByBaseCurrency godoc
// @Summary List all exchange rates by base currency
// @Description List all exchange rates by base currency
// @Tags fx
// @Accept json
// @Produce json
// @Param baseCurrency path string true "Base currency"
// @Success 200 {array} entities.FX
// @Router /fx/{baseCurrency} [get]
func (ctrl *FXController) ListByBaseCurrency(ctx *gin.Context) {
	baseCurrency := strings.ToUpper(ctx.Param("baseCurrency"))

	fxs, err := ctrl.fxService.ListByBaseCurrency(ctx, baseCurrency)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, fxs)
}

// GetByBaseAndTargetCurrency godoc
// @Summary Get the exchange rate for a base and target currency
// @Description Get the exchange rate for a base and target currency
// @Tags fx
// @Accept json
// @Produce json
// @Param baseCurrency path string true "Base currency"
// @Param targetCurrency path string true "Target currency"
// @Success 200 {object} entities.FX
// @Router /fx/{baseCurrency}/{targetCurrency} [get]
func (ctrl *FXController) GetByBaseAndTargetCurrency(ctx *gin.Context) {
	baseCurrency := strings.ToUpper(ctx.Param("baseCurrency"))
	targetCurrency := strings.ToUpper(ctx.Param("targetCurrency"))

	fx, err := ctrl.fxService.GetByBaseAndTargetCurrency(ctx, baseCurrency, targetCurrency)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, fx)
}
