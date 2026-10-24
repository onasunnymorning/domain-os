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

type fakeSuspensions struct {
	list   []entities.EPPLoginSuspension
	lifted []string
}

func (f *fakeSuspensions) ListSuspensions(context.Context) ([]entities.EPPLoginSuspension, error) {
	return f.list, nil
}

func (f *fakeSuspensions) LiftSuspension(_ context.Context, login string) (*entities.EPPLoginSuspension, error) {
	f.lifted = append(f.lifted, login)
	for _, s := range f.list {
		if s.Login == login {
			return &s, nil
		}
	}
	return nil, entities.ErrEPPSuspensionNotFound
}

type recordingPublisher struct {
	events []entities.DomainEvent
	err    error
}

func (p *recordingPublisher) Publish(_ context.Context, events ...entities.DomainEvent) error {
	if p.err != nil {
		return p.err
	}
	p.events = append(p.events, events...)
	return nil
}

func TestEPPSuspensionService(t *testing.T) {
	ctx := appcontext.WithUserID(context.Background(), "ops@example.com")
	creds := memstore.NewEPPCredentials("rar-a", "rar-b")
	_, err := creds.UpdateAccess(ctx, "rar-a", func(a *entities.EPPRegistrarAccess) ([]entities.DomainEvent, error) {
		_, err := a.AddLogin("login-a", entities.EPPLoginRoleTransform, "test", time.Now())
		return nil, err
	})
	require.NoError(t, err)
	store := &fakeSuspensions{list: []entities.EPPLoginSuspension{
		{ClID: "rar-a", Login: "login-a", Scope: "registrar"},
		{ClID: "rar-b", Login: "login-b", Scope: "family", Family: "acme"},
	}}
	events := &recordingPublisher{}
	svc := NewEPPSuspensionService(store, creds, events)

	all, err := svc.List(ctx, "")
	require.NoError(t, err)
	assert.Len(t, all, 2)
	mine, _ := svc.List(ctx, "rar-a")
	assert.Equal(t, []entities.EPPLoginSuspension{store.list[0]}, mine)

	_, err = svc.Lift(ctx, "rar-b", "login-a")
	assert.ErrorIs(t, err, entities.ErrEPPLoginNotFound, "another registrar's login")
	assert.Empty(t, store.lifted)

	t.Run("an event that cannot be written lifts nothing", func(t *testing.T) {
		events.err = errors.New("database down")
		_, err := svc.Lift(ctx, "rar-a", "login-a")
		require.Error(t, err)
		assert.Empty(t, store.lifted)
		events.err = nil
	})

	lifted, err := svc.Lift(ctx, "rar-a", "login-a")
	require.NoError(t, err)
	assert.Equal(t, "login-a", lifted.Login)
	assert.Equal(t, []string{"login-a"}, store.lifted)
	require.Len(t, events.events, 1)
	assert.Equal(t, entities.EventEPPLoginSuspensionLifted, events.events[0].Type)
	assert.Equal(t, "ops@example.com", events.events[0].Actor)

	store.list = nil
	_, err = svc.Lift(ctx, "rar-a", "login-a")
	assert.ErrorIs(t, err, entities.ErrEPPSuspensionNotFound)
}
