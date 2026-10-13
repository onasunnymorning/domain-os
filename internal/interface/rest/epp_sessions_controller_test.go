package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/netip"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/memstore"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

type recordingPublisher struct{ events []entities.DomainEvent }

func (p *recordingPublisher) Publish(_ context.Context, events ...entities.DomainEvent) error {
	p.events = append(p.events, events...)
	return nil
}

func eppSessionsRouter(t *testing.T, scopes ...string) (*gin.Engine, *memstore.EPPSessions, *recordingPublisher) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	reg := memstore.NewEPPSessions()
	pub := &recordingPublisher{}
	r := gin.New()
	grant := func(c *gin.Context) {
		ctx := appcontext.WithAuthScopes(c.Request.Context(), scopes)
		c.Request = c.Request.WithContext(appcontext.WithUserID(ctx, "ops@example.com"))
		c.Next()
	}
	NewEPPSessionsController(r, services.NewEPPSessionService(reg, entities.EPPSessionCaps{}, 0, pub), grant)
	return r, reg, pub
}

func seedSession(t *testing.T, reg *memstore.EPPSessions, id, clid string, cred uuid.UUID) {
	t.Helper()
	s := &entities.EPPSession{ID: id, Node: "node-a", ClientIP: netip.MustParseAddr("192.0.2.7"), ConnectedAt: time.Now()}
	if clid != "" {
		s.Authenticate(&entities.EPPAuthenticatedLogin{ClID: entities.ClIDType(clid), LoginID: entities.ClIDType(clid), PasswordID: cred, PinID: uuid.New()}, time.Now())
	}
	require.NoError(t, reg.Open(context.Background(), s, time.Minute, 100))
}

func TestEPPSessionsRequirePermission(t *testing.T) {
	r, _, _ := eppSessionsRouter(t, ScopeEPPCredentialsAdmin)
	w := serveEPPCredential(r, http.MethodGet, "/epp/sessions", "")
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), ScopeEPPSessionsAdmin)
}

func TestEPPSessionsListFiltersAndPages(t *testing.T) {
	r, reg, _ := eppSessionsRouter(t, ScopeEPPSessionsAdmin)
	for _, id := range []string{"S1", "S2", "S3"} {
		seedSession(t, reg, id, "rar-001", uuid.New())
	}
	seedSession(t, reg, "S4", "rar-002", uuid.New())

	w := serveEPPCredential(r, http.MethodGet, "/epp/sessions?clid=rar-001&pagesize=2", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var page struct {
		Meta struct{ PageCursor, NextLink string }
		Data []entities.EPPSession
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	require.Len(t, page.Data, 2)
	require.NotEmpty(t, page.Meta.PageCursor)
	require.Contains(t, page.Meta.NextLink, "clid=rar-001")

	w = serveEPPCredential(r, http.MethodGet, "/epp/sessions?clid=rar-001&pagesize=2&cursor="+page.Meta.PageCursor, "")
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	require.Len(t, page.Data, 1)
	require.Equal(t, "S3", page.Data[0].ID)

	w = serveEPPCredential(r, http.MethodGet, "/epp/sessions?client_ip=not-an-ip", "")
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestEPPSessionsKill(t *testing.T) {
	r, reg, pub := eppSessionsRouter(t, ScopeEPPSessionsAdmin)
	seedSession(t, reg, "S1", "rar-001", uuid.New())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	kills, err := reg.KillRequests(ctx)
	require.NoError(t, err)

	w := serveEPPCredential(r, http.MethodDelete, "/epp/sessions/S1", "")
	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
	require.Equal(t, "S1", <-kills)
	require.Len(t, pub.events, 1)
	require.Equal(t, entities.EventEPPSessionKilled, pub.events[0].Type)
	require.Equal(t, "ops@example.com", pub.events[0].Actor)
	require.Equal(t, "rar-001", pub.events[0].Subject)

	w = serveEPPCredential(r, http.MethodDelete, "/epp/sessions/S9", "")
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestEPPSessionsKillByCredential(t *testing.T) {
	r, reg, _ := eppSessionsRouter(t, ScopeEPPSessionsAdmin)
	cred := uuid.New()
	seedSession(t, reg, "S1", "rar-001", cred)
	seedSession(t, reg, "S2", "rar-001", cred)
	seedSession(t, reg, "S3", "rar-001", uuid.New())

	w := serveEPPCredential(r, http.MethodDelete, "/epp/sessions?credential_id="+cred.String(), "")
	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
	var got struct{ Killed int }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, 2, got.Killed)

	w = serveEPPCredential(r, http.MethodDelete, "/epp/sessions", "")
	require.Equal(t, http.StatusBadRequest, w.Code, "an empty filter would end every session")
}

func TestEPPSessionsRegistryUnavailable(t *testing.T) {
	r, reg, _ := eppSessionsRouter(t, ScopeEPPSessionsAdmin)
	reg.Fail(errors.New("redis down"))
	w := serveEPPCredential(r, http.MethodGet, "/epp/sessions", "")
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.NotContains(t, w.Body.String(), "redis down", "internal detail stays in the logs")
}
