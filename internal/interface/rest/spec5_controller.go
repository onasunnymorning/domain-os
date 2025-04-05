package rest

import (
	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

// Spec5Controller is the controller for Spec5 endpoints
type Spec5Controller struct {
	Spec5Service interfaces.Spec5Service
}

// NewSpec5Controller creates a new Spec5Controller and registers the endpoints
func NewSpec5Controller(e *gin.Engine, spec5Service interfaces.Spec5Service, handler gin.HandlerFunc) *Spec5Controller {
	controller := &Spec5Controller{
		Spec5Service: spec5Service,
	}

	spec5Routes := e.Group("/spec5labels", handler)

	{
		spec5Routes.GET("", controller.List)
	}

	return controller
}

// List godoc
// @Summary List Spec5 labels
// @Description List Spec5 labels from our internal repository. If you need to update the Spec5 label list, please use the /sync endpoint.
// @Tags Spec5Labels
// @Produce json
// @Success 200 {array} entities.Spec5Label
// @Failure 500
// @Router /spec5labels [get]
func (ctrl *Spec5Controller) List(ctx *gin.Context) {
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
	// Get the filters from the query string
	filter, err := getSpec5FilterFromContext(ctx)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	query.Filter = filter
	// Get the list of Spec5Labels
	spec5Labels, cursor, err := ctrl.Spec5Service.List(ctx, query)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Set the meta and data if there are results only
	response.Data = spec5Labels
	response.SetMeta(ctx, cursor, len(spec5Labels), query.PageSize, query.Filter)

	ctx.JSON(200, response)
}

// getSpec5FilterFromContext gets the filter from the context
func getSpec5FilterFromContext(ctx *gin.Context) (queries.ListSpec5LabelsFilter, error) {
	filter := queries.ListSpec5LabelsFilter{}
	// Get the filter from the query string
	filter.LabelLike = ctx.Query("label_like")
	filter.TypeEquals = ctx.Query("type_equals")

	return filter, nil
}
