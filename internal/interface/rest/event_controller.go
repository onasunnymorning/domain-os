package rest

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
)

// EventController handles global event endpoints
type EventController struct {
	domainService interfaces.DomainService
}

// NewEventController creates and registers event routes
func NewEventController(e *gin.Engine, domService interfaces.DomainService, handler gin.HandlerFunc) *EventController {
	controller := &EventController{
		domainService: domService,
	}

	eventsGroup := e.Group("/events", handler)
	{
		eventsGroup.GET("", controller.ListRecentEvents)
	}

	return controller
}

// ListRecentEvents godoc
// @Summary List recent events
// @Description List the most recent domain events across all entities
// @Tags events
// @Produce json
// @Param limit query int false "Maximum number of events to return (default 20, max 100)"
// @Success 200 {array} entities.DomainEvent
// @Failure 500 {object} map[string]string
// @Router /events [get]
func (ctrl *EventController) ListRecentEvents(ctx *gin.Context) {
	limit := 20
	if l := ctx.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	events, err := ctrl.domainService.ListRecentEvents(ctx.Request.Context(), limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to list recent events: " + err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, events)
}
