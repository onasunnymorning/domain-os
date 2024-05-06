package rest

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// RegistryOperatorController is the controller for Registry Operator endpoints
type RegistryOperatorController struct {
	ryService interfaces.RegistryOperatorService
}

// NewRegistryOperatorController creates a new RegistryOperatorController
func NewRegistryOperatorController(e *gin.Engine, ryService interfaces.RegistryOperatorService) *RegistryOperatorController {
	ctrl := &RegistryOperatorController{
		ryService: ryService,
	}

	// Add routes
	e.POST("/registry-operators", ctrl.Create)
	e.GET("/registry-operators/:ryid", ctrl.GetByRyID)
	e.PUT("/registry-operators/:ryid", ctrl.Update)
	e.DELETE("/registry-operators/:ryid", ctrl.DeleteByRyID)

	return ctrl
}

// Create godoc
// @Summary Create a Registry Operator
// @Description Create a Registry Operator
// @Tags Registry Operators
// @Accept json
// @Produce json
// @Param body body commands.CreateRegistryOperatorCommand true "Registry Operator"
// @Success 201 {object} entities.RegistryOperator
// @Failure 400
// @Failure 500
// @Router /registry-operators [post]
func (ctrl *RegistryOperatorController) Create(ctx *gin.Context) {
	var cmd commands.CreateRegistryOperatorCommand
	if err := ctx.ShouldBindJSON(&cmd); err != nil {
		if err.Error() == "EOF" {
			ctx.JSON(400, gin.H{"error": "missing request body"})
			return
		}
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	ry, err := ctrl.ryService.Create(ctx, &cmd)
	if err != nil {
		if errors.Is(err, entities.ErrInvalidRegistryOperator) {
			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(201, ry)
}

// GetByRyID godoc
// @Summary Get a Registry Operator by RyID
// @Description Get a Registry Operator by RyID
// @Tags Registry Operators
// @Produce json
// @Param ryid path string true "Registry Operator RyID"
// @Success 200 {object} entities.RegistryOperator
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /registry-operators/{ryid} [get]
func (ctrl *RegistryOperatorController) GetByRyID(ctx *gin.Context) {
	ryid := ctx.Param("ryid")

	ry, err := ctrl.ryService.GetByRyID(ctx, ryid)
	if err != nil {
		if errors.Is(err, entities.ErrRegistryOperatorNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, ry)
}

// Update godoc
// @Summary Update a Registry Operator
// @Description Update a Registry Operator
// @Tags Registry Operators
// @Accept json
// @Produce json
// @Param ryid path string true "Registry Operator RyID"
// @Param body body entities.RegistryOperator true "Registry Operator"
// @Success 200 {object} entities.RegistryOperator
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /registry-operators/{ryid} [put]
func (ctrl *RegistryOperatorController) Update(ctx *gin.Context) {
	ryid := ctx.Param("ryid")

	var cmd entities.RegistryOperator
	if err := ctx.ShouldBindJSON(&cmd); err != nil {
		if err.Error() == "EOF" {
			ctx.JSON(400, gin.H{"error": "missing request body"})
			return
		}
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if ryid != cmd.RyID.String() {
		ctx.JSON(400, gin.H{"error": "RyID cannot be changed"})
		return
	}

	ry, err := ctrl.ryService.Update(ctx, &cmd)
	if err != nil {
		if errors.Is(err, entities.ErrRegistryOperatorNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, ry)
}

// DeleteByRyID godoc
// @Summary Delete a Registry Operator by RyID
// @Description Delete a Registry Operator by RyID
// @Tags Registry Operators
// @Produce json
// @Param ryid path string true "Registry Operator RyID"
// @Success 204
// @Failure 400
// @Failure 500
// @Router /registry-operators/{ryid} [delete]
func (ctrl *RegistryOperatorController) DeleteByRyID(ctx *gin.Context) {
	ryid := ctx.Param("ryid")

	err := ctrl.ryService.DeleteByRyID(ctx, ryid)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(204, nil)
}
