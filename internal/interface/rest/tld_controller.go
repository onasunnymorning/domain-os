package rest

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/request"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

type TLDController struct {
	tldService interfaces.TLDService
	domService interfaces.DomainService
}

func NewTLDController(e *gin.Engine, tldService interfaces.TLDService, dnss interfaces.DomainService, handler gin.HandlerFunc) *TLDController {
	controller := &TLDController{
		tldService: tldService,
		domService: dnss,
	}

	tldRoutes := e.Group("/tlds", handler)

	{
		tldRoutes.GET(":tldName", controller.GetTLDByName)
		tldRoutes.GET("", controller.ListTLDs)
		tldRoutes.GET("count", controller.GetTLDCount)
		tldRoutes.POST("", controller.CreateTLD)
		tldRoutes.DELETE(":tldName", controller.DeleteTLDByName)
		tldRoutes.POST(":tldName/status/:status", controller.SetTLDStatus)
		tldRoutes.DELETE(":tldName/status", controller.DeleteTLDStatus)
		tldRoutes.GET(":tldName/dns/resource-records", controller.GetTLDHeader)
		tldRoutes.GET(":tldName/dns/domain-delegations", controller.GetNSRecordsPerTLD)
		tldRoutes.GET(":tldName/dns/glue-records", controller.GetGlueRecordsPerTLD)
	}
	return controller
}

// GetTLDByName godoc
// @Summary Get a TLD by name
// @Description Get a TLD by name
// @Tags TLDs
// @Produce json
// @Param tldName path string true "TLD Name"
// @Success 200 {object} entities.TLD
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /tlds/{tldName} [get]
func (ctrl *TLDController) GetTLDByName(ctx *gin.Context) {
	name := ctx.Param("tldName")

	tld, err := ctrl.tldService.GetTLDByName(ctx, name, false)
	if err != nil {
		if errors.Is(err, entities.ErrTLDNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, tld)
}

// ListTLDs godoc
// @Summary List TLDs
// @Description Returns a list of TLDs, ordered alphabetically by name, with pagination support. The cursor is the name of the last TLD in the previous page, base64 encoded.
// @Description The response includes a metadata object with the cursor for the next page.
// @Description You can filter by name (partial match), type (exact match), and ryid (exact match).
// @Tags TLDs
// @Produce json
// @Param pagesize query int false "Page size"
// @Param cursor query string false "Cursor"
// @Param name_like query string false "Filter by name, partial match"
// @Param type_equals query string false "Filter by type, exact match"
// @Param ryid_equals query string false "Filter by ryid, exact match"
// @Success 200 {array} response.ListItemResult
// @Failure 400 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /tlds [get]
func (ctrl *TLDController) ListTLDs(ctx *gin.Context) {
	var err error
	// Prepare the response
	response := response.ListItemResult{}

	// Prepare the query
	query := queries.ListItemsQuery{}

	// Get the pagesize from the request
	query.PageSize, err = GetPageSize(ctx)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// Get the cursor from the request
	query.PageCursor, err = GetAndDecodeCursor(ctx)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// Set the Filters
	query.Filter = getTldListFilterFromContext(ctx)

	// Get the tlds from the service
	tlds, cursor, err := ctrl.tldService.ListTLDs(ctx, query)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Set the Data and metadata if there are results only
	response.Data = tlds
	response.SetMeta(ctx, cursor, len(tlds), query.PageSize, query.Filter)

	// Return the response
	ctx.JSON(200, response)
}

// DeleteTLDByName godoc
// @Summary Delete a TLD by Name
// @Description Delete a TLD by Name
// @Tags TLDs
// @Produce json
// @Param tldName path string true "TLD Name"
// @Success 204
// @Failure 400
// @Failure 500
// @Router /tlds/{tldName} [delete]
func (ctrl *TLDController) DeleteTLDByName(ctx *gin.Context) {
	name := ctx.Param("tldName")

	err := ctrl.tldService.DeleteTLDByName(ctx, name)
	if err != nil {
		if errors.Is(err, services.ErrCannotDeleteTLDWithActivePhases) {
			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(204, nil)
}

// CreateTLD godoc
// @Summary Create a new TLD
// @Description Create a new TLD
// @Tags TLDs
// @Accept json
// @Produce json
// @Param registrar body commands.CreateTLDCommand true "TLD"
// @Success 200 {object} commands.CreateTLDCommandResult
// @Failure 400
// @Failure 500
// @Router /tlds [post]
func (ctrl *TLDController) CreateTLD(ctx *gin.Context) {
	var req request.CreateTLDRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		if err.Error() == "EOF" {
			ctx.JSON(400, gin.H{"error": "missing request body"})
			return
		}
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	cmd, err := req.ToCreateTLDCommand()
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result, err := ctrl.tldService.CreateTLD(ctx, cmd)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCreateTLDCommand) {
			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(201, result)
}

// GetTLDHeader godoc
// @Summary Get a TLD header
// @Description Get a TLD header
// @Tags TLDs
// @Produce json
// @Param tldName path string true "TLD Name"
// @Param format query string false "Output Format"
// @Success 200 {object} entities.TLDHeader
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /tlds/{tldName}/dns/resource-records [get]
func (ctrl *TLDController) GetTLDHeader(ctx *gin.Context) {
	name := ctx.Param("tldName")

	tldHeader, err := ctrl.tldService.GetTLDHeader(ctx, name)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	if format := ctx.Query("format"); format == "text" {
		ctx.String(200, "%s", tldHeader.String())
		return
	}

	ctx.JSON(200, tldHeader)
}

// GetNSRecordsPerTLD godoc
// @Summary Get NS records for a TLD
// @Description Get NS records for a TLD in JSON format (default) or text format
// @Tags TLDs
// @Produce json
// @Param tld path string true "TLD"
// @Param format query string false "Output format"
// @Param pagesize query string false "pagesize"
// @Param cursor query string false "cursor"
// @Success 200 {array} dns.RR
// @Failure 404
// @Failure 500
// @Router /tlds/{tldName}/dns/domain-delegations [get]
func (c *TLDController) GetNSRecordsPerTLD(ctx *gin.Context) {
	pagesize, err := GetPageSize(ctx)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Check if the TLD exists
	tldName := strings.ToLower(ctx.Param("tldName"))
	_, err = c.tldService.GetTLDByName(ctx, tldName, false)
	if err != nil {
		ctx.JSON(404, gin.H{"error": "TLD not found"})
		return
	}

	// Create our query object
	query := queries.ActiveDomainsWithHostsQuery{
		TldName:  tldName,
		PageSize: pagesize,
	}

	// Get the cursor from the request
	query.PageCursor, err = GetAndDecodeCursor(ctx)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	rrs, err := c.domService.GetNSRecordsPerTLD(ctx, query)

	if err != nil {
		ctx.JSON(500, gin.H{"error": "Error getting NS records"})
		return
	}

	if format := ctx.Query("format"); format == "text" {
		var stringResponse string
		for _, rr := range rrs {
			stringResponse += rr.String() + "\n"
		}

		ctx.String(200, "%s", stringResponse)
		return
	}

	ctx.JSON(200, rrs)
}

// GetGlueRecordsPerTLD godoc
// @Summary Get Glue records for a TLD
// @Description Get Glue records for a TLD
// @Tags TLDs
// @Produce json
// @Param tld path string true "TLD"
// @Param format query string false "Output format"
// @Success 200 {array} dns.RR
// @Failure 404
// @Failure 500
// @Router /tlds/{tldName}/dns/glue-records [get]
func (c *TLDController) GetGlueRecordsPerTLD(ctx *gin.Context) {
	// Check if the TLD exists
	tldName := ctx.Param("tldName")
	_, err := c.tldService.GetTLDByName(ctx, tldName, false)
	if err != nil {
		ctx.JSON(404, gin.H{"error": "TLD not found"})
		return
	}

	rrs, err := c.domService.GetGlueRecordsPerTLD(ctx, tldName)

	if err != nil {
		ctx.JSON(500, gin.H{"error": fmt.Sprintf("Error getting Glue records: %s", err.Error())})
		return
	}

	if format := ctx.Query("format"); format == "text" {
		var stringResponse string
		for _, rr := range rrs {
			stringResponse += rr.String() + "\n"
		}

		ctx.String(200, "%s", stringResponse)
		return
	}

	ctx.JSON(200, rrs)
}

// GetTLDCount godoc
// @Summary Get TLD count
// @Description Get TLD count
// @Tags TLDs
// @Produce json
// @Param name_like query string false "Filter by name, partial match"
// @Param type_equals query string false "Filter by type, exact match"
// @Param ryid_equals query string false "Filter by ryid, exact match"
// @Success 200 {object} response.CountResult
// @Failure 500
// @Router /tlds/count [get]
func (ctrl *TLDController) GetTLDCount(ctx *gin.Context) {
	filter := getTldListFilterFromContext(ctx)
	count, err := ctrl.tldService.CountTLDs(ctx, filter)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, response.CountResult{
		ObjectType: "TLD",
		Count:      count,
		Timestamp:  time.Now().UTC(),
		Filter:     filter,
	})
}

// SetTLDStatus godoc
// @Summary Set TLD status
// @Description Toggle the status for a TLD to ON. Currently supports AllowEscrowImport only
// @Tags TLDs
// @Produce json
// @Param tldName path string true "TLD Name"
// @Param status path string true "Status"
// @Success 204
// @Failure 400
// @Failure 409
// @Failure 500
// @Router /tlds/{tldName}/status/{status} [post]
func (ctrl *TLDController) SetTLDStatus(ctx *gin.Context) {
	// check if the status string is valid
	status := ctx.Param("status")
	if status != "AllowEscrowImport" {
		ctx.JSON(400, gin.H{"error": "Invalid status"})
		return
	}

	tldName := ctx.Param("tldName")

	// Use the service to set the status
	_, err := ctrl.tldService.SetAllowEscrowImport(ctx, tldName, true)
	if err != nil {
		// Return 409 conflict if the error is ErrCannotSetEscrowImportWithActivePhases
		if errors.Is(err, entities.ErrCannotSetEscrowImportWithActivePhases) {
			ctx.JSON(409, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(204, nil)
}

// DeleteTLDStatus godoc
// @Summary Delete TLD status
// @Description Toggle the status for a TLD to OFF. Currently supports AllowEscrowImport only
// @Tags TLDs
// @Produce json
// @Param tldName path string true "TLD Name"
// @Param status path string true "Status"
// @Success 204
// @Failure 400
// @Failure 409
// @Failure 500
// @Router /tlds/{tldName}/status/:status [delete]
func (ctrl *TLDController) DeleteTLDStatus(ctx *gin.Context) {
	// check if the status string is valid
	status := ctx.Param("status")
	if status != "AllowEscrowImport" {
		ctx.JSON(400, gin.H{"error": "Invalid status"})
		return
	}

	tldName := ctx.Param("tldName")

	// Use the service to set the status
	_, err := ctrl.tldService.SetAllowEscrowImport(ctx, tldName, false)
	if err != nil {
		// Return 409 conflict if the error is ErrCannotSetEscrowImportWithActivePhases
		if errors.Is(err, entities.ErrCannotSetEscrowImportWithActivePhases) {
			ctx.JSON(409, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(204, nil)
}

func getTldListFilterFromContext(ctx *gin.Context) queries.ListTldsFilter {
	filter := queries.ListTldsFilter{}
	filter.NameLike = ctx.Query("name_like")
	filter.TypeEquals = ctx.Query("type_equals")
	filter.RyIDEquals = ctx.Query("ryid_equals")
	return filter
}
