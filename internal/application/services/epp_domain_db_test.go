package services

import (
	"context"
	"database/sql"
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

// TestEPPDomainCreateOverPostgres runs create and info through the real
// registration path — DomainService.RegisterDomain on the Postgres
// repositories, as cmd/epp wires it — over a TLD, phase, registrars,
// contacts and hosts set up through the admin services.
func TestEPPDomainCreateOverPostgres(t *testing.T) {
	db := eppTestDB(t)
	ctx := context.Background()
	ids, err := snowflakeidgenerator.NewIDGenerator()
	require.NoError(t, err)
	roids := NewRoidService(ids)
	events := postgres.NewPostgresEventPublisher(db, zap.NewNop(), false)

	// Names unique to this run, so reruns on a used database do not collide.
	run := strconv.FormatInt(time.Now().UnixNano()%1e9, 36)
	tld, ry, rar1, rar2 := "t"+run, "ry"+run, "r1"+run, "r2"+run

	tldRepo := postgres.NewGormTLDRepo(db)
	rarRepo := postgres.NewGormRegistrarRepository(db)
	phaseRepo := postgres.NewGormPhaseRepository(db)
	contactRepo := postgres.NewContactRepository(db)
	hostRepo := postgres.NewGormHostRepository(db)
	domainRepo := postgres.NewDomainRepository(db)

	_, err = NewRegistryOperatorService(postgres.NewGORMRegistryOperatorRepository(db)).Create(ctx, &commands.CreateRegistryOperatorCommand{
		RyID: ry, Name: ry, Email: "ry@example.com", URL: "https://ry.example.com"})
	require.NoError(t, err)
	_, err = NewTLDService(tldRepo, postgres.NewGormDNSRecordRepository(db)).CreateTLD(ctx, &commands.CreateTLDCommand{Name: tld, RyID: ry})
	require.NoError(t, err)
	_, err = NewPhaseService(phaseRepo, tldRepo, events).CreatePhase(ctx, &commands.CreatePhaseCommand{
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
		postgres.NewGORMPremiumLabelRepository(db), postgres.NewFXRepository(db), rarRepo, events)
	svc := NewEPPDomainService(EPPDomainServiceDeps{
		Domains: domainRepo, NNDNs: postgres.NewGormNNDNRepository(db), TLDs: tldRepo, Phases: phaseRepo,
		Contacts: contactRepo, Hosts: hostRepo, Registrars: rarRepo, Register: registry, CheckParallelism: 2,
	})
	scope1, scope2 := entities.RegistrarClID(rar1), entities.RegistrarClID(rar2)
	create := commands.EPPCreateDomainCommand{
		Name: "epp." + tld, Period: 3, PeriodUnit: "y", RegistrantID: "c" + rar1,
		Contacts:  []commands.EPPDomainContact{{Type: "tech", ID: "c" + rar1}},
		HostNames: []string{"ns1." + rar1 + ".net"}, AuthInfo: "Domain-Auth-1!",
		DSData: []commands.EPPDSData{{KeyTag: 2371, Algorithm: 13, DigestType: 2, Digest: "49FD46E6C4B45C55D4AC69CBD3CD34AC1AFE51DE3DC8B27C3F72E84BC2E1D3F4"}},
	}

	t.Run("a registrar not accredited for the TLD is refused", func(t *testing.T) {
		c := create
		c.RegistrantID, c.Contacts, c.HostNames = "c"+rar2, []commands.EPPDomainContact{{Type: "tech", ID: "c" + rar2}}, nil
		_, err := svc.Create(ctx, scope2, c)
		require.ErrorIs(t, err, ErrRegistrarNotAccredited)
	})

	// The EPP adapter marks the context with the session's login, as here.
	dom, err := svc.Create(appcontext.WithUserID(ctx, "epp:"+rar1), scope1, create)
	require.NoError(t, err)
	assert.Equal(t, entities.ClIDType(rar1), dom.ClID)
	assert.Regexp(t, `^\d+_DOM-\w+$`, dom.RoID.String())

	t.Run("the domain is stored as registered", func(t *testing.T) {
		info, err := svc.Info(ctx, scope1, create.Name)
		require.NoError(t, err)
		d := info.Domain
		assert.Equal(t, entities.ClIDType(rar1), d.CrRr)
		assert.Equal(t, entities.ClIDType("c"+rar1), d.RegistrantID)
		assert.Equal(t, entities.AuthInfoType("Domain-Auth-1!"), d.AuthInfo)
		require.Len(t, d.Hosts, 1)
		assert.Equal(t, "ns1."+rar1+".net", d.Hosts[0].Name.String())
		assert.WithinDuration(t, d.CreatedAt.AddDate(3, 0, 0), d.ExpiryDate, time.Second)
		assert.True(t, time.Now().Before(d.RGPStatus.AddPeriodEnd), "in its add grace period")
		require.Len(t, d.DSRecords, 1, "with its DS record")
		assert.Equal(t, uint16(2371), d.DSRecords[0].KeyTag)
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
		_, err := svc.Info(ctx, scope2, create.Name)
		require.ErrorIs(t, err, entities.ErrEPPNotSponsor)
	})
	t.Run("its registration event does not carry the authInfo (#484)", func(t *testing.T) {
		var rows []struct{ Command, BeforeState, AfterState, Data string }
		require.NoError(t, db.Table("domain_events").Select("command::text AS command, coalesce(before_state::text, '') AS before_state, after_state::text AS after_state, data::text AS data").
			Where("subject = ? AND type = ?", create.Name, "domain.registered").Scan(&rows).Error)
		require.Len(t, rows, 1)
		for _, col := range []string{rows[0].Command, rows[0].BeforeState, rows[0].AfterState, rows[0].Data} {
			assert.NotContains(t, col, "Domain-Auth-1!")
		}
		assert.Contains(t, rows[0].Command, "[redacted]")
	})
	t.Run("its registration event names the session's login as actor", func(t *testing.T) {
		var actors []string
		require.NoError(t, db.Table("domain_events").Where("subject = ? AND type = ?", create.Name, "domain.registered").Pluck("actor", &actors).Error)
		assert.Equal(t, []string{"epp:" + rar1}, actors)
	})
}
