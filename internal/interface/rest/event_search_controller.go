package rest

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// EventSearchController handles the unified event search endpoint.
type EventSearchController struct {
	searchService *services.EventSearchService
}

// NewEventSearchController creates and registers event search routes.
// The search endpoint lives under /events/search alongside the existing /events
// endpoint for backward compatibility.
func NewEventSearchController(e *gin.Engine, searchService *services.EventSearchService, handler gin.HandlerFunc) *EventSearchController {
	controller := &EventSearchController{
		searchService: searchService,
	}

	eventsGroup := e.Group("/events", handler)
	{
		eventsGroup.GET("search", controller.SearchEvents)
	}

	return controller
}

// SearchEvents godoc
// @Summary Search events with filters
// @Description Search domain events with filtering by subject, type, source, actor, ROID, and date range. Supports cursor-based pagination across hot (PostgreSQL) and warm (S3 archive) storage tiers.
// @Tags events
// @Produce json
// @Param subject query string false "Filter by subject (exact match, e.g. domain name)"
// @Param type query string false "Filter by event type (exact match or prefix with * suffix, e.g. 'domain.*')"
// @Param source query string false "Filter by event source (exact match)"
// @Param actor query string false "Filter by actor (exact match)"
// @Param roid query string false "Filter by ROID (exact match)"
// @Param trace_id query string false "Filter by Trace ID (exact match)"
// @Param correlation_id query string false "Filter by Correlation ID (exact match)"
// @Param after query string false "Filter events after this time (ISO 8601, inclusive)"
// @Param before query string false "Filter events before this time (ISO 8601, exclusive)"
// @Param limit query int false "Maximum number of events per page (default 50, max 200)"
// @Param cursor query string false "Opaque cursor for pagination (from previous response's nextCursor)"
// @Success 200 {object} entities.EventSearchResult
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /events/search [get]
func (ctrl *EventSearchController) SearchEvents(ctx *gin.Context) {
	filter, err := parseSearchFilter(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := ctrl.searchService.Search(ctx.Request.Context(), filter)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "event search failed: " + err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, result)
}

// parseSearchFilter extracts and validates search filter parameters from the
// query string.
func parseSearchFilter(ctx *gin.Context) (entities.EventSearchFilter, error) {
	filter := entities.EventSearchFilter{
		Subject:       ctx.Query("subject"),
		Type:          ctx.Query("type"),
		Source:        ctx.Query("source"),
		Actor:         ctx.Query("actor"),
		RoID:          ctx.Query("roid"),
		TraceID:       ctx.Query("trace_id"),
		CorrelationID: ctx.Query("correlation_id"),
		Cursor:        ctx.Query("cursor"),
	}

	// Parse limit
	if l := ctx.Query("limit"); l != "" {
		parsed, err := strconv.Atoi(l)
		if err != nil || parsed < 1 {
			return filter, fmt.Errorf("invalid limit parameter: %q — must be a positive integer", l)
		}
		filter.Limit = parsed
	}

	// Parse after (ISO 8601)
	if after := ctx.Query("after"); after != "" {
		t, err := time.Parse(time.RFC3339, after)
		if err != nil {
			return filter, fmt.Errorf("invalid after parameter: %q — must be ISO 8601 format (e.g. 2026-01-15T00:00:00Z)", after)
		}
		filter.After = &t
	}

	// Parse before (ISO 8601)
	if before := ctx.Query("before"); before != "" {
		t, err := time.Parse(time.RFC3339, before)
		if err != nil {
			return filter, fmt.Errorf("invalid before parameter: %q — must be ISO 8601 format (e.g. 2026-06-30T23:59:59Z)", before)
		}
		filter.Before = &t
	}

	return filter, nil
}
