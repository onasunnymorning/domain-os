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

// DomainController
type DomainController struct {
	domainService interfaces.DomainService
}

func NewDomainController(e *gin.Engine, domService interfaces.DomainService) *DomainController {
	controller := &DomainController{
		domainService: domService,
	}

	e.POST("/domains", controller.CreateDomain)
	e.GET("/domains/:name", controller.GetDomainByName)
	e.PUT("/domains/:name", controller.UpdateDomain)
	e.DELETE("/domains/:name", controller.DeleteDomainByName)
	e.GET("/domains", controller.ListDomains)

	e.POST("/domains/:name/hosts/:roid", controller.AddHostToDomain)
	e.DELETE("/domains/:name/hosts/:roid", controller.RemoveHostFromDomain)

	return controller
}

// GetDomainByName godoc
// @Summary Get a domain by name
// @Description Get a domain by name
// @Tags Domains
// @Produce json
// @Param name path string true "Domain Name"
// @Success 200 {object} entities.Domain
// @Failure 404
// @Failure 500
// @Router /domains/{name} [get]
func (ctrl *DomainController) GetDomainByName(ctx *gin.Context) {
	name := ctx.Param("name")

	domain, err := ctrl.domainService.GetDomainByName(ctx, name)
	if err != nil {
		if errors.Is(err, entities.ErrDomainNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, domain)
}

// CreateDomain godoc
// @Summary Create a domain
// @Description Create a domain
// @Tags Domains
// @Accept json
// @Produce json
// @Param domain body commands.CreateDomainCommand true "Domain"
// @Success 201 {object} entities.Domain
// @Failure 400
// @Failure 500
// @Router /domains [post]
func (ctrl *DomainController) CreateDomain(ctx *gin.Context) {
	var req commands.CreateDomainCommand
	if err := ctx.ShouldBindJSON(&req); err != nil {
		if err.Error() == "EOF" {
			ctx.JSON(400, gin.H{"error": "missing request body"})
			return
		}
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	domain, err := ctrl.domainService.CreateDomain(ctx, &req)
	if err != nil {
		if errors.Is(err, entities.ErrInvalidDomain) {
			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(201, domain)
}

// DeleteDomainByName godoc
// @Summary Delete a domain by name
// @Description Delete a domain by name
// @Tags Domains
// @Param name path string true "Domain Name"
// @Success 204
// @Failure 404
// @Failure 500
// @Router /domains/{name} [delete]
func (ctrl *DomainController) DeleteDomainByName(ctx *gin.Context) {
	name := ctx.Param("name")

	err := ctrl.domainService.DeleteDomainByName(ctx, name)
	if err != nil {
		if errors.Is(err, entities.ErrDomainNotFound) {
			// Return 204 if the domain was not found to make idempotent
			ctx.JSON(204, nil)
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(204, nil)
}

// ListDomains godoc
// @Summary List domains
// @Description List domains
// @Tags Domains
// @Produce json
// @Param pageSize query int false "Page Size"
// @Param cursor query string false "Cursor"
// @Success 200 {array} response.ListItemResult
// @Failure 400
// @Failure 500
// @Router /domains [get]
func (ctrl *DomainController) ListDomains(ctx *gin.Context) {
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

	// Get the list of domains
	domains, err := ctrl.domainService.ListDomains(ctx, pageSize, pageCursor)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Set the response MetaData
	response.Data = domains
	if len(domains) > 0 {
		response.SetMeta(ctx, domains[len(domains)-1].RoID.String(), len(domains), pageSize)
	}

	// Return the Response
	ctx.JSON(200, response)
}

// UpdateDomain godoc
// @Summary Update a domain
// @Description Update a domain
// @Tags Domains
// @Accept json
// @Produce json
// @Param name path string true "Domain Name"
// @Param domain body commands.UpdateDomainCommand true "Domain"
// @Success 200 {object} entities.Domain
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /domains/{name} [put]
func (ctrl *DomainController) UpdateDomain(ctx *gin.Context) {
	name := ctx.Param("name")

	var req commands.UpdateDomainCommand
	if err := ctx.ShouldBindJSON(&req); err != nil {
		if err.Error() == "EOF" {
			ctx.JSON(400, gin.H{"error": "missing request body"})
			return
		}
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	domain, err := ctrl.domainService.UpdateDomain(ctx, name, &req)
	if err != nil {
		if errors.Is(err, entities.ErrInvalidDomain) {
			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, entities.ErrDomainNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, domain)
}

// AddHostToDomain godoc
// @Summary Add a host to a domain
// @Description Add a host to a domain
// @Tags Domains
// @Produce json
// @Param name path string true "Domain Name"
// @Param roid path string true "Host RoID"
// @Success 204
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /domains/{name}/hosts/{roid} [post]
func (ctrl *DomainController) AddHostToDomain(ctx *gin.Context) {
	// Use the service to add the host to the domain
	err := ctrl.domainService.AddHostToDomain(ctx, ctx.Param("name"), ctx.Param("roid"))
	if err != nil {
		if errors.Is(err, entities.ErrDomainNotFound) || errors.Is(err, entities.ErrHostNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}
	// Return result
	ctx.JSON(204, nil)
}

// RemoveHostFromDomain godoc
// @Summary Remove a host from a domain
// @Description Remove a host from a domain
// @Tags Domains
// @Produce json
// @Param name path string true "Domain Name"
// @Param roid path string true "Host RoID"
// @Success 204
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /domains/{name}/hosts/{roid} [delete]
func (ctrl *DomainController) RemoveHostFromDomain(ctx *gin.Context) {
	// Use the service to remove the host from the domain
	err := ctrl.domainService.RemoveHostFromDomain(ctx, ctx.Param("name"), ctx.Param("roid"))
	if err != nil {
		if errors.Is(err, entities.ErrDomainNotFound) || errors.Is(err, entities.ErrHostNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Return result
	ctx.JSON(204, nil)
}
