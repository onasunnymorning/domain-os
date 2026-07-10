package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/cmd/api/ry-admin/config"
	"github.com/onasunnymorning/domain-os/internal/buildinfo"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	appservices "github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/askg"
	anthropicprovider "github.com/onasunnymorning/domain-os/internal/askg/provider/anthropic"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/snowflakeidgenerator"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/storage"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/web/ianaregistrars"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/web/icannspec5"
	"github.com/onasunnymorning/domain-os/internal/interface/rest"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"os"

	"github.com/apex/gateway"
	"github.com/gin-contrib/cors"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	ginprometheus "github.com/zsais/go-gin-prometheus"

	docs "github.com/onasunnymorning/domain-os/docs" // Import docs pkg to be able to access docs.json https://github.com/swaggo/swag/issues/830#issuecomment-725587162
	swaggerFiles "github.com/swaggo/files"           // swagger embed files
	ginSwagger "github.com/swaggo/gin-swagger"       // gin-swagger middleware

	// NeW Relic APM
	"github.com/newrelic/go-agent/v3/newrelic"
)

const (
	APP_NAME = entities.AppAdminAPI
)

var (
	JWT_TOKEN = os.Getenv("ADMIN_TOKEN")
)

// inLambda returns true if the code is running in AWS Lambda
func inLambda() bool {
	if lambdaTaskRoot := os.Getenv("LAMBDA_TASK_ROOT"); lambdaTaskRoot != "" {
		return true
	}
	return false
}

// setSwaggerInfo sets the swagger API documentation variables based on the environment variables. These are used to generate the swagger documentation, such as version, address, host, etc.
func setSwaggerInfo(cfg *config.AdminApiConfig) {
	docs.SwaggerInfo.Version = fmt.Sprintf("%s+%s", buildinfo.Version, buildinfo.GitSHA)
	docs.SwaggerInfo.Host = fmt.Sprintf("%s:%s", cfg.ApiHost, cfg.ApiPort)
	docs.SwaggerInfo.Title = cfg.ApiName
}

// runningInDocker returns true if the code is running in a Docker container. We determine this by looking for the /.dockerenv file
func runningInDocker() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	return false
}

// initNewRelicAPM initializes New Relic APM
func initNewRelicAPM() (*newrelic.Application, error) {
	return newrelic.NewApplication(
		newrelic.ConfigAppName(APP_NAME),
		newrelic.ConfigLicense(os.Getenv("NEW_RELIC_LICENSE_KEY")),
		newrelic.ConfigAppLogForwardingEnabled(true),
	)

}

// initPrometheusMetrics initializes Prometheus metrics middleware
func initPrometheusMetrics(r *gin.Engine) {
	p := ginprometheus.NewPrometheus("gin")
	p.Use(r) // Attach it to the Gin router
}

// TokenAuthMiddleware checks for the constant JWT token in the Authorization header
func TokenAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		// Check if the Authorization header is present and properly formatted
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header missing or malformed"})
			return
		}

		// Extract the token from the header
		token := strings.TrimPrefix(authHeader, "Bearer ")

		// Compare the token with the constant JWT token
		if token != JWT_TOKEN {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		// Token is valid; proceed to the next handler
		c.Next()
	}
}



// @title Domain OS Admin API
// @license.name Geoffrey De Prins All rights reserved
func main() {
	// create a new logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to create logger: %s", err)
	}

	// Load the APP configuration and log it
	cfg := config.LoadConfig(buildinfo.GitSHA)
	logger.Info("Starting Admin API with following config", zap.Any("config", cfg))
	logger.Info("Build info", zap.String("version", buildinfo.Version), zap.String("git_sha", buildinfo.GitSHA), zap.String("build_date", buildinfo.BuildDate))

	// Check for init-registrars command
	if len(os.Args) > 1 && os.Args[1] == "init-registrars" {
		runInitRegistrars(cfg, logger)
		return
	}

	// Try and determine the runtime environment
	if !runningInDocker() {
		if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
			logger.Info("Detected we are Running in Kubernetes")
		} else {
			logger.Warn("Could not determine runtime environment")
		}
	} else {
		logger.Info("Detected we are running in Docker")
	}

	// Initialize New Relic APM if enabled
	if cfg.NewRelicEnabled {
		logger.Info("Initializing New Relic APM - remove/setFalse environment variable 'NEW_RELIC_ENABED' to disable")
		app, err := initNewRelicAPM()
		if err != nil {
			logger.Error("Failed to initialize New Relic APM", zap.Error(err))
		}
		defer app.Shutdown(0)
	}

	// Initialize variables for the Swagger API documentation
	setSwaggerInfo(cfg)

	// Set up the GORM DB connection.
	// Prefer DATABASE_URL (single connection string, e.g. from Neon) over individual vars.
	var gormDB *gorm.DB
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		logger.Info("Connecting to database using DATABASE_URL")
		gormDB, err = postgres.NewConnectionFromURL(dbURL, cfg.AutoMigrate)
	} else {
		logger.Info("Connecting to database using individual DB_* env vars")
		gormDB, err = postgres.NewConnection(
			postgres.Config{
				User:        os.Getenv("DB_USER"),
				Pass:        os.Getenv("DB_PASS"),
				Host:        os.Getenv("DB_HOST"),
				Port:        os.Getenv("DB_PORT"),
				DBName:      os.Getenv("DB_NAME"),
				SSLmode:     os.Getenv("DB_SSLMODE"),
				AutoMigrate: cfg.AutoMigrate,
			},
		)
	}
	if err != nil {
		logger.Panic("Failed to connect to the database", zap.Error(err))
	}

	// Set up EventPublisher (PostgreSQL outbox)
	eventPublisher := postgres.NewPostgresEventPublisher(gormDB, logger, cfg.LogEvents)

	// SET UP SERVICES
	// Roid
	idGenerator, err := snowflakeidgenerator.NewIDGenerator()
	if err != nil {
		logger.Panic("Failed to create ID Generator", zap.Error(err))
	}
	roidService := appservices.NewRoidService(idGenerator)
	// TODO: Register the Node ID in Redis or something. Then we can add a check to avoid the unlikely scenario of a duplicate Node ID.
	log.Printf("Snowflake Node ID: %d", roidService.ListNode())
	// Registry Operators
	registryOperatorRepo := postgres.NewGORMRegistryOperatorRepository(gormDB)
	registryOperatorService := appservices.NewRegistryOperatorService(registryOperatorRepo)
	// TLDs
	tldRepo := postgres.NewGormTLDRepo(gormDB)
	dnsRecRepo := postgres.NewGormDNSRecordRepository(gormDB)
	// tldService is created below after registrar deps are initialized
	// Phases
	phaseRepo := postgres.NewGormPhaseRepository(gormDB)
	phaseService := appservices.NewPhaseService(phaseRepo, tldRepo, eventPublisher)
	// Fees
	feeRepo := postgres.NewFeeRepository(gormDB)
	feeService := appservices.NewFeeService(phaseRepo, feeRepo)
	// Prices
	priceRepo := postgres.NewGormPriceRepository(gormDB)
	priceService := appservices.NewPriceService(phaseRepo, priceRepo)
	// Premium Lists
	premiumListRepo := postgres.NewGORMPremiumListRepository(gormDB)
	premiumListService := appservices.NewPremiumListService(premiumListRepo)
	// Premium Labels
	premiumLabelRepo := postgres.NewGORMPremiumLabelRepository(gormDB)
	premiumLabelService := appservices.NewPremiumLabelService(premiumLabelRepo)
	// NNDNs
	nndnRepo := postgres.NewGormNNDNRepository(gormDB)
	nndnService := appservices.NewNNDNService(nndnRepo)
	// FX
	fxRepo := postgres.NewFXRepository(gormDB)
	fxService := appservices.NewFXService(fxRepo)
	// Sync
	ianaRepo := ianaregistrars.NewIANARRepository()
	icannRepo := icannspec5.NewICANNRepo()
	spec5Repo := postgres.NewSpec5Repository(gormDB)
	iregistrarRepo := postgres.NewIANARegistrarRepository(gormDB)
	syncService := appservices.NewSyncService(iregistrarRepo, spec5Repo, icannRepo, ianaRepo, fxRepo)
	// Spec5
	spec5Service := appservices.NewSpec5Service(spec5Repo)
	// IANA Registrars
	ianaRegistrarService := appservices.NewIANARegistrarService(iregistrarRepo)
	// Registrars
	registrarRepo := postgres.NewGormRegistrarRepository(gormDB)
	registrarService := appservices.NewRegistrarService(registrarRepo, eventPublisher)
	// Accreditations
	accreditationRepo := postgres.NewAccreditationRepository(gormDB)
	accreditationService := appservices.NewAccreditationService(accreditationRepo, registrarRepo, tldRepo, eventPublisher)
	// Now create TLDService with operator registrar auto-provisioning deps
	tldService := appservices.NewTLDService(tldRepo, dnsRecRepo,
		appservices.WithOperatorRegistrarDeps(registrarRepo, accreditationRepo, registryOperatorRepo, eventPublisher),
	)
	// Contacts
	contactRepo := postgres.NewContactRepository(gormDB)
	contactService := appservices.NewContactService(contactRepo, *roidService, eventPublisher)
	// Hosts
	hostRepo := postgres.NewGormHostRepository(gormDB)
	hostAddressRepo := postgres.NewGormHostAddressRepository(gormDB)
	hostService := appservices.NewHostService(hostRepo, hostAddressRepo, roidService, eventPublisher)
	// Domains
	domainRepo := postgres.NewDomainRepository(gormDB)
	domainService := appservices.NewDomainService(domainRepo, hostRepo, *roidService, nndnRepo, tldRepo, phaseRepo, premiumLabelRepo, fxRepo, registrarRepo, eventPublisher)

	// Domain Tombstones — archival records for purged domains
	tombstoneRepo := postgres.NewGormTombstoneRepository(gormDB)
	tombstoneService := appservices.NewTombstoneService(tombstoneRepo)
	domainService.SetTombstoneRepo(tombstoneRepo)

	// Zone Slaving — SOA serial drift monitoring for zone migrations
	serialDriftRepo := postgres.NewSerialDriftRepository(gormDB)
	zoneSlavingService := appservices.NewZoneSlavingService(serialDriftRepo)

	// Event Search — unified search across hot (PG) and warm (S3) tiers
	eventRepo := postgres.NewPostgresEventRepository(gormDB)
	var archiveReader *storage.EventArchiveReader
	s3Client, s3Err := storage.NewEventLogsS3Client()
	if s3Err != nil {
		logger.Warn("S3 not configured — warm-tier event search disabled",
			zap.Error(s3Err))
	} else {
		archiveReader = storage.NewEventArchiveReader(s3Client)
	}
	eventSearchService := appservices.NewEventSearchService(eventRepo, archiveReader, 0)

	// Whois
	whoisService := appservices.NewWhoisService(domainRepo, registrarRepo)

	// Dnssec
	dnssecService := appservices.NewDnssecService()



	// Create Gin Engine/Router
	// r := gin.Default()
	// Create a new Gin router without any default middleware.
	r := gin.New()
	// Use ginzap middleware to log requests with Zap, skipping the /ping endpoint to reduce log noise
	r.Use(ginzap.GinzapWithConfig(logger, &ginzap.Config{
		TimeFormat: time.RFC3339,
		UTC:        true,
		SkipPaths:  []string{"/ping"},
	}))

	// Keep multipart memory small so large uploads spill to disk
	r.MaxMultipartMemory = 8 << 20 // 8 MiB

	// Use ginzap recovery middleware to catch panics and log with Zap
	r.Use(ginzap.RecoveryWithZap(logger, true))

	// Configure CORS middleware from CORS_ALLOWED_ORIGINS env var (comma-separated).
	allowedOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = "http://localhost:3000"
	}
	corsConfig := cors.Config{
		AllowOrigins:     strings.Split(allowedOrigins, ","),
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Tenant-ID"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	r.Use(cors.New(corsConfig))

	// Attach context propagation middleware
	r.Use(rest.ContextMiddleware())

	// Attach the Prometheus Middleware
	if cfg.PrometheusEnabled {
		initPrometheusMetrics(r)
	}

	// Determine the authentication middleware (Hybrid: Auth0 + Legacy Token)
	logger.Info("Initializing Hybrid authentication",
		zap.Bool("auth0_enabled", cfg.Auth0Enabled),
		zap.String("auth0_domain", cfg.Auth0Domain))

	auth0Middleware := rest.Auth0Middleware(cfg.Auth0Domain, cfg.Auth0Audience, JWT_TOKEN, cfg.Auth0Enabled)
	authMiddleware := func(c *gin.Context) {
		auth0Middleware(c)
		if !c.IsAborted() {
			rest.ContextPropagationMiddleware()(c)
		}
		c.Next()
	}

	rest.NewPingController(r)
	rest.NewRegistryOperatorController(r, registryOperatorService, authMiddleware)
	rest.NewTLDController(r, tldService, domainService, authMiddleware)
	rest.NewNNDNController(r, nndnService, authMiddleware)
	rest.NewSyncController(r, syncService, authMiddleware)
	rest.NewSpec5Controller(r, spec5Service, authMiddleware)
	rest.NewIANARegistrarController(r, ianaRegistrarService, authMiddleware)
	rest.NewRegistrarController(r, registrarService, ianaRegistrarService, authMiddleware)
	rest.NewContactController(r, contactService, authMiddleware)
	rest.NewHostController(r, hostService, authMiddleware)
	rest.NewDomainController(r, domainService, authMiddleware)
	rest.NewTombstoneController(r, tombstoneService, authMiddleware)
	rest.NewEventController(r, domainService, authMiddleware)
	rest.NewEventSearchController(r, eventSearchService, authMiddleware)
	rest.NewPhaseController(r, phaseService, authMiddleware)
	rest.NewFeeController(r, feeService, authMiddleware)
	rest.NewPriceController(r, priceService, authMiddleware)
	rest.NewAccreditationController(r, accreditationService, authMiddleware)
	rest.NewPremiumController(r, premiumListService, premiumLabelService, authMiddleware)
	rest.NewFXController(r, fxService, authMiddleware)
	// rest.NewQuoteController(r, quoteService, authMiddleware)
	rest.NewWhoisController(r, whoisService, authMiddleware)
	rest.NewDnssecController(r, dnssecService, authMiddleware)
	// Workflows
	rest.NewWorkflowController(r, authMiddleware)
	// Escrow
	rest.NewEscrowController(r, authMiddleware)
	// Zone Slaving (serial drift monitoring)
	rest.NewZoneSlavingController(r, zoneSlavingService, authMiddleware)

	// Agent Alpaca (Ask G) — only enabled when ANTHROPIC_API_KEY is configured.
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		askgModel := os.Getenv("LLM_MODEL")
		if askgModel == "" {
			askgModel = anthropicprovider.DefaultModel
		}

		askgCfg := askg.Config{
			Provider:        "anthropic",
			Model:           askgModel,
			ClassifierModel: anthropicprovider.DefaultClassifier,
			MaxIterations:   askg.DefaultMaxIterations,
			APIKey:          apiKey,
			BaseURL:         os.Getenv("ANTHROPIC_BASE_URL"),
		}

		askgProvider := anthropicprovider.NewAdapter(askgCfg)
		askgLogger := slog.Default()

		// KnowledgeService — optional; if docs/index.yaml is not found or
		// cannot be loaded, the answer_system_question tool is simply not
		// registered and the agent falls back to data-only tools.
		var knowledgeSvc interfaces.KnowledgeService
		projectRoot := os.Getenv("KNOWLEDGE_BASE_DIR")
		if projectRoot == "" {
			// Fall back to the current working directory.
			projectRoot, _ = os.Getwd()
		}
		ks, ksErr := appservices.NewKnowledgeService(projectRoot)
		if ksErr != nil {
			logger.Warn("KnowledgeService not available — answer_system_question tool disabled",
				zap.Error(ksErr),
				zap.String("project_root", projectRoot))
		} else {
			knowledgeSvc = ks
			logger.Info("KnowledgeService loaded",
				zap.Int("docs", ks.DocCount()),
				zap.Int("chunks", ks.ChunkCount()))
		}

		askgExecutor := askg.NewInProcessToolExecutor(domainService, tldService, knowledgeSvc, askgLogger)
		askgOrch := askg.NewOrchestrator(askgProvider, askgExecutor, askgCfg, askgLogger)

		rest.NewAgentController(r, askgOrch, authMiddleware)
		logger.Info("Agent Alpaca (Ask G) enabled", zap.String("model", askgCfg.Model))
	} else {
		logger.Warn("Agent Alpaca (Ask G) disabled — ANTHROPIC_API_KEY not set. Set it via Doppler to enable the /agent endpoints.")
	}


	// Serve the swagger documentation
	r.GET("/swagger/*any", ginSwagger.WrapHandler(
		swaggerFiles.Handler,
		ginSwagger.DocExpansion("none"))) // collapse all endpoints by default

	if inLambda() {
		logger.Info("Determined we are running in AWS Lambda")
		// Start the server using the AWS Lambda proxy
		log.Fatal(gateway.ListenAndServe(os.Getenv("API_PORT"), r))
	} else {
		// Start the server using the standard HTTP server
		r.Run(":" + os.Getenv("API_PORT"))
	}

}
