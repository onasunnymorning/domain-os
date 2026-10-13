package rest

import (
	"errors"
	"net/http"
	"net/netip"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// EPPSessionsController shows who is connected over EPP and lets operators
// end sessions (docs/epp/ARCHITECTURE.md §3). Every endpoint needs the
// epp:sessions:admin permission.
type EPPSessionsController struct {
	svc interfaces.EPPSessionService
}

// NewEPPSessionsController registers the routes under /epp/sessions.
func NewEPPSessionsController(e *gin.Engine, svc interfaces.EPPSessionService, handler gin.HandlerFunc) *EPPSessionsController {
	ctrl := &EPPSessionsController{svc: svc}
	g := e.Group("/epp/sessions", handler, func(ctx *gin.Context) {
		if requireAuthScope(ctx, ScopeEPPSessionsAdmin) {
			ctx.Next()
		}
	})
	{
		g.GET("", ctrl.List)
		g.GET(":id", ctrl.Get)
		g.DELETE(":id", ctrl.Kill)
		g.DELETE("", ctrl.KillMatching)
	}
	return ctrl
}

// sessionFilter reads clid, login, client_ip and credential_id.
func sessionFilter(ctx *gin.Context) (queries.ListEPPSessionsFilter, error) {
	f := queries.ListEPPSessionsFilter{
		ClID:    ctx.Query("clid"),
		LoginID: ctx.Query("login"),
	}
	if ip := ctx.Query("client_ip"); ip != "" {
		addr, err := netip.ParseAddr(ip)
		if err != nil {
			return f, errors.New("client_ip is not an IP address")
		}
		f.ClientIP = addr.Unmap().String()
	}
	if raw := ctx.Query("credential_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return f, errors.New("credential_id is not a UUID")
		}
		f.CredentialID = id
	}
	return f, nil
}

// List godoc
// @Summary List live EPP sessions
// @Description Every live session on every node: who it is (registrar, login), where from, which node, which credentials it authenticated with, when it connected and logged in, and its command count and last command time (as of the node's last heartbeat). Filter by clid, login, client_ip or credential_id.
// @Tags EPP sessions
// @Produce json
// @Param clid query string false "Registrar client ID"
// @Param login query string false "EPP login ID"
// @Param client_ip query string false "Source address"
// @Param credential_id query string false "Password, pin or allowed network ID"
// @Param pagesize query int false "Page size"
// @Param cursor query string false "Cursor"
// @Success 200 {object} response.ListItemResult
// @Failure 400
// @Failure 403
// @Failure 503
// @Router /epp/sessions [get]
func (ctrl *EPPSessionsController) List(ctx *gin.Context) {
	var err error
	q := queries.ListItemsQuery{}
	if q.PageSize, err = GetPageSize(ctx); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if q.PageCursor, err = GetAndDecodeCursor(ctx); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	f, err := sessionFilter(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	q.Filter = f
	sessions, cursor, err := ctrl.svc.List(ctx.Request.Context(), q)
	if err != nil {
		eppSessionError(ctx, err)
		return
	}
	if sessions == nil {
		sessions = []*entities.EPPSession{}
	}
	res := response.ListItemResult{Data: sessions}
	res.SetMeta(ctx, cursor, len(sessions), q.PageSize, f)
	ctx.JSON(http.StatusOK, res)
}

// Get godoc
// @Summary Get a live EPP session
// @Tags EPP sessions
// @Produce json
// @Param id path string true "Session ID"
// @Success 200 {object} entities.EPPSession
// @Failure 404
// @Failure 503
// @Router /epp/sessions/{id} [get]
func (ctrl *EPPSessionsController) Get(ctx *gin.Context) {
	s, err := ctrl.svc.Get(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		eppSessionError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, s)
}

// Kill godoc
// @Summary End a live EPP session
// @Description Asks the node holding the session to close it. The request is recorded as an epp.session.kill_requested event first, and is not sent if it cannot be recorded. It is accepted, not confirmed: a node that cannot reach Redis at that moment misses it.
// @Tags EPP sessions
// @Produce json
// @Param id path string true "Session ID"
// @Success 202 {object} entities.EPPSession
// @Failure 404
// @Failure 503
// @Router /epp/sessions/{id} [delete]
func (ctrl *EPPSessionsController) Kill(ctx *gin.Context) {
	s, err := ctrl.svc.Kill(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		eppSessionError(ctx, err)
		return
	}
	ctx.JSON(http.StatusAccepted, s)
}

// KillMatching godoc
// @Summary End every live EPP session matching a filter
// @Description Ends every session of a registrar (clid), a login, a source address (client_ip), or that authenticated with a credential (credential_id): the second half of a compromise retire. At least one filter is required.
// @Tags EPP sessions
// @Produce json
// @Param clid query string false "Registrar client ID"
// @Param login query string false "EPP login ID"
// @Param client_ip query string false "Source address"
// @Param credential_id query string false "Password, pin or allowed network ID"
// @Success 202
// @Failure 400
// @Failure 503
// @Router /epp/sessions [delete]
func (ctrl *EPPSessionsController) KillMatching(ctx *gin.Context) {
	f, err := sessionFilter(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	killed, err := ctrl.svc.KillMatching(ctx.Request.Context(), f)
	if err != nil {
		eppSessionError(ctx, err)
		return
	}
	if killed == nil {
		killed = []*entities.EPPSession{}
	}
	ctx.JSON(http.StatusAccepted, gin.H{"killed": len(killed), "sessions": killed})
}

func eppSessionError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, entities.ErrEPPSessionNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"error": entities.ErrEPPSessionNotFound.Error()})
	case errors.Is(err, entities.ErrInvalidEPPSessionFilter):
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, entities.ErrEPPSessionsUnavailable):
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": entities.ErrEPPSessionsUnavailable.Error()})
	default:
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "the EPP session request failed"})
	}
}
