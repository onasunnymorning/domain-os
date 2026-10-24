package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/memstore"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

type fakeLockouts struct {
	list   []entities.EPPLoginLockout
	lifted []string
}

func (f *fakeLockouts) ListLockouts(context.Context) ([]entities.EPPLoginLockout, error) {
	return f.list, nil
}

func (f *fakeLockouts) LiftLockouts(_ context.Context, login string) ([]entities.EPPLoginLockout, error) {
	f.lifted = append(f.lifted, login)
	return nil, nil
}

func TestEPPLockoutService(t *testing.T) {
	ctx := appcontext.WithUserID(context.Background(), "ops@example.com")
	creds := memstore.NewEPPCredentials("rar-a", "rar-b")
	for clid, login := range map[string]string{"rar-a": "login-a", "rar-b": "login-b"} {
		_, err := creds.UpdateAccess(ctx, clid, func(a *entities.EPPRegistrarAccess) ([]entities.DomainEvent, error) {
			_, err := a.AddLogin(login, entities.EPPLoginRoleTransform, "test", time.Now())
			return nil, err
		})
		require.NoError(t, err)
	}
	store := &fakeLockouts{list: []entities.EPPLoginLockout{
		{Login: "login-a", ClientIP: "192.0.2.1"}, {Login: "login-a", ClientIP: "2001:db8::1"},
		{Login: "login-b", ClientIP: "192.0.2.1"}, {Login: "deleted-login", ClientIP: "192.0.2.9"},
	}}
	events := &recordingPublisher{}
	svc := NewEPPLockoutService(store, creds, events)

	all, err := svc.List(ctx, "")
	require.NoError(t, err)
	require.Len(t, all, 3, "a login that no longer exists is left out")
	assert.Equal(t, entities.ClIDType("rar-a"), all[0].ClID, "each named with its registrar")
	mine, _ := svc.List(ctx, "rar-b")
	require.Len(t, mine, 1)

	_, err = svc.Lift(ctx, "rar-b", "login-a")
	assert.ErrorIs(t, err, entities.ErrEPPLoginNotFound)

	events.err = errors.New("database down")
	_, err = svc.Lift(ctx, "rar-a", "login-a")
	require.Error(t, err)
	assert.Empty(t, store.lifted, "nothing lifted without its event")
	events.err = nil

	lifted, err := svc.Lift(ctx, "rar-a", "login-a")
	require.NoError(t, err)
	assert.Len(t, lifted, 2, "every source")
	assert.Equal(t, []string{"login-a"}, store.lifted)
	require.Len(t, events.events, 1)
	assert.Equal(t, entities.EventEPPLoginLockoutLifted, events.events[0].Type)
	assert.Equal(t, "ops@example.com", events.events[0].Actor)

	store.list = nil
	_, err = svc.Lift(ctx, "rar-a", "login-a")
	assert.ErrorIs(t, err, entities.ErrEPPLockoutNotFound)
}
