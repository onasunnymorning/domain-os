package rest

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

var (
	ErrInvalidGurID = fmt.Errorf("invalid gurID, gurID must be an integer")
)

// IANARegistrarController is the controller for IANARegistrar endpoints
type IANARegistrarController struct {
	IanaRegistrarService interfaces.IANARegistrarService
}

// NewIANARegistrarController creates a new IANARegistrarController and registers the endpoints
func NewIANARegistrarController(e *gin.Engine, ianaRegistrarService interfaces.IANARegistrarService, handler gin.HandlerFunc) *IANARegistrarController {
	controller := &IANARegistrarController{
		IanaRegistrarService: ianaRegistrarService,
	}

	ianaRarGroup := e.Group("/ianaregistrars", handler)
	{
		ianaRarGroup.GET("", controller.List)
		ianaRarGroup.GET("count", controller.Count)
		ianaRarGroup.GET(":gurID", controller.GetByGurID)
	}
	return controller
}

// List godoc
// @Summary List IANARegistrars
// @Description List IANARegistrars from our internal repository. If you need to update the IANA registrar list, please use the /sync endpoint.
// @Tags Registrars
// @Param pagesize query int false "Page Size"
// @Param cursor query string false "Cursor"
// @Param name_like query string false "Name Like (case insensitive search on name)"
// @Param status query string false "Status ('Terminated', 'Reserved', 'Accredited')"
// @Produce json
// @Success 200 {array} entities.IANARegistrar
// @Failure 500
// @Router /ianaregistrars [get]
func (ctrl *IANARegistrarController) List(ctx *gin.Context) {
	query := queries.ListItemsQuery{}
	var err error
	// Prepare the response
	response := response.ListItemResult{}

	// Get the pagesize from the context
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

	// This endpoint allows searching by name
	// Get nameSearchString if there is one (case insensitive search on name)
	nameSearchString := ctx.Query("name_like")

	// Get the status query parameter
	status := ctx.Query("status")

	// Get the list of IANARegistrars
	ianaRegistrars, err := ctrl.IanaRegistrarService.List(ctx, pageSize, pageCursor, nameSearchString, status)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Set the meta and data if there are results only
	// TODO: FIXME: If we are using a search string, will the nextlink include the search?
	if len(ianaRegistrars) > 0 {
		response.Data = ianaRegistrars
		response.SetMeta(ctx, fmt.Sprintf("%d", ianaRegistrars[len(ianaRegistrars)-1].GurID), len(ianaRegistrars), pageSize, query.Filter)
	}
	ctx.JSON(200, response)
}

// GetByGurID godoc
// @Summary Get IANARegistrar by GurID
// @Description Get IANARegistrar by GurID from our internal repository.
// @Tags Registrars
// @Produce json
// @Param gurID path int true "GurID"
// @Success 200 {object} entities.IANARegistrar
// @Failure 404
// @Failure 500
// @Router /ianaregistrars/{gurID} [get]
func (ctrl *IANARegistrarController) GetByGurID(ctx *gin.Context) {
	// Get the gurID from the path
	gurID := ctx.Param("gurID")
	// convert it to an int
	gurIDInt, err := strconv.Atoi(gurID)
	if err != nil {
		ctx.JSON(400, gin.H{"error": ErrInvalidGurID.Error()})
		return
	}

	// Get the IANARegistrar
	ianaRegistrar, err := ctrl.IanaRegistrarService.GetByGurID(ctx, gurIDInt)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, ianaRegistrar)
}

// Count godoc
// @Summary Count IANARegistrars
// @Description Count IANARegistrars from our internal repository.
// @Tags Registrars
// @Produce json
// @Success 200 {object} response.CountResult
// @Failure 500
// @Router /ianaregistrars/count [get]
func (ctrl *IANARegistrarController) Count(ctx *gin.Context) {
	// Get the count of IANARegistrars
	count, err := ctrl.IanaRegistrarService.Count(ctx)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, response.CountResult{
		ObjectType: "IANARegistrar",
		Count:      int64(count),
		Timestamp:  time.Now().UTC(),
	})
}
