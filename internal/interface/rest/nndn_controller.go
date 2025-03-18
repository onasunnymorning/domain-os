package rest

import (
	"errors"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/request"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

type NNDNController struct {
	nndnService interfaces.NNDNService
}

func NewNNDNController(e *gin.Engine, nndnService interfaces.NNDNService, handler gin.HandlerFunc) *NNDNController {
	controller := &NNDNController{
		nndnService: nndnService,
	}

	nndnRouter := e.Group("/nndns", handler)
	{
		nndnRouter.GET(":name", controller.GetNNDNByName)
		nndnRouter.GET("", controller.ListNNDNs)
		nndnRouter.POST("", controller.CreateNNDN)
		nndnRouter.DELETE(":name", controller.DeleteNNDNByName)
	}

	return controller
}

// GetNNDNByName godoc
// @Summary Get NNDN by name
// @Description Get NNDN by name
// @Tags NNDNs
// @Produce json
// @Param name path string true "NNDN name"
// @Success 200 {object} entities.NNDN
// @Failure 404
// @Failure 500
// @Router /nndns/{name} [get]
func (ctrl *NNDNController) GetNNDNByName(ctx *gin.Context) {
	name := ctx.Param("name")

	nndn, err := ctrl.nndnService.GetNNDNByName(ctx, name)
	if err != nil {
		if errors.Is(err, entities.ErrNNDNNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, nndn)
}

// Count godoc
// @Summary      Count NNDNs
// @Description  Applies filtering options to count NNDNs.
// @Tags         nndn
// @Accept       json
// @Produce      json
// @Param        filter  query     string  false  "Filter options for NNDNs"
// @Success      200     {object}  response.CountResult "Count of NNDNs"
// @Failure      400     {object}  gin.H "Error message when client fails to provide the correct filter"
// @Failure      500     {object}  gin.H "Error message when server fails to process the count"
// @Router       /nndn/count [get]
func (ctrl *NNDNController) Count(ctx *gin.Context) {
	result := response.CountResult{}

	filter, err := getListNndnsFilterFromContext(ctx)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	result.Filter = filter

	result.Count, err = ctrl.nndnService.Count(ctx, filter)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, result)
}

// ListNNDNs godoc
// @Summary List NNDNs
// @Description List NNDNs
// @Tags NNDNs
// @Produce json
// @Param cursor query string false "Cursor"
// @Param page_size query int false "Page size"
// @Success 200 {object} response.ListItemResult
// @Failure 500
// @Router /nndns [get]
func (ctrl *NNDNController) ListNNDNs(ctx *gin.Context) {
	query := queries.ListItemsQuery{}
	resp := response.ListItemResult{}

	filter, err := getListNndnsFilterFromContext(ctx)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	query.Filter = filter

	query.PageSize, err = GetPageSize(ctx)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	query.PageCursor, err = GetAndDecodeCursor(ctx)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	nndns, cursor, err := ctrl.nndnService.ListNNDNs(ctx, query)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	resp.Data = nndns
	resp.SetMeta(ctx, cursor, len(nndns), query.PageSize, query.Filter)

	ctx.JSON(200, resp)
}

// DeleteNNDNByName godoc
// @Summary Delete NNDN by name
// @Description Delete NNDN by name
// @Tags NNDNs
// @Produce json
// @Param name path string true "NNDN name"
// @Success 204
// @Failure 500
// @Router /nndns/{name} [delete]
func (ctrl *NNDNController) DeleteNNDNByName(ctx *gin.Context) {
	name := ctx.Param("name")

	err := ctrl.nndnService.DeleteNNDNByName(ctx, name)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(204, nil)
	e, ok := ctx.Get("event")

	if !ok {
		log.Println("Event not found in context")
	}
	event := e.(*entities.Event)
	event.Details.Result = entities.EventResultSuccess
	event.Action = entities.EventTypeDelete
	event.ObjectType = entities.ObjectTypeNNDN
	ctx.Set("event", event)
}

// CreateNNDN godoc
// @Summary Create NNDN
// @Description Create NNDN
// @Tags NNDNs
// @Accept json
// @Produce json
// @Param body body request.CreateNNDNRequest true "NNDN"
// @Success 201 {object} entities.NNDN
// @Failure 400
// @Failure 500
// @Router /nndns [post]
func (ctrl *NNDNController) CreateNNDN(ctx *gin.Context) {
	var req request.CreateNNDNRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		if err.Error() == "EOF" {
			ctx.JSON(400, gin.H{"error": "missing request body"})
			return
		}
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	cmd, err := req.ToCreateNNDNCommand()
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result, err := ctrl.nndnService.CreateNNDN(ctx, cmd)
	if err != nil {
		if errors.Is(err, entities.ErrInvalidNNDN) {
			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(201, result)
}

func getListNndnsFilterFromContext(ctx *gin.Context) (queries.ListNndnsFilter, error) {
	filter := queries.ListNndnsFilter{}
	filter.NameLike = ctx.Query("name_like")
	filter.ReasonLike = ctx.Query("reason_like")
	filter.ReasonEquals = ctx.Query("reason_equals")
	filter.TldEquals = ctx.Query("tld_equals")

	return filter, nil
}
