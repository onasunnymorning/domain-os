package rest

import (
	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/request"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

type TLDController struct {
	tldService interfaces.TLDService
}

func NewTLDController(e *gin.Engine, tldService interfaces.TLDService) *TLDController {
	controller := &TLDController{
		tldService: tldService,
	}

	e.GET("/tlds/:name", controller.GetTLDByName)
	e.GET("/tlds", controller.ListTLDs)
	e.POST("/tlds", controller.CreateTLD)
	e.DELETE("/tlds/:name", controller.DeleteTLDByName)

	return controller
}

func (ctrl *TLDController) GetTLDByName(ctx *gin.Context) {
	name := ctx.Param("name")

	tld, err := ctrl.tldService.GetTLDByName(name)
	// TODO: If the TLD does not exist, return a 404
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, tld)
}

func (ctrl *TLDController) ListTLDs(ctx *gin.Context) {
	var err error
	// Prepare the response
	response := response.ListItemResult{}
	// Get the pagesize from the query string
	pageSize, err := GetPageSize(ctx)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// Get the cursor from the query string
	pageCursor, err := GetAndDecodeCursor(ctx)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Get the tlds from the service
	tlds, err := ctrl.tldService.ListTLDs(pageSize, pageCursor)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}
	// Populate the response.Data with the tlds
	response.Data = tlds

	// Set the metadata
	response.SetMeta(ctx, tlds[len(tlds)-1].Name.String(), len(tlds), pageSize)

	// Return the response
	ctx.JSON(200, response)
}

func (ctrl *TLDController) DeleteTLDByName(ctx *gin.Context) {
	name := ctx.Param("name")

	err := ctrl.tldService.DeleteTLDByName(name)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(204, nil)
}

func (ctrl *TLDController) CreateTLD(ctx *gin.Context) {
	var req request.CreateTLDRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	cmd, err := req.ToCreateTLDCommand()
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result, err := ctrl.tldService.CreateTLD(cmd)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(201, result)
}
