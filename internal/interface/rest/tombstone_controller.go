package rest

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

type tombstoneController struct {
	svc interfaces.TombstoneService
}

// NewTombstoneController registers tombstone routes on the given engine.
func NewTombstoneController(r *gin.Engine, svc interfaces.TombstoneService, authMW gin.HandlerFunc) {
	ctrl := &tombstoneController{svc: svc}

	g := r.Group("/tombstones", authMW)
	{
		g.GET("/count", ctrl.CountTombstones)
		g.GET("/by-name/:name", ctrl.GetTombstonesByName)
		g.GET("/:roid", ctrl.GetTombstoneByRoID)
		g.GET("", ctrl.ListTombstones)
	}
}

// GetTombstoneByRoID godoc
// @Summary Get tombstone by ROID
// @Description Get a single domain tombstone by its ROID
// @Tags Tombstones
// @Produce json
// @Param roid path string true "Tombstone ROID"
// @Success 200 {object} entities.DomainTombstone
// @Failure 404
// @Failure 500
// @Router /tombstones/{roid} [get]
func (ctrl *tombstoneController) GetTombstoneByRoID(ctx *gin.Context) {
	roid := ctx.Param("roid")

	tombstone, err := ctrl.svc.GetTombstoneByRoID(ctx.Request.Context(), roid)
	if err != nil {
		if errors.Is(err, entities.ErrTombstoneNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, tombstone)
}

// GetTombstonesByName godoc
// @Summary Get tombstones by domain name
// @Description Get all tombstone incarnations for a given domain name
// @Tags Tombstones
// @Produce json
// @Param name path string true "Domain name"
// @Success 200 {array} entities.DomainTombstone
// @Failure 500
// @Router /tombstones/by-name/{name} [get]
func (ctrl *tombstoneController) GetTombstonesByName(ctx *gin.Context) {
	name := ctx.Param("name")

	tombstones, err := ctrl.svc.GetTombstonesByName(ctx.Request.Context(), name)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, tombstones)
}

// ListTombstones godoc
// @Summary List tombstones
// @Description List domain tombstones with cursor-based pagination and optional filters
// @Tags Tombstones
// @Produce json
// @Param cursor query string false "Cursor"
// @Param pagesize query int false "Page size"
// @Param name query string false "Name equals"
// @Param name_like query string false "Name like"
// @Param tld query string false "TLD equals"
// @Param registrar query string false "Registrar client ID"
// @Param purge_reason query string false "Purge reason"
// @Success 200 {object} response.ListItemResult
// @Failure 400
// @Failure 500
// @Router /tombstones [get]
func (ctrl *tombstoneController) ListTombstones(ctx *gin.Context) {
	query := queries.ListItemsQuery{}
	resp := response.ListItemResult{}

	filter := getListTombstonesFilterFromContext(ctx)
	query.Filter = filter

	var err error
	query.PageSize, err = GetPageSize(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	query.PageCursor, err = GetAndDecodeCursor(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tombstones, cursor, err := ctrl.svc.ListTombstones(ctx.Request.Context(), query)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp.Data = tombstones
	resp.SetMeta(ctx, cursor, len(tombstones), query.PageSize, query.Filter)

	ctx.JSON(http.StatusOK, resp)
}

// CountTombstones godoc
// @Summary Count tombstones
// @Description Count domain tombstones matching the given filters
// @Tags Tombstones
// @Produce json
// @Param name query string false "Name equals"
// @Param name_like query string false "Name like"
// @Param tld query string false "TLD equals"
// @Param registrar query string false "Registrar client ID"
// @Param purge_reason query string false "Purge reason"
// @Success 200 {object} response.CountResult
// @Failure 500
// @Router /tombstones/count [get]
func (ctrl *tombstoneController) CountTombstones(ctx *gin.Context) {
	result := response.CountResult{}

	filter := getListTombstonesFilterFromContext(ctx)
	result.Filter = filter

	var err error
	result.Count, err = ctrl.svc.CountTombstones(ctx.Request.Context(), filter)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, result)
}

func getListTombstonesFilterFromContext(ctx *gin.Context) queries.ListTombstonesFilter {
	return queries.ListTombstonesFilter{
		NameEquals:    ctx.Query("name"),
		NameLike:      ctx.Query("name_like"),
		TLDEquals:     ctx.Query("tld"),
		RegistrarClID: ctx.Query("registrar"),
		PurgeReason:   ctx.Query("purge_reason"),
	}
}
