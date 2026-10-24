package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

type fakeSuspensionService struct{ lifted []string }

func (fakeSuspensionService) List(_ context.Context, clid string) ([]entities.EPPLoginSuspension, error) {
	return []entities.EPPLoginSuspension{{ClID: entities.ClIDType(clid), Login: "login-a", Scope: "registrar"}}, nil
}

func (f *fakeSuspensionService) Lift(_ context.Context, clid, login string) (*entities.EPPLoginSuspension, error) {
	if login != "login-a" {
		return nil, entities.ErrEPPSuspensionNotFound
	}
	f.lifted = append(f.lifted, clid+"/"+login)
	return &entities.EPPLoginSuspension{ClID: entities.ClIDType(clid), Login: login, Scope: "registrar"}, nil
}

func suspensionsRouter(svc *fakeSuspensionService, scopes ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewEPPSuspensionsController(r, svc, func(c *gin.Context) {
		c.Request = c.Request.WithContext(appcontext.WithAuthScopes(c.Request.Context(), scopes))
		c.Next()
	})
	return r
}

func TestEPPSuspensionsAPI(t *testing.T) {
	svc := &fakeSuspensionService{}
	r := suspensionsRouter(svc, ScopeEPPPolicyAdmin)

	w := do(r, http.MethodGet, "/epp/suspensions?clid=rar-a", "")
	require.Equal(t, http.StatusOK, w.Code)
	var got []entities.EPPLoginSuspension
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, entities.ClIDType("rar-a"), got[0].ClID)

	w = do(r, http.MethodDelete, "/registrars/rar-a/epp/logins/login-a/suspension", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, []string{"rar-a/login-a"}, svc.lifted)
	assert.Equal(t, http.StatusNotFound, do(r, http.MethodDelete, "/registrars/rar-a/epp/logins/other/suspension", "").Code)

	denied := suspensionsRouter(svc, ScopeEPPSessionsAdmin)
	assert.Equal(t, http.StatusForbidden, do(denied, http.MethodGet, "/epp/suspensions", "").Code)
	assert.Equal(t, http.StatusForbidden, do(denied, http.MethodDelete, "/registrars/rar-a/epp/logins/login-a/suspension", "").Code)
}
