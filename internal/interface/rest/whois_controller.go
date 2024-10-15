package rest

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// WhoisController is the controller for the whois service
type WhoisController struct {
	whoisService interfaces.WhoisService
}

// NewWhoisController creates a new instance of WhoisController
func NewWhoisController(e *gin.Engine, whoisService interfaces.WhoisService, handler gin.HandlerFunc) *WhoisController {
	ctrl := &WhoisController{
		whoisService: whoisService,
	}

	e.GET("/whois/:domainName", handler, ctrl.GetWhois)

	return ctrl
}

// GetWhois godoc
// @Summary Get the whois information of a domain
// @Description Get the whois information of a domain
// @Tags Whois
// @Produce json
// @Param domainName path string true "Domain Name"
// @Success 200 {object} entities.WhoisResponse
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /whois/{domainName} [get]
func (ctrl *WhoisController) GetWhois(ctx *gin.Context) {
	domainName := ctx.Param("domainName")

	whois, err := ctrl.whoisService.GetDomainWhois(ctx, domainName)
	if err != nil {
		if errors.Is(err, entities.ErrDomainNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, whois)
}
