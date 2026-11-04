package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/memstore"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// applicationRegistry is objectRegistry with a landrush phase that
// collects applications ("auction", priced 1500 USD a year, labels of
// three or more) and a first-come open phase ("rush").
func applicationRegistry(t *testing.T) (*EPPDomainService, *memstore.Registry) {
	t.Helper()
	svc, reg, _ := objectRegistry(t)
	now := time.Now().UTC()
	auction := reg.AddLaunchPhase("test", "auction", entities.LaunchSettings{
		Mode: entities.LaunchModeLandrush, Model: entities.RegistrationModelApplication, AcceptSignedMarks: true,
	}, now.Add(-time.Hour), nil)
	auction.Policy.MinLabelLength = 3
	price, err := entities.NewPrice("USD", 1500, 1000, 1000, 1000)
	require.NoError(t, err)
	_, err = auction.AddPrice(*price)
	require.NoError(t, err)
	reg.AddLaunchPhase("test", "rush", entities.LaunchSettings{
		Mode: entities.LaunchModeOpen, Model: entities.RegistrationModelFirstCome, AcceptSignedMarks: true,
	}, now.Add(-time.Hour), nil)
	return svc, reg
}

func applicationCreate(name, typ string) commands.EPPCreateDomainCommand {
	cmd := validCreate(name)
	cmd.Launch = &commands.EPPLaunchCreate{Phase: commands.EPPLaunchPhase{Value: "landrush"}, Type: typ}
	return cmd
}

func TestEPPDomainCreateFilesApplication(t *testing.T) {
	svc, reg := applicationRegistry(t)
	ctx := context.Background()

	res, err := svc.Create(ctx, rar1, applicationCreate("bid.test", "application"))
	require.NoError(t, err)
	require.Nil(t, res.Domain, "nothing is registered")
	app := res.Application
	require.NotNil(t, app)
	assert.Equal(t, entities.APPLICATION_ROID_ID, app.ID.ObjectIdentifier())
	assert.Equal(t, entities.ApplicationStatusPendingAllocation, app.Status)
	assert.Equal(t, entities.DomainName("bid.test"), app.Name)
	assert.Equal(t, entities.ClIDType("auction"), app.PhaseName)
	assert.Equal(t, entities.LaunchModeLandrush, app.PhaseMode)
	assert.Equal(t, entities.ClIDType(rar1), app.ClID)
	assert.Equal(t, entities.ClIDType("rar-one-c"), app.RegistrantID)
	assert.Equal(t, entities.ClIDType("rar-one-c"), app.AdminID)
	assert.Equal(t, []string{"ns1.rar-one.net"}, app.HostNames)
	assert.Equal(t, 2, app.Years)
	assert.True(t, app.AuthInfo.Matches("Tr4nsf3r-C0de-Xq9#Lm2!"))
	assert.Equal(t, entities.ApplicationQuote{Currency: "USD", Amount: 3000}, app.Quote, "two years at the phase's price, kept for allocation")
	assert.Empty(t, reg.Registered)
	assert.NotNil(t, reg.Application(app.ID.String()), "stored")

	t.Run("the type attribute may be left out", func(t *testing.T) {
		cmd := applicationCreate("bid.test", "")
		cmd.RegistrantID, cmd.HostNames = "rar-two-c", []string{"ns1.rar-two.net"}
		cmd.Contacts = []commands.EPPDomainContact{{Type: "tech", ID: "rar-two-c"}}
		res, err := svc.Create(ctx, rar2, cmd)
		require.NoError(t, err)
		assert.NotNil(t, res.Application, "a second registrar applies for the same name")
	})
	t.Run("one open application per registrar, name and phase", func(t *testing.T) {
		_, err := svc.Create(ctx, rar1, applicationCreate("bid.test", "application"))
		assert.ErrorIs(t, err, entities.ErrApplicationExists)
	})
	t.Run("pendingValidation when the phase requires it", func(t *testing.T) {
		reg.SetPolicy("test", func(p *entities.PhasePolicy) { v := true; p.RequiresValidation = &v })
		defer reg.SetPolicy("test", func(p *entities.PhasePolicy) { v := false; p.RequiresValidation = &v })
		res, err := svc.Create(ctx, rar1, applicationCreate("vetted.test", ""))
		require.NoError(t, err)
		assert.Equal(t, entities.ApplicationStatusPendingValidation, res.Application.Status)
	})
	t.Run("the create's rules still apply", func(t *testing.T) {
		_, err := svc.Create(ctx, rar1, validCreate("mine.test"))
		require.NoError(t, err)
		for _, tc := range []struct {
			name string
			edit func(*commands.EPPCreateDomainCommand)
			want error
		}{
			{"type=registration", func(c *commands.EPPCreateDomainCommand) { c.Launch.Type = "registration" }, entities.ErrLaunchCreateTypeMismatch},
			{"a mark", func(c *commands.EPPCreateDomainCommand) { c.Launch.Marks = 1 }, entities.ErrLaunchEvidenceUnsupported},
			{"the phase's label rules", func(c *commands.EPPCreateDomainCommand) { c.Name = "ab.test" }, entities.ErrLabelNotValidInPhase},
			{"a registered name", func(c *commands.EPPCreateDomainCommand) { c.Name = "mine.test" }, ErrDomainExists},
			{"a reserved name", func(c *commands.EPPCreateDomainCommand) { c.Name = "held.test" }, ErrDomainBlocked},
			{"another registrar's contact", func(c *commands.EPPCreateDomainCommand) { c.RegistrantID = "rar-two-c" }, entities.ErrEPPNotSponsor},
			{"an unknown host", func(c *commands.EPPCreateDomainCommand) { c.HostNames = []string{"ns9.rar-one.net"} }, entities.ErrHostNotFound},
			{"a weak code", func(c *commands.EPPCreateDomainCommand) { c.AuthInfo = "weak" }, entities.ErrAuthInfoNotSecure},
		} {
			t.Run(tc.name, func(t *testing.T) {
				cmd := applicationCreate("other.test", "application")
				tc.edit(&cmd)
				_, err = svc.Create(ctx, rar1, cmd)
				assert.ErrorIs(t, err, tc.want)
			})
		}
	})
	t.Run("a first-come phase registers at once", func(t *testing.T) {
		cmd := validCreate("quick.test")
		cmd.Launch = &commands.EPPLaunchCreate{Phase: commands.EPPLaunchPhase{Value: "open"}}
		res, err := svc.Create(ctx, rar1, cmd)
		require.NoError(t, err)
		assert.Nil(t, res.Application)
		assert.Equal(t, entities.ClIDType("rush"), res.Domain.RegistrationPhaseName)
	})
	t.Run("a registry without an application store refuses the create", func(t *testing.T) {
		bare, reg := applicationRegistry(t)
		bare.applications, bare.roids = nil, nil
		_, err := bare.Create(ctx, rar1, applicationCreate("bid.test", "application"))
		assert.ErrorIs(t, err, errApplicationsNotConfigured)
		assert.Empty(t, reg.Registered)
	})
}

func TestEPPApplicationInfo(t *testing.T) {
	svc, _ := applicationRegistry(t)
	ctx := context.Background()
	res, err := svc.Create(ctx, rar1, applicationCreate("bid.test", "application"))
	require.NoError(t, err)
	id := res.Application.ID.String()

	t.Run("by ID, to its registrar", func(t *testing.T) {
		app, err := svc.InfoApplication(ctx, rar1, "bid.test", commands.EPPLaunchRef{ApplicationID: id})
		require.NoError(t, err)
		assert.Equal(t, res.Application.ID, app.ID)
	})
	t.Run("with the phase, by mode or by name", func(t *testing.T) {
		_, err := svc.InfoApplication(ctx, rar1, "bid.test", commands.EPPLaunchRef{Phase: commands.EPPLaunchPhase{Value: "landrush"}, ApplicationID: id})
		assert.NoError(t, err)
		_, err = svc.InfoApplication(ctx, rar1, "bid.test", commands.EPPLaunchRef{Phase: commands.EPPLaunchPhase{Value: "landrush", Name: "auction"}, ApplicationID: id})
		assert.NoError(t, err)
	})
	for _, tc := range []struct {
		name  string
		scope entities.RegistrarClID
		dom   string
		ref   commands.EPPLaunchRef
		want  error
	}{
		{"another registrar", rar2, "bid.test", commands.EPPLaunchRef{ApplicationID: id}, entities.ErrEPPNotSponsor},
		{"no application named", rar1, "bid.test", commands.EPPLaunchRef{Phase: commands.EPPLaunchPhase{Value: "landrush"}}, entities.ErrApplicationIDRequired},
		{"an unknown application", rar1, "bid.test", commands.EPPLaunchRef{ApplicationID: "99_APP-TEST"}, entities.ErrApplicationNotFound},
		{"another name", rar1, "other.test", commands.EPPLaunchRef{ApplicationID: id}, entities.ErrApplicationMismatch},
		{"another phase by name", rar1, "bid.test", commands.EPPLaunchRef{Phase: commands.EPPLaunchPhase{Value: "landrush", Name: "rush"}, ApplicationID: id}, entities.ErrApplicationMismatch},
		{"another mode", rar1, "bid.test", commands.EPPLaunchRef{Phase: commands.EPPLaunchPhase{Value: "open"}, ApplicationID: id}, entities.ErrApplicationMismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.InfoApplication(ctx, tc.scope, tc.dom, tc.ref)
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestEPPApplicationUpdate(t *testing.T) {
	svc, reg := applicationRegistry(t)
	ctx := context.Background()
	reg.AddHost("ns2.rar-one.net", rar1.String())
	reg.AddContact("rar-one-d", rar1.String())
	res, err := svc.Create(ctx, rar1, applicationCreate("bid.test", "application"))
	require.NoError(t, err)
	id := res.Application.ID.String()
	ref := func() *commands.EPPLaunchRef { return &commands.EPPLaunchRef{ApplicationID: id} }

	t.Run("refusals", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			edit func(*commands.EPPUpdateDomainCommand)
			want error
		}{
			{"another registrar", func(c *commands.EPPUpdateDomainCommand) {}, entities.ErrEPPNotSponsor},
			{"a status", func(c *commands.EPPUpdateDomainCommand) { c.AddStatuses = []string{entities.DomainStatusClientHold} }, entities.ErrApplicationUpdateUnsupported},
			{"a restore", func(c *commands.EPPUpdateDomainCommand) { c.Restore = &commands.EPPRestore{} }, entities.ErrApplicationUpdateUnsupported},
			{"host attributes", func(c *commands.EPPUpdateDomainCommand) { c.HostAttributes = true }, entities.ErrEPPHostAttributes},
			{"a nameserver not set", func(c *commands.EPPUpdateDomainCommand) { c.RemoveHostNames = []string{"ns2.rar-one.net"} }, entities.ErrEPPNameserverNotSet},
			{"a nameserver already set", func(c *commands.EPPUpdateDomainCommand) { c.AddHostNames = []string{"NS1.rar-one.net"} }, entities.ErrDuplicateHost},
			{"another registrar's host", func(c *commands.EPPUpdateDomainCommand) { c.AddHostNames = []string{"ns1.rar-two.net"} }, entities.ErrHostNotFound},
			{"a contact not set", func(c *commands.EPPUpdateDomainCommand) {
				c.RemoveContacts = []commands.EPPDomainContact{{Type: "billing", ID: "rar-one-c"}}
			}, entities.ErrEPPDomainContactNotSet},
			{"a second contact of a type", func(c *commands.EPPUpdateDomainCommand) {
				c.AddContacts = []commands.EPPDomainContact{{Type: "tech", ID: "rar-one-d"}}
			}, entities.ErrEPPDuplicateContactType},
			{"another registrar's contact", func(c *commands.EPPUpdateDomainCommand) { c.Registrant = ptr("rar-two-c") }, entities.ErrEPPNotSponsor},
			{"a weak code", func(c *commands.EPPUpdateDomainCommand) { c.AuthInfo = ptr("weak") }, entities.ErrAuthInfoNotSecure},
			{"urgent DNSSEC", func(c *commands.EPPUpdateDomainCommand) { c.DNSSEC = &commands.EPPDNSSECUpdate{Urgent: true} }, entities.ErrEPPDNSSECUrgent},
			{"a DS record not set", func(c *commands.EPPUpdateDomainCommand) {
				c.DNSSEC = &commands.EPPDNSSECUpdate{RemoveDS: []commands.EPPDSData{ds(13, 2, digest256[:62]+"00")}}
			}, entities.ErrEPPDSNotSet},
		} {
			t.Run(tc.name, func(t *testing.T) {
				scope := rar1
				if tc.name == "another registrar" {
					scope = rar2
				}
				cmd := commands.EPPUpdateDomainCommand{Name: "bid.test", Launch: ref()}
				tc.edit(&cmd)
				_, err := svc.UpdateApplication(ctx, scope, cmd)
				assert.ErrorIs(t, err, tc.want)
			})
		}
	})
	t.Run("contacts, nameservers, DS records and the code", func(t *testing.T) {
		pw := "N3w-C0de-Zq8#Lm2!xyWv7Qp$Rt" // #nosec G101 -- a test transfer code, not a credential
		app, err := svc.UpdateApplication(ctx, rar1, commands.EPPUpdateDomainCommand{
			Name: "bid.test", Launch: ref(),
			AddHostNames: []string{"NS2.rar-one.net"}, RemoveHostNames: []string{"ns1.rar-one.net"},
			AddContacts:    []commands.EPPDomainContact{{Type: "billing", ID: "rar-one-d"}},
			RemoveContacts: []commands.EPPDomainContact{{Type: "admin", ID: "rar-one-c"}},
			Registrant:     ptr("rar-one-d"), AuthInfo: &pw,
			DNSSEC: &commands.EPPDNSSECUpdate{AddDS: []commands.EPPDSData{ds(13, 2, digest256)}},
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"ns2.rar-one.net"}, app.HostNames)
		assert.Equal(t, entities.ClIDType("rar-one-d"), app.BillingID)
		assert.Empty(t, app.AdminID)
		assert.Equal(t, entities.ClIDType("rar-one-d"), app.RegistrantID)
		assert.True(t, app.AuthInfo.Matches(pw))
		require.Len(t, app.DSRecords, 1)
		assert.True(t, app.UpdatedAt.After(app.CreatedAt))
		stored := reg.Application(id)
		assert.Equal(t, app.HostNames, stored.HostNames, "written")
	})
	t.Run("a final application cannot change", func(t *testing.T) {
		reg.SetApplication(id, func(a *entities.LaunchApplication) { a.Status = entities.ApplicationStatusRejected })
		_, err := svc.UpdateApplication(ctx, rar1, commands.EPPUpdateDomainCommand{Name: "bid.test", Launch: ref(), AddHostNames: []string{"ns1.rar-one.net"}})
		assert.ErrorIs(t, err, entities.ErrApplicationFinal)
		assert.ErrorIs(t, svc.DeleteApplication(ctx, rar1, "bid.test", *ref()), entities.ErrApplicationFinal)
	})
}

func TestEPPApplicationDelete(t *testing.T) {
	svc, reg := applicationRegistry(t)
	ctx := context.Background()
	res, err := svc.Create(ctx, rar1, applicationCreate("bid.test", "application"))
	require.NoError(t, err)
	id := res.Application.ID.String()
	ref := commands.EPPLaunchRef{ApplicationID: id}

	assert.ErrorIs(t, svc.DeleteApplication(ctx, rar2, "bid.test", ref), entities.ErrEPPNotSponsor)
	assert.ErrorIs(t, svc.DeleteApplication(ctx, rar1, "bid.test", commands.EPPLaunchRef{}), entities.ErrApplicationIDRequired)
	require.NoError(t, svc.DeleteApplication(ctx, rar1, "bid.test", ref))
	assert.Nil(t, reg.Application(id), "withdrawn")
	assert.ErrorIs(t, svc.DeleteApplication(ctx, rar1, "bid.test", ref), entities.ErrApplicationNotFound)
	_, err = svc.InfoApplication(ctx, rar1, "bid.test", ref)
	assert.ErrorIs(t, err, entities.ErrApplicationNotFound)

	t.Run("the registrar may then apply again", func(t *testing.T) {
		_, err := svc.Create(ctx, rar1, applicationCreate("bid.test", "application"))
		assert.NoError(t, err)
	})
}
