package rest

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

// DNSController is the controller for the DNS REST API
type DNSController struct {
	tldService *services.TLDService
}

// NewDNSController creates a new DNSController
func NewDNSController(e *gin.Engine, ts *services.TLDService) *DNSController {
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

	// send a dummy response for now
	response := response.NSRecordResponse{
		TLD:       tldName,
		NSRecords: []response.NSRecord{},
		Timestamp: time.Now().UTC(),
	}
	ctx.JSON(200, response)
}
