package rest

import (
	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
)

// PhaseController is the controller for the Phase entity
type PhaseController struct {
	phaseService interfaces.PhaseService
}

// NewPhaseController returns a new instance of PhaseController
func NewPhaseController(e *gin.Engine, phaseService interfaces.PhaseService) *PhaseController {
	ctrl := &PhaseController{
		phaseService: phaseService,
	}

	e.POST("/tlds/:tldName/phases", ctrl.CreatePhase)
	e.GET("/tlds/:tldName/phases", ctrl.ListPhases)
	e.GET("/tlds/:tldName/phases/active", ctrl.ListActivePhases)
	e.GET("/tlds/:tldName/phases/:phaseName", ctrl.GetPhase)
	e.DELETE("/tlds/:tldName/phases/:phaseName", ctrl.DeletePhase)

	return ctrl
}

// CreatePhase godoc
// @Summary Create a new phase
// @Description Create a new phase
// @Tags Phases
// @Accept json
// @Produce json
// @Param phase body commands.CreatePhaseCommand true "Phase to create"
// @Param tldName path string true "TLD name"
// @Success 200 {object} entities.Phase
// @Failure 400
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
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, phase)

}

// GetPhase godoc
// @Summary Get a phase by name and tld name
// @Description Get a phase by name and tld name
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
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, phase)
}

// DeletePhase godoc
// @Summary Delete a phase by name and tld name
// @Description Delete a phase by name and tld name
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
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(204, nil)
}

// ListPhases godoc
// @Summary List all phases for a TLD
// @Description List all phases for a TLD
// @Tags Phases
// @Produce json
// @Param tldName path string true "TLD name"
// @Success 200 {array} entities.Phase
// @Failure 500
// @Router /tlds/{tldName}/phases [get]
func (ctrl *PhaseController) ListPhases(ctx *gin.Context) {
	phases, err := ctrl.phaseService.ListPhasesByTLD(ctx, ctx.Param("tldName"))
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, phases)
}

// ListActivePhases godoc
// @Summary List all active phases for a TLD
// @Description List all active phases for a TLD
// @Tags Phases
// @Produce json
// @Param tldName path string true "TLD name"
// @Success 200 {array} entities.Phase
// @Failure 500
// @Router /tlds/{tldName}/phases/active [get]
func (ctrl *PhaseController) ListActivePhases(ctx *gin.Context) {
	phases, err := ctrl.phaseService.ListActivePhasesByTLD(ctx, ctx.Param("tldName"))
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, phases)
}
