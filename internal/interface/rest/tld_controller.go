package rest

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/request"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

type TLDController struct {
	tldService interfaces.TLDService
}

func NewTLDController(e *gin.Engine, tldService interfaces.TLDService) *TLDController {
	controller := &TLDController{
		tldService: tldService,
	}

	e.GET("/tlds/:tldName", controller.GetTLDByName)
	e.GET("/tlds", controller.ListTLDs)
	e.POST("/tlds", controller.CreateTLD)
	e.DELETE("/tlds/:tldName", controller.DeleteTLDByName)
	e.GET("/tlds/:tldName/header", controller.GetTLDHeader)

	return controller
}

// GetTLDByName godoc
// @Summary Get a TLD by name
// @Description Get a TLD by name
// @Tags TLDs
// @Produce json
// @Param tldName path string true "TLD Name"
// @Success 200 {object} entities.TLD
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /tlds/{tldName} [get]
func (ctrl *TLDController) GetTLDByName(ctx *gin.Context) {
	name := ctx.Param("tldName")

	tld, err := ctrl.tldService.GetTLDByName(ctx, name, false)
	if err != nil {
		if errors.Is(err, entities.ErrTLDNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, tld)
}

// ListTLDs godoc
// @Summary List TLDs
// @Description List TLDs.
// @Tags TLDs
// @Produce json
// @Param pagesize query int false "Page size"
// @Param cursor query string false "Cursor"
// @Success 200 {array} response.ListItemResult
// @Failure 400
// @Failure 500
// @Router /tlds [get]
func (ctrl *TLDController) ListTLDs(ctx *gin.Context) {
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

	// Get the tlds from the service
	tlds, err := ctrl.tldService.ListTLDs(ctx, pageSize, pageCursor)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Set the Data and metadata if there are results only
	response.Data = tlds
	if len(tlds) > 0 {
		response.SetMeta(ctx, tlds[len(tlds)-1].Name.String(), len(tlds), pageSize)
	}

	// Return the response
	ctx.JSON(200, response)
}

// DeleteTLDByName godoc
// @Summary Delete a TLD by Name
// @Description Delete a TLD by Name
// @Tags TLDs
// @Produce json
// @Param tldName path string true "TLD Name"
// @Success 204
// @Failure 400
// @Failure 500
// @Router /tlds/{tldName} [delete]
func (ctrl *TLDController) DeleteTLDByName(ctx *gin.Context) {
	name := ctx.Param("tldName")

	// Get the Event from the context
	event := GetEventFromContext(ctx)

	err := ctrl.tldService.DeleteTLDByName(ctx, name)
	if err != nil {
		event.Details.Error = err.Error()
		if errors.Is(err, services.ErrCannotDeleteTLDWithActivePhases) {
			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(204, nil)
}

// CreateTLD godoc
// @Summary Create a new TLD
// @Description Create a new TLD
// @Tags TLDs
// @Accept json
// @Produce json
// @Param registrar body commands.CreateTLDCommand true "TLD"
// @Success 200 {object} commands.CreateTLDCommandResult
// @Failure 400
// @Failure 500
// @Router /tlds [post]
func (ctrl *TLDController) CreateTLD(ctx *gin.Context) {
	var req request.CreateTLDRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		if err.Error() == "EOF" {
			ctx.JSON(400, gin.H{"error": "missing request body"})
			return
		}
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Get the Event from the context
	event := GetEventFromContext(ctx)
	// Set the event details.command
	event.Details.Command = req

	cmd, err := req.ToCreateTLDCommand()
	if err != nil {
		event.Details.Error = err.Error()
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result, err := ctrl.tldService.CreateTLD(ctx, cmd)
	if err != nil {
		event.Details.Error = err.Error()
		if errors.Is(err, entities.ErrinvalIdDomainNameLength) || errors.Is(err, entities.ErrInvalidLabelLength) || errors.Is(err, entities.ErrInvalidLabelDash) || errors.Is(err, entities.ErrInvalidLabelDoubleDash) || errors.Is(err, entities.ErrInvalidLabelIDN) || errors.Is(err, entities.ErrLabelContainsInvalidCharacter) {
			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Set the event details.after
	event.Details.After = result

	ctx.JSON(201, result)
}

// GetTLDHeader godoc
// @Summary Get a TLD header
// @Description Get a TLD header
// @Tags TLDs
// @Produce json
// @Param tldName path string true "TLD Name"
// @Success 200 {object} entities.TLDHeader
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /tlds/{tldName}/header [get]
func (ctrl *TLDController) GetTLDHeader(ctx *gin.Context) {
	name := ctx.Param("tldName")

	tldHeader, err := ctrl.tldService.GetTLDHeader(ctx, name)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, tldHeader)
}
