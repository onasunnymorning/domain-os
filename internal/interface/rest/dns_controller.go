package rest

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/services"
)

// DNSController is the controller for the DNS REST API
type DNSController struct {
	tldService *services.TLDService
	domService *services.DomainService
}

// NewDNSController creates a new DNSController
func NewDNSController(e *gin.Engine, ts *services.TLDService, dnss *services.DomainService) *DNSController {
	ctrl := &DNSController{
		tldService: ts,
		domService: dnss,
	}
	e.GET("/dns/:tld/domains/delegations", ctrl.GetNSRecordsPerTLD)
	e.GET("/dns/:tld/domains/glue", ctrl.GetGlueRecordsPerTLD)
	return ctrl
}

// GetNSRecordsPerTLD godoc
// @Summary Get NS records for a TLD
// @Description Get NS records for a TLD in JSON format (default) or text format
// @Tags DNS
// @Produce json
// @Param tld path string true "TLD"
// @Param format query string false "Output format"
// @Success 200 {array} dns.RR
// @Failure 404
// @Failure 500
// @Router /dns/{tld}/ns [get]
func (c *DNSController) GetNSRecordsPerTLD(ctx *gin.Context) {
	// Check if the TLD exists
	tldName := ctx.Param("tld")
	_, err := c.tldService.GetTLDByName(ctx, tldName, false)
	if err != nil {
		ctx.JSON(404, gin.H{"error": "TLD not found"})
		return
	}

	rrs, err := c.domService.GetNSRecordsPerTLD(ctx, tldName)

	if err != nil {
		ctx.JSON(500, gin.H{"error": "Error getting NS records"})
		return
	}

	if format := ctx.Query("format"); format == "text" {
		var stringResponse string
		for _, rr := range rrs {
			stringResponse += rr.String() + "\n"
		}

		ctx.String(200, "%s", stringResponse)
		return
	}

	ctx.JSON(200, rrs)
}

// GetGlueRecordsPerTLD godoc
// @Summary Get Glue records for a TLD
// @Description Get Glue records for a TLD
// @Tags DNS
// @Produce json
// @Param tld path string true "TLD"
// @Param format query string false "Output format"
// @Success 200 {array} dns.RR
// @Failure 404
// @Failure 500
// @Router /dns/{tld}/glue [get]
func (c *DNSController) GetGlueRecordsPerTLD(ctx *gin.Context) {
	// Check if the TLD exists
	tldName := ctx.Param("tld")
	_, err := c.tldService.GetTLDByName(ctx, tldName, false)
	if err != nil {
		ctx.JSON(404, gin.H{"error": "TLD not found"})
		return
	}

	rrs, err := c.domService.GetGlueRecordsPerTLD(ctx, tldName)

	if err != nil {
		ctx.JSON(500, gin.H{"error": fmt.Sprintf("Error getting Glue records: %s", err.Error())})
		return
	}

	if format := ctx.Query("format"); format == "text" {
		var stringResponse string
		for _, rr := range rrs {
			stringResponse += rr.String() + "\n"
		}

		ctx.String(200, "%s", stringResponse)
		return
	}

	ctx.JSON(200, rrs)
}
