package rest

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// DomainCheckController is the controller for doing domain checks
type DomainCheckController struct {
	DomainCheckService interfaces.DomainCheckService
}

// NewDomainCheckController creates a new DomainCheckController
func NewDomainCheckController(e *gin.Engine, svc interfaces.DomainCheckService) *DomainCheckController {
	ctrl := &DomainCheckController{
		DomainCheckService: svc,
	}

	e.GET("/domain/check/:name", ctrl.CheckDomain)

	return ctrl
}

// CheckDomain godoc
// @Summary Check if a domain is available
// @Description Check if a domain is available
// @Tags Domain
// @Produce json
// @Param name path string true "Domain Name"
// @Param includeFees query bool false "Include fees in the response"
// @Success 200 {object} queries.DomainCheckResult
// @Failure 400
// @Failure 500
// @Router /domain/check/{name} [get]
func (ctrl *DomainCheckController) CheckDomain(ctx *gin.Context) {
	name := ctx.Param("name")
	includeFees := ctx.DefaultQuery("includeFees", "false")

	// Create a query object
	q, err := queries.NewDomainCheckQuery(name, includeFees == "true")
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Set the phase name if it was provided
	q.PhaseName = ctx.Query("phase")
	q.Currency = ctx.Query("currency")

	// Call the service to check the domain
	result, err := ctrl.DomainCheckService.CheckDomain(ctx, q)
	if err != nil {
		if errors.Is(err, entities.ErrTLDNotFound) || errors.Is(err, entities.ErrPhaseNotFound) || errors.Is(err, entities.ErrNoActivePhase) {
			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, result)
}
