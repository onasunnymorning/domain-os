package rest

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// PriceController is the controller for the Price entity
type PriceController struct {
	priceService interfaces.PriceService
}

// NewPriceController returns a new instance of PriceController
func NewPriceController(e *gin.Engine, priceService interfaces.PriceService) *PriceController {
	controller := &PriceController{
		priceService: priceService,
	}
	// Add the routes
	e.POST("/tlds/:tldName/phases/:phaseName/prices", controller.CreatePrice)
	e.GET("/tlds/:tldName/phases/:phaseName/prices", controller.ListPrices)
	e.DELETE("/tlds/:tldName/phases/:phaseName/prices/:currency", controller.DeletePrice)

	return controller
}

// CreatePrice godoc
// @Summary Create a new Price
// @Description Create a new Price. TLD Name and Phase Name are case sensitive. Currency Code will be converted to uppercase before storing. If the price defined its Currency already exists in the phase a 400 will be returned. If the TLD or Phase do not exist a 404 will be returned. Amounts should be in the smallest unit of the currency (e.g. cents for USD).
// @Tags Prices
// @Accept json
// @Produce json
// @Param fee body commands.CreatePriceCommand true "Price to create"
// @Param tldName path string true "TLD name"
// @Param phaseName path string true "Phase name"
// @Success 201 {object} entities.Price
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /tlds/{tldName}/phases/{phaseName}/prices [post]
func (ctrl *PriceController) CreatePrice(ctx *gin.Context) {
	// Bind the request body to the command
	var cmd commands.CreatePriceCommand
	if err := ctx.ShouldBindJSON(&cmd); err != nil {
		if err.Error() == "EOF" {
			ctx.JSON(400, gin.H{"error": "missing request body"})
			return
		}
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// Set the TLD and phase in the command
	cmd.TLDName = ctx.Param("tldName")
	cmd.PhaseName = ctx.Param("phaseName")

	// Call the service to create the fee
	price, err := ctrl.priceService.CreatePrice(ctx, &cmd)
	if err != nil {
		if errors.Is(err, entities.ErrInvalidPrice) {
			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Return the response
	ctx.JSON(201, price)
}

// ListPrices godoc
// @Summary List all Prices for a given phase
// @Description List all Prices for a given phase. There is no pagination on this endpoint. TLD Name and Phase Name are case sensitive.
// @Tags Prices
// @Produce json
// @Param tldName path string true "TLD name"
// @Param phaseName path string true "Phase name"
// @Success 200 {array} entities.Price
// @Failure 404
// @Failure 500
// @Router /tlds/{tldName}/phases/{phaseName}/prices [get]
func (ctrl *PriceController) ListPrices(ctx *gin.Context) {
	// Call the service to list the Prices
	prices, err := ctrl.priceService.ListPrices(ctx, ctx.Param("phaseName"), ctx.Param("tldName"))
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Return the response
	ctx.JSON(200, prices)
}

// DeletePrice godoc
// @Summary Delete a Price
// @Description Deletes a Price for a given phase. The fee is identified by its currency. TLD Name and Phase Name are case sensitive. Currency is not (we always store currency codes in uppercase and will convert the input given to uppercase ). If the Price does not exist a 204 will be returned. If either the TLD or Phase do not exist a 404 will be returned.
// @Tags Prices
// @Produce json
// @Param tldName path string true "TLD name"
// @Param phaseName path string true "Phase name"
// @Param currency path string true "Currency"
// @Success 204
// @Failure 404
// @Failure 500
// @Router /tlds/{tldName}/phases/{phaseName}/prices/{currency} [delete]
func (ctrl *PriceController) DeletePrice(ctx *gin.Context) {
	// Call the service to delete the fee
	err := ctrl.priceService.DeletePrice(ctx, ctx.Param("phaseName"), ctx.Param("tldName"), ctx.Param("currency"))
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Return the response
	ctx.JSON(204, nil)
}
