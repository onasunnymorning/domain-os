package rest

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// LaunchApplicationController is the operator's side of launch applications
// (ADR 0024 §3): the applications registrars filed over EPP in an
// application phase, and the decisions about them. Reading needs the TLD's
// operator in X-Tenant-ID (or registry:platform:admin); deciding needs
// registry:launch:admin as well.
type LaunchApplicationController struct {
	svc interfaces.LaunchApplicationService
}

// NewLaunchApplicationController registers the routes.
func NewLaunchApplicationController(e *gin.Engine, svc interfaces.LaunchApplicationService, handler gin.HandlerFunc) *LaunchApplicationController {
	ctrl := &LaunchApplicationController{svc: svc}
	permitted := func(ctx *gin.Context) {
		if requireAuthScope(ctx, ScopeRegistryLaunchAdmin) {
			ctx.Next()
		}
	}
	e.GET("/tlds/:tldName/phases/:phaseName/applications", handler, ctrl.ListApplications)
	g := e.Group("/launch-applications/:applicationID", handler)
	{
		g.GET("", ctrl.GetApplication)
		g.POST("validate", permitted, ctrl.Validate)
		g.POST("allocate", permitted, ctrl.Allocate)
		g.POST("reject", permitted, ctrl.Reject)
	}
	return ctrl
}

// ListApplications godoc
// @Summary List the applications filed in a launch phase
// @Description A page of the applications registrars filed in an application phase (ADR 0024 §3), oldest first; status narrows to one status (pendingValidation, pendingAllocation, allocated, rejected, invalid); pagesize and cursor page as every list does (the cursor is the one the previous page's meta gave). The request names the TLD's operator in X-Tenant-ID (another operator's TLD is 404), or holds registry:platform:admin.
// @Tags Launch applications
// @Produce json
// @Param tldName path string true "TLD name"
// @Param phaseName path string true "Phase name"
// @Param status query string false "One status"
// @Param pagesize query int false "Page size"
// @Param cursor query string false "The previous page's cursor"
// @Success 200 {object} response.ListItemResult
// @Failure 400
// @Failure 404
// @Router /tlds/{tldName}/phases/{phaseName}/applications [get]
func (ctrl *LaunchApplicationController) ListApplications(ctx *gin.Context) {
	scope, ok := RegistryScopeFromRequest(ctx)
	if !ok {
		return
	}
	pageSize, err := GetPageSize(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	pageCursor, err := GetAndDecodeCursor(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	filter := queries.ListApplicationsFilter{Status: ctx.Query("status")}
	apps, err := ctrl.svc.ListApplications(ctx.Request.Context(), scope, ctx.Param("tldName"), ctx.Param("phaseName"), entities.ApplicationStatus(filter.Status), pageSize, pageCursor)
	if err != nil {
		applicationError(ctx, err)
		return
	}
	if apps == nil {
		apps = []*entities.LaunchApplication{}
	}
	res := response.ListItemResult{Data: apps}
	if len(apps) > 0 {
		id, _ := apps[len(apps)-1].ID.Int64()
		res.SetMeta(ctx, strconv.FormatInt(id, 10), len(apps), pageSize, filter)
	}
	ctx.JSON(http.StatusOK, res)
}

// GetApplication godoc
// @Summary Get a launch application
// @Description One application by its ID, within the operator's scope.
// @Tags Launch applications
// @Produce json
// @Param applicationID path string true "Application ID"
// @Success 200 {object} entities.LaunchApplication
// @Failure 404
// @Router /launch-applications/{applicationID} [get]
func (ctrl *LaunchApplicationController) GetApplication(ctx *gin.Context) {
	scope, ok := RegistryScopeFromRequest(ctx)
	if !ok {
		return
	}
	app, err := ctrl.svc.GetApplication(ctx.Request.Context(), scope, ctx.Param("applicationID"))
	if err != nil {
		applicationError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, app)
}

// Validate godoc
// @Summary Record the validation of a launch application
// @Description Moves a pendingValidation application to pendingAllocation (valid) or ends it as invalid with the reason (valid false), and tells the applicant by poll message. Needs registry:launch:admin. An application in another status is 409.
// @Tags Launch applications
// @Accept json
// @Produce json
// @Param applicationID path string true "Application ID"
// @Param outcome body commands.ValidateApplicationCommand true "The outcome"
// @Success 200 {object} entities.LaunchApplication
// @Failure 400
// @Failure 403
// @Failure 404
// @Failure 409
// @Router /launch-applications/{applicationID}/validate [post]
func (ctrl *LaunchApplicationController) Validate(ctx *gin.Context) {
	var cmd commands.ValidateApplicationCommand
	if err := ctx.ShouldBindJSON(&cmd); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cmd.ApplicationID = ctx.Param("applicationID")
	scope, ok := RegistryScopeFromRequest(ctx)
	if !ok {
		return
	}
	app, err := ctrl.svc.Validate(ctx.Request.Context(), scope, &cmd)
	if err != nil {
		applicationError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, app)
}

// Allocate godoc
// @Summary Allocate a name to a launch application
// @Description Registers the name to a pendingAllocation application in its phase (which may have ended), under the quote from filing; rejects every other open application for the name; and tells each applicant by poll message, all in one transaction. Needs registry:launch:admin. An application in another status is 409; a name registered meanwhile is 409.
// @Tags Launch applications
// @Produce json
// @Param applicationID path string true "Application ID"
// @Success 200 {object} entities.LaunchApplication
// @Failure 403
// @Failure 404
// @Failure 409
// @Router /launch-applications/{applicationID}/allocate [post]
func (ctrl *LaunchApplicationController) Allocate(ctx *gin.Context) {
	scope, ok := RegistryScopeFromRequest(ctx)
	if !ok {
		return
	}
	app, err := ctrl.svc.Allocate(ctx.Request.Context(), scope, &commands.AllocateApplicationCommand{ApplicationID: ctx.Param("applicationID")})
	if err != nil {
		applicationError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, app)
}

// Reject godoc
// @Summary Reject a launch application
// @Description Ends a non-final application with the reason and tells the applicant by poll message. Needs registry:launch:admin. A final application is 409.
// @Tags Launch applications
// @Accept json
// @Produce json
// @Param applicationID path string true "Application ID"
// @Param reason body commands.RejectApplicationCommand true "The reason"
// @Success 200 {object} entities.LaunchApplication
// @Failure 400
// @Failure 403
// @Failure 404
// @Failure 409
// @Router /launch-applications/{applicationID}/reject [post]
func (ctrl *LaunchApplicationController) Reject(ctx *gin.Context) {
	var cmd commands.RejectApplicationCommand
	if err := ctx.ShouldBindJSON(&cmd); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cmd.ApplicationID = ctx.Param("applicationID")
	scope, ok := RegistryScopeFromRequest(ctx)
	if !ok {
		return
	}
	app, err := ctrl.svc.Reject(ctx.Request.Context(), scope, &cmd)
	if err != nil {
		applicationError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, app)
}

// applicationError maps the service's refusals to status codes: 404 for an
// application or TLD outside the scope or missing; 409 for a status that
// does not allow the decision, a name taken meanwhile, or a contact or
// nameserver the application names that is gone (the operator has the
// applicant repair it); 400 for bad input; 500 otherwise.
func applicationError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, entities.ErrApplicationNotFound), errors.Is(err, entities.ErrTLDNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, entities.ErrApplicationTransition), errors.Is(err, entities.ErrApplicationFinal),
		errors.Is(err, entities.ErrApplicationChanged), errors.Is(err, entities.ErrDomainAlreadyExists),
		errors.Is(err, entities.ErrDomainExists), errors.Is(err, entities.ErrApplicationContactGone),
		errors.Is(err, entities.ErrApplicationHostGone),
		// A host or contact gone between the allocation's check and its
		// write: the same repairable conflict, however it surfaces.
		errors.Is(err, entities.ErrHostNotFound), errors.Is(err, entities.ErrContactNotFound):
		ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, entities.ErrReasonRequired), errors.Is(err, entities.ErrDecisionRequired), errors.Is(err, entities.ErrInvalidApplication),
		errors.Is(err, entities.ErrInvalidDomain), errors.Is(err, entities.ErrInvalidRegistryScope),
		errors.Is(err, entities.ErrContactDataPolicyViolation):
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
