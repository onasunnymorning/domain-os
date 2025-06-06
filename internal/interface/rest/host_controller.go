package rest

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

// HostController is the controller for the HostService
type HostController struct {
	hostService interfaces.HostService
}

// NewHostController creates a new instance of HostController
func NewHostController(e *gin.Engine, hostService interfaces.HostService, handler gin.HandlerFunc) *HostController {
	c := &HostController{
		hostService: hostService,
	}

	hostGroup := e.Group("/hosts", handler)

	{
		hostGroup.GET(":roid", c.GetHostByRoID)
		hostGroup.GET("", c.ListHosts)
		hostGroup.DELETE(":roid", c.DeleteHostByRoID)
		hostGroup.POST("", c.CreateHost)
		hostGroup.POST("/bulk", c.BulkCreate)
		hostGroup.POST(":roid/addresses/:ip", c.AddAddressToHost)
		hostGroup.DELETE(":roid/addresses/:ip", c.RemoveAddressFromHost)
	}

	e.GET("/hostname/:name/:clid", handler, c.GetHostByNameAndClid)

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
	// event := GetEventFromContext(ctx)
	// Temporarily disable this to overcome infra issues with message broker
	event := entities.NewEvent("domain-os", "admin", "DELETE", "Host", ctx.Param("roid"), ctx.Request.URL.RequestURI())
	roidString := ctx.Param("roid")

	err := ctrl.hostService.DeleteHostByRoID(ctx, roidString)
	if err != nil {
		event.Details.Error = err.Error()
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
	query := queries.ListItemsQuery{}
	var err error
	// Prepare the response
	response := response.ListItemResult{}
	// Get the pagesize from the query string
	query.PageSize, err = GetPageSize(ctx)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// Get the cursor from the query string
	query.PageCursor, err = GetAndDecodeCursor(ctx)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Get the filter from the query string
	filter := queries.ListHostsFilter{
		NameLike:        ctx.Query("name_like"),
		ClidEquals:      ctx.Query("clid_equals"),
		RoidGreaterThan: ctx.Query("roid_greater_than"),
		RoidLessThan:    ctx.Query("roid_less_than"),
	}
	query.Filter = filter

	// Get the contacts from the service
	hosts, cursor, err := ctrl.hostService.ListHosts(ctx, query)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	response.Data = hosts
	response.SetMeta(ctx, cursor, len(hosts), query.PageSize, query.Filter)

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
// @Success 201 {object} entities.Host
// @Failure      400
// @Failure      404
// @Failure      500
// @Failure 500
// @Router /hosts/{roid}/addresses/{ip}  [post]
func (ctrl *HostController) AddAddressToHost(ctx *gin.Context) {
	// event := GetEventFromContext(ctx)
	// Temporarily disable this to overcome infra issues with message broker
	event := entities.NewEvent("domain-os", "admin", "CREATE", "Host Address", ctx.Param("ip"), ctx.Request.URL.RequestURI())
	// Try and add the address
	updatedHost, err := ctrl.hostService.AddHostAddress(ctx, ctx.Param("roid"), ctx.Param("ip"))
	if err != nil {
		event.Details.Error = err.Error()
		if errors.Is(err, entities.ErrHostNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, entities.ErrHostUpdateProhibited) {
			ctx.JSON(403, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	event.Details.Command = ctx.Param("ip")
	event.Details.After = updatedHost

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
// @Success 201 {object} entities.Host
// @Failure      400
// @Failure      404
// @Failure      500
// @Failure 500
// @Router /hosts/{roid}/addresses/{ip}  [delete]
func (ctrl *HostController) RemoveAddressFromHost(ctx *gin.Context) {
	// event := GetEventFromContext(ctx)
	// Temporarily disable this to overcome infra issues with message broker
	event := entities.NewEvent("domain-os", "admin", "DELETE", "Host Address", ctx.Param("ip"), ctx.Request.URL.RequestURI())

	// Try and remove the address
	updatedHost, err := ctrl.hostService.RemoveHostAddress(ctx, ctx.Param("roid"), ctx.Param("ip"))
	if err != nil {
		event.Details.Error = err.Error()
		if errors.Is(err, entities.ErrHostNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	event.Details.Command = ctx.Param("ip")
	event.Details.After = updatedHost

	// Return the response
	ctx.JSON(200, updatedHost)

}

// GetHostByNameAndClid godoc
// @Summary Get a host by its name and clid
// @Description Get a host by its name and clid
// @Tags Hosts
// @Produce json
// @Param name path string true "Name"
// @Param clid path string true "Clid"
// @Success 200 {object} entities.Host
// @Failure 404
// @Failure 500
// @Router /hostname/{name}/{clid} [get]
func (ctrl *HostController) GetHostByNameAndClid(ctx *gin.Context) {
	name := ctx.Param("name")
	clid := ctx.Param("clid")

	host, err := ctrl.hostService.GetHostByNameAndClID(ctx, name, clid)
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

// BulkCreate godoc
// @Summary Create multiple hosts
// @Description Create multiple hosts
// @Tags Hosts
// @Accept json
// @Produce json
// @Param hosts body []commands.CreateHostCommand true "Hosts"
// @Success 201
// @Failure 400
// @Failure 500
// @Router /hosts/bulk [post]
func (ctrl *HostController) BulkCreate(ctx *gin.Context) {
	var req []*commands.CreateHostCommand
	if err := ctx.ShouldBindJSON(&req); err != nil {
		if err.Error() == "EOF" {
			ctx.JSON(400, gin.H{"error": "missing request body"})
			return
		}
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := ctrl.hostService.BulkCreate(ctx, req)
	if err != nil {
		if errors.Is(err, entities.ErrInvalidHost) {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, nil)
}
