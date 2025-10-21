package rest

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/services"
)

// DNSController handles DNS monitoring and management endpoints
type DNSController struct {
	dnsService *services.DNSService
}

// NewDNSController creates a new DNS controller
func NewDNSController(e *gin.Engine, dnsService *services.DNSService, handler gin.HandlerFunc) *DNSController {
	c := &DNSController{
		dnsService: dnsService,
	}

	dnsGroup := e.Group("/api/v1/dns", handler)
	{
		// Queue monitoring
		queueGroup := dnsGroup.Group("/queue")
		{
			queueGroup.GET("/stats", c.GetQueueStats)
			queueGroup.GET("/stats/:zone", c.GetQueueStatsForZone)
			queueGroup.GET("/pending", c.GetPendingChanges)
			queueGroup.GET("/errors", c.GetErroredChanges)
		}

		// Health check
		dnsGroup.GET("/health", c.GetHealth)
	}

	return c
}

// GetQueueStats godoc
// @Summary Get DNS queue statistics for all zones
// @Description Returns statistics about pending, published, and errored DNS changes across all zones
// @Tags DNS
// @Produce json
// @Success 200 {object} services.DNSQueueStatsResponse
// @Failure 500 {object} gin.H
// @Router /api/v1/dns/queue/stats [get]
func (ctrl *DNSController) GetQueueStats(ctx *gin.Context) {
	stats, err := ctrl.dnsService.GetQueueStats(ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, stats)
}

// GetQueueStatsForZone godoc
// @Summary Get DNS queue statistics for a specific zone
// @Description Returns statistics about pending, published, and errored DNS changes for a specific zone
// @Tags DNS
// @Produce json
// @Param zone path string true "Zone name (e.g., 'tld.')"
// @Success 200 {object} dnsevents.QueueStats
// @Failure 500 {object} gin.H
// @Router /api/v1/dns/queue/stats/{zone} [get]
func (ctrl *DNSController) GetQueueStatsForZone(ctx *gin.Context) {
	zone := ctx.Param("zone")

	stats, err := ctrl.dnsService.GetQueueStatsForZone(ctx, zone)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, stats)
}

// GetPendingChanges godoc
// @Summary Get pending DNS changes
// @Description Returns a list of DNS changes that are queued but not yet published
// @Tags DNS
// @Produce json
// @Param zone query string false "Filter by zone name"
// @Param domain query string false "Filter by domain name"
// @Param change_type query string false "Filter by change type (ADD or DELETE)"
// @Param record_type query string false "Filter by record type (NS, A, AAAA)"
// @Param limit query int false "Maximum number of results (default: 100)"
// @Param offset query int false "Pagination offset (default: 0)"
// @Success 200 {object} services.DNSPendingChangesResponse
// @Failure 400 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /api/v1/dns/queue/pending [get]
func (ctrl *DNSController) GetPendingChanges(ctx *gin.Context) {
	zone := ctx.Query("zone")
	domain := ctx.Query("domain")
	changeType := ctx.Query("change_type")
	recordType := ctx.Query("record_type")

	// Parse pagination parameters
	limit := 100 // default
	if limitStr := ctx.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	offset := 0 // default
	if offsetStr := ctx.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	changes, err := ctrl.dnsService.GetPendingChanges(ctx, zone, domain, changeType, recordType, limit, offset)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, changes)
}

// GetErroredChanges godoc
// @Summary Get DNS changes with errors
// @Description Returns a list of DNS changes that have failed to publish
// @Tags DNS
// @Produce json
// @Param zone query string false "Filter by zone name"
// @Param limit query int false "Maximum number of results (default: 50)"
// @Param offset query int false "Pagination offset (default: 0)"
// @Success 200 {object} services.DNSErroredChangesResponse
// @Failure 400 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /api/v1/dns/queue/errors [get]
func (ctrl *DNSController) GetErroredChanges(ctx *gin.Context) {
	zone := ctx.Query("zone")

	// Parse pagination parameters
	limit := 50 // default
	if limitStr := ctx.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	offset := 0 // default
	if offsetStr := ctx.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	errors, err := ctrl.dnsService.GetErroredChanges(ctx, zone, limit, offset)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, errors)
}

// GetHealth godoc
// @Summary Get DNS system health status
// @Description Returns health check information for the DNS batch publishing system
// @Tags DNS
// @Produce json
// @Success 200 {object} services.DNSHealthResponse
// @Failure 500 {object} gin.H
// @Router /api/v1/dns/health [get]
func (ctrl *DNSController) GetHealth(ctx *gin.Context) {
	health, err := ctrl.dnsService.GetHealth(ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Set appropriate HTTP status based on health
	statusCode := http.StatusOK
	if health.Status == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	} else if health.Status == "degraded" {
		statusCode = http.StatusOK // 200 but with issues noted
	}

	ctx.JSON(statusCode, health)
}
