package rest

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

// RegistryOperatorController is the controller for Registry Operator endpoints
type RegistryOperatorController struct {
	ryService interfaces.RegistryOperatorService
}

// NewRegistryOperatorController creates a new RegistryOperatorController
func NewRegistryOperatorController(e *gin.Engine, ryService interfaces.RegistryOperatorService, handler gin.HandlerFunc) *RegistryOperatorController {
	ctrl := &RegistryOperatorController{
		ryService: ryService,
	}

	ryOpGroup := e.Group("/registry-operators", handler)

	{
		ryOpGroup.POST("", ctrl.Create)
		ryOpGroup.GET("", ctrl.List)
		ryOpGroup.GET(":ryid", ctrl.GetByRyID)
		ryOpGroup.PUT(":ryid", ctrl.Update)
		ryOpGroup.DELETE(":ryid", ctrl.DeleteByRyID)
	}
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

// List godoc
// @Summary List Registry Operators
// @Description List Registry Operators
// @Tags Registry Operators
// @Produce json
// @Param pagesize query int false "Page size"
// @Param pagecursor query string false "Page cursor"
// @Success 200 {array} entities.RegistryOperator
// @Failure 400
// @Failure 500
// @Router /registry-operators [get]
func (ctrl *RegistryOperatorController) List(ctx *gin.Context) {
	var err error
	// Prepare the response
	response := response.ListItemResult{}
	// Get the pagesize from the query string
	pageSize, err := GetPageSize(ctx)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// Get the cursor from the query string
	pageCursor, err := GetAndDecodeCursor(ctx)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	ros, err := ctrl.ryService.List(ctx, pageSize, pageCursor)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	response.Data = ros
	// Set the metadata if there are results only
	if len(ros) > 0 {
		response.SetMeta(ctx, ros[len(ros)-1].RyID.String(), len(ros), pageSize)
	}

	ctx.JSON(200, response)
}
