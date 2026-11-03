package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/memstore"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

func TestLifecyclePolicyService(t *testing.T) {
	repo := memstore.NewLifecyclePolicies()
	svc := NewLifecyclePolicyService(repo)
	ctx := appcontext.WithUserID(context.Background(), "ops@example.com")
	three := 3

	t.Run("gtld-standard is there from the start", func(t *testing.T) {
		ps, err := svc.ListPolicies(ctx)
		require.NoError(t, err)
		require.Len(t, ps, 1)
		assert.Equal(t, entities.LifecyclePolicyStandard, ps[0].Name)
		assert.Equal(t, entities.DefaultLifecycle(), ps[0].Resolved())
	})
	t.Run("created, with its event and actor", func(t *testing.T) {
		p, err := svc.SavePolicy(ctx, "short", commands.SaveLifecyclePolicyCommand{Description: "demo", Settings: entities.LifecycleSettings{RedemptionGP: &three}})
		require.NoError(t, err)
		assert.Equal(t, "ops@example.com", p.UpdatedBy)
		assert.Equal(t, 3, p.RedemptionDays())
		require.Len(t, repo.Events, 1)
		e := repo.Events[0]
		assert.Equal(t, entities.EventLifecyclePolicyCreated, e.Type)
		assert.Equal(t, "ops@example.com", e.Actor)
		assert.Nil(t, e.BeforeState)
		assert.Equal(t, p.Name, e.AfterState.(*entities.LifecyclePolicy).Name)
	})
	t.Run("updated, with before and after", func(t *testing.T) {
		p, err := svc.SavePolicy(ctx, "short", commands.SaveLifecyclePolicyCommand{Settings: entities.LifecycleSettings{RedemptionGP: &three, PendingDeleteGP: &three}})
		require.NoError(t, err)
		assert.Equal(t, 3, p.PendingDeleteDays())
		e := repo.Events[len(repo.Events)-1]
		assert.Equal(t, entities.EventLifecyclePolicyUpdated, e.Type)
		assert.Equal(t, entities.PendingDeleteGP, e.BeforeState.(*entities.LifecyclePolicy).PendingDeleteDays())
		assert.Equal(t, 3, e.AfterState.(*entities.LifecyclePolicy).PendingDeleteDays())
	})
	t.Run("refused: a bad name, a setting out of bounds, nothing saved", func(t *testing.T) {
		n := len(repo.Events)
		_, err := svc.SavePolicy(ctx, "Bad", commands.SaveLifecyclePolicyCommand{})
		assert.ErrorIs(t, err, entities.ErrInvalidLifecyclePolicy)
		big := 400
		_, err = svc.SavePolicy(ctx, "ok", commands.SaveLifecyclePolicyCommand{Settings: entities.LifecycleSettings{RedemptionGP: &big}})
		assert.ErrorIs(t, err, entities.ErrInvalidLifecyclePolicy)
		assert.Len(t, repo.Events, n)
	})
	t.Run("conditional saves", func(t *testing.T) {
		_, err := svc.SavePolicy(ctx, "short", commands.SaveLifecyclePolicyCommand{CreateOnly: true})
		assert.ErrorIs(t, err, entities.ErrLifecyclePolicyExists)
		stale := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		_, err = svc.SavePolicy(ctx, "short", commands.SaveLifecyclePolicyCommand{ExpectedUpdatedAt: &stale})
		assert.ErrorIs(t, err, entities.ErrLifecyclePolicyChanged)
		cur, err := svc.GetPolicy(ctx, "short")
		require.NoError(t, err)
		_, err = svc.SavePolicy(ctx, "short", commands.SaveLifecyclePolicyCommand{ExpectedUpdatedAt: &cur.UpdatedAt})
		assert.NoError(t, err)
	})
	t.Run("the built-in can be changed, not deleted", func(t *testing.T) {
		p, err := svc.SavePolicy(ctx, entities.LifecyclePolicyStandard, commands.SaveLifecyclePolicyCommand{Settings: entities.LifecycleSettings{RedemptionGP: &three}})
		require.NoError(t, err)
		assert.Equal(t, 3, p.RedemptionDays())
		assert.ErrorIs(t, svc.DeletePolicy(ctx, entities.LifecyclePolicyStandard), entities.ErrLifecyclePolicyBuiltIn)
	})
	t.Run("a read says which phases name the policy", func(t *testing.T) {
		repo.Uses["short"] = []entities.LifecyclePolicyUse{{TLD: "example", Phase: "ga-2026"}}
		p, err := svc.GetPolicy(ctx, "short")
		require.NoError(t, err)
		assert.Equal(t, repo.Uses["short"], p.UsedBy)
		assert.ErrorIs(t, svc.DeletePolicy(ctx, "short"), entities.ErrLifecyclePolicyInUse)
		repo.Uses["short"] = nil
		require.NoError(t, svc.DeletePolicy(ctx, "short"))
		assert.Equal(t, entities.EventLifecyclePolicyDeleted, repo.Events[len(repo.Events)-1].Type)
		_, err = svc.GetPolicy(ctx, "short")
		assert.ErrorIs(t, err, entities.ErrLifecyclePolicyNotFound)
	})
}
