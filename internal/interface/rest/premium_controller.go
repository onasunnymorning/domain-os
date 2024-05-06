package rest

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

// PremiumController is the controller for the Premium Lists and Labels
type PremiumController struct {
	listService interfaces.PremiumListService
}

// NewPremiumController returns a new instance of PremiumController
func NewPremiumController(e *gin.Engine, listService interfaces.PremiumListService) *PremiumController {
	ctrl := &PremiumController{listService: listService}

	e.POST("/premium/lists", ctrl.CreateList)
	e.GET("/premium/lists/:name", ctrl.GetListByName)
	e.GET("/premium/lists", ctrl.ListPremiumLists)

	return ctrl
}

// CreateList godoc
// @Summary Create a new Premium List
// @Description Create a new Premium List. The name must be unique.
// @Tags Premium Lists
// @Accept json
// @Produce json
// @Param list body commands.CreatePremiumListCommand true "Premium List to create"
// @Success 201 {object} entities.PremiumList
// @Failure 400
// @Failure 500
// @Router /premium/lists [post]
func (ctrl *PremiumController) CreateList(ctx *gin.Context) {
	// Bind the request body to the command
	var cmd commands.CreatePremiumListCommand
	if err := ctx.ShouldBindJSON(&cmd); err != nil {
		if err.Error() == "EOF" {
			ctx.JSON(400, gin.H{"error": "missing request body"})
			return
		}
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Call the service to create the list
	list, err := ctrl.listService.CreateList(ctx, cmd)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(201, list)
}

// GetListByName godoc
// @Summary Get a Premium List by name
// @Description Get a Premium List by name
// @Tags Premium Lists
// @Accept json
// @Produce json
// @Param name path string true "Name of the Premium List"
// @Success 200 {object} entities.PremiumList
// @Failure 404
// @Failure 500
// @Router /premium/lists/{name} [get]
func (ctrl *PremiumController) GetListByName(ctx *gin.Context) {
	// Call the service to get the list
	list, err := ctrl.listService.GetListByName(ctx, ctx.Param("name"))
	if err != nil {
		if errors.Is(err, entities.ErrPremiumListNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, list)
}

// ListPremiumLists godoc
// @Summary List all Premium Lists
// @Description List all Premium Lists. There is no pagination on this endpoint.
// @Tags Premium Lists
// @Produce json
// @Success 200 {array} entities.PremiumList
// @Failure 500
// @Router /premium/lists [get]
func (ctrl *PremiumController) ListPremiumLists(ctx *gin.Context) {
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

	lists, err := ctrl.listService.List(ctx, pageSize, pageCursor)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	response.Data = lists
	// Set the metadata if there are results only
	if len(lists) > 0 {
		response.SetMeta(ctx, lists[len(lists)-1].Name, len(lists), pageSize)
	}

	ctx.JSON(200, response)
}
