package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/services"
)

type DnssecController struct {
	Service *services.DnssecService
}

func NewDnssecController(r *gin.Engine, s *services.DnssecService, authMiddleware gin.HandlerFunc) {
	c := &DnssecController{Service: s}

	// Create a new router group for the dnssec endpoint
	api := r.Group("/api/v1", authMiddleware)
	{
		api.GET("/dnssec", c.Visualize)
	}
}

// Visualize godoc
// @Summary Visualize DNSSEC for a domain
// @Description Runs dnsviz to generate an interactive map of DNSSEC trust chains for the provided domain
// @Tags DNSSEC
// @Accept json
// @Produce json
// @Param domain query string true "Domain Name"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/dnssec [get]
func (c *DnssecController) Visualize(ctx *gin.Context) {
	domain := ctx.Query("domain")
	if domain == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "domain parameter is required"})
		return
	}

	result, err := c.Service.Visualize(ctx.Request.Context(), domain)
	if err != nil {
		if err.Error() == "invalid domain format" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, result)
}
