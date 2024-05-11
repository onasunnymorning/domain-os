package rest

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

// PremiumController is the controller for the Premium Lists and Labels
type PremiumController struct {
	listService  interfaces.PremiumListService
	labelService interfaces.PremiumLabelService
}

// NewPremiumController returns a new instance of PremiumController
func NewPremiumController(e *gin.Engine, listService interfaces.PremiumListService, labelService interfaces.PremiumLabelService) *PremiumController {
	ctrl := &PremiumController{listService: listService, labelService: labelService}

	e.POST("/premium/lists", ctrl.CreateList)
	e.GET("/premium/lists/:name", ctrl.GetListByName)
	e.DELETE("/premium/lists/:name", ctrl.DeleteListByName)
	e.GET("/premium/lists", ctrl.ListPremiumLists)

	e.GET("/premium/labels", ctrl.ListPremiumLabels)

	e.POST("/premium/lists/:name/labels", ctrl.CreateLabel)
	e.GET("/premium/lists/:name/labels/:label/:currency", ctrl.GetLabelByLabelListAndCurrency)
	e.DELETE("/premium/lists/:name/labels/:label/:currency", ctrl.DeleteLabelByLabelListAndCurrency)

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

// DeleteListByName godoc
// @Summary Delete a Premium List by name
// @Description Delete a Premium List by name
// @Tags Premium Lists
// @Produce json
// @Param name path string true "Name of the Premium List"
// @Success 204
// @Failure 404
// @Failure 500
// @Router /premium/lists/{name} [delete]
func (ctrl *PremiumController) DeleteListByName(ctx *gin.Context) {
	name := ctx.Param("name")

	err := ctrl.listService.DeleteListByName(ctx, name)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(204, nil)
}

// CreateLabel godoc
// @Summary Create a new Premium Label in a Premium List
// @Description Create a new Premium Label in a Premium List. The label+currency must be unique within the list.
// @Tags Premium Labels
// @Accept json
// @Produce json
// @Param name path string true "Name of the Premium List"
// @Param label body commands.CreatePremiumLabelCommand true "Premium Label to create"
// @Success 201 {object} entities.PremiumLabel
// @Failure 400
// @Failure 500
// @Router /premium/lists/{name}/labels [post]
func (ctrl *PremiumController) CreateLabel(ctx *gin.Context) {
	// Bind the request body to the command
	var cmd commands.CreatePremiumLabelCommand
	if err := ctx.ShouldBindJSON(&cmd); err != nil {
		if err.Error() == "EOF" {
			ctx.JSON(400, gin.H{"error": "missing request body"})
			return
		}
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Call the service to create the label
	label, err := ctrl.labelService.CreateLabel(ctx, cmd)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(201, label)
}

// GetLabelByLabelListAndCurrency godoc
// @Summary Get a Premium Label by label, list, and currency
// @Description Get a Premium Label by label, list, and currency
// @Tags Premium Labels
// @Accept json
// @Produce json
// @Param name path string true "Name of the Premium List"
// @Param label path string true "Label of the Premium Label"
// @Param currency path string true "Currency of the Premium Label"
// @Success 200 {object} entities.PremiumLabel
// @Failure 404
// @Failure 500
// @Router /premium/lists/{name}/labels/{label}/{currency} [get]
func (ctrl *PremiumController) GetLabelByLabelListAndCurrency(ctx *gin.Context) {
	// Call the service to get the label
	label, err := ctrl.labelService.GetLabelByLabelListAndCurrency(ctx, ctx.Param("label"), ctx.Param("name"), ctx.Param("currency"))
	if err != nil {
		if errors.Is(err, entities.ErrPremiumLabelNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, label)
}

// DeleteLabelByLabelListAndCurrency godoc
// @Summary Delete a Premium Label by label, list, and currency
// @Description Delete a Premium Label by label, list, and currency
// @Tags Premium Labels
// @Produce json
// @Param name path string true "Name of the Premium List"
// @Param label path string true "Label of the Premium Label"
// @Param currency path string true "Currency of the Premium Label"
// @Success 204
// @Failure 404
// @Failure 500
// @Router /premium/lists/{name}/labels/{label}/{currency} [delete]
func (ctrl *PremiumController) DeleteLabelByLabelListAndCurrency(ctx *gin.Context) {
	err := ctrl.labelService.DeleteLabelByLabelListAndCurrency(ctx, ctx.Param("label"), ctx.Param("name"), ctx.Param("currency"))
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(204, nil)
}

// ListPremiumLabels godoc
// @Summary List Premium Labels
// @Description List Premium Labels.
// @Tags Premium Labels
// @Produce json
// @Success 200 {array} entities.PremiumLabel
// @Failure 500
// @Router /premium/labels [get]
func (ctrl *PremiumController) ListPremiumLabels(ctx *gin.Context) {
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

	// filter options
	// this endpoint allows filtering by currency, listName and label
	listName := ctx.Query("list")
	currency := strings.ToUpper(ctx.Query("currency"))
	label := ctx.Query("label")

	labels, err := ctrl.labelService.ListLabels(ctx, pageSize, pageCursor, listName, currency, label)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	response.Data = labels
	// Set the metadata if there are results only
	if len(labels) > 0 {
		response.SetMeta(ctx, labels[len(labels)-1].Label.String(), len(labels), pageSize)
	}

	ctx.JSON(200, response)
}
