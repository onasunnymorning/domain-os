package rest

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

// HostController is the controller for the HostService
type HostController struct {
	hostService interfaces.HostService
}

// NewHostController creates a new instance of HostController
func NewHostController(e *gin.Engine, hostService interfaces.HostService) *HostController {
	c := &HostController{
		hostService: hostService,
	}

	e.GET("/hosts/:roid", c.GetHostByRoID)
	e.GET("/hosts", c.ListHosts)
	e.DELETE("/hosts/:roid", c.DeleteHostByRoID)
	e.POST("/hosts", c.CreateHost)

	e.POST("/hosts/:roid/addresses/:ip", c.AddAddressToHost)
	e.DELETE("/hosts/:roid/addresses/:ip", c.RemoveAddressFromHost)

	return c
}

// GetHostByRoID godoc
// @Summary Get a host by its RoID
// @Description Get a host by its RoID
// @Tags Hosts
// @Produce json
// @Param roid path int true "RoID"
// @Success 200 {object} entities.Host
// @Failure 404
// @Failure 500
// @Router /hosts/{roid} [get]
func (ctrl *HostController) GetHostByRoID(ctx *gin.Context) {
	roidString := ctx.Param("roid")

	host, err := ctrl.hostService.GetHostByRoID(ctx, roidString)
	if err != nil {
		if errors.Is(err, entities.ErrHostNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, host)
}

// DeleteHostByRoID godoc
// @Summary Delete a host by its RoID
// @Description Delete a host by its RoID
// @Tags Hosts
// @Param roid path int true "RoID"
// @Success 204
// @Failure 500
// @Router /hosts/{roid} [delete]
func (ctrl *HostController) DeleteHostByRoID(ctx *gin.Context) {
	roidString := ctx.Param("roid")

	err := ctrl.hostService.DeleteHostByRoID(ctx, roidString)
	if err != nil {
		if errors.Is(err, entities.ErrInvalidRoid) {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusNoContent, nil)
}

// CreateHost godoc
// @Summary Create a host
// @Description Create a host
// @Tags Hosts
// @Accept json
// @Produce json
// @Param host body commands.CreateHostCommand true "Host"
// @Success 201 {object} entities.Host
// @Failure 400
// @Failure 500
// @Router /hosts [post]
func (ctrl *HostController) CreateHost(ctx *gin.Context) {
	var req commands.CreateHostCommand
	if err := ctx.ShouldBindJSON(&req); err != nil {
		if err.Error() == "EOF" {
			ctx.JSON(400, gin.H{"error": "missing request body"})
			return
		}
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	host, err := ctrl.hostService.CreateHost(ctx, &req)
	if err != nil {
		if errors.Is(err, entities.ErrInvalidHost) {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, host)
}

// ListHosts godoc
// @Summary List hosts
// @Description List hosts
// @Tags Hosts
// @Produce json
// @Param pageSize query int false "Page Size"
// @Param cursor query string false "Cursor"
// @Success 200 {array} response.ListItemResult
// @Failure 500
// @Router /hosts [get]
func (ctrl *HostController) ListHosts(ctx *gin.Context) {
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

	// Get the contacts from the service
	hosts, err := ctrl.hostService.ListHosts(ctx, pageSize, pageCursor)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	response.Data = hosts
	if len(hosts) > 0 {
		response.SetMeta(ctx, hosts[len(hosts)-1].RoID.String(), len(hosts), pageSize)
	}

	// Return the response
	ctx.JSON(200, response)
}

// AddAddressToHost godoc
// @Summary adds an IP address to an existing host
// @Description adds an IP address to an existing host
// @Tags Hosts
// @Param        roid path string true "Host ROID"
// @Param        ip path string true "Host Address"
// @Produce json
// @Success 201 {object} entities.host
// @Failure      400
// @Failure      404
// @Failure      500
// @Failure 500
// @Router /hosts/{roid}/addresses/{ip}  [post]
func (ctrl *HostController) AddAddressToHost(ctx *gin.Context) {
	// Try and add the address
	updatedHost, err := ctrl.hostService.AddHostAddress(ctx, ctx.Param("roid"), ctx.Param("ip"))
	if err != nil {
		if errors.Is(err, entities.ErrHostNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Return the response
	ctx.JSON(200, updatedHost)

}

// RemoveAddressFromHost godoc
// @Summary removes an IP address from an existing host
// @Description removes an IP address to from existing host
// @Tags Hosts
// @Param        roid path string true "Host ROID"
// @Param        ip path string true "Host Address"
// @Produce json
// @Success 201 {object} entities.host
// @Failure      400
// @Failure      404
// @Failure      500
// @Failure 500
// @Router /hosts/{roid}/addresses/{ip}  [delete]
func (ctrl *HostController) RemoveAddressFromHost(ctx *gin.Context) {
	// Try and remove the address
	updatedHost, err := ctrl.hostService.RemoveHostAddress(ctx, ctx.Param("roid"), ctx.Param("ip"))
	if err != nil {
		if errors.Is(err, entities.ErrHostNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Return the response
	ctx.JSON(200, updatedHost)

}
