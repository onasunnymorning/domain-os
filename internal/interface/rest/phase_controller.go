package rest

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

// PhaseController is the controller for the Phase entity
type PhaseController struct {
	phaseService interfaces.PhaseService
}

// NewPhaseController returns a new instance of PhaseController
func NewPhaseController(e *gin.Engine, phaseService interfaces.PhaseService, handler gin.HandlerFunc) *PhaseController {
	ctrl := &PhaseController{
		phaseService: phaseService,
	}

	phaseGroup := e.Group("/tlds/:tldName/phases", handler)
	{
		phaseGroup.POST("", ctrl.CreatePhase)
		phaseGroup.GET("", ctrl.ListPhases)
		phaseGroup.GET("active", ctrl.ListActivePhasesPerTLD)
		phaseGroup.GET(":phaseName", ctrl.GetPhase)
		phaseGroup.PUT(":phaseName/policy", ctrl.UpdatePhasePolicy)
		phaseGroup.DELETE(":phaseName", ctrl.DeletePhase)
		phaseGroup.PUT(":phaseName/end", ctrl.EndPhase)
		phaseGroup.POST(":phaseName/premium-list/:premiumListName", ctrl.SetPremiumList)
		phaseGroup.DELETE(":phaseName/premium-list/:premiumListName", ctrl.UnSetPremiumList)
	}
	e.GET("/phases/active/ga", handler, ctrl.ListActiveGAPhases)
	return ctrl
}

// CreatePhase godoc
// @Summary Create a new phase
// @Description Create a new phase. The phase name must be unique within the TLD. The phase name must be a valid slug and is case sensitive. If the TLD does not exist a 404 will be returned. GA phases can not overlap with each other. Launch phases can overlap with each other and can run in parallel with GA phases.
// @Tags Phases
// @Accept json
// @Produce json
// @Param phase body commands.CreatePhaseCommand true "Phase to create"
// @Param tldName path string true "TLD name"
// @Success 201 {object} entities.Phase
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /tlds/{tldName}/phases [post]
func (ctrl *PhaseController) CreatePhase(ctx *gin.Context) {
	// Bind the request body to the command
	var cmd commands.CreatePhaseCommand
	if err := ctx.ShouldBindJSON(&cmd); err != nil {
		if err.Error() == "EOF" {
			ctx.JSON(400, gin.H{"error": "missing request body"})
			return
		}
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// Set the TLD in the command
	cmd.TLDName = ctx.Param("tldName")

	// Create the phase
	phase, err := ctrl.phaseService.CreatePhase(ctx, &cmd)
	if err != nil {
		if errors.Is(err, entities.ErrTLDNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, entities.ErrInvalidPhase) {
			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(201, phase)

}

// GetPhase godoc
// @Summary Get a phase by name and tld name
// @Description Get a phase by name and tld name. TLD name and phase name are case sensitive.
// @Tags Phases
// @Produce json
// @Param tldName path string true "TLD name"
// @Param phaseName path string true "Phase name"
// @Success 200 {object} entities.Phase
// @Failure 404
// @Failure 500
// @Router /tlds/{tldName}/phases/{phaseName} [get]
func (ctrl *PhaseController) GetPhase(ctx *gin.Context) {
	phase, err := ctrl.phaseService.GetPhaseByTLDAndName(ctx, ctx.Param("tldName"), ctx.Param("phaseName"))
	if err != nil {
		if errors.Is(err, entities.ErrPhaseNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, phase)
}

// EndPhase godoc
// @Summary Sets or updates an end date on a phase by name and tld name
// @Description Sets or updates an end date on a phase by name and tld name. End date must be in the future and after the start date. TLD name and phase name are case sensitive. The resulting phase will be checked for validity and will be returned if valid.
// @Tags Phases
// @Accept json
// @Produce json
// @Param tldName path string true "TLD name"
// @Param phaseName path string true "Phase name"
// @Param endDate body commands.EndPhaseCommand true "End date to set"
// @Success 200 {object} entities.Phase
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /tlds/{tldName}/phases/{phaseName}/end [put]
func (ctrl *PhaseController) EndPhase(ctx *gin.Context) {
	// Bind the request body to the command
	var cmd commands.EndPhaseCommand
	if err := ctx.ShouldBindJSON(&cmd); err != nil {
		if err.Error() == "EOF" {
			ctx.JSON(400, gin.H{"error": "missing request body"})
			return
		}
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// Set the TLD in the command
	cmd.TLDName = ctx.Param("tldName")

	// Set the PhaseName in the command
	cmd.PhaseName = ctx.Param("phaseName")

	// Pass the new enddate through our entity
	phase, err := ctrl.phaseService.EndPhase(ctx, &cmd)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, phase)

}

// DeletePhase godoc
// @Summary Delete a phase by name and tld name
// @Description Delete a phase by name and tld name. TLD name and phase name are case sensitive. You cannot delete the current phase or historical phases, this will result in a 400 error. You can only delete future phases that haven't started yet. Deleting a phase will delete fees and prices associated with the phase if they exists.
// @Tags Phases
// @Produce json
// @Param tldName path string true "TLD name"
// @Param phaseName path string true "Phase name"
// @Success 204
// @Failure 500
// @Router /tlds/{tldName}/phases/{phaseName} [delete]
func (ctrl *PhaseController) DeletePhase(ctx *gin.Context) {
	err := ctrl.phaseService.DeletePhaseByTLDAndName(ctx, ctx.Param("tldName"), ctx.Param("phaseName"))
	if err != nil {
		if errors.Is(err, entities.ErrDeleteCurrentPhase) || errors.Is(err, entities.ErrDeleteHistoricPhase) {
			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(204, nil)
}

// ListPhases godoc
// @Summary List all phases for a TLD
// @Description List all phases for a TLD. Phases are returned in order of creation and this endpoint offers pagination. The cursor is the last phase name in the previous page. The pagesize is the number of phases to return. The first page should be requested without a cursor.
// @Tags Phases
// @Produce json
// @Param tldName path string true "TLD name"
// @Success 200 {array} response.ListItemResult
// @Failure 500
// @Router /tlds/{tldName}/phases [get]
func (ctrl *PhaseController) ListPhases(ctx *gin.Context) {
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

	phases, err := ctrl.phaseService.ListPhasesByTLD(ctx, ctx.Param("tldName"), pageSize, pageCursor)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Set the Data and metadata if there are results only
	response.Data = phases
	if len(phases) > 0 {
		response.SetMeta(ctx, phases[len(phases)-1].Name.String(), len(phases), pageSize)
	}

	ctx.JSON(200, response)
}

// ListActivePhasesPerTLD godoc
// @Summary List all active phases for a TLD
// @Description List all active phases for a TLD. Same as ListPhases but only returns active phases (GA and Launch). Phases are returned in order of creation and this endpoint offers pagination. The cursor is the last phase name in the previous page. The pagesize is the number of phases to return. The first page should be requested without a cursor.
// @Tags Phases
// @Produce json
// @Param tldName path string true "TLD name"
// @Success 200 {array} response.ListItemResult
// @Failure 500
// @Router /tlds/{tldName}/phases/active [get]
func (ctrl *PhaseController) ListActivePhasesPerTLD(ctx *gin.Context) {
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

	phases, err := ctrl.phaseService.ListActivePhasesByTLD(ctx, ctx.Param("tldName"), pageSize, pageCursor)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Set the Data and metadata if there are results only
	response.Data = phases
	if len(phases) > 0 {
		response.SetMeta(ctx, phases[len(phases)-1].Name.String(), len(phases), pageSize)
	}

	ctx.JSON(200, response)
}

// ListActiveGAPhases godoc
// @Summary List all active GA phases for all TLDs
// @Description returns a list of current GA phase for all TLDs on the system
// @Tags Phases
// @Produce json
// @Success 200 {array} response.ListItemResult
// @Failure 500
// @Router /phases/active/ga [get]
func (ctrl *PhaseController) ListActiveGAPhases(ctx *gin.Context) {
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

	phases, err := ctrl.phaseService.ListActiveGAPhases(ctx, pageSize, pageCursor)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Set the Data and metadata if there are results only
	response.Data = phases
	if len(phases) > 0 {
		response.SetMeta(ctx, phases[len(phases)-1].Name.String(), len(phases), pageSize)
	}

	ctx.JSON(200, response)
}

// SetPremiumList godoc
// @Summary Set a premium list for a phase
// @Description Set a premium list for a phase. The premium list must exist.
// @Tags Phases
// @Produce json
// @Param tldName path string true "TLD name"
// @Param phaseName path string true "Phase name"
// @Param premiumListName path string true "Premium list name"
// @Success 200
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /tlds/{tldName}/phases/{phaseName}/premium-list/{premiumListName} [put]
func (ctrl *PhaseController) SetPremiumList(ctx *gin.Context) {
	phase, err := ctrl.phaseService.GetPhaseByTLDAndName(ctx, ctx.Param("tldName"), ctx.Param("phaseName"))
	if err != nil {
		if errors.Is(err, entities.ErrPhaseNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	plist := ctx.Param("premiumListName")
	phase.PremiumListName = &plist

	phase, err = ctrl.phaseService.UpdatePhase(ctx, phase)

	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, phase)
}

// UnSetPremiumList godoc
// @Summary Unset a premium list for a phase
// @Description Unset a premium list for a phase.
// @Tags Phases
// @Produce json
// @Param tldName path string true "TLD name"
// @Param phaseName path string true "Phase name"
// @Param premiumListName path string true "Premium list name"
// @Success 200
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /tlds/{tldName}/phases/{phaseName}/premium-list/{premiumListName} [delete]
func (ctrl *PhaseController) UnSetPremiumList(ctx *gin.Context) {
	phase, err := ctrl.phaseService.GetPhaseByTLDAndName(ctx, ctx.Param("tldName"), ctx.Param("phaseName"))
	if err != nil {
		if errors.Is(err, entities.ErrPhaseNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	phase.PremiumListName = nil

	phase, err = ctrl.phaseService.UpdatePhase(ctx, phase)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, phase)
}

// UpdatePhasePolicy godoc
// @Summary Update a phase's polkcy
// @Description Update a phase's policy.
// @Tags Phases
// @Accept json
// @Produce json
// @Param tldName path string true "TLD name"
// @Param phaseName path string true "Phase name"
// @Param phase body commands.UpdatePhasePolicyCommand true "Phase to update"
// @Success 200 {object} entities.Phase
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /tlds/{tldName}/phases/{phaseName}/policy [put]
func (ctrl *PhaseController) UpdatePhasePolicy(ctx *gin.Context) {
	// Bind the request body to the command
	var cmd commands.UpdatePhasePolicyCommand
	if err := ctx.ShouldBindJSON(&cmd); err != nil {
		if err.Error() == "EOF" {
			ctx.JSON(400, gin.H{"error": "missing request body"})
			return
		}
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// get the phase
	phase, err := ctrl.phaseService.GetPhaseByTLDAndName(ctx, ctx.Param("tldName"), ctx.Param("phaseName"))
	if err != nil {
		if errors.Is(err, entities.ErrPhaseNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Check if we are allowed to update
	if _, err := phase.CanUpdate(); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// update the policy
	if cmd.Policy != nil {
		phase.Policy.UpdatePolicy(cmd.Policy)
	}

	// Update the phase
	updatedPhase, err := ctrl.phaseService.UpdatePhase(ctx, phase)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, updatedPhase)
}
