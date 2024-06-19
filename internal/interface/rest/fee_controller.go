package rest

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// FeeController is the controller for the Fee entity
type FeeController struct {
	feeService interfaces.FeeService
}

// NewFeeController returns a new instance of FeeController
func NewFeeController(e *gin.Engine, feeService interfaces.FeeService) *FeeController {
	controller := &FeeController{
		feeService: feeService,
	}
	// Add the routes
	e.POST("/tlds/:tldName/phases/:phaseName/fees", controller.CreateFee)
	e.GET("/tlds/:tldName/phases/:phaseName/fees", controller.ListFees)
	e.DELETE("/tlds/:tldName/phases/:phaseName/fees/:feeName/:currency", controller.DeleteFee)

	return controller
}

// CreateFee godoc
// @Summary Create a new fee
// @Description Create a new fee. TLD Name and Phase Name are case sensitive. Currency Code will be converted to uppercase before storing. If the fee (defined by the Name + Currency tuple) already exists in the phase a 400 will be returned. If the TLD or Phase do not exist a 404 will be returned. Amounts should be in the smallest unit of the currency (e.g. cents for USD). Refundable is a boolean that indicates if the fee is refundable. If omitted, it will default to false.
// @Tags Fees
// @Accept json
// @Produce json
// @Param fee body commands.CreateFeeCommand true "Fee to create"
// @Param tldName path string true "TLD name"
// @Param phaseName path string true "Phase name"
// @Success 201 {object} entities.Fee
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /tlds/{tldName}/phases/{phaseName}/fees [post]
func (ctrl *FeeController) CreateFee(ctx *gin.Context) {
	// Bind the request body to the command
	var cmd commands.CreateFeeCommand
	if err := ctx.ShouldBindJSON(&cmd); err != nil {
		if err.Error() == "EOF" {
			ctx.JSON(400, gin.H{"error": "missing request body"})
			return
		}
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	event := GetEventFromContext(ctx)
	event.Details.Command = cmd

	// Set the TLD and phase in the command
	cmd.TLDName = ctx.Param("tldName")
	cmd.PhaseName = ctx.Param("phaseName")

	// Call the service to create the fee
	fee, err := ctrl.feeService.CreateFee(ctx, &cmd)
	if err != nil {
		event.Details.Error = err
		if errors.Is(err, entities.ErrInvalidFee) {
			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	event.Details.After = fee

	// Return the response
	ctx.JSON(201, fee)
}

// ListFees godoc
// @Summary List all fees for a given phase
// @Description List all fees for a given phase. There is no pagination on this endpoint. TLD Name and Phase Name are case sensitive.
// @Tags Fees
// @Produce json
// @Param tldName path string true "TLD name"
// @Param phaseName path string true "Phase name"
// @Success 200 {array} entities.Fee
// @Failure 404
// @Failure 500
// @Router /tlds/{tldName}/phases/{phaseName}/fees [get]
func (ctrl *FeeController) ListFees(ctx *gin.Context) {
	// Call the service to list the fees
	fees, err := ctrl.feeService.ListFees(ctx, ctx.Param("phaseName"), ctx.Param("tldName"))
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Return the response
	ctx.JSON(200, fees)
}

// DeleteFee godoc
// @Summary Delete a fee
// @Description Deletes a fee for a given phase. The fee is identified by its name and currency. TLD Name, Fee Name and Phase Name are case sensitive. Currency is not (we always store currency codes in uppercase and will convert the input given to uppercase ). If the fee does not exist a 204 will be returned. If either the TLD or Phase do not exist a 404 will be returned.
// @Tags Fees
// @Produce json
// @Param tldName path string true "TLD name"
// @Param phaseName path string true "Phase name"
// @Param feeName path string true "Fee name"
// @Param currency path string true "Currency"
// @Success 204
// @Failure 404
// @Failure 500
// @Router /tlds/{tldName}/phases/{phaseName}/fees/{feeName}/{currency} [delete]
func (ctrl *FeeController) DeleteFee(ctx *gin.Context) {
	event := GetEventFromContext(ctx)
	event.Details.Command = ctx.Param("feeName") + ctx.Param("currency")
	// Call the service to delete the fee
	err := ctrl.feeService.DeleteFee(ctx, ctx.Param("phaseName"), ctx.Param("tldName"), ctx.Param("feeName"), ctx.Param("currency"))
	if err != nil {
		event.Details.Error = err
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Return the response
	ctx.JSON(204, nil)
}
