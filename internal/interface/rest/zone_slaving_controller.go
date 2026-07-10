package rest

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

type zoneSlavingController struct {
	svc interfaces.ZoneSlavingService
}

// NewZoneSlavingController registers zone slaving routes on the given engine.
func NewZoneSlavingController(r *gin.Engine, svc interfaces.ZoneSlavingService, authMW gin.HandlerFunc) {
	ctrl := &zoneSlavingController{svc: svc}

	g := r.Group("/zone-slavings", authMW)
	{
		g.POST("", ctrl.CreateSlaving)
		g.GET("", ctrl.ListActiveSlavings)
		g.GET("/:id", ctrl.GetSlaving)
		g.PATCH("/:id", ctrl.UpdateSlavingStatus)
		g.GET("/:id/confidence", ctrl.GetConfidenceRollup)
		g.GET("/:id/observations", ctrl.ListObservations)
	}
}

// getTenantID extracts the tenant ID from the X-Tenant-ID header or query param.
func getTenantID(ctx *gin.Context) string {
	tenantID := ctx.GetHeader("X-Tenant-ID")
	if tenantID == "" {
		tenantID = ctx.Query("tenant_id")
	}
	return tenantID
}

// parseSlavingID parses the :id path param as a UUID.
func parseSlavingID(ctx *gin.Context) (uuid.UUID, error) {
	idStr := ctx.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// CreateSlaving godoc
// @Summary Create zone slaving monitor
// @Description Create a new zone slaving monitor and start its Temporal schedule
// @Tags ZoneSlavings
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "Tenant ID (RegistryOperator RyID)"
// @Param body body interfaces.CreateSlavingRequest true "Slaving configuration"
// @Success 201 {object} entities.ZoneSlaving
// @Failure 400
// @Failure 500
// @Router /zone-slavings [post]
func (ctrl *zoneSlavingController) CreateSlaving(ctx *gin.Context) {
	tenantID := getTenantID(ctx)
	if tenantID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID header or tenant_id query param is required"})
		return
	}

	var req interfaces.CreateSlavingRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	zs, err := ctrl.svc.CreateSlaving(ctx.Request.Context(), tenantID, req)
	if err != nil {
		if errors.Is(err, entities.ErrInvalidZoneSlaving) {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create zone slaving monitor: " + err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, zs)
}

// GetSlaving godoc
// @Summary Get zone slaving monitor
// @Description Get a zone slaving monitor by ID
// @Tags ZoneSlavings
// @Produce json
// @Param X-Tenant-ID header string true "Tenant ID"
// @Param id path string true "Slaving ID (UUID)"
// @Success 200 {object} entities.ZoneSlaving
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /zone-slavings/{id} [get]
func (ctrl *zoneSlavingController) GetSlaving(ctx *gin.Context) {
	tenantID := getTenantID(ctx)
	if tenantID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID header or tenant_id query param is required"})
		return
	}

	id, err := parseSlavingID(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid slaving ID: " + err.Error()})
		return
	}

	zs, err := ctrl.svc.GetSlaving(ctx.Request.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, entities.ErrZoneSlavingNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get zone slaving monitor: " + err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, zs)
}

// updateStatusRequest is the request body for PATCH /zone-slavings/:id
type updateStatusRequest struct {
	Action string `json:"action" binding:"required,oneof=complete abandon"`
}

// UpdateSlavingStatus godoc
// @Summary Update zone slaving monitor status
// @Description Complete or abandon a zone slaving monitor
// @Tags ZoneSlavings
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "Tenant ID"
// @Param id path string true "Slaving ID (UUID)"
// @Param body body updateStatusRequest true "Action: complete or abandon"
// @Success 200
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /zone-slavings/{id} [patch]
func (ctrl *zoneSlavingController) UpdateSlavingStatus(ctx *gin.Context) {
	tenantID := getTenantID(ctx)
	if tenantID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID header or tenant_id query param is required"})
		return
	}

	id, err := parseSlavingID(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid slaving ID: " + err.Error()})
		return
	}

	var req updateStatusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	switch req.Action {
	case "complete":
		err = ctrl.svc.CompleteSlaving(ctx.Request.Context(), tenantID, id)
	case "abandon":
		err = ctrl.svc.AbandonSlaving(ctx.Request.Context(), tenantID, id)
	}

	if err != nil {
		if errors.Is(err, entities.ErrZoneSlavingNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update zone slaving monitor: " + err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ListActiveSlavings godoc
// @Summary List active zone slaving monitors
// @Description List all active zone slaving monitors for a tenant
// @Tags ZoneSlavings
// @Produce json
// @Param X-Tenant-ID header string true "Tenant ID"
// @Success 200 {array} entities.ZoneSlaving
// @Failure 400
// @Failure 500
// @Router /zone-slavings [get]
func (ctrl *zoneSlavingController) ListActiveSlavings(ctx *gin.Context) {
	tenantID := getTenantID(ctx)
	if tenantID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID header or tenant_id query param is required"})
		return
	}

	slavings, err := ctrl.svc.ListActiveSlavings(ctx.Request.Context(), tenantID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list zone slaving monitors: " + err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, slavings)
}

// GetConfidenceRollup godoc
// @Summary Get slaving confidence rollup
// @Description Get the current migration confidence state for a zone slaving monitor
// @Tags ZoneSlavings
// @Produce json
// @Param X-Tenant-ID header string true "Tenant ID"
// @Param id path string true "Slaving ID (UUID)"
// @Success 200 {object} entities.SlavingConfidenceRollup
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /zone-slavings/{id}/confidence [get]
func (ctrl *zoneSlavingController) GetConfidenceRollup(ctx *gin.Context) {
	tenantID := getTenantID(ctx)
	if tenantID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID header or tenant_id query param is required"})
		return
	}

	id, err := parseSlavingID(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid slaving ID: " + err.Error()})
		return
	}

	rollup, err := ctrl.svc.GetConfidenceRollup(ctx.Request.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, entities.ErrZoneSlavingNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get confidence rollup: " + err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, rollup)
}

// ListObservations godoc
// @Summary List observation history
// @Description List serial observation history with cursor-based pagination
// @Tags ZoneSlavings
// @Produce json
// @Param X-Tenant-ID header string true "Tenant ID"
// @Param id path string true "Slaving ID (UUID)"
// @Param cursor query string false "Pagination cursor"
// @Param pagesize query int false "Page size (default 25, max 200)"
// @Success 200 {object} map[string]interface{}
// @Failure 400
// @Failure 500
// @Router /zone-slavings/{id}/observations [get]
func (ctrl *zoneSlavingController) ListObservations(ctx *gin.Context) {
	tenantID := getTenantID(ctx)
	if tenantID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID header or tenant_id query param is required"})
		return
	}

	id, err := parseSlavingID(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid slaving ID: " + err.Error()})
		return
	}

	pageSize, err := GetPageSize(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cursor, err := GetAndDecodeCursor(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	observations, nextCursor, err := ctrl.svc.ListObservationHistory(ctx.Request.Context(), tenantID, id, pageSize, cursor)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list observations: " + err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data":       observations,
		"nextCursor": nextCursor,
		"count":      len(observations),
	})
}
