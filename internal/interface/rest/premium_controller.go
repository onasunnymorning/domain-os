package rest

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

// PremiumController is the controller for the Premium Lists and Labels
type PremiumController struct {
	listService  interfaces.PremiumListService
	labelService interfaces.PremiumLabelService
}

// NewPremiumController returns a new instance of PremiumController
func NewPremiumController(e *gin.Engine, listService interfaces.PremiumListService, labelService interfaces.PremiumLabelService, handler gin.HandlerFunc) *PremiumController {
	ctrl := &PremiumController{listService: listService, labelService: labelService}

	premiumGroup := e.Group("/premium", handler)

	{
		premiumGroup.POST("lists", ctrl.CreateList)
		premiumGroup.GET("lists/:name", ctrl.GetListByName)
		premiumGroup.DELETE("lists/:name", ctrl.DeleteListByName)
		premiumGroup.GET("lists", ctrl.ListPremiumLists)

		premiumGroup.GET("labels", ctrl.ListPremiumLabels)
		premiumGroup.POST("lists/:name/labels", ctrl.CreateLabel)
		premiumGroup.GET("lists/:name/labels/:label/:currency", ctrl.GetLabelByLabelListAndCurrency)
		premiumGroup.DELETE("lists/:name/labels/:label/:currency", ctrl.DeleteLabelByLabelListAndCurrency)
	}
	return ctrl
}

// CreateList godoc
// @Summary Create a new Premium List
// @Description Create a new Premium List. The name must be unique.
// @Tags Premiums
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
// @Tags Premiums
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
// @Tags Premiums
// @Produce json
// @Success 200 {array} entities.PremiumList
// @Failure 500
// @Router /premium/lists [get]
func (ctrl *PremiumController) ListPremiumLists(ctx *gin.Context) {
	query := queries.ListItemsQuery{}
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
		response.SetMeta(ctx, lists[len(lists)-1].Name, len(lists), pageSize, query.Filter)
	}

	ctx.JSON(200, response)
}

// DeleteListByName godoc
// @Summary Delete a Premium List by name
// @Description Delete a Premium List by name
// @Tags Premiums
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
// @Tags Premiums
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
// @Tags Premiums
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
// @Tags Premiums
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
// @Description Pull Premium labels with optional filters. The results are paginated.
// @Tags Premiums
// @Produce json
// @Param pagesize query int false "Page Size"
// @Param cursor query string false "Page Cursor"
// @Param label_like query string false "Label like"
// @Param premium_list_name_equals query string false "Premium List Name equals"
// @Param currency_equals query string false "Currency equals"
// @Param class_equals query string false "Class equals"
// @Param registration_amount_equals query string false "Registration Amount equals"
// @Param renewal_amount_equals query string false "Renewal Amount equals"
// @Param transfer_amount_equals query string false "Transfer Amount equals"
// @Param restore_amount_equals query string false "Restore Amount equals"
// @Success 200 {array} entities.PremiumLabel
// @Failure 500
// @Router /premium/labels [get]
func (ctrl *PremiumController) ListPremiumLabels(ctx *gin.Context) {
	query := queries.ListItemsQuery{}
	var err error
	// Prepare the response
	response := response.ListItemResult{}

	// Get the pagesize from the query string
	query.PageSize, err = GetPageSize(ctx)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// Get the cursor from the query string
	query.PageCursor, err = GetAndDecodeCursor(ctx)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Get the filter from the query string
	filter, err := getPremiumLabelFilterFromContext(ctx)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	query.Filter = filter

	labels, cursor, err := ctrl.labelService.ListLabels(ctx, query)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	response.Data = labels
	response.SetMeta(ctx, cursor, len(labels), query.PageSize, query.Filter)

	ctx.JSON(200, response)
}

func getPremiumLabelFilterFromContext(ctx *gin.Context) (queries.ListPremiumLabelsFilter, error) {
	filter := queries.ListPremiumLabelsFilter{}

	// Get the filter from the query string
	filter.LabelLike = ctx.Query("label_like")
	filter.PremiumListNameEquals = ctx.Query("premium_list_name_equals")
	filter.CurrencyEquals = strings.ToUpper(ctx.Query("currency_equals"))
	filter.ClassEquals = ctx.Query("class_equals")
	filter.RegistrationAmountEquals = ctx.Query("registration_amount_equals")
	filter.RenewalAmountEquals = ctx.Query("renewal_amount_equals")
	filter.TransferAmountEquals = ctx.Query("transfer_amount_equals")
	filter.RestoreAmountEquals = ctx.Query("restore_amount_equals")

	return filter, nil
}

func getPremiumListFilterFromContext(ctx *gin.Context) (queries.ListPremiumListsFilter, error) {
	filter := queries.ListPremiumListsFilter{}

	// Get the filter from the query string
	filter.NameLike = ctx.Query("name_like")
	filter.RyIDEquals = ctx.Query("ryid_equals")
	filter.CreatedBefore = ctx.Query("created_before")
	filter.CreatedAfter = ctx.Query("created_after")

	return filter, nil
}
