package services

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/memstore"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// allocationRegistry is applicationRegistry with the TLD's operator set,
// two applications for bid.test (one per registrar) and one for
// other.test, all pendingAllocation, and the allocation service over the
// registry.
func allocationRegistry(t *testing.T) (*LaunchAllocationService, *memstore.Registry, []*entities.LaunchApplication) {
	t.Helper()
	epp, reg := applicationRegistry(t)
	reg.SetOperator("test", "ry-one")
	ctx := context.Background()
	var apps []*entities.LaunchApplication
	for _, f := range []struct {
		scope entities.RegistrarClID
		name  string
	}{{rar1, "bid.test"}, {rar2, "bid.test"}, {rar1, "other.test"}} {
		cmd := applicationCreate(f.name, "application")
		if f.scope == rar2 {
			cmd.RegistrantID, cmd.HostNames = "rar-two-c", []string{"ns1.rar-two.net"}
			cmd.Contacts = []commands.EPPDomainContact{{Type: "tech", ID: "rar-two-c"}}
		}
		res, err := epp.Create(ctx, f.scope, cmd)
		require.NoError(t, err)
		apps = append(apps, res.Application)
	}
	svc := NewLaunchAllocationService(reg.Applications(), reg.Phases(), reg.TLDs(), reg.Contacts(), reg.Hosts(), reg, passThroughTx{}, nil)
	return svc, reg, apps
}

var (
	ryOne   = entities.OperatorRegistryScope("ry-one")
	ryOther = entities.OperatorRegistryScope("ry-two")
)

func TestLaunchAllocation_Allocate(t *testing.T) {
	svc, reg, apps := allocationRegistry(t)
	ctx := context.Background()
	winner, loser, other := apps[0], apps[1], apps[2]

	allocated, err := svc.Allocate(ctx, ryOne, &commands.AllocateApplicationCommand{ApplicationID: winner.ID.String()})
	require.NoError(t, err)
	assert.Equal(t, entities.ApplicationStatusAllocated, allocated.Status)

	dom := reg.Domain("bid.test")
	require.NotNil(t, dom, "registered to the winner")
	assert.Equal(t, dom.RoID, allocated.DomainRoID)
	assert.Equal(t, entities.ClIDType(rar1), dom.ClID)
	assert.Equal(t, entities.ClIDType("auction"), dom.RegistrationPhaseName)
	assert.True(t, dom.AuthInfo.Matches("Tr4nsf3r-C0de-Xq9#Lm2!"), "the applicant's code")
	assert.Equal(t, []string{"bid.test"}, reg.Registered)

	rejected := reg.Application(loser.ID.String())
	assert.Equal(t, entities.ApplicationStatusRejected, rejected.Status, "the other registrar's application for the name")
	assert.Contains(t, rejected.Reason, "allocated to another application")
	assert.Equal(t, entities.ApplicationStatusPendingAllocation, reg.Application(other.ID.String()).Status, "another name is untouched")

	msgs1, msgs2 := reg.Messages(rar1.String()), reg.Messages(rar2.String())
	require.Len(t, msgs1, 1)
	assert.Equal(t, entities.PollMsgApplicationAllocated+" for bid.test", msgs1[0].Message)
	assert.Equal(t, entities.ApplicationStatusAllocated, msgs1[0].Application.Status)
	require.Len(t, msgs2, 1)
	assert.Equal(t, entities.ApplicationStatusRejected, msgs2[0].Application.Status)

	t.Run("not pendingAllocation", func(t *testing.T) {
		_, err := svc.Allocate(ctx, ryOne, &commands.AllocateApplicationCommand{ApplicationID: winner.ID.String()})
		assert.ErrorIs(t, err, entities.ErrApplicationTransition, "allocated already")
		_, err = svc.Allocate(ctx, ryOne, &commands.AllocateApplicationCommand{ApplicationID: loser.ID.String()})
		assert.ErrorIs(t, err, entities.ErrApplicationTransition, "rejected")
	})
	t.Run("the phase may have ended", func(t *testing.T) {
		ended := time.Now().UTC().Add(-time.Minute)
		reg.SetPhase("test", "auction", func(p *entities.Phase) { p.Ends = &ended })
		got, err := svc.Allocate(ctx, ryOne, &commands.AllocateApplicationCommand{ApplicationID: other.ID.String()})
		require.NoError(t, err)
		assert.Equal(t, entities.ApplicationStatusAllocated, got.Status)
		assert.NotNil(t, reg.Domain("other.test"))
	})
}

func TestLaunchAllocation_ValidateAndReject(t *testing.T) {
	svc, reg, apps := allocationRegistry(t)
	ctx := context.Background()
	app := apps[0]

	t.Run("validate needs pendingValidation", func(t *testing.T) {
		_, err := svc.Validate(ctx, ryOne, &commands.ValidateApplicationCommand{ApplicationID: app.ID.String(), Valid: boolPtr(true)})
		assert.ErrorIs(t, err, entities.ErrApplicationTransition)
	})
	t.Run("validated", func(t *testing.T) {
		reg.SetApplication(app.ID.String(), func(a *entities.LaunchApplication) { a.Status = entities.ApplicationStatusPendingValidation })
		got, err := svc.Validate(ctx, ryOne, &commands.ValidateApplicationCommand{ApplicationID: app.ID.String(), Valid: boolPtr(true)})
		require.NoError(t, err)
		assert.Equal(t, entities.ApplicationStatusPendingAllocation, got.Status)
		msgs := reg.Messages(rar1.String())
		require.Len(t, msgs, 1)
		assert.Equal(t, entities.PollMsgApplicationValidated+" for bid.test", msgs[0].Message)
	})
	t.Run("invalid needs a reason", func(t *testing.T) {
		reg.SetApplication(app.ID.String(), func(a *entities.LaunchApplication) { a.Status = entities.ApplicationStatusPendingValidation })
		_, err := svc.Validate(ctx, ryOne, &commands.ValidateApplicationCommand{ApplicationID: app.ID.String(), Reason: "not eligible"})
		assert.ErrorIs(t, err, ErrDecisionRequired, "an omitted outcome decides nothing")
		_, err = svc.Validate(ctx, ryOne, &commands.ValidateApplicationCommand{ApplicationID: app.ID.String(), Valid: boolPtr(false)})
		assert.ErrorIs(t, err, ErrReasonRequired)
		got, err := svc.Validate(ctx, ryOne, &commands.ValidateApplicationCommand{ApplicationID: app.ID.String(), Valid: boolPtr(false), Reason: "not eligible"})
		require.NoError(t, err)
		assert.Equal(t, entities.ApplicationStatusInvalid, got.Status)
		assert.Equal(t, "not eligible", got.Reason)
	})
	t.Run("reject", func(t *testing.T) {
		other := apps[2]
		_, err := svc.Reject(ctx, ryOne, &commands.RejectApplicationCommand{ApplicationID: other.ID.String()})
		assert.ErrorIs(t, err, ErrReasonRequired)
		got, err := svc.Reject(ctx, ryOne, &commands.RejectApplicationCommand{ApplicationID: other.ID.String(), Reason: "withdrawn by the operator"})
		require.NoError(t, err)
		assert.Equal(t, entities.ApplicationStatusRejected, got.Status)
		_, err = svc.Reject(ctx, ryOne, &commands.RejectApplicationCommand{ApplicationID: other.ID.String(), Reason: "again"})
		assert.ErrorIs(t, err, entities.ErrApplicationTransition, "final")
		assert.Nil(t, reg.Domain("other.test"))
	})
}

func TestLaunchAllocation_Scope(t *testing.T) {
	svc, _, apps := allocationRegistry(t)
	ctx := context.Background()
	id := apps[0].ID.String()

	_, err := svc.GetApplication(ctx, ryOther, id)
	assert.ErrorIs(t, err, entities.ErrTLDNotFound, "another operator's TLD")
	_, err = svc.GetApplication(ctx, entities.RegistryScope{}, id)
	assert.Error(t, err, "an invalid scope")
	got, err := svc.GetApplication(ctx, ryOne, id)
	require.NoError(t, err)
	assert.Equal(t, id, got.ID.String())
	got, err = svc.GetApplication(ctx, testPlatformScope, id)
	require.NoError(t, err)
	assert.Equal(t, id, got.ID.String())
	_, err = svc.GetApplication(ctx, ryOne, "999999_APP-TEST")
	assert.ErrorIs(t, err, entities.ErrApplicationNotFound)

	_, err = svc.Allocate(ctx, ryOther, &commands.AllocateApplicationCommand{ApplicationID: id})
	assert.ErrorIs(t, err, entities.ErrTLDNotFound)

	list, err := svc.ListApplications(ctx, ryOne, "test", "auction", "", 25, "")
	require.NoError(t, err)
	assert.Len(t, list, 3)
	list, err = svc.ListApplications(ctx, ryOne, "test", "auction", entities.ApplicationStatusAllocated, 25, "")
	require.NoError(t, err)
	assert.Empty(t, list)
	_, err = svc.ListApplications(ctx, ryOne, "test", "auction", "nope", 25, "")
	assert.ErrorIs(t, err, entities.ErrInvalidApplication)
	_, err = svc.ListApplications(ctx, ryOther, "test", "auction", "", 25, "")
	assert.ErrorIs(t, err, entities.ErrTLDNotFound)

	t.Run("pages by cursor", func(t *testing.T) {
		first, err := svc.ListApplications(ctx, ryOne, "test", "auction", "", 2, "")
		require.NoError(t, err)
		require.Len(t, first, 2)
		last, _ := first[1].ID.Int64()
		rest, err := svc.ListApplications(ctx, ryOne, "test", "auction", "", 2, strconv.FormatInt(last, 10))
		require.NoError(t, err)
		require.Len(t, rest, 1)
		assert.NotContains(t, []entities.RoidType{first[0].ID, first[1].ID}, rest[0].ID)
	})
}

func TestLaunchAllocation_ContactsAndValidation(t *testing.T) {
	svc, reg, apps := allocationRegistry(t)
	ctx := context.Background()

	t.Run("a contact deleted since filing refuses the allocation", func(t *testing.T) {
		n, err := reg.Contacts().CountContactLinks(ctx, "rar-one-c")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, n, int64(2), "open applications count as links, so the registrar cannot delete the contact itself")
		require.NoError(t, reg.Contacts().DeleteContactByID(ctx, "rar-one-c")) // an administrator's delete
		_, err = svc.Allocate(ctx, ryOne, &commands.AllocateApplicationCommand{ApplicationID: apps[0].ID.String()})
		assert.ErrorIs(t, err, entities.ErrApplicationContactGone)
		assert.Nil(t, reg.Domain("bid.test"))
		assert.Equal(t, entities.ApplicationStatusPendingAllocation, reg.Application(apps[0].ID.String()).Status, "nothing changed")
	})
	t.Run("a nameserver deleted since filing refuses the allocation", func(t *testing.T) {
		host := reg.Host("ns1.rar-two.net", rar2.String())
		require.NotNil(t, host)
		id, _ := host.RoID.Int64()
		n, err := reg.Hosts().GetHostAssociationCount(ctx, id)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, n, int64(1), "the open application counts as a link, so the registrar cannot delete the host itself")
		require.NoError(t, reg.Hosts().DeleteHostByRoid(ctx, id)) // an administrator's delete
		_, err = svc.Allocate(ctx, ryOne, &commands.AllocateApplicationCommand{ApplicationID: apps[1].ID.String()})
		assert.ErrorIs(t, err, entities.ErrApplicationHostGone)
		reg.AddHost("ns1.rar-two.net", rar2.String())
	})
	t.Run("a contact re-created by another registrar refuses the allocation", func(t *testing.T) {
		reg.AddContact("rar-one-c", rar2.String())
		_, err := svc.Allocate(ctx, ryOne, &commands.AllocateApplicationCommand{ApplicationID: apps[0].ID.String()})
		assert.ErrorIs(t, err, entities.ErrApplicationContactGone)
	})
	t.Run("an allocated domain is not pendingCreate, whatever the phase requires", func(t *testing.T) {
		reg.SetPolicy("test", func(p *entities.PhasePolicy) { v := true; p.RequiresValidation = &v })
		got, err := svc.Allocate(ctx, ryOne, &commands.AllocateApplicationCommand{ApplicationID: apps[1].ID.String()})
		require.NoError(t, err)
		assert.Equal(t, entities.ApplicationStatusAllocated, got.Status)
		dom := reg.Domain("bid.test")
		require.NotNil(t, dom)
		assert.False(t, dom.Status.PendingCreate, "validated as an application already")
		assert.True(t, dom.Status.OK)
	})
}

// failingPublisher refuses every event.
type failingPublisher struct{}

func (failingPublisher) Publish(context.Context, ...entities.DomainEvent) error {
	return errors.New("outbox unavailable")
}

func TestLaunchAllocation_PublishFailureFailsTheDecision(t *testing.T) {
	_, reg, apps := allocationRegistry(t)
	svc := NewLaunchAllocationService(reg.Applications(), reg.Phases(), reg.TLDs(), reg.Contacts(), reg.Hosts(), reg, passThroughTx{}, failingPublisher{})
	ctx := context.Background()
	_, err := svc.Reject(ctx, ryOne, &commands.RejectApplicationCommand{ApplicationID: apps[2].ID.String(), Reason: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outbox unavailable")
	_, err = svc.Allocate(ctx, ryOne, &commands.AllocateApplicationCommand{ApplicationID: apps[0].ID.String()})
	require.Error(t, err, "the publisher's failure reaches the transaction's finisher, which rolls the decision back on Postgres")
	assert.Contains(t, err.Error(), "outbox unavailable")
}

func boolPtr(b bool) *bool { return &b }
