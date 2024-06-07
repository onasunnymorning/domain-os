package rest

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

// AccreditationController is the controller for the Registrar <> TLD Accreditations
type AccreditationController struct {
	accService interfaces.AccreditationService
}

// NewAccreditationController returns a new AccreditationController
func NewAccreditationController(e *gin.Engine, accService interfaces.AccreditationService) *AccreditationController {
	controller := &AccreditationController{
		accService: accService,
	}

	e.POST("/accreditations/:tldName/:rarClID", controller.Accredit)
	e.DELETE("/accreditations/:tldName/:rarClID", controller.Deaccredit)
	e.GET("/accreditations/registrar/:rarClID", controller.ListRegistarAccreditations)
	e.GET("/accreditations/tld/:tldName", controller.ListTLDRegistrars)

	return controller
}

// Accredit godoc
// @Summary Accredit a Registrar for a TLD
// @Description Accredit a Registrar for a TLD. Will return 201 if successful, even if a registrar is already accredited.
// @Tags Accreditations
// @Produce json
// @Param tldName path string true "TLD Name"
// @Param rarClID path string true "Registrar ClID"
// @Success 201
// @Failure 400
// @Failure 500
// @Router /accredit/{tldName}/{rarClID} [post]
func (ctrl *AccreditationController) Accredit(ctx *gin.Context) {
	tldName := ctx.Param("tldName")
	rarClID := ctx.Param("rarClID")

	err := ctrl.accService.CreateAccreditation(ctx, tldName, rarClID)
	if err != nil {
		if errors.Is(err, services.ErrInvalidAccreditation) {
			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.Status(201)
	e := entities.NewEvent(ctx.GetString("App"), "admin-userid-1", entities.EventTypeAccreditation, entities.ObjectTypeTLD, tldName, ctx.FullPath())
	e.Details.Result = entities.EventResultSuccess
	e.Details.After = fmt.Sprintf("Registrar %s accredited for TLD %s", rarClID, tldName)
	logMessage(ctx, e)
}

// Deaccredit godoc
// @Summary Deaccredit a Registrar for a TLD
// @Description Deaccredit a Registrar for a TLD. Will return 204 if successful, even if a registrar is not accredited.
// @Tags Accreditations
// @Produce json
// @Param tldName path string true "TLD Name"
// @Param rarClID path string true "Registrar ClID"
// @Success 204
// @Failure 400
// @Failure 500
// @Router /deaccredit/{tldName}/{rarClID} [delete]
func (ctrl *AccreditationController) Deaccredit(ctx *gin.Context) {
	tldName := ctx.Param("tldName")
	rarClID := ctx.Param("rarClID")

	err := ctrl.accService.DeleteAccreditation(ctx, tldName, rarClID)
	if err != nil {
		if errors.Is(err, services.ErrInvalidAccreditation) {
			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.Status(204)
}

// ListRegistarAccreditations godoc
// @Summary List all accreditations for a Registrar
// @Description List all accreditations for a Registrar. Returns a list of TLDs the registrar is accredited for.
// @Tags Accreditations
// @Produce json
// @Param rarClID path string true "Registrar ClID"
// @Success 200
// @Failure 404
// @Failure 400
// @Failure 500
// @Router /accreditations/registrar/{rarClID} [get]
func (ctrl *AccreditationController) ListRegistarAccreditations(ctx *gin.Context) {
	rarClID := ctx.Param("rarClID")
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

	tlds, err := ctrl.accService.ListRegistrarTLDs(ctx, pageSize, pageCursor, rarClID)
	if err != nil {
		if errors.Is(err, entities.ErrRegistrarNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}
	// Pour into our response struct
	response.Data = tlds
	if len(tlds) > 0 {
		response.SetMeta(ctx, tlds[len(tlds)-1].Name.String(), len(tlds), pageSize)
	}

	// Return the response
	ctx.JSON(200, response)
}

// ListTLDRegistrars godoc
// @Summary List all registrars accredited for a TLD
// @Description List all registrars accredited for a TLD. Returns a list of Registrars accredited for the TLD.
// @Tags Accreditations
// @Produce json
// @Param tldName path string true "TLD Name"
// @Success 200
// @Failure 404
// @Failure 400
// @Failure 500
// @Router /accreditations/tld/{tldName} [get]
func (ctrl *AccreditationController) ListTLDRegistrars(ctx *gin.Context) {
	tldName := ctx.Param("tldName")
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

	rars, err := ctrl.accService.ListTLDRegistrars(ctx, pageSize, pageCursor, tldName)
	if err != nil {
		if errors.Is(err, entities.ErrTLDNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}
	// Pour into our response struct
	response.Data = rars
	if len(rars) > 0 {
		response.SetMeta(ctx, rars[len(rars)-1].ClID.String(), len(rars), pageSize)
	}

	// Return the response
	ctx.JSON(200, response)
}
