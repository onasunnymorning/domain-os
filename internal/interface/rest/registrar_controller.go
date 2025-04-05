package rest

import (
	"strconv"
	"time"

	"errors"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

// RegistrarController is the controller for TLD endpoints
type RegistrarController struct {
	ianaRegistrarService interfaces.IANARegistrarService
	rarService           interfaces.RegistrarService
}

// NewRegistrarController creates a new RegistrarController
func NewRegistrarController(e *gin.Engine, rarService interfaces.RegistrarService, ianaRegistrarService interfaces.IANARegistrarService, handler gin.HandlerFunc) *RegistrarController {
	controller := &RegistrarController{
		ianaRegistrarService: ianaRegistrarService,
		rarService:           rarService,
	}

	rarGroup := e.Group("/registrars", handler)
	{
		rarGroup.GET(":clid", controller.GetByClID)
		rarGroup.GET("gurid/:gurid", controller.GetByGurID)
		rarGroup.GET("", controller.List)
		rarGroup.GET("count", controller.GetRegistrarCount)
		rarGroup.POST("", controller.Create)
		rarGroup.POST("/bulk", controller.BulkCreate)
		rarGroup.PUT(":clid", controller.UpdateRegistrar)
		rarGroup.PUT(":clid/status/:status", controller.SetRegistrarStatus)
		// REQUEST REMOVAL rarGroup.POST(":gurid", controller.CreateRegistrarByGurID)
		rarGroup.DELETE(":clid", controller.DeleteRegistrarByClID)
	}

	e.POST("/registrars-bulk", handler, controller.BulkCreate)

	return controller
}

// GetByClID godoc
// @Summary Get a Registrar by name
// @Description Get a Registrar by name
// @Tags Registrars
// @Produce json
// @Param clid path string true "Registrar Client ID"
// @Success 200 {object} entities.Registrar
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /registrars/{clid} [get]
func (ctrl *RegistrarController) GetByClID(ctx *gin.Context) {
	clid := ctx.Param("clid")

	rar, err := ctrl.rarService.GetByClID(ctx, clid, true)
	if err != nil {
		if errors.Is(err, entities.ErrRegistrarNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, rar)
}

// GetByGurID godoc
// @Summary Get a Registrar by GurID
// @Description Get a Registrar by GurID
// @Tags Registrars
// @Produce json
// @Param gurid path int true "Registrar GurID"
// @Success 200 {object} entities.Registrar
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /registrars/gurid/{gurid} [get]
func (ctrl *RegistrarController) GetByGurID(ctx *gin.Context) {
	guridString := ctx.Param("gurid")
	gurid, err := strconv.Atoi(guridString)
	if err != nil {
		ctx.JSON(400, gin.H{"error": ErrInvalidGurID.Error()})
		return
	}

	rar, err := ctrl.rarService.GetByGurID(ctx, gurid)
	if err != nil {
		if errors.Is(err, entities.ErrRegistrarNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, rar)
}

// List godoc
// @Summary Returns paginated list of Registrars optinally filtered
// @Description Returns paginated list of Registrars optinally filtered.
// @Tags Registrars
// @Produce json
// @Param pagesize query int false "Page size"
// @Param cursor query string false "Cursor"
// @Param clid_like query string false "ClID like"
// @Param name_like query string false "Name like"
// @Param nick_name_like query string false "NickName like"
// @Param gurid_equals query int false "GurID equals"
// @Param email_like query string false "Email like"
// @Param status_equals query string false "Status equals"
// @Param iana_status_equals query string false "IANAStatus equals"
// @Param autorenew_equals query string false "Autorenew equals"
// @Success 200 {array} entities.RegistrarListItem
// @Failure 400
// @Failure 500
// @Router /registrars [get]
func (ctrl *RegistrarController) List(ctx *gin.Context) {
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

	// Apply filters
	filter, err := getRegistrarListFilterFromContext(ctx)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	query.Filter = *filter

	// List the Registrars
	rars, cursor, err := ctrl.rarService.List(ctx, query)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Set the response Data and Meta
	response.Data = rars
	response.SetMeta(ctx, cursor, len(rars), query.PageSize, query.Filter)

	ctx.JSON(200, response)
}

// Create godoc
// @Summary Create a new Registrar, will be created with the status 'readonly'
// @Description Create a new Registrar using CreateRegistrarCommand. ClID, email,
// @Description name and at least one postal info is required. A new Registrar will be created with the status 'readonly'.
// @Description A RegistrarLifecycleEvent will be created with the status 'created'.
// @Description This should trigger the Registrar onboarding process.
// @Tags Registrars
// @Accept json
// @Produce json
// @Param registrar body commands.CreateRegistrarCommand true "Registrar"
// @Success 200 {object} entities.Registrar
// @Failure 400
// @Failure 500
// @Router /registrars [post]
func (ctrl *RegistrarController) Create(ctx *gin.Context) {
	var cmd commands.CreateRegistrarCommand
	if err := ctx.ShouldBindJSON(&cmd); err != nil {
		if err.Error() == "EOF" {
			ctx.JSON(400, gin.H{"error": "missing request body"})
			return
		}
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result, err := ctrl.rarService.Create(ctx, &cmd)
	if err != nil {
		if errors.Is(err, entities.ErrInvalidRegistrar) {
			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return

	}

	ctx.JSON(201, result)
}

// BulkCreate godoc
// @Summary Bulk create Registrars
// @Description Bulk create Registrars can create up to 1000 registrars at a time
// @Tags Registrars
// @Accept json
// @Produce json
// @Param registrars body []commands.CreateRegistrarCommand true "Registrars"
// @Success 201
// @Failure 400
// @Failure 500
// @Router /registrars/bulk [post]
func (ctrl *RegistrarController) BulkCreate(ctx *gin.Context) {
	var cmd []*commands.CreateRegistrarCommand
	if err := ctx.ShouldBindJSON(&cmd); err != nil {
		if err.Error() == "EOF" {
			ctx.JSON(400, gin.H{"error": "missing request body"})
			return
		}
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	err := ctrl.rarService.BulkCreate(ctx, cmd)
	if err != nil {
		if errors.Is(err, entities.ErrInvalidRegistrar) {
			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return

	}

	ctx.JSON(201, nil)
}

// DeleteRegistrarByClID godoc
// @Summary Delete a Registrar by ClID
// @Description Delete a Registrar by ClID
// @Tags Registrars
// @Produce json
// @Param clid path string true "Registrar Client ID"
// @Success 204
// @Failure 400
// @Failure 500
// @Router /registrars/{clid} [delete]
func (ctrl *RegistrarController) DeleteRegistrarByClID(ctx *gin.Context) {
	clid := ctx.Param("clid")

	err := ctrl.rarService.Delete(ctx, clid)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(204, nil)
}

// UpdateRegistrar godoc
// @Summary Update a Registrar
// @Description Update a Registrar
// @Tags Registrars
// @Accept json
// @Produce json
// @Param clid path string true "Registrar Client ID"
// @Param registrar body entities.Registrar true "Registrar"
// @Success 200 {object} entities.Registrar
// @Failure 400
// @Failure 500
// @Router /registrars/{clid} [put]
func (ctrl *RegistrarController) UpdateRegistrar(ctx *gin.Context) {
	var rar entities.Registrar
	if err := ctx.ShouldBindJSON(&rar); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// We don't allow changing the ClID since it is the main identifier that links to all other objects
	if rar.ClID.String() != ctx.Param("clid") {
		ctx.JSON(400, gin.H{"error": "ClID cannot be changed"})
		return
	}

	// Make sure we are saving a valid registrar
	if err := rar.Validate(); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result, err := ctrl.rarService.Update(ctx, &rar)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, result)
}

// SetRegistrarStatus godoc
// @Summary Set the status of a Registrar
// @Description Set the status of a Registrar. Allowed values are: 'terminated', 'accredited', 'readonly' see https://www.iana.org/assignments/registrar-ids/registrar-ids.xhtml
// @Description Status will always be set in lowecase, if you provide an uppercase status, it will be converted to lowercase but won't thow an error.
// @Tags Registrars
// @Produce json
// @Param clid path string true "Registrar Client ID"
// @Param status path string true "Registrar Status"
// @Success 200
// @Failure 400
// @Failure 500
// @Router /registrars/{clid}/status/{status} [put]
func (ctrl *RegistrarController) SetRegistrarStatus(ctx *gin.Context) {
	clid := ctx.Param("clid")
	status := entities.RegistrarStatus(ctx.Param("status"))

	err := ctrl.rarService.SetStatus(ctx, clid, status)
	if err != nil {
		if errors.Is(err, entities.ErrRegistrarNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(204, nil)
}

// GetRegistrarCount godoc
// @Summary Get the number of registrars
// @Description Get the number of registrars
// @Tags Registrars
// @Produce json
// @Success 200 {object} response.CountResult
// @Failure 500
// @Router /registrars/count [get]
func (ctrl *RegistrarController) GetRegistrarCount(ctx *gin.Context) {
	count, err := ctrl.rarService.Count(ctx)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, response.CountResult{
		ObjectType: "Registrar",
		Count:      count,
		Timestamp:  time.Now().UTC(),
	})
}

func getRegistrarListFilterFromContext(ctx *gin.Context) (*queries.ListRegistrarsFilter, error) {
	var err error
	filter := &queries.ListRegistrarsFilter{}
	filter.ClidLike = ctx.Query("clid_like")
	filter.NameLike = ctx.Query("name_like")
	filter.NickNameLike = ctx.Query("nick_name_like")

	if ctx.Query("gurid_equals") != "" {
		filter.GuridEquals, err = strconv.Atoi(ctx.Query("gurid_equals"))
		if err != nil {
			return nil, err
		}
	}

	filter.EmailLike = ctx.Query("email_like")
	filter.StatusEquals = ctx.Query("status_equals")
	filter.IANAStatusEquals = ctx.Query("iana_status_equals")
	filter.AutorenewEquals = ctx.Query("autorenew_equals")

	return filter, nil
}
