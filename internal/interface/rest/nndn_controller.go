package rest

import (
	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/request"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

type NNDNController struct {
	nndnService interfaces.NNDNService
}

func NewNNDNController(e *gin.Engine, nndnService interfaces.NNDNService) *NNDNController {
	controller := &NNDNController{
		nndnService: nndnService,
	}

	e.GET("/nndns/:name", controller.GetNNDNByName)
	e.GET("/nndns", controller.ListNNDNs)
	e.POST("/nndns", controller.CreateNNDN)
	e.DELETE("/nndns/:name", controller.DeleteNNDNByName)

	return controller
}

func (ctrl *NNDNController) GetNNDNByName(ctx *gin.Context) {
	name := ctx.Param("name")

	nndn, err := ctrl.nndnService.GetNNDNByName(ctx, name)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, nndn)
}

func (ctrl *NNDNController) ListNNDNs(ctx *gin.Context) {
	resp := response.ListItemResult{}

	pageSize, err := GetPageSize(ctx)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	pageCursor, err := GetAndDecodeCursor(ctx)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	nndns, err := ctrl.nndnService.ListNNDNs(ctx, pageSize, pageCursor)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	resp.Data = nndns
	if len(nndns) > 0 {
		resp.SetMeta(ctx, nndns[len(nndns)-1].Name.String(), len(nndns), pageSize)
	}

	ctx.JSON(200, resp)
}

func (ctrl *NNDNController) DeleteNNDNByName(ctx *gin.Context) {
	name := ctx.Param("name")

	err := ctrl.nndnService.DeleteNNDNByName(ctx, name)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(204, nil)
}

func (ctrl *NNDNController) CreateNNDN(ctx *gin.Context) {
	var req request.CreateNNDNRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
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
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(201, result)
}
