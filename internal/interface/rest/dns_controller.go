package rest

import (
	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/services"
)

// DNSController is the controller for the DNS REST API
type DNSController struct {
	tldService *services.TLDService
	dnsService *services.DNSService
}

// NewDNSController creates a new DNSController
func NewDNSController(e *gin.Engine, ts *services.TLDService, dnss *services.DNSService) *DNSController {
	ctrl := &DNSController{
		tldService: ts,
	}
	e.GET("/dns/:tld/ns", ctrl.GetNSRecordsPerTLD)
	return ctrl
}

// GetNSRecordsPerTLD godoc
// @Summary Get NS records for a TLD
// @Description Get NS records for a TLD
// @Tags DNS
// @Produce json
// @Param tld path string true "TLD"
// @Success 200 {object} response.NSRecordResponse
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

	// Create the response object
	// response := response.NSRecordResponse{
	// 	TLD:       tldName,
	// 	NSRecords: []response.NSRecord{},
	// 	Timestamp: time.Now().UTC(),
	// }

	// Get the NS records for the TLD from the service
	rrs, err := c.dnsService.GetNSRecordsPerTLD(tldName)

	if err != nil {
		ctx.JSON(500, gin.H{"error": "Error getting NS records"})
		return
	}

	ctx.JSON(200, rrs)
}
