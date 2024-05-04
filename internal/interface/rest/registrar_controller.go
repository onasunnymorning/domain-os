package rest

import (
	"strconv"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/pkg/errors"

	"github.com/docker/docker/pkg/namesgenerator"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/request"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

// RegistrarController is the controller for TLD endpoints
type RegistrarController struct {
	ianaRegistrarService interfaces.IANARegistrarService
	rarService           interfaces.RegistrarService
}

// NewRegistrarController creates a new RegistrarController
func NewRegistrarController(e *gin.Engine, rarService interfaces.RegistrarService, ianaRegistrarService interfaces.IANARegistrarService) *RegistrarController {
	controller := &RegistrarController{
		ianaRegistrarService: ianaRegistrarService,
		rarService:           rarService,
	}

	e.GET("/registrars/:clid", controller.GetByClID)
	e.GET("/registrars/gurid/:gurid", controller.GetByGurID)
	e.GET("/registrars", controller.List)
	e.POST("/registrars", controller.Create)
	e.PUT("/registrars/:clid", controller.UpdateRegistrar)
	e.POST("/registrars/:gurid", controller.CreateRegistrarByGurID)
	e.DELETE("/registrars/:clid", controller.DeleteRegistrarByClID)

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
// @Summary List Registrars
// @Description List Registrars.
// @Tags Registrars
// @Produce json
// @Param pagesize query int false "Page size"
// @Param cursor query string false "Cursor"
// @Success 200 {array} entities.Registrar
// @Failure 400
// @Failure 500
// @Router /registrars [get]
func (ctrl *RegistrarController) List(ctx *gin.Context) {
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

	rars, err := ctrl.rarService.List(ctx, pageSize, pageCursor)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	response.Data = rars
	// Set the metadata if there are results only
	if len(rars) > 0 {
		response.SetMeta(ctx, rars[len(rars)-1].ClID.String(), len(rars), pageSize)
	}

	ctx.JSON(200, response)
}

// Create godoc
// @Summary Create a new Registrar
// @Description Create a new Registrar
// @Tags Registrars
// @Accept json
// @Produce json
// @Param registrar body commands.CreateRegistrarCommand true "Registrar"
// @Success 200 {object} commands.CreateRegistrarCommandResult
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

// CreateRegistrarByGurID godoc
// @Summary Create a new Registrar by GurID
// @Description Creates a registrar by looking up the GurID in the IANA repository and taking the data from there. You will need to supply an email only. All the other data will be taken from the IANA repository.
// @Tags Registrars
// @Accept json
// @Produce json
// @Param gurid path int true "Registrar GurID"
// @Param registrarEmail body request.CreateRegistrarFromGurIDRequest true "RegistrarEmail"
// @Success 200 {object} commands.CreateRegistrarCommandResult
// @Failure 400
// @Failure 500
// @Router /registrars/{gurid} [post]
func (ctrl *RegistrarController) CreateRegistrarByGurID(ctx *gin.Context) {
	// Get the email from the request body
	var req request.CreateRegistrarFromGurIDRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// get the GurID from the request
	guridString := ctx.Param("gurid")
	// turn the GurID into an int
	gurid, err := strconv.Atoi(guridString)
	if err != nil {
		ctx.JSON(400, gin.H{"error": ErrInvalidGurID.Error()})
		return
	}
	// TODO: Check if a registrar already exists with that gurid (if we want to keep it quite strict)

	// look up the GurID in our internal repository
	ianaRar, err := ctrl.ianaRegistrarService.GetByGurID(ctx, gurid)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// if it does exist, create a new registrar using the data from the IANA registrar
	var cmd commands.CreateRegistrarCommand
	cmd.ClID = namesgenerator.GetRandomName(0)
	cmd.Name = ianaRar.Name
	cmd.Email = req.Email
	cmd.GurID = ianaRar.GurID
	cmd.PostalInfo = [2]*entities.RegistrarPostalInfo{
		{Type: "int"}, {Type: "loc"},
	}

	result, err := ctrl.rarService.Create(ctx, &cmd)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, result)
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
