package rest

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

type fakeLockoutService struct{}

func (fakeLockoutService) List(_ context.Context, clid string) ([]entities.EPPLoginLockout, error) {
	return []entities.EPPLoginLockout{{ClID: entities.ClIDType(clid), Login: "login-a"}}, nil
}

func (fakeLockoutService) Lift(_ context.Context, clid, login string) ([]entities.EPPLoginLockout, error) {
	if login != "login-a" {
		return nil, entities.ErrEPPLockoutNotFound
	}
	return []entities.EPPLoginLockout{{ClID: entities.ClIDType(clid), Login: login}}, nil
}

func TestEPPLockoutsAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := func(scopes ...string) *gin.Engine {
		r := gin.New()
		NewEPPLockoutsController(r, fakeLockoutService{}, func(c *gin.Context) {
			c.Request = c.Request.WithContext(appcontext.WithAuthScopes(c.Request.Context(), scopes))
			c.Next()
		})
		return r
	}
	r := router(ScopeEPPCredentialsAdmin)
	assert.Equal(t, http.StatusOK, do(r, http.MethodGet, "/epp/lockouts?clid=rar-a", "").Code)
	assert.Equal(t, http.StatusOK, do(r, http.MethodDelete, "/registrars/rar-a/epp/logins/login-a/lockouts", "").Code)
	assert.Equal(t, http.StatusNotFound, do(r, http.MethodDelete, "/registrars/rar-a/epp/logins/other/lockouts", "").Code)
	denied := router(ScopeEPPPolicyAdmin)
	assert.Equal(t, http.StatusForbidden, do(denied, http.MethodGet, "/epp/lockouts", "").Code)
}
