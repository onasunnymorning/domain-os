package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	pgdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/snowflakeidgenerator"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// eppTestDB connects to a database of its own on the throwaway test
// Postgres (TEST_DB_HOST/TEST_DB_PORT, CI defaults): other packages' tests
// clear tables in the shared database while this one runs. It fails rather
// than skips, so the registration path is proven wherever `make test` runs.
func eppTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	host, port := os.Getenv("TEST_DB_HOST"), os.Getenv("TEST_DB_PORT")
	if host == "" {
		host = "127.0.0.1"
	}
	if port == "" {
		port = "5432"
	}
	const name = "dos_epp_unittests"
	dsn := fmt.Sprintf("postgres://postgres:unittest@%s:%s/%s?sslmode=require", host, port, name)
	db, err := gorm.Open(pgdriver.Open(dsn))
	if err != nil && strings.Contains(err.Error(), "3D000") {
		admin, aerr := sql.Open("postgres", fmt.Sprintf("host=%s port=%s user=postgres password=unittest sslmode=require", host, port))
		require.NoError(t, aerr)
		_, _ = admin.Exec("CREATE DATABASE " + name)
		_ = admin.Close()
		db, err = gorm.Open(pgdriver.Open(dsn))
	}
	require.NoError(t, err, "test Postgres unavailable; run via `make test` (port 5433)")
	require.NoError(t, postgres.AutoMigrate(db))
	return db
}

// eppFixture is a registry on the test Postgres, set up through the admin
// services: a TLD with a GA phase, two registrars each with a contact
// (c<clid>) and a host (ns1.<clid>.net), the first accredited for the
// TLD; and the EPP domain service over it, wired as cmd/epp wires it.
type eppFixture struct {
	db              *gorm.DB
	tld, rar1, rar2 string
	roids           *RoidService
	events          *postgres.PostgresEventPublisher
	tldRepo         *postgres.GormTLDRepository
	hostRepo        *postgres.HostRepository
	domainRepo      *postgres.DomainRepository
	registry        *DomainService
	domains         *EPPDomainService
}

func newEPPFixture(t *testing.T) *eppFixture {
	t.Helper()
	db := eppTestDB(t)
	ctx := context.Background()
	ids, err := snowflakeidgenerator.NewIDGenerator()
	require.NoError(t, err)
	roids := NewRoidService(ids, "EPPTEST")
	events := postgres.NewPostgresEventPublisher(db, zap.NewNop(), false)

	// Names unique to this run, so reruns on a used database do not collide.
	run := strconv.FormatInt(time.Now().UnixNano()%1e9, 36)
	tld, ry, rar1, rar2 := "t"+run, "ry"+run, "r1"+run, "r2"+run

	tldRepo := postgres.NewGormTLDRepo(db)
	rarRepo := postgres.NewGormRegistrarRepository(db)
	phaseRepo := postgres.NewGormPhaseRepository(db)
	contactRepo := postgres.NewContactRepository(db, "EPPTEST")
	hostRepo := postgres.NewGormHostRepository(db, "EPPTEST")
	domainRepo := postgres.NewDomainRepository(db, "EPPTEST")

	_, err = NewRegistryOperatorService(postgres.NewGORMRegistryOperatorRepository(db)).Create(ctx, &commands.CreateRegistryOperatorCommand{
		RyID: ry, Name: ry, Email: "ry@example.com", URL: "https://ry.example.com"})
	require.NoError(t, err)
	_, err = NewTLDService(tldRepo, postgres.NewGormDNSRecordRepository(db)).CreateTLD(ctx, &commands.CreateTLDCommand{Name: tld, RyID: ry})
	require.NoError(t, err)
	_, err = NewPhaseService(phaseRepo, tldRepo, postgres.NewLifecyclePolicyRepository(db), nil, events).CreatePhase(ctx, &commands.CreatePhaseCommand{
		Name: "GA1", Type: "GA", Starts: time.Now().UTC().AddDate(0, 0, -1), TLDName: tld})
	require.NoError(t, err)

	registrars := NewRegistrarService(rarRepo, events)
	accreditations := NewAccreditationService(postgres.NewAccreditationRepository(db), rarRepo, tldRepo, events)
	contacts := NewContactService(contactRepo, *roids, events)
	hosts := NewHostService(hostRepo, postgres.NewGormHostAddressRepository(db), roids, events)
	for i, clid := range []string{rar1, rar2} {
		addr, err := entities.NewAddress("Brussels", "BE")
		require.NoError(t, err)
		postal, err := entities.NewRegistrarPostalInfo("int", addr)
		require.NoError(t, err)
		_, err = registrars.Create(ctx, &commands.CreateRegistrarCommand{
			ClID: clid, Name: clid, Email: "ops@" + clid + ".example.com", GurID: 9000 + i,
			PostalInfo: [2]*entities.RegistrarPostalInfo{postal, nil}, URL: "https://" + clid + ".example.com",
			Status: string(entities.RegistrarStatusOK), IANAStatus: entities.IANARegistrarStatusAccredited,
		})
		require.NoError(t, err)
		cpostal, err := entities.NewContactPostalInfo("int", "Test Contact", addr)
		require.NoError(t, err)
		_, err = contacts.CreateContact(ctx, &commands.CreateContactCommand{
			ID: "c" + clid, Email: "c@" + clid + ".example.com", AuthInfo: "Contact-Auth-1!", ClID: clid,
			PostalInfo: [2]*entities.ContactPostalInfo{cpostal, nil},
		})
		require.NoError(t, err)
		_, err = hosts.CreateHost(ctx, &commands.CreateHostCommand{
			Name: "ns1." + clid + ".net", ClID: entities.ClIDType(clid), CrRr: entities.ClIDType(clid),
		})
		require.NoError(t, err)
	}
	require.NoError(t, accreditations.CreateAccreditation(ctx, tld, rar1))

	registry := NewDomainService(domainRepo, hostRepo, *roids, postgres.NewGormNNDNRepository(db), tldRepo, phaseRepo,
		postgres.NewGORMPremiumLabelRepository(db), postgres.NewFXRepository(db), rarRepo, events, postgres.NewTransactor(db))
	svc := NewEPPDomainService(EPPDomainServiceDeps{
		Domains: domainRepo, NNDNs: postgres.NewGormNNDNRepository(db), TLDs: tldRepo, Phases: phaseRepo,
		Contacts: contactRepo, Hosts: hostRepo, Registrars: rarRepo, Register: registry,
	})
	return &eppFixture{
		db: db, tld: tld, rar1: rar1, rar2: rar2, roids: roids, events: events,
		tldRepo: tldRepo, hostRepo: hostRepo, domainRepo: domainRepo, registry: registry, domains: svc,
	}
}

// TestEPPDomainCreateOverPostgres runs create and info through the real
// registration path — DomainService.RegisterDomain on the Postgres
// repositories, as cmd/epp wires it — over a TLD, phase, registrars,
// contacts and hosts set up through the admin services.
func TestEPPDomainCreateOverPostgres(t *testing.T) {
	f := newEPPFixture(t)
	db, ctx, svc, registry := f.db, context.Background(), f.domains, f.registry
	tld, rar1, rar2 := f.tld, f.rar1, f.rar2
	scope1, scope2 := entities.RegistrarClID(rar1), entities.RegistrarClID(rar2)
	create := commands.EPPCreateDomainCommand{
		Name: "epp." + tld, Period: 3, PeriodUnit: "y", RegistrantID: "c" + rar1,
		Contacts:  []commands.EPPDomainContact{{Type: "tech", ID: "c" + rar1}},
		HostNames: []string{"ns1." + rar1 + ".net"}, AuthInfo: "Tr4nsf3r-C0de-Xq9#Lm2!",
		DSData: []commands.EPPDSData{{KeyTag: 2371, Algorithm: 13, DigestType: 2, Digest: "49FD46E6C4B45C55D4AC69CBD3CD34AC1AFE51DE3DC8B27C3F72E84BC2E1D3F4"}},
	}

	t.Run("a registrar not accredited for the TLD is refused", func(t *testing.T) {
		c := create
		c.RegistrantID, c.Contacts, c.HostNames = "c"+rar2, []commands.EPPDomainContact{{Type: "tech", ID: "c" + rar2}}, nil
		_, err := svc.Create(ctx, scope2, c)
		require.ErrorIs(t, err, ErrRegistrarNotAccredited)
	})

	// The EPP adapter marks the context with the session's login, as here.
	res, err := svc.Create(appcontext.WithUserID(ctx, "epp:"+rar1), scope1, create)
	require.NoError(t, err)
	dom := res.Domain
	require.NoError(t, err)
	assert.Equal(t, entities.ClIDType(rar1), dom.ClID)
	assert.Regexp(t, `^\d+_DOM-EPPTEST$`, dom.RoID.String(), "minted with the configured repository ID")

	t.Run("the domain is stored as registered", func(t *testing.T) {
		info, err := svc.Info(ctx, scope1, create.Name, nil)
		require.NoError(t, err)
		d := info.Domain
		assert.Equal(t, dom.RoID, d.RoID, "read back as it was minted")
		assert.Equal(t, entities.ClIDType(rar1), d.CrRr)
		assert.Equal(t, entities.ClIDType("c"+rar1), d.RegistrantID)
		assert.True(t, d.AuthInfo.Matches("Tr4nsf3r-C0de-Xq9#Lm2!"), "stored as a hash that matches the code (RFC 9154)")
		assert.NotContains(t, string(d.AuthInfo), "Tr4nsf3r-C0de-Xq9#Lm2!")
		require.Len(t, d.Hosts, 1)
		assert.Equal(t, "ns1."+rar1+".net", d.Hosts[0].Name.String())
		assert.WithinDuration(t, d.CreatedAt.AddDate(3, 0, 0), d.ExpiryDate, time.Second)
		assert.True(t, time.Now().Before(d.RGPStatus.AddPeriodEnd), "in its add grace period")
		require.Len(t, d.DSRecords, 1, "with its DS record")
		assert.Equal(t, uint16(2371), d.DSRecords[0].KeyTag)
	})
	t.Run("only the ROID's number is stored", func(t *testing.T) {
		// The repository ID is the installation's, appended on every read
		// (ADR 0017): a repository configured with another shows it.
		other, err := postgres.NewDomainRepository(db, "OTHER").GetDomainByName(ctx, create.Name, false)
		require.NoError(t, err)
		n, err := dom.RoID.Int64()
		require.NoError(t, err)
		assert.Equal(t, entities.RoidType(strconv.FormatInt(n, 10)+"_DOM-OTHER"), other.RoID)
	})
	t.Run("its DS record is published with the TLD's delegations", func(t *testing.T) {
		rrs, err := registry.GetDSRecordsPerTLD(ctx, tld)
		require.NoError(t, err)
		require.Len(t, rrs, 1)
		assert.Contains(t, rrs[0].String(), "epp."+tld+".\t3600\tIN\tDS\t2371 13 2 49FD46E6")
	})
	t.Run("a second create of the name is refused as existing", func(t *testing.T) {
		_, err := svc.Create(ctx, scope1, create)
		require.ErrorIs(t, err, ErrDomainExists)
	})
	t.Run("another registrar cannot read it", func(t *testing.T) {
		_, err := svc.Info(ctx, scope2, create.Name, nil)
		require.ErrorIs(t, err, entities.ErrEPPNotSponsor)
	})
	t.Run("its registration event does not carry the authInfo (#484)", func(t *testing.T) {
		var rows []struct{ Command, BeforeState, AfterState, Data string }
		require.NoError(t, db.Table("domain_events").Select("command::text AS command, coalesce(before_state::text, '') AS before_state, after_state::text AS after_state, data::text AS data").
			Where("subject = ? AND type = ?", create.Name, "domain.registered").Scan(&rows).Error)
		require.Len(t, rows, 1)
		for _, col := range []string{rows[0].Command, rows[0].BeforeState, rows[0].AfterState, rows[0].Data} {
			assert.NotContains(t, col, "Tr4nsf3r-C0de-Xq9#Lm2!")
		}
		assert.Contains(t, rows[0].Command, "[redacted]")
	})
	t.Run("its registration event names the session's login as actor", func(t *testing.T) {
		var actors []string
		require.NoError(t, db.Table("domain_events").Where("subject = ? AND type = ?", create.Name, "domain.registered").Pluck("actor", &actors).Error)
		assert.Equal(t, []string{"epp:" + rar1}, actors)
	})
}

// TestEPPLaunchApplicationOverPostgres files, reads, changes and withdraws
// an application through the Postgres repositories, as cmd/epp wires them,
// in a landrush phase that collects applications, priced through the
// registration path's quote.
func TestEPPLaunchApplicationOverPostgres(t *testing.T) {
	f := newEPPFixture(t)
	db, ctx, tld, rar1 := f.db, context.Background(), f.tld, f.rar1
	scope := entities.RegistrarClID(rar1)
	phases := postgres.NewGormPhaseRepository(db)
	applications := postgres.NewLaunchApplicationRepository(db, "EPPTEST")
	phaseService := NewPhaseService(phases, f.tldRepo, postgres.NewLifecyclePolicyRepository(db), applications, f.events)
	_, err := phaseService.CreatePhase(ctx, &commands.CreatePhaseCommand{
		Name: "auction", Type: "Launch", Starts: time.Now().UTC().AddDate(0, 0, -1), TLDName: tld,
		Launch: &entities.LaunchSettings{Mode: entities.LaunchModeLandrush, Model: entities.RegistrationModelApplication, AcceptSignedMarks: true},
	})
	require.NoError(t, err)
	price, err := entities.NewPrice("USD", 2500, 1000, 1000, 1000)
	require.NoError(t, err)
	_, err = NewPriceService(phases, postgres.NewGormPriceRepository(db)).CreatePrice(ctx, &commands.CreatePriceCommand{
		TLDName: tld, PhaseName: "auction", Currency: price.Currency,
		RegistrationAmount: price.RegistrationAmount, RenewalAmount: price.RenewalAmount, TransferAmount: price.TransferAmount, RestoreAmount: price.RestoreAmount,
	})
	require.NoError(t, err)

	svc := NewEPPDomainService(EPPDomainServiceDeps{
		Domains: f.domainRepo, NNDNs: postgres.NewGormNNDNRepository(db), TLDs: f.tldRepo, Phases: phases,
		Contacts: postgres.NewContactRepository(db, "EPPTEST"), Hosts: f.hostRepo, Registrars: postgres.NewGormRegistrarRepository(db),
		Register: f.registry, Applications: applications, Roids: f.roids,
	})
	create := commands.EPPCreateDomainCommand{
		Name: "bid." + tld, Period: 2, PeriodUnit: "y", RegistrantID: "c" + rar1,
		Contacts: []commands.EPPDomainContact{{Type: "tech", ID: "c" + rar1}}, HostNames: []string{"ns1." + rar1 + ".net"},
		AuthInfo: "Tr4nsf3r-C0de-Xq9#Lm2!",
		Launch:   &commands.EPPLaunchCreate{Phase: commands.EPPLaunchPhase{Value: "landrush"}, Type: "application"},
	}
	res, err := svc.Create(ctx, scope, create)
	require.NoError(t, err)
	require.NotNil(t, res.Application)
	app := res.Application
	assert.Equal(t, "EPPTEST", app.ID.SystemIdentifier())
	assert.Equal(t, entities.ApplicationStatusPendingAllocation, app.Status)
	assert.Equal(t, entities.ApplicationQuote{Currency: "USD", Amount: 5000, Class: "standard"}, app.Quote, "two years at the phase's price")
	_, err = f.domainRepo.GetDomainByName(ctx, "bid."+tld, false)
	assert.ErrorIs(t, err, entities.ErrDomainNotFound, "nothing is registered")

	_, err = svc.Create(ctx, scope, create)
	assert.ErrorIs(t, err, entities.ErrApplicationExists)

	ref := commands.EPPLaunchRef{Phase: commands.EPPLaunchPhase{Value: "landrush"}, ApplicationID: app.ID.String()}
	got, err := svc.InfoApplication(ctx, scope, "bid."+tld, ref)
	require.NoError(t, err)
	assert.Equal(t, app.HostNames, got.HostNames)

	updated, err := svc.UpdateApplication(ctx, scope, commands.EPPUpdateDomainCommand{
		Name: "bid." + tld, Launch: &ref, RemoveHostNames: []string{"ns1." + rar1 + ".net"},
	})
	require.NoError(t, err)
	assert.Empty(t, updated.HostNames)

	n, err := applications.CountApplicationsByPhase(ctx, tld, "auction")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	_, err = phaseService.SetLaunchSettings(ctx, testPlatformScope, &commands.SetLaunchSettingsCommand{
		LaunchSettings: entities.LaunchSettings{Mode: entities.LaunchModeLandrush, Model: entities.RegistrationModelFirstCome, AcceptSignedMarks: true},
		TLDName:        tld, PhaseName: "auction",
	})
	assert.ErrorIs(t, err, entities.ErrLaunchModelFrozen, "a phase with applications keeps its model")

	require.NoError(t, svc.DeleteApplication(ctx, scope, "bid."+tld, ref))
	_, err = svc.InfoApplication(ctx, scope, "bid."+tld, ref)
	assert.ErrorIs(t, err, entities.ErrApplicationNotFound)
}

// TestLaunchAllocationOverPostgres allocates an application through the
// Postgres repositories and the registration path, as the admin API wires
// them: the domain is registered to the winner in its phase with the quote
// from filing, the other application for the name is rejected, and both
// applicants get a poll message, in one transaction.
func TestLaunchAllocationOverPostgres(t *testing.T) {
	f := newEPPFixture(t)
	db, ctx, tld, rar1, rar2 := f.db, context.Background(), f.tld, f.rar1, f.rar2
	phases := postgres.NewGormPhaseRepository(db)
	applications := postgres.NewLaunchApplicationRepository(db, "EPPTEST")
	_, err := NewPhaseService(phases, f.tldRepo, postgres.NewLifecyclePolicyRepository(db), applications, f.events).CreatePhase(ctx, &commands.CreatePhaseCommand{
		Name: "auction", Type: "Launch", Starts: time.Now().UTC().AddDate(0, 0, -1), TLDName: tld,
		Launch: &entities.LaunchSettings{Mode: entities.LaunchModeLandrush, Model: entities.RegistrationModelApplication, AcceptSignedMarks: true},
	})
	require.NoError(t, err)
	_, err = NewPriceService(phases, postgres.NewGormPriceRepository(db)).CreatePrice(ctx, &commands.CreatePriceCommand{
		TLDName: tld, PhaseName: "auction", Currency: "USD", RegistrationAmount: 2500, RenewalAmount: 1000, TransferAmount: 1000, RestoreAmount: 1000,
	})
	require.NoError(t, err)
	rarRepo := postgres.NewGormRegistrarRepository(db)
	require.NoError(t, NewAccreditationService(postgres.NewAccreditationRepository(db), rarRepo, f.tldRepo, f.events).CreateAccreditation(ctx, tld, rar2))
	svc := NewEPPDomainService(EPPDomainServiceDeps{
		Domains: f.domainRepo, NNDNs: postgres.NewGormNNDNRepository(db), TLDs: f.tldRepo, Phases: phases,
		Contacts: postgres.NewContactRepository(db, "EPPTEST"), Hosts: f.hostRepo, Registrars: rarRepo,
		Register: f.registry, Applications: applications, Roids: f.roids,
	})
	file := func(rar string) *entities.LaunchApplication {
		res, err := svc.Create(ctx, entities.RegistrarClID(rar), commands.EPPCreateDomainCommand{
			Name: "won." + tld, Period: 2, PeriodUnit: "y", RegistrantID: "c" + rar,
			Contacts: []commands.EPPDomainContact{{Type: "tech", ID: "c" + rar}}, HostNames: []string{"ns1." + rar + ".net"},
			AuthInfo: "Tr4nsf3r-C0de-Xq9#Lm2!",
			Launch:   &commands.EPPLaunchCreate{Phase: commands.EPPLaunchPhase{Value: "landrush"}, Type: "application"},
		})
		require.NoError(t, err)
		return res.Application
	}
	winner, loser := file(rar1), file(rar2)

	alloc := NewLaunchAllocationService(applications, phases, f.tldRepo, f.registry, postgres.NewTransactor(db), f.events)
	allocated, err := alloc.Allocate(ctx, testPlatformScope, &commands.AllocateApplicationCommand{ApplicationID: winner.ID.String()})
	require.NoError(t, err)
	assert.Equal(t, entities.ApplicationStatusAllocated, allocated.Status)

	dom, err := f.domainRepo.GetDomainByName(ctx, "won."+tld, true)
	require.NoError(t, err)
	assert.Equal(t, dom.RoID, allocated.DomainRoID)
	assert.Equal(t, entities.ClIDType(rar1), dom.ClID)
	assert.Equal(t, entities.ClIDType("auction"), dom.RegistrationPhaseName)
	assert.Equal(t, dom.CreatedAt.AddDate(2, 0, 0), dom.ExpiryDate)
	assert.True(t, dom.AuthInfo.Matches("Tr4nsf3r-C0de-Xq9#Lm2!"), "the applicant's code")
	require.Len(t, dom.Hosts, 1)

	rejected, err := applications.GetApplicationByID(ctx, loser.ID)
	require.NoError(t, err)
	assert.Equal(t, entities.ApplicationStatusRejected, rejected.Status)

	polls := postgres.NewDomainTransferRepository(db, "EPPTEST")
	msg, n, err := polls.OldestPollMessage(ctx, entities.ClIDType(rar2))
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	assert.Equal(t, entities.ApplicationStatusRejected, msg.Application.Status)
	msg, _, err = polls.OldestPollMessage(ctx, entities.ClIDType(rar1))
	require.NoError(t, err)
	assert.Equal(t, entities.ApplicationStatusAllocated, msg.Application.Status)

	var registered postgres.DomainEventRecord
	require.NoError(t, db.Where("type = ? AND subject = ?", "domain.registered", "won."+tld).First(&registered).Error)
	var payload struct {
		Quote struct {
			Price struct {
				Amount float64 `json:"amount"`
			}
		}
	}
	require.NoError(t, json.Unmarshal(registered.Data, &payload))
	assert.Equal(t, float64(5000), payload.Quote.Price.Amount, "the quote from filing, not a new one")

	_, err = alloc.Allocate(ctx, testPlatformScope, &commands.AllocateApplicationCommand{ApplicationID: loser.ID.String()})
	assert.ErrorIs(t, err, entities.ErrApplicationTransition)
}
