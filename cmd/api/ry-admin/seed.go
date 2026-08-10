package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"go.uber.org/zap"

	"github.com/onasunnymorning/domain-os/cmd/api/ry-admin/config"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/snowflakeidgenerator"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// Seeding local development data.
//
// This exists so `make dev` can reach a populated database with no network
// access at all. The alternative — `init-registrars`, which drives
// SyncRegistrarsWorkflow — fetches the registrar list live from iana.org, so it
// cannot run on a laptop that is offline or behind a proxy, and it gives no
// domains to work with even when it succeeds.
//
// Everything below is generated. No value is derived from an escrow deposit or
// from any production database. gofakeit is pinned to a fixed seed so two runs
// produce the same names, addresses, and contacts.
//
// Determinism, precisely: the logical dataset is identical on every run — same
// TLD, same registrars, same domain names, same lifecycle states, same
// relationships. Two values are necessarily not byte-identical: RoIDs come from
// a snowflake generator (time + node ordered), and lifecycle dates are anchored
// to the current UTC day so that "expires in 30 days" stays true whenever you
// run it. Anchoring those to a constant instead would make every RGP subject
// drift into the past and stop being a useful test subject within a week.

const (
	// seedFakerSeed fixes gofakeit's PRNG. Any change to this constant changes
	// every generated name and address, so leave it alone unless you intend to
	// regenerate the whole dataset.
	seedFakerSeed = 20260806

	// seedRyID is the synthetic registry operator that owns the seeded TLD.
	seedRyID = "dosdev"

	// seedTLD is ".test", reserved by RFC 2606 §2 for exactly this purpose. It
	// can never be delegated, so a seeded domain can never collide with a real
	// registration or leak into a real resolver.
	seedTLD = "test"

	// seedPhase is the GA phase domains are created under. Phase names are
	// ClIDType values, so this must be 3-16 characters — plain "ga" is rejected.
	seedPhase = "ga-phase"

	// seedAuthInfo satisfies AuthInfoType.Validate (upper + lower + special).
	seedAuthInfo = "LocalDev!23"
)

// seedRegistrars is the fixed registrar set. GurIDs sit in a high range that
// IANA does not assign, so these can never be confused with real accreditations
// if this data ever ends up somewhere it shouldn't.
var seedRegistrars = []struct {
	ClID  string
	Name  string
	GurID int
}{
	{"9001-northwind", "Northwind Registrar Ltd", 9001},
	{"9002-lakeshore", "Lakeshore Domains Inc", 9002},
	{"9003-fabrikam", "Fabrikam Registry Services", 9003},
	{"9004-contoso", "Contoso Name Services", 9004},
}

// seedDomain describes one domain to create and the lifecycle state to put it
// in. The lifecycle field is applied after creation by applyLifecycle.
type seedDomain struct {
	Label     string
	Registrar string
	Lifecycle string
}

// seedDomains spans the states the lifecycle workflows actually branch on, so
// there is a real subject for each one without having to hand-build fixtures.
var seedDomains = []seedDomain{
	{"active-one", "9001-northwind", "active"},
	{"active-two", "9002-lakeshore", "active"},
	{"locked", "9001-northwind", "locked"},
	{"just-registered", "9003-fabrikam", "addgrace"},
	{"just-renewed", "9002-lakeshore", "renewgrace"},
	{"auto-renewed", "9004-contoso", "autorenewgrace"},
	{"expiring-soon", "9001-northwind", "expiring"},
	{"in-redemption", "9003-fabrikam", "redemption"},
	{"pending-delete", "9004-contoso", "pendingdelete"},
}

// runSeed populates an empty database with a synthetic local dataset.
//
// It is idempotent: every step checks for its own output first and skips if it
// is already there, so running it twice is a no-op rather than an error. That
// matters because `make dev` runs it on every start, not just the first.
func runSeed(cfg *config.AdminApiConfig, logger *zap.Logger) {
	ctx := context.Background()

	gormDB, err := postgres.NewConnection(postgres.Config{
		User:        os.Getenv("DB_USER"),
		Pass:        os.Getenv("DB_PASS"),
		Host:        os.Getenv("DB_HOST"),
		Port:        os.Getenv("DB_PORT"),
		DBName:      os.Getenv("DB_NAME"),
		SSLmode:     os.Getenv("DB_SSLMODE"),
		AutoMigrate: false, // admin-api owns migration; we only ever write rows
	})
	if err != nil {
		logger.Fatal("seed: failed to connect to the database", zap.Error(err))
	}

	faker := gofakeit.New(seedFakerSeed)

	idGenerator, err := snowflakeidgenerator.NewIDGenerator()
	if err != nil {
		logger.Fatal("seed: failed to create ID generator", zap.Error(err))
	}
	roidService := services.NewRoidService(idGenerator)

	eventPublisher := postgres.NewPostgresEventPublisher(gormDB, logger, cfg.LogEvents)

	registryOperatorRepo := postgres.NewGORMRegistryOperatorRepository(gormDB)
	tldRepo := postgres.NewGormTLDRepo(gormDB)
	dnsRecRepo := postgres.NewGormDNSRecordRepository(gormDB)
	phaseRepo := postgres.NewGormPhaseRepository(gormDB)
	registrarRepo := postgres.NewGormRegistrarRepository(gormDB)
	accreditationRepo := postgres.NewAccreditationRepository(gormDB)
	contactRepo := postgres.NewContactRepository(gormDB)
	hostRepo := postgres.NewGormHostRepository(gormDB)
	hostAddressRepo := postgres.NewGormHostAddressRepository(gormDB)
	domainRepo := postgres.NewDomainRepository(gormDB)
	nndnRepo := postgres.NewGormNNDNRepository(gormDB)
	premiumLabelRepo := postgres.NewGORMPremiumLabelRepository(gormDB)
	fxRepo := postgres.NewFXRepository(gormDB)

	ryService := services.NewRegistryOperatorService(registryOperatorRepo)
	tldService := services.NewTLDService(tldRepo, dnsRecRepo,
		services.WithOperatorRegistrarDeps(registrarRepo, accreditationRepo, registryOperatorRepo, eventPublisher),
	)
	phaseService := services.NewPhaseService(phaseRepo, tldRepo, eventPublisher)
	registrarService := services.NewRegistrarService(registrarRepo, eventPublisher)
	accreditationService := services.NewAccreditationService(accreditationRepo, registrarRepo, tldRepo, eventPublisher)
	contactService := services.NewContactService(contactRepo, *roidService, eventPublisher)
	hostService := services.NewHostService(hostRepo, hostAddressRepo, roidService, eventPublisher)
	domainService := services.NewDomainService(domainRepo, hostRepo, *roidService, nndnRepo, tldRepo,
		phaseRepo, premiumLabelRepo, fxRepo, registrarRepo, eventPublisher)

	s := &seeder{
		logger:      logger,
		faker:       faker,
		ry:          ryService,
		tld:         tldService,
		phase:       phaseService,
		registrar:   registrarService,
		accredit:    accreditationService,
		contact:     contactService,
		host:        hostService,
		domain:      domainService,
		domainRepo:  domainRepo,
		anchorToday: time.Now().UTC().Truncate(24 * time.Hour),
	}

	if err := s.run(ctx); err != nil {
		logger.Fatal("seed: failed", zap.Error(err))
	}

	logger.Info("seed: complete",
		zap.String("tld", seedTLD),
		zap.Int("registrars", len(seedRegistrars)),
		zap.Int("domains", len(seedDomains)),
	)
}

type seeder struct {
	logger *zap.Logger
	faker  *gofakeit.Faker

	ry        *services.RegistryOperatorService
	tld       *services.TLDService
	phase     *services.PhaseService
	registrar *services.RegistrarService
	accredit  *services.AccreditationService
	contact   *services.ContactService
	host      *services.HostService
	domain    *services.DomainService

	domainRepo interface {
		GetDomainByName(ctx context.Context, name string, preloadAll bool) (*entities.Domain, error)
		UpdateDomain(ctx context.Context, d *entities.Domain) (*entities.Domain, error)
	}

	// anchorToday is midnight UTC of the current day. Every lifecycle date is
	// derived from it so a run is stable within a day and the relative
	// positions (30 days out, 5 days ago) never change.
	anchorToday time.Time
}

func (s *seeder) run(ctx context.Context) error {
	if err := s.seedRegistryOperator(ctx); err != nil {
		return fmt.Errorf("registry operator: %w", err)
	}
	if err := s.seedTLDAndPhase(ctx); err != nil {
		return fmt.Errorf("tld: %w", err)
	}
	if err := s.seedRegistrars(ctx); err != nil {
		return fmt.Errorf("registrars: %w", err)
	}
	if err := s.seedDomains(ctx); err != nil {
		return fmt.Errorf("domains: %w", err)
	}
	return nil
}

func (s *seeder) seedRegistryOperator(ctx context.Context) error {
	if existing, err := s.ry.GetByRyID(ctx, seedRyID); err == nil && existing != nil {
		s.logger.Info("seed: registry operator already present, skipping", zap.String("ryid", seedRyID))
		return nil
	}

	_, err := s.ry.Create(ctx, &commands.CreateRegistryOperatorCommand{
		RyID:  seedRyID,
		Name:  "Domain OS Local Development Registry",
		Email: "registry@example.com",
		URL:   "https://registry.example.com",
	})
	if err != nil {
		return err
	}
	s.logger.Info("seed: created registry operator", zap.String("ryid", seedRyID))
	return nil
}

func (s *seeder) seedTLDAndPhase(ctx context.Context) error {
	if existing, err := s.tld.GetTLDByName(ctx, seedTLD, false); err == nil && existing != nil {
		s.logger.Info("seed: tld already present, skipping", zap.String("tld", seedTLD))
	} else {
		if _, err := s.tld.CreateTLD(ctx, &commands.CreateTLDCommand{
			Name:                     seedTLD,
			RyID:                     seedRyID,
			CreateOperatorRegistrars: true,
			AllowEscrowImport:        true,
		}); err != nil {
			return err
		}
		s.logger.Info("seed: created tld", zap.String("tld", seedTLD))
	}

	if existing, err := s.phase.GetPhaseByTLDAndName(ctx, seedTLD, seedPhase); err == nil && existing != nil {
		s.logger.Info("seed: phase already present, skipping", zap.String("phase", seedPhase))
		return nil
	}

	// Start the GA phase a year back so every seeded domain falls inside it.
	if _, err := s.phase.CreatePhase(ctx, &commands.CreatePhaseCommand{
		Name:    seedPhase,
		Type:    "GA",
		Starts:  s.anchorToday.AddDate(-1, 0, 0),
		TLDName: seedTLD,
	}); err != nil {
		return err
	}
	s.logger.Info("seed: created phase", zap.String("phase", seedPhase))
	return nil
}

func (s *seeder) seedRegistrars(ctx context.Context) error {
	for _, r := range seedRegistrars {
		if existing, err := s.registrar.GetByClID(ctx, r.ClID, false); err == nil && existing != nil {
			s.logger.Info("seed: registrar already present, skipping", zap.String("clid", r.ClID))
			continue
		}

		addr, err := entities.NewAddress(s.faker.City(), s.faker.CountryAbr())
		if err != nil {
			return fmt.Errorf("address for %s: %w", r.ClID, err)
		}
		postal, err := entities.NewRegistrarPostalInfo("int", addr)
		if err != nil {
			return fmt.Errorf("postal info for %s: %w", r.ClID, err)
		}

		if _, err := s.registrar.Create(ctx, &commands.CreateRegistrarCommand{
			ClID:       r.ClID,
			Name:       r.Name,
			Email:      fmt.Sprintf("ops@%s.example.com", strings.Split(r.ClID, "-")[1]),
			GurID:      r.GurID,
			PostalInfo: [2]*entities.RegistrarPostalInfo{postal, nil},
			URL:        fmt.Sprintf("https://%s.example.com", strings.Split(r.ClID, "-")[1]),
			// NewRegistrar defaults to "readonly", which Registrar.AccreditFor
			// rejects. A generic TLD additionally requires IANAStatus
			// "Accredited" alongside a GurID — set both so accreditation below
			// succeeds for either TLD type.
			Status:     string(entities.RegistrarStatusOK),
			IANAStatus: entities.IANARegistrarStatusAccredited,
		}); err != nil {
			return fmt.Errorf("create %s: %w", r.ClID, err)
		}

		if err := s.accredit.CreateAccreditation(ctx, seedTLD, r.ClID); err != nil {
			return fmt.Errorf("accredit %s: %w", r.ClID, err)
		}
		s.logger.Info("seed: created registrar", zap.String("clid", r.ClID))
	}
	return nil
}

func (s *seeder) seedDomains(ctx context.Context) error {
	for _, d := range seedDomains {
		name := fmt.Sprintf("%s.%s", d.Label, seedTLD)

		if existing, err := s.domainRepo.GetDomainByName(ctx, name, false); err == nil && existing != nil {
			s.logger.Info("seed: domain already present, skipping", zap.String("domain", name))
			continue
		}

		registrantID, err := s.seedContact(ctx, d.Label, d.Registrar)
		if err != nil {
			return fmt.Errorf("contact for %s: %w", name, err)
		}
		hostNames, err := s.seedHosts(ctx, d.Label, d.Registrar)
		if err != nil {
			return fmt.Errorf("hosts for %s: %w", name, err)
		}

		created, err := s.domain.Create(ctx, &commands.CreateDomainCommand{
			Name:         name,
			ClID:         d.Registrar,
			CrRr:         d.Registrar,
			AuthInfo:     seedAuthInfo,
			RegistrantID: registrantID,
			AdminID:      registrantID,
			TechID:       registrantID,
			BillingID:    registrantID,
			ExpiryDate:   s.expiryFor(d.Lifecycle),
		})
		if err != nil {
			return fmt.Errorf("create %s: %w", name, err)
		}

		// Delegate the domain to its nameservers. Without this the domain stays
		// Inactive and never reaches the OK status, which makes an "active"
		// seed subject misleading — and leaves nothing for the DNS and zone
		// generation paths to act on.
		for _, hostName := range hostNames {
			if err := s.domain.AddHostToDomainByHostName(ctx, name, hostName, true); err != nil {
				return fmt.Errorf("delegate %s to %s: %w", name, hostName, err)
			}
		}

		// Re-read: AddHostToDomainByHostName has mutated status since Create.
		created, err = s.domainRepo.GetDomainByName(ctx, name, false)
		if err != nil {
			return fmt.Errorf("reload %s: %w", name, err)
		}

		if err := s.applyLifecycle(ctx, created, d.Lifecycle); err != nil {
			return fmt.Errorf("lifecycle %s for %s: %w", d.Lifecycle, name, err)
		}
		s.logger.Info("seed: created domain",
			zap.String("domain", name), zap.String("lifecycle", d.Lifecycle))
	}
	return nil
}

// seedContact creates a synthetic registrant. Every field is faker-generated;
// none of it corresponds to a real person.
func (s *seeder) seedContact(ctx context.Context, label, clid string) (string, error) {
	contactID := fmt.Sprintf("c-%s", label)
	if len(contactID) > entities.CLID_MAX_LENGTH {
		contactID = contactID[:entities.CLID_MAX_LENGTH]
	}

	addr, err := entities.NewAddress(s.faker.City(), s.faker.CountryAbr())
	if err != nil {
		return "", err
	}
	postal, err := entities.NewContactPostalInfo("int", s.faker.Name(), addr)
	if err != nil {
		return "", err
	}

	if _, err := s.contact.CreateContact(ctx, &commands.CreateContactCommand{
		ID:         contactID,
		Email:      fmt.Sprintf("%s@example.com", contactID),
		AuthInfo:   seedAuthInfo,
		ClID:       clid,
		PostalInfo: [2]*entities.ContactPostalInfo{postal, nil},
	}); err != nil {
		// A contact left over from a previous run is fine — reuse it.
		s.logger.Debug("seed: contact create returned error, assuming it exists",
			zap.String("contact", contactID), zap.Error(err))
	}
	return contactID, nil
}

// seedHosts creates the two nameservers a domain is delegated to. They live
// under the seeded TLD so nothing resolves outside it.
func (s *seeder) seedHosts(ctx context.Context, label, clid string) ([]string, error) {
	names := make([]string, 0, 2)
	for i := 1; i <= 2; i++ {
		hostName := fmt.Sprintf("ns%d.%s.%s", i, label, seedTLD)
		if _, err := s.host.CreateHost(ctx, &commands.CreateHostCommand{
			Name:      hostName,
			ClID:      entities.ClIDType(clid),
			CrRr:      entities.ClIDType(clid),
			Addresses: []string{s.faker.IPv4Address()},
		}); err != nil {
			s.logger.Debug("seed: host create returned error, assuming it exists",
				zap.String("host", hostName), zap.Error(err))
		}
		names = append(names, hostName)
	}
	return names, nil
}

// expiryFor places the expiry date so the requested lifecycle state is coherent
// with it. A domain in redemption must already have expired; an active one must
// not have.
func (s *seeder) expiryFor(lifecycle string) time.Time {
	switch lifecycle {
	case "expiring":
		return s.anchorToday.AddDate(0, 0, 10)
	case "redemption":
		return s.anchorToday.AddDate(0, 0, -10)
	case "pendingdelete":
		return s.anchorToday.AddDate(0, 0, -40)
	case "addgrace", "renewgrace", "autorenewgrace":
		return s.anchorToday.AddDate(1, 0, 0)
	default: // active, locked
		return s.anchorToday.AddDate(0, 6, 0)
	}
}

// applyLifecycle stamps the RGP timers and status flags for the requested
// state, then persists. Creation always yields a plain active domain, so the
// interesting states are set here rather than through CreateDomainCommand —
// the command's RGPStatus is not applied to an entity built by NewDomain.
func (s *seeder) applyLifecycle(ctx context.Context, d *entities.Domain, lifecycle string) error {
	switch lifecycle {
	case "active":
		// Nothing to do — NewDomain already yields an active domain.
		return nil

	case "locked":
		d.Status.ClientTransferProhibited = true
		d.Status.ClientUpdateProhibited = true
		d.Status.ClientDeleteProhibited = true
		d.Status.OK = false

	case "addgrace":
		d.RGPStatus.AddPeriodEnd = s.anchorToday.AddDate(0, 0, 3)
		d.RGPStatus.TransferLockPeriodEnd = s.anchorToday.AddDate(0, 0, 60)

	case "renewgrace":
		d.RGPStatus.RenewPeriodEnd = s.anchorToday.AddDate(0, 0, 4)

	case "autorenewgrace":
		d.RGPStatus.AutoRenewPeriodEnd = s.anchorToday.AddDate(0, 0, 40)

	case "expiring":
		// Expiry date alone carries this one; no RGP timer applies yet.
		return nil

	case "redemption":
		// Expired, deleted, still restorable.
		d.Status.PendingDelete = true
		d.Status.ClientDeleteProhibited = false
		d.Status.ServerDeleteProhibited = false
		d.Status.OK = false
		d.RGPStatus.RedemptionPeriodEnd = s.anchorToday.AddDate(0, 0, 20)
		d.RGPStatus.PurgeDate = s.anchorToday.AddDate(0, 0, 25)

	case "pendingdelete":
		// Redemption has lapsed; awaiting purge and no longer restorable.
		d.Status.PendingDelete = true
		d.Status.ClientDeleteProhibited = false
		d.Status.ServerDeleteProhibited = false
		d.Status.OK = false
		d.RGPStatus.RedemptionPeriodEnd = s.anchorToday.AddDate(0, 0, -5)
		d.RGPStatus.PurgeDate = s.anchorToday.AddDate(0, 0, 2)

	default:
		return fmt.Errorf("unknown lifecycle state %q", lifecycle)
	}

	_, err := s.domainRepo.UpdateDomain(ctx, d)
	return err
}
