package rest

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
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

	// Admin endpoints
	e.POST("/domains", controller.CreateDomain) // use this when importing or creating a domain as an admin with full control
	e.GET("/domains/:name", controller.GetDomainByName)
	e.PUT("/domains/:name", controller.UpdateDomain)
	e.DELETE("/domains/:name", controller.DeleteDomainByName)
	e.GET("/domains", controller.ListDomains)
	e.GET("/domains/count", controller.CountDomains)
	// Add and remove hosts
	e.POST("/domains/:name/hosts/:roid", controller.AddHostToDomain)
	e.POST("/domains/:name/hostname/:hostName", controller.AddHostToDomainByHostName)
	e.DELETE("/domains/:name/hosts/:roid", controller.RemoveHostFromDomain)
	e.DELETE("/domains/:name/hostname/:hostName", controller.RemoveHostFromDomainByHostName)

	// Set domain to dropcatch (will be blocked when deleted)
	e.POST("/domains/:name/dropcatch", controller.SetDropCatch)
	e.DELETE("/domains/:name/dropcatch", controller.UnSetDropCatch)
	// Registrar endpoints - These are similar to the EPP commands and are used by registrars, or if an admin wants to pretend to be a registrar
	e.GET("/domains/:name/check", controller.CheckDomain)
	e.POST("/domains/:name/register", controller.RegisterDomain)
	e.POST("/domains/:name/renew", controller.RenewDomain)
	e.DELETE("/domains/:name/markdelete", controller.MarkDomainForDeletion)
	e.POST("/domains/:name/restore", controller.RestoreDomain)

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

	domain, err := ctrl.domainService.GetDomainByName(ctx, name, true)
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
// @Param force query bool false "Force the addition of the host"
// @Success 204
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /domains/{name}/hosts/{roid} [post]
func (ctrl *DomainController) AddHostToDomain(ctx *gin.Context) {
	var force bool
	if ctx.Query("force") == "true" {
		force = true
	}
	// Use the service to add the host to the domain
	err := ctrl.domainService.AddHostToDomain(ctx, ctx.Param("name"), ctx.Param("roid"), force)
	if err != nil {
		if errors.Is(err, entities.ErrDomainNotFound) || errors.Is(err, entities.ErrHostNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, entities.ErrInBailiwickHostsMustHaveAddress) {
			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}
	// Return result
	ctx.JSON(204, nil)
}

// AddHostToDomainByHostName godoc
// @Summary Add a host to a domain by host name
// @Description Add a host to a domain by host name. Use this when you don't know the RoID of the host. The domain must not have an update prohibition. Use the force parameter to override this, but use it with care. For example when importing Escrows, you might create the domain object including its prohibitions, and link it to a host. In this case you would use the force parameter to add the host to the domain.
// @Tags Domains
// @Produce json
// @Param domainName path string true "Domain Name"
// @Param hostName path string true "Host Name"
// @Param force query bool false "Force the addition of the host"
// @Success 204
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /domains/{name}/hostname/{hostName} [post]
func (ctrl *DomainController) AddHostToDomainByHostName(ctx *gin.Context) {
	var force bool
	if ctx.Query("force") == "true" {
		force = true
	}
	// Use the service to add the host to the domain
	err := ctrl.domainService.AddHostToDomainByHostName(ctx, ctx.Param("name"), ctx.Param("hostName"), force)
	if err != nil {
		if errors.Is(err, entities.ErrDomainNotFound) || errors.Is(err, entities.ErrHostNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, entities.ErrInBailiwickHostsMustHaveAddress) || errors.Is(err, entities.ErrDomainUpdateNotAllowed) {
			ctx.JSON(403, gin.H{"error": err.Error()})
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

// RemoveHostFromDomainByHostName godoc
// @Summary Remove a host from a domain by host name
// @Description Remove a host from a domain by host name
// @Tags Domains
// @Produce json
// @Param domainName path string true "Domain Name"
// @Param hostName path string true "Host Name"
// @Success 204
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /domains/{name}/hostname/{hostName} [delete]
func (ctrl *DomainController) RemoveHostFromDomainByHostName(ctx *gin.Context) {
	// Use the service to remove the host from the domain
	err := ctrl.domainService.RemoveHostFromDomainByHostName(ctx, ctx.Param("name"), ctx.Param("hostName"))
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

// RegisterDomain godoc
// @Summary Register a domain as a Registrar
// @Description Register a domain as a Registrar. Is modelled after the EPP create command.
// @Tags Domains
// @Accept json
// @Produce json
// @Param domain body commands.RegisterDomainCommand true "Domain"
// @Success 201 {object} entities.Domain
// @Failure 400
// @Failure 500
// @Router /domains/{name}/register [post]
func (ctrl *DomainController) RegisterDomain(ctx *gin.Context) {
	name := ctx.Param("name")
	var req commands.RegisterDomainCommand
	if err := ctx.ShouldBindJSON(&req); err != nil {
		if err.Error() == "EOF" {
			ctx.JSON(400, gin.H{"error": "missing request body"})
			return
		}
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name != name {
		ctx.JSON(400, gin.H{"error": "name in body must match name in path"})
		return
	}

	domain, err := ctrl.domainService.RegisterDomain(ctx, &req)
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

// CheckDomain godoc
// @Summary Check if a domain is available
// @Description Check if a domain is available
// @Tags Domains
// @Produce json
// @Param name path string true "Domain Name"
// @Param includeFees query bool false "Include fees in the response"
// @Success 200 {object} queries.DomainCheckResult
// @Failure 400
// @Failure 500
// @Router /domains/{name}/check [get]
func (ctrl *DomainController) CheckDomain(ctx *gin.Context) {
	name := ctx.Param("name")
	includeFees := ctx.DefaultQuery("includeFees", "false")

	// Create a query object
	q, err := queries.NewDomainCheckQuery(name, includeFees == "true")
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Set the phase name and currency if it was provided
	q.PhaseName = ctx.Query("phase")
	q.Currency = ctx.Query("currency")
	if q.IncludeFees && q.Currency == "" {
		ctx.JSON(400, gin.H{"error": "currency is required when requesting fees"})
		return
	}

	// Call the service to check the domain
	result, err := ctrl.domainService.CheckDomain(ctx, q)
	if err != nil {
		if errors.Is(err, entities.ErrTLDNotFound) || errors.Is(err, entities.ErrPhaseNotFound) || errors.Is(err, entities.ErrNoActivePhase) {
			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, result)
}

// RenewDomain godoc
// @Summary Renew a domain as a Registrar
// @Description Renew a domain as a Registrar. Is modelled after the EPP renew command.
// @Tags Domains
// @Accept json
// @Produce json
// @Param domain body commands.RenewDomainCommand true "Domain"
// @Success 201 {object} entities.Domain
// @Failure 400
// @Failure 500
// @Router /domains/{name}/renew [post]
func (ctrl *DomainController) RenewDomain(ctx *gin.Context) {
	name := ctx.Param("name")
	var req commands.RenewDomainCommand
	if err := ctx.ShouldBindJSON(&req); err != nil {
		if err.Error() == "EOF" {
			ctx.JSON(400, gin.H{"error": "missing request body"})
			return
		}
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name != name {
		ctx.JSON(400, gin.H{"error": "name in body must match name in path"})
		return
	}

	domain, err := ctrl.domainService.RenewDomain(ctx, &req)
	if err != nil {
		if errors.Is(err, entities.ErrInvalidRenewal) {
			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, domain)
}

// MarkDomainForDeletion godoc
// @Summary Mark a domain for deletion
// @Description Mark a domain for deletion. Is modelled after the EPP delete command.
// @Tags Domains
// @Produce json
// @Param domain path string true "Domain Name"
// @Success 200 {object} entities.Domain
// @Failure 400
// @Failure 500
// @Router /domains/{name}/markdelete [delete]
func (ctrl *DomainController) MarkDomainForDeletion(ctx *gin.Context) {
	dom, err := ctrl.domainService.MarkDomainForDeletion(ctx, ctx.Param("name"))
	if err != nil {
		if errors.Is(err, entities.ErrDomainNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, entities.ErrDomainDeleteNotAllowed) {
			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, dom)
}

// RestoreDomain godoc
// @Summary Restore a domain
// @Description Restore a domain. It marks the domain as pendingRestore and fires off an event. The domain will be restored by the registry when the restore event is processed.
// @Tags Domains
// @Produce json
// @Param domain path string true "Domain Name"
// @Success 200 {object} entities.Domain
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /domains/{name}/restore [post]
func (ctrl *DomainController) RestoreDomain(ctx *gin.Context) {
	dom, err := ctrl.domainService.RestoreDomain(ctx, ctx.Param("name"))
	if err != nil {
		if errors.Is(err, entities.ErrDomainNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, entities.ErrDomainRestoreNotAllowed) {
			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, dom)
}

// SetDropCatch godoc
// @Summary Set a domain to dropcatch
// @Description Set a domain to dropcatch. When it gets deleted it will automatically create an NNDN with this name and set the category to dropcatched.
// @Tags Domains
// @Produce json
// @Param domain path string true "Domain Name"
// @Success 204
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /domains/{name}/dropcatch [post]
func (ctrl *DomainController) SetDropCatch(ctx *gin.Context) {
	// use the service to set the domain.DropCatch flag
	err := ctrl.domainService.DropCatchDomain(ctx, ctx.Param("name"), true)
	if err != nil {
		if errors.Is(err, entities.ErrDomainNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(204, nil)
}

// UnSetDropCatch godoc
// @Summary Removes the dropcatch flag from the domain
// @Description Removes the dropcatch flag from the domain
// @Tags Domains
// @Produce json
// @Param domain path string true "Domain Name"
// @Success 204
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /domains/{name}/dropcatch [delete]
func (ctrl *DomainController) UnSetDropCatch(ctx *gin.Context) {
	// use the service to un set the domain.DropCatch flag
	err := ctrl.domainService.DropCatchDomain(ctx, ctx.Param("name"), false)
	if err != nil {
		if errors.Is(err, entities.ErrDomainNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(204, nil)

}

// CountDomains godoc
// @Summary Count domains
// @Description Count domains
// @Tags Domains
// @Produce json
// @Success 200 {object} response.CountResult
// @Failure 500
// @Router /domains/count [get]
func (ctrl *DomainController) CountDomains(ctx *gin.Context) {
	count, err := ctrl.domainService.Count(ctx)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, response.CountResult{
		Count:      count,
		ObjectType: "Domain",
		Timestamp:  time.Now().UTC(),
	})
}
