package rest

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
)

// DomainController
type DomainController struct {
	domainService interfaces.DomainService
}

func NewDomainController(e *gin.Engine, domService interfaces.DomainService, handler gin.HandlerFunc) *DomainController {
	controller := &DomainController{
		domainService: domService,
	}

	domainGroup := e.Group("/domains", handler)

	{

		// Admin endpoints
		domainGroup.POST("", controller.CreateDomain) // use this when importing or creating a domain as an admin with full control
		domainGroup.GET(":name", controller.GetDomainByName)
		domainGroup.PUT(":name", controller.UpdateDomain)
		domainGroup.DELETE(":name", controller.DeleteDomainByName)
		domainGroup.GET("", controller.ListDomains)
		domainGroup.GET("count", controller.CountDomains)
		domainGroup.POST("quote", controller.GetQuote)
		// Add and remove hosts
		domainGroup.POST(":name/hosts/:roid", controller.AddHostToDomain)
		domainGroup.POST(":name/hostname/:hostName", controller.AddHostToDomainByHostName)
		domainGroup.DELETE(":name/hosts/:roid", controller.RemoveHostFromDomain)
		domainGroup.DELETE(":name/hostname/:hostName", controller.RemoveHostFromDomainByHostName)

		// Set domain to dropcatch (will be blocked when deleted)
		domainGroup.POST(":name/dropcatch", controller.SetDropCatch)
		domainGroup.DELETE(":name/dropcatch", controller.UnSetDropCatch)

		// Registrar endpoints - These are similar to the EPP commands and are used by registrars, or if an admin wants to pretend to be a registrar
		domainGroup.GET(":name/available", controller.CheckDomainAvailability)
		domainGroup.POST(":name/register", controller.RegisterDomain)
		domainGroup.POST(":name/renew", controller.RenewDomain)
		domainGroup.GET(":name/canautorenew", controller.CanAutoRenew)
		domainGroup.POST(":name/autorenew", controller.AutoRenewDomain)
		domainGroup.DELETE(":name/markdelete", controller.MarkDomainForDeletion)
		domainGroup.DELETE(":name/expire", controller.Expire)
		domainGroup.POST(":name/restore", controller.RestoreDomain)

		// Lifecycle endpoints
		domainGroup.GET("expiring", controller.ListExpiringDomains)
		domainGroup.GET("expiring/count", controller.CountExpiringDomains)
		domainGroup.GET("purgeable", controller.ListPurgeableDomains)
		domainGroup.GET("purgeable/count", controller.CountPurgeableDomains)
	}

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
// @Summary Create a domain as an ADMIN with full control
// @Description Do not use this for registrar activity or domain lifecycle activity. Use this to create/import domains as an admin with full control. For example during a migration IN. If you are looking to register a domain as a registrar, use the /register endpoint.
// @Description If you need this endpoint to enforce a current GA phase policy, enable thisby setting commands.CreateDomainCommand.EnforcePhasePolicy to true (defaults to false)
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

	// Get the Event from the context
	event := GetEventFromContext(ctx)
	// Set the event details.command
	event.Details.Command = req

	domain, err := ctrl.domainService.CreateDomain(ctx, &req)
	if err != nil {
		event.Details.Error = err.Error()
		if errors.Is(err, entities.ErrInvalidDomain) {
			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Set the event details.after
	event.Details.After = domain
	ctx.Set("event", event)

	ctx.JSON(201, domain)
}

// DeleteDomainByName godoc
// @Summary Delete a domain by name
// @Description Delete a domain by name
// @Tags Domains
// @Param name path string true "Domain Name"
// @Param drophosts query bool false "Delete all hosts associated with the domain prior to deleting the domain"
// @Success 204
// @Failure 404
// @Failure 500
// @Router /domains/{name} [delete]
func (ctrl *DomainController) DeleteDomainByName(ctx *gin.Context) {
	name := ctx.Param("name")

	if ctx.Query("drophosts") == "true" {

		err := ctrl.domainService.RemoveAllDomainHosts(ctx, name)
		if err != nil {
			if errors.Is(err, entities.ErrDomainNotFound) {
				ctx.JSON(404, gin.H{"error": err.Error()})
				return
			}
			ctx.JSON(500, gin.H{"error": err.Error()})
			return
		}
	}

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
// @Description This operation requires the Registrar to be accredited for the TLD. If the Registrar is not accredited, the request will fail with a 403 status code.
// @Description If the domain is invalid in some way, the request will fail with a 400 status code with an error message.
// @Tags Domains
// @Accept json
// @Produce json
// @Param domain body commands.RegisterDomainCommand true "Domain"
// @Param correlation_id path string false "Correlation ID"
// @Success 201 {object} entities.Domain
// @Failure 400
// @Failure 403
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
		if errors.Is(err, entities.ErrInvalidDomain) ||
			errors.Is(err, entities.ErrContactDataPolicyViolation) {

			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, services.ErrRegistrarNotAccredited) {
			ctx.JSON(403, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(201, domain)
}

// CheckDomainAvailability godoc
// @Summary Check if a domain is available
// @Description A Domain is available if:
// @Description - The domain does not exist
// @Description - No NNDN exists with the same name
// @Description - The domain label is valid in the TLDs current GA phase OR the provided phase name)
// @Description It will return a 400 error if the TLD is not found, the phase is not found, the phase is not active, or the label is not valid in the phase.
// @Description It will return a 500 error if an unexpected error occurs.
// @Tags Domains
// @Produce json
// @Param name path string true "Domain Name"
// @Param phase query string false "Phase Name"
// @Success 200 {object} queries.DomainCheckResult
// @Failure 400
// @Failure 500
// @Router /domains/{name}/available [get]
func (ctrl *DomainController) CheckDomainAvailability(ctx *gin.Context) {
	// Call the service to check the domain
	result, err := ctrl.domainService.CheckDomainAvailability(ctx, ctx.Param("name"), ctx.Query("phase"))
	if err != nil {
		// Return 400 if we encounter missing configuration to make a decision
		if errors.Is(
			err, entities.ErrTLDNotFound) ||
			errors.Is(err, entities.ErrPhaseNotFound) ||
			errors.Is(err, entities.ErrNoActivePhase) ||
			errors.Is(err, entities.ErrLabelNotValidInPhase) {
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
// @Success 200 {object} entities.Domain
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

// CanAutoRenew godoc
// @Summary Check if a domain can be auto-renewed
// @Description CanAutoRenew handles the HTTP request to check if a domain can be auto-renewed.
// @Description It expects a domain name as a URL parameter and returns a JSON response indicating
// @Description whether the domain can be auto-renewed or not.
// @Tags Domains
// @Accept json
// @Produce json
// @Param name path string true "Domain Name"
// @Success 200 {object} response.CanAutoRenewResponse
// @Failure 400
// @Failure 404
// @Failure 500
// @Router /domains/{name}/canautorenew [get]
func (ctrl *DomainController) CanAutoRenew(ctx *gin.Context) {
	domainName := ctx.Param("name")
	if domainName == "" {
		ctx.JSON(400, gin.H{"error": "missing domain name"})
		return
	}

	canAutoRenew, err := ctrl.domainService.CanAutoRenew(ctx, domainName)
	if err != nil {
		if errors.Is(err, entities.ErrDomainNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	resp := response.CanAutoRenewResponse{
		CanAutoRenew: canAutoRenew,
		DomainName:   domainName,
		Timestamp:    time.Now().UTC(),
	}

	ctx.JSON(200, resp)
}

// AutoRenewDomain godoc
// @Summary Auto-renew a domain
// @Description Auto-renews a domain for the specified number of years (defaults to 1 if the 'years' query param is not provided or invalid).
// @Description This operation requires **both** `Registrar.AutoRenew` and `TLD.CurrentGAPhase.AllowAutoRenew` to be enabled.
// @Description If either is not enabled, the request will fail with a 403 status code.
// @Tags Domains
// @Accept  json
// @Produce json
// @Param name path string true "Domain Name"
// @Param years query int false "Number of years to renew, defaults to 1"
// @Success 200 {object} entities.Domain "Domain was successfully renewed"
// @Failure 400
// @Failure 403
// @Failure 404
// @Failure 500
// @Router /domains/{name}/autorenew [post]
func (ctrl *DomainController) AutoRenewDomain(ctx *gin.Context) {
	yearsStr := ctx.DefaultQuery("years", "1")
	years, err := strconv.Atoi(yearsStr)
	if err != nil {
		ctx.JSON(400, gin.H{"error": fmt.Sprintf("error converting years string to years int: %s", err.Error())})
		return
	}

	domain, err := ctrl.domainService.AutoRenewDomain(ctx, ctx.Param("name"), years)
	if err != nil {
		if errors.Is(err, entities.ErrDomainNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, services.ErrAutoRenewNotEnabledRar) || errors.Is(err, services.ErrAutoRenewNotEnabledTLD) {
			ctx.JSON(403, gin.H{"error": err.Error()})
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

// Expire godoc
// @Summary Expire handles the expiration of a domain by its name.
// @Description It retrieves the domain name from the request context, calls the domainService to expire the domain,
// @Description and returns the appropriate JSON response based on the outcome.
// @Description If the domain is not found, it returns a 404 status code with an error message.
// @Description If the domain is not allowed to be expired (domains can expire only on the day of their expirydate or after), it returns a 403 status code with an error message.
// @Description If the the TLD does not have an active GA phase (Phase.Policy contains the applicable EOL policy), it returns a 403 status code with an error message.
// @Description For other errors, it returns a 500 status code with an error message.
// @Description On success, it returns a 200 status code with the expired domain information.
// @Tags Domains
// @Accept json
// @Produce json
// @Param name path string true "Domain Name"
// @Success 200 {object} entities.Domain
// @Failure 404
// @Failure 500
// @Router /domains/{name}/expire [post]
func (ctrl *DomainController) Expire(ctx *gin.Context) {
	domain, err := ctrl.domainService.ExpireDomain(ctx, ctx.Param("name"))
	if err != nil {
		if errors.Is(err, entities.ErrDomainNotFound) {
			ctx.JSON(404, gin.H{"error": err.Error()})
			return
		}
		// Return 403 if there is no active phase or the domain has not reached the expiry date yet
		if errors.Is(err, entities.ErrDomainExpiryNotAllowed) || errors.Is(err, entities.ErrNoActivePhase) {
			ctx.JSON(403, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, domain)
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

// ListExpiringDomains godoc
// @Summary List expiring domains
// @Description Lists domains that expire before now (if no date is provided) or before the provided time in RFC3339 format. You can optionally filter by registrar ClID and TLD.
// @Tags Domains
// @Produce json
// @Param before query int false "List domains that expire before the provided time in RFC3339 format (optional, default=current UTC time)"
// @Param clid query string false "Registrar ClID (optional, default=empty=all registrars)"
// @Param tld query string false "TLD Name (optional, default=empty=all TLDs)"
// @Param pageSize query int false "Page Size"
// @Param cursor query string false "Cursor"
// @Success 200 {array} response.ListItemResult
// @Failure 400
// @Failure 500
// @Router /domains/expiring [get]
func (ctrl *DomainController) ListExpiringDomains(ctx *gin.Context) {
	var err error
	// Prepare the response
	resp := response.ListItemResult{}

	q, err := queries.NewExpiringDomainsQuery(ctx.Query("clid"), ctx.Query("before"), ctx.Query("tld"))
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

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
	domains, err := ctrl.domainService.ListExpiringDomains(ctx, q, pageSize, pageCursor)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Move the domains into a domain expiry item
	expiryItems := make([]response.DomainExpiryItem, len(domains))
	for i, d := range domains {
		expiryItems[i] = response.DomainExpiryItem{
			RoID:       d.RoID.String(),
			Name:       d.Name.String(),
			ExpiryDate: d.ExpiryDate,
		}
	}

	// Set the response MetaData
	resp.Data = expiryItems
	if len(domains) > 0 {
		resp.SetMeta(ctx, domains[len(domains)-1].RoID.String(), len(domains), pageSize)
	}

	// Return the Response
	ctx.JSON(200, resp)
}

// CountExpiringDomains godoc
// @Summary Count expiring domains
// @Description Counts domains that expire before the provided time in RFC3339 format. If no time is provided it will default to the current UTC time.
// @Tags Domains
// @Produce json
// @Param before query int false "List domains that expire before the provided time in RFC3339 format (optional, default=current UTC time)"
// @Param clid query string false "Registrar ClID (optional)"
// @Param tld query string false "TLD Name (optional, default=empty=all TLDs)"
// @Success 200 {object} response.CountResult
// @Failure 400
// @Failure 500
// @Router /domains/expiring/count [get]
func (ctrl *DomainController) CountExpiringDomains(ctx *gin.Context) {

	q, err := queries.NewExpiringDomainsQuery(ctx.Query("clid"), ctx.Query("before"), ctx.Query("tld"))
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	count, err := ctrl.domainService.CountExpiringDomains(ctx, q)
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

// CountPurgeableDomains godoc
// @Summary Count purgeable domains
// @Description Counts domains that are purgeable. This means they are pending delete and the applicable grace period has expired. You can optionally filter by registrar ClID and TLD.
// @Tags Domains
// @Produce json
// @Param after query int false "List domains that are purgeable after the provided time in RFC3339 format (optional, default=current UTC time)"
// @Param clid query string false "Registrar ClID (optional)"
// @Param tld query string false "TLD Name (optional, default=empty=all TLDs)"
// @Success 200 {object} response.CountResult
// @Failure 500
// @Router /domains/purgeable/count [get]
func (ctrl *DomainController) CountPurgeableDomains(ctx *gin.Context) {

	q, err := queries.NewPurgeableDomainsQuery(ctx.Query("clid"), ctx.Query("after"), ctx.Query("tld"))
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	count, err := ctrl.domainService.CountPurgeableDomains(ctx, q)
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

// ListPurgeableDomains godoc
// @Summary List purgeable domains
// @Description Lists domains that are purgeable. This means they are pending delete and the applicable grace period has expired. You can optionally filter by registrar ClID and TLD.
// @Tags Domains
// @Produce json
// @Param after query int false "List domains that are purgeable after the provided time in RFC3339 format (optional, default=current UTC time)"
// @Param clid query string false "Registrar ClID (optional)"
// @Param tld query string false "TLD Name (optional, default=empty=all TLDs)"
// @Param pageSize query int false "Page Size"
// @Param cursor query string false "Cursor"
// @Success 200 {array} response.ListItemResult
// @Failure 400
// @Failure 500
// @Router /domains/purgeable [get]
func (ctrl *DomainController) ListPurgeableDomains(ctx *gin.Context) {
	var err error
	// Prepare the response
	resp := response.ListItemResult{}

	q, err := queries.NewPurgeableDomainsQuery(ctx.Query("clid"), ctx.Query("after"), ctx.Query("tld"))
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

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
	domains, err := ctrl.domainService.ListPurgeableDomains(ctx, q, pageSize, pageCursor)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Move the domains into a domain expiry item
	expiryItems := make([]response.DomainExpiryItem, len(domains))
	for i, d := range domains {
		expiryItems[i] = response.DomainExpiryItem{
			RoID:       d.RoID.String(),
			Name:       d.Name.String(),
			ExpiryDate: d.ExpiryDate,
		}
	}

	// Set the response MetaData
	resp.Data = expiryItems
	if len(domains) > 0 {
		resp.SetMeta(ctx, domains[len(domains)-1].RoID.String(), len(domains), pageSize)
	}

	// Return the Response
	ctx.JSON(200, resp)
}

// GetQuote godoc
// @Summary returns a quote for a transaction
// @Description Takes a QuoteRequest and returns a Quote for the transaction including a breakdown of costs.
// @Description The QuoteRequest parameters are all required, except for phaseName which defaults to Currently Active GA Phase
// @Description The resulting Quote contains a final price for the transaction as well as all the relevant configured pricepoints including currency conversion if applicable
// @Tags Domains
// @Accept  json
// @Produce  json
// @Param quoteRequest body queries.QuoteRequest true "QuoteRequest"
// @Success 200 {object} entities.Quote
// @Failure 400
// @Failure 500
// @Router /domains/quote [post]
func (ctrl *DomainController) GetQuote(ctx *gin.Context) {
	var qr queries.QuoteRequest
	if err := ctx.ShouldBindJSON(&qr); err != nil {
		if err.Error() == "EOF" {
			ctx.JSON(400, gin.H{"error": "missing request body"})
			return
		}
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}

	quote, err := ctrl.domainService.GetQuote(ctx, &qr)
	if err != nil {
		if errors.Is(err, entities.ErrPhaseNotFound) {
			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(200, quote)
}
