package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
		Name: name, Period: 2, PeriodUnit: "y", RegistrantID: "rar-one-c",
		Contacts:  []commands.EPPDomainContact{{Type: "admin", ID: "rar-one-c"}, {Type: "tech", ID: "rar-one-c"}},
		HostNames: []string{"ns1.rar-one.net"}, AuthInfo: "Ab3!Ab3!Ab3!xy",
	}
}

const digest256 = "49FD46E6C4B45C55D4AC69CBD3CD34AC1AFE51DE3DC8B27C3F72E84BC2E1D3F4"

func ds(alg, digestType int, digest string) commands.EPPDSData {
	return commands.EPPDSData{KeyTag: 2371, Algorithm: alg, DigestType: digestType, Digest: digest}
}

func TestEPPDomainCreate(t *testing.T) {
	svc, reg, _ := objectRegistry(t)
	cmd := validCreate("new.test")
	cmd.DSData = []commands.EPPDSData{ds(13, 2, strings.ToLower(digest256)), ds(15, 4, strings.Repeat("ab", 48))}
	dom, err := svc.Create(context.Background(), rar1, cmd)
	require.NoError(t, err)
	assert.Equal(t, []entities.DSRecord{
		{KeyTag: 2371, Algorithm: 13, DigestType: 2, Digest: digest256},
		{KeyTag: 2371, Algorithm: 15, DigestType: 4, Digest: strings.Repeat("AB", 48)},
	}, dom.DSRecords, "registered with its DS records, digests in upper case")
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
		{"TLD not served", rar1, func(c *commands.EPPCreateDomainCommand) { c.Name = "x.nowhere" }, entities.ErrEPPTLDNotServed},
		{"TLD not open", rar1, func(c *commands.EPPCreateDomainCommand) { c.Name = "x.closed" }, entities.ErrEPPTLDNotOpen},
		{"eleven years", rar1, func(c *commands.EPPCreateDomainCommand) { c.Period = 11 }, entities.ErrEPPInvalidPeriod},
		{"121 months", rar1, func(c *commands.EPPCreateDomainCommand) { c.Period, c.PeriodUnit = 120+1, "m" }, entities.ErrEPPPeriodNotWholeYears},
		{"132 months", rar1, func(c *commands.EPPCreateDomainCommand) { c.Period, c.PeriodUnit = 132, "m" }, entities.ErrEPPInvalidPeriod},
		{"registered", rar1, func(c *commands.EPPCreateDomainCommand) { c.Name = "taken.test" }, ErrDomainExists},
		{"reserved", rar1, func(c *commands.EPPCreateDomainCommand) { c.Name = "held.test" }, ErrDomainBlocked},
		{"a contact without a type", rar1, func(c *commands.EPPCreateDomainCommand) { c.Contacts[0].Type = "" }, entities.ErrEPPContactTypeMissing},
		{"two contacts of one type", rar1, func(c *commands.EPPCreateDomainCommand) { c.Contacts[0].Type = "tech" }, entities.ErrEPPDuplicateContactType},
		{"unknown contact", rar1, func(c *commands.EPPCreateDomainCommand) { c.Contacts[0].ID = "nobody" }, entities.ErrContactNotFound},
		{"another registrar's contact", rar1, func(c *commands.EPPCreateDomainCommand) { c.Contacts[1].ID = "rar-two-c" }, entities.ErrEPPNotSponsor},
		{"host attributes", rar1, func(c *commands.EPPCreateDomainCommand) { c.HostNames, c.HostAttributes = nil, true }, entities.ErrEPPHostAttributes},
		{"a host named twice", rar1, func(c *commands.EPPCreateDomainCommand) {
			c.HostNames = []string{"ns1.rar-one.net", "NS1.rar-one.net"}
		}, entities.ErrDuplicateHost},
		{"unknown host", rar1, func(c *commands.EPPCreateDomainCommand) { c.HostNames = []string{"ns9.nowhere.net"} }, entities.ErrHostNotFound},
		{"another registrar's host", rar1, func(c *commands.EPPCreateDomainCommand) { c.HostNames = []string{"ns1.rar-two.net"} }, entities.ErrHostNotFound},
		{"too many hosts", rar1, func(c *commands.EPPCreateDomainCommand) {
			c.HostNames = nil
			for i := range entities.MaxHostsPerDomain + 1 {
				c.HostNames = append(c.HostNames, fmt.Sprintf("ns%d.rar-one.net", i))
			}
		}, entities.ErrMaxHostsPerDomainExceeded},
		{"weak authInfo", rar1, func(c *commands.EPPCreateDomainCommand) { c.AuthInfo = "" }, entities.ErrInvalidAuthInfo},
		{"maxSigLife", rar1, func(c *commands.EPPCreateDomainCommand) { c.MaxSigLife = 604800 }, entities.ErrEPPMaxSigLife},
		{"key data", rar1, func(c *commands.EPPCreateDomainCommand) { c.KeyData = true }, entities.ErrEPPKeyData},
		{"RSASHA1 DS", rar1, func(c *commands.EPPCreateDomainCommand) { c.DSData = []commands.EPPDSData{ds(5, 2, digest256)} }, entities.ErrEPPDSAlgorithm},
		{"SHA-1 digest", rar1, func(c *commands.EPPCreateDomainCommand) { c.DSData = []commands.EPPDSData{ds(13, 1, digest256[:40])} }, entities.ErrEPPDSDigestType},
		{"short digest", rar1, func(c *commands.EPPCreateDomainCommand) { c.DSData = []commands.EPPDSData{ds(13, 2, digest256[:40])} }, entities.ErrInvalidDSRecord},
		{"key tag out of range", rar1, func(c *commands.EPPCreateDomainCommand) {
			d := ds(13, 2, digest256)
			d.KeyTag = 70000
			c.DSData = []commands.EPPDSData{d}
		}, entities.ErrInvalidDSRecord},
		{"nine DS", rar1, func(c *commands.EPPCreateDomainCommand) {
			for range MaxDSRecords + 1 {
				c.DSData = append(c.DSData, ds(13, 2, digest256))
			}
		}, entities.ErrEPPTooManyDS},
		{"a malformed DS record is reported before an unaccepted algorithm in another", rar1, func(c *commands.EPPCreateDomainCommand) {
			c.DSData = []commands.EPPDSData{ds(5, 2, digest256), ds(13, 2, digest256[:40])}
		}, entities.ErrInvalidDSRecord},
		{"duplicate DS", rar1, func(c *commands.EPPCreateDomainCommand) {
			c.DSData = []commands.EPPDSData{ds(13, 2, digest256), ds(13, 2, strings.ToLower(digest256))}
		}, entities.ErrDuplicateDSRecord},
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
		cmd.RegistrantID, cmd.Contacts, cmd.HostNames = "", nil, nil
		reg.AddTLD("other", true)
		cmd.Name = "x.other"
		_, err := svc.Create(context.Background(), rar1, cmd)
		require.ErrorIs(t, err, ErrRegistrarNotAccredited)
	})
}

// TestEPPDomainCreateRefusalOrder pins that of several problems the first
// in Create's documented order is reported.
func TestEPPDomainCreateRefusalOrder(t *testing.T) {
	svc, reg, _ := objectRegistry(t)
	reg.AddTLD("open", true) // nobody is accredited for it
	bad := func(edit func(*commands.EPPCreateDomainCommand)) commands.EPPCreateDomainCommand {
		c := validCreate("order.test")
		c.Period, c.PeriodUnit = 13, "m"                                                  // not whole years
		c.HostAttributes = true                                                           // host attributes
		c.MaxSigLife = 1                                                                  // an unsupported DNSSEC option
		c.Contacts = append(c.Contacts, commands.EPPDomainContact{Type: "tech", ID: "x"}) // a second tech
		edit(&c)
		return c
	}
	for _, tc := range []struct {
		name  string
		scope entities.RegistrarClID
		edit  func(*commands.EPPCreateDomainCommand)
		want  error
	}{
		{"an invalid name comes first", rar1, func(c *commands.EPPCreateDomainCommand) { c.Name = "-bad-.test" }, entities.ErrInvalidDomain},
		{"then the TLD", rar1, func(c *commands.EPPCreateDomainCommand) { c.Name = "x.closed" }, entities.ErrEPPTLDNotOpen},
		{"then accreditation", rar1, func(c *commands.EPPCreateDomainCommand) { c.Name = "x.open" }, ErrRegistrarNotAccredited},
		{"then the period", rar1, func(*commands.EPPCreateDomainCommand) {}, entities.ErrEPPPeriodNotWholeYears},
		{"then the contacts", rar1, func(c *commands.EPPCreateDomainCommand) { c.Period = 12 }, entities.ErrEPPDuplicateContactType},
		{"then the nameservers", rar1, func(c *commands.EPPCreateDomainCommand) { c.Period, c.Contacts = 12, c.Contacts[:2] }, entities.ErrEPPHostAttributes},
		{"then the DNSSEC data", rar1, func(c *commands.EPPCreateDomainCommand) {
			c.Period, c.Contacts, c.HostAttributes, c.AuthInfo = 12, c.Contacts[:2], false, "weak"
		}, entities.ErrEPPMaxSigLife},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Create(context.Background(), tc.scope, bad(tc.edit))
			require.ErrorIs(t, err, tc.want)
		})
	}
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
	require.ErrorIs(t, err, entities.ErrEPPContactRequired)
	assert.ErrorContains(t, err, "registrant")

	cmd = validCreate("policy.test")
	cmd.Contacts = append(cmd.Contacts, commands.EPPDomainContact{Type: "billing", ID: "rar-one-c"})
	_, err = svc.Create(context.Background(), rar1, cmd)
	require.ErrorIs(t, err, entities.ErrEPPContactProhibited, "a prohibited contact is refused, not silently dropped")
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
	require.ErrorIs(t, err, entities.ErrEPPNotSponsor, "ADR 0006: another registrar gets 2201")
	_, err = svc.Info(context.Background(), rar1, "absent.test")
	require.ErrorIs(t, err, entities.ErrDomainNotFound)
	_, err = svc.Info(context.Background(), rar1, "-bad-.test")
	require.ErrorIs(t, err, entities.ErrInvalidDomain)
	_, err = svc.Info(context.Background(), "", "mine.test")
	require.ErrorIs(t, err, entities.ErrInvalidRegistrarClID, "a missing scope is refused, never treated as all")
}
