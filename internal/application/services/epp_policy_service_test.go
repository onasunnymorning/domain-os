package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/memstore"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// knownRegistrars is a registrar repository that knows only these ClIDs.
type knownRegistrars struct {
	repositories.RegistrarRepository
	clids map[string]bool
}

func (k knownRegistrars) GetByClID(_ context.Context, clid string, _ bool) (*entities.Registrar, error) {
	if !k.clids[clid] {
		return nil, entities.ErrRegistrarNotFound
	}
	return &entities.Registrar{ClID: entities.ClIDType(clid)}, nil
}

func policyTestService() (*EPPPolicyService, *memstore.EPPPolicies, context.Context) {
	repo := memstore.NewEPPPolicies()
	svc := NewEPPPolicyService(repo, knownRegistrars{clids: map[string]bool{"rar-a": true, "rar-b": true, "rar-c": true}}, memstore.NewEPPCreateOutcomes())
	return svc, repo, appcontext.WithUserID(context.Background(), "ops@example.com")
}

func TestEPPPolicyServicePolicies(t *testing.T) {
	svc, repo, ctx := policyTestService()
	limits := entities.DefaultEPPPolicyLimits()
	limits.SessionsPerRegistrar, limits.SessionsPerLogin, limits.SessionsPerFamily, limits.CheckNames = 20, 5, 60, 25

	t.Run("the built-in policies are there before anything is stored", func(t *testing.T) {
		ps, err := svc.ListPolicies(ctx)
		require.NoError(t, err)
		require.Len(t, ps, 2)
		assert.Equal(t, entities.EPPPolicyDefault, ps[0].Name)
		assert.Equal(t, entities.DefaultEPPPolicyLimits(), ps[0].Limits)
		p, err := svc.GetPolicy(ctx, entities.EPPPolicyProbation)
		require.NoError(t, err)
		assert.Equal(t, entities.ProbationEPPPolicyLimits(), p.Limits)
	})
	t.Run("saved, with its event and actor", func(t *testing.T) {
		p, err := svc.SavePolicy(ctx, "gold", commands.SaveEPPPolicyCommand{Description: "Large registrars", Limits: limits})
		require.NoError(t, err)
		assert.Equal(t, "ops@example.com", p.UpdatedBy)
		require.Len(t, repo.Events, 1)
		e := repo.Events[0]
		assert.Equal(t, entities.EventEPPPolicySaved, e.Type)
		assert.Equal(t, "ops@example.com", e.Actor)
		assert.Equal(t, "gold", e.Subject)
	})
	t.Run("refused: a limit below 1, or a bad name", func(t *testing.T) {
		bad := limits
		bad.CheckNames = 0
		_, err := svc.SavePolicy(ctx, "silver", commands.SaveEPPPolicyCommand{Limits: bad})
		assert.ErrorIs(t, err, entities.ErrInvalidEPPPolicy)
		_, err = svc.SavePolicy(ctx, "Gold Plus", commands.SaveEPPPolicyCommand{Limits: limits})
		assert.ErrorIs(t, err, entities.ErrInvalidEPPPolicy)
	})
	t.Run("the built-in policies' limits can change, but they cannot be deleted", func(t *testing.T) {
		_, err := svc.SavePolicy(ctx, entities.EPPPolicyDefault, commands.SaveEPPPolicyCommand{Limits: limits})
		require.NoError(t, err)
		p, err := svc.GetPolicy(ctx, entities.EPPPolicyDefault)
		require.NoError(t, err)
		assert.Equal(t, limits, p.Limits)
		assert.ErrorIs(t, svc.DeletePolicy(ctx, entities.EPPPolicyDefault), entities.ErrEPPPolicyBuiltIn)
	})
	t.Run("a policy in use is not deleted", func(t *testing.T) {
		_, err := svc.AssignPolicy(ctx, "rar-a", commands.AssignEPPPolicyCommand{Policy: "gold"})
		require.NoError(t, err)
		assert.ErrorIs(t, svc.DeletePolicy(ctx, "gold"), entities.ErrEPPPolicyInUse)
		_, err = svc.AssignPolicy(ctx, "rar-a", commands.AssignEPPPolicyCommand{Policy: entities.EPPPolicyDefault})
		require.NoError(t, err)
		require.NoError(t, svc.DeletePolicy(ctx, "gold"))
		assert.ErrorIs(t, svc.DeletePolicy(ctx, "gold"), entities.ErrEPPPolicyNotFound)
	})
}

func TestEPPPolicyServiceAssignmentsAndFamilies(t *testing.T) {
	svc, repo, ctx := policyTestService()
	gold := entities.DefaultEPPPolicyLimits()
	gold.SessionsPerRegistrar, gold.SessionsPerLogin, gold.SessionsPerFamily, gold.CheckNames = 20, 5, 3, 25
	_, err := svc.SavePolicy(ctx, "gold", commands.SaveEPPPolicyCommand{Limits: gold})
	require.NoError(t, err)

	t.Run("a registrar has default until assigned", func(t *testing.T) {
		rp, err := svc.RegistrarPolicy(ctx, "rar-a")
		require.NoError(t, err)
		assert.Equal(t, entities.EPPPolicyDefault, rp.Policy.Name)
		assert.Nil(t, rp.Family)
	})
	t.Run("assigned", func(t *testing.T) {
		rp, err := svc.AssignPolicy(ctx, "rar-a", commands.AssignEPPPolicyCommand{Policy: "gold"})
		require.NoError(t, err)
		assert.Equal(t, "gold", rp.Policy.Name)
		e := repo.Events[len(repo.Events)-1]
		assert.Equal(t, entities.EventEPPPolicyAssigned, e.Type)
		assert.Equal(t, entities.EPPPolicyAssignment{ClID: "rar-a", Policy: ""}, e.BeforeState)
	})
	t.Run("refused: an unknown registrar or policy", func(t *testing.T) {
		_, err := svc.AssignPolicy(ctx, "nobody", commands.AssignEPPPolicyCommand{Policy: "gold"})
		assert.ErrorIs(t, err, entities.ErrRegistrarNotFound)
		_, err = svc.AssignPolicy(ctx, "rar-b", commands.AssignEPPPolicyCommand{Policy: "platinum"})
		assert.ErrorIs(t, err, entities.ErrEPPPolicyNotFound)
	})
	t.Run("a family, under its own policy", func(t *testing.T) {
		f, err := svc.SaveFamily(ctx, "acme", commands.SaveRegistrarFamilyCommand{Policy: "gold", Members: []entities.ClIDType{"rar-b", "rar-a", "rar-a"}})
		require.NoError(t, err)
		assert.Equal(t, []entities.ClIDType{"rar-a", "rar-b"}, f.Members, "sorted, without repeats")
		rp, err := svc.RegistrarPolicy(ctx, "rar-b")
		require.NoError(t, err)
		assert.Equal(t, entities.EPPPolicyDefault, rp.Policy.Name, "its own policy is unchanged")
		require.NotNil(t, rp.Family)
		assert.Equal(t, "acme", rp.Family.Name)
		assert.Equal(t, "gold", rp.FamilyPolicy.Name)
		limits := rp.SessionLimits(0)
		assert.Equal(t, entities.EPPSessionLimits{PerRegistrar: 10, PerLogin: 10, Family: "acme", PerFamily: 3}, limits)
		assert.Equal(t, 4, rp.SessionLimits(4).PerLogin, "a login's own cap replaces the policy's")
	})
	t.Run("refused: a member of another family, an unknown member or policy", func(t *testing.T) {
		_, err := svc.SaveFamily(ctx, "other", commands.SaveRegistrarFamilyCommand{Members: []entities.ClIDType{"rar-a"}})
		assert.ErrorIs(t, err, entities.ErrRegistrarInFamily)
		_, err = svc.SaveFamily(ctx, "other", commands.SaveRegistrarFamilyCommand{Members: []entities.ClIDType{"nobody"}})
		assert.ErrorIs(t, err, entities.ErrRegistrarNotFound)
		_, err = svc.SaveFamily(ctx, "other", commands.SaveRegistrarFamilyCommand{Policy: "platinum"})
		assert.ErrorIs(t, err, entities.ErrEPPPolicyNotFound)
	})
	t.Run("a family's policy is in use", func(t *testing.T) {
		_, err := svc.AssignPolicy(ctx, "rar-a", commands.AssignEPPPolicyCommand{})
		require.NoError(t, err)
		assert.ErrorIs(t, svc.DeletePolicy(ctx, "gold"), entities.ErrEPPPolicyInUse)
	})
	t.Run("deleted, its members belong to none", func(t *testing.T) {
		require.NoError(t, svc.DeleteFamily(ctx, "acme"))
		rp, err := svc.RegistrarPolicy(ctx, "rar-b")
		require.NoError(t, err)
		assert.Nil(t, rp.Family)
		assert.ErrorIs(t, svc.DeleteFamily(ctx, "acme"), entities.ErrRegistrarFamilyNotFound)
	})
}
