package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/memstore"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

const (
	rar1 = entities.RegistrarClID("rar-one")
	rar2 = entities.RegistrarClID("rar-two")
)

// objectRegistry seeds an in-memory registry: .test open, .closed not, both
// registrars accredited for .test, rar-one only for .closed; contacts and
// hosts for each.
func objectRegistry(t *testing.T) (*EPPDomainService, *memstore.Registry, *entities.Phase) {
	t.Helper()
	reg := memstore.NewRegistry()
	phase := reg.AddTLD("test", true)
	reg.AddTLD("closed", false)
	reg.Accredit(rar1.String(), "test")
	reg.Accredit(rar2.String(), "test")
	reg.Accredit(rar1.String(), "closed")
	for _, r := range []entities.RegistrarClID{rar1, rar2} {
		reg.AddContact(r.String()+"-c", r.String())
		reg.AddHost("ns1."+r.String()+".net", r.String())
	}
	reg.Reserve("held.test")
	svc := NewEPPDomainService(EPPDomainServiceDeps{
		Domains: reg.Domains(), NNDNs: reg.NNDNs(), TLDs: reg.TLDs(), Phases: reg.Phases(),
		Contacts: reg.Contacts(), Hosts: reg.Hosts(), Registrars: reg.Registrars(), Register: reg,
		CheckParallelism: 2,
	})
	return svc, reg, phase
}

func validCreate(name string) commands.EPPCreateDomainCommand {
	return commands.EPPCreateDomainCommand{
		Name: name, Years: 2, RegistrantID: "rar-one-c", AdminID: "rar-one-c", TechID: "rar-one-c",
		HostNames: []string{"ns1.rar-one.net"}, AuthInfo: "Ab3!Ab3!Ab3!xy",
	}
}

func TestEPPDomainCreate(t *testing.T) {
	svc, reg, _ := objectRegistry(t)
	dom, err := svc.Create(context.Background(), rar1, validCreate("new.test"))
	require.NoError(t, err)
	assert.Equal(t, entities.ClIDType(rar1), dom.ClID, "the sponsor is the session's registrar")
	assert.Equal(t, entities.ClIDType(rar1), dom.CrRr)
	assert.Equal(t, dom.CreatedAt.AddDate(2, 0, 0), dom.ExpiryDate)
	require.Len(t, dom.Hosts, 1)
	assert.Equal(t, []string{"new.test"}, reg.Registered)
}

func TestEPPDomainCreateRefusals(t *testing.T) {
	cases := []struct {
		name  string
		scope entities.RegistrarClID
		edit  func(*commands.EPPCreateDomainCommand)
		want  error
	}{
		{"invalid name", rar1, func(c *commands.EPPCreateDomainCommand) { c.Name = "-bad-.test" }, entities.ErrInvalidDomain},
		{"single label", rar1, func(c *commands.EPPCreateDomainCommand) { c.Name = "test" }, entities.ErrInvalidDomain},
		{"TLD not served", rar1, func(c *commands.EPPCreateDomainCommand) { c.Name = "x.nowhere" }, ErrEPPTLDNotServed},
		{"TLD not open", rar1, func(c *commands.EPPCreateDomainCommand) { c.Name = "x.closed" }, ErrEPPTLDNotOpen},
		{"zero years", rar1, func(c *commands.EPPCreateDomainCommand) { c.Years = 0 }, ErrEPPInvalidPeriod},
		{"eleven years", rar1, func(c *commands.EPPCreateDomainCommand) { c.Years = 11 }, ErrEPPInvalidPeriod},
		{"registered", rar1, func(c *commands.EPPCreateDomainCommand) { c.Name = "taken.test" }, ErrDomainExists},
		{"reserved", rar1, func(c *commands.EPPCreateDomainCommand) { c.Name = "held.test" }, ErrDomainBlocked},
		{"unknown contact", rar1, func(c *commands.EPPCreateDomainCommand) { c.AdminID = "nobody" }, entities.ErrContactNotFound},
		{"another registrar's contact", rar1, func(c *commands.EPPCreateDomainCommand) { c.TechID = "rar-two-c" }, ErrEPPNotSponsor},
		{"unknown host", rar1, func(c *commands.EPPCreateDomainCommand) { c.HostNames = []string{"ns9.nowhere.net"} }, entities.ErrHostNotFound},
		{"another registrar's host", rar1, func(c *commands.EPPCreateDomainCommand) { c.HostNames = []string{"ns1.rar-two.net"} }, entities.ErrHostNotFound},
		{"too many hosts", rar1, func(c *commands.EPPCreateDomainCommand) {
			c.HostNames = make([]string, entities.MaxHostsPerDomain+1)
		}, entities.ErrMaxHostsPerDomainExceeded},
		{"weak authInfo", rar1, func(c *commands.EPPCreateDomainCommand) { c.AuthInfo = "" }, entities.ErrInvalidAuthInfo},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, reg, _ := objectRegistry(t)
			reg.AddDomain(&entities.Domain{Name: "taken.test", ClID: "rar-two"})
			cmd := validCreate("new.test")
			tc.edit(&cmd)
			_, err := svc.Create(context.Background(), tc.scope, cmd)
			require.ErrorIs(t, err, tc.want)
			assert.Empty(t, reg.Registered, "nothing is registered")
		})
	}

	t.Run("a registrar not accredited for the TLD", func(t *testing.T) {
		svc, reg, _ := objectRegistry(t)
		cmd := validCreate("x.test")
		cmd.RegistrantID, cmd.AdminID, cmd.TechID, cmd.HostNames = "", "", "", nil
		reg.AddTLD("other", true)
		cmd.Name = "x.other"
		_, err := svc.Create(context.Background(), rar1, cmd)
		require.ErrorIs(t, err, ErrRegistrarNotAccredited)
	})
}

func TestEPPDomainCreateContactPolicy(t *testing.T) {
	svc, _, phase := objectRegistry(t)
	phase.Policy.ContactDataPolicy = entities.ContactDataPolicy{
		RegistrantContactDataPolicy: entities.ContactDataPolicyTypeMandatory,
		BillingContactDataPolicy:    entities.ContactDataPolicyTypeProhibited,
	}

	cmd := validCreate("policy.test")
	cmd.RegistrantID = ""
	_, err := svc.Create(context.Background(), rar1, cmd)
	require.ErrorIs(t, err, ErrEPPContactRequired)
	assert.ErrorContains(t, err, "registrant")

	cmd = validCreate("policy.test")
	cmd.BillingID = "rar-one-c"
	_, err = svc.Create(context.Background(), rar1, cmd)
	require.ErrorIs(t, err, ErrEPPContactProhibited, "a prohibited contact is refused, not silently dropped")
	assert.ErrorContains(t, err, "billing")
}

func TestEPPDomainCreateLostRace(t *testing.T) {
	svc, reg, _ := objectRegistry(t)
	// Another registrar registers the name between the checks and the
	// registration.
	reg.OnRegister(func(cmd *commands.RegisterDomainCommand) error {
		reg.AddDomain(&entities.Domain{Name: entities.DomainName(cmd.Name), ClID: "rar-two"})
		return errors.New("duplicate key value violates unique constraint")
	})
	_, err := svc.Create(context.Background(), rar1, validCreate("race.test"))
	require.ErrorIs(t, err, ErrDomainExists, "a lost race is reported as the name existing")
}

func TestEPPDomainInfo(t *testing.T) {
	svc, reg, _ := objectRegistry(t)
	_, err := svc.Create(context.Background(), rar1, validCreate("mine.test"))
	require.NoError(t, err)
	reg.AddHost("ns1.mine.test", rar1.String())
	reg.AddHost("ns2.mine.test", rar1.String())
	reg.AddHost("ns1.notmine.test", rar1.String())

	info, err := svc.Info(context.Background(), rar1, "MINE.test")
	require.NoError(t, err)
	assert.Equal(t, entities.DomainName("mine.test"), info.Domain.Name)
	assert.Equal(t, []string{"ns1.mine.test", "ns2.mine.test"}, info.SubordinateHosts)

	_, err = svc.Info(context.Background(), rar2, "mine.test")
	require.ErrorIs(t, err, ErrEPPNotSponsor, "ADR 0006: another registrar gets 2201")
	_, err = svc.Info(context.Background(), rar1, "absent.test")
	require.ErrorIs(t, err, entities.ErrDomainNotFound)
	_, err = svc.Info(context.Background(), rar1, "-bad-.test")
	require.ErrorIs(t, err, entities.ErrInvalidDomain)
	_, err = svc.Info(context.Background(), "", "mine.test")
	require.ErrorIs(t, err, entities.ErrInvalidRegistrarClID, "a missing scope is refused, never treated as all")
}
