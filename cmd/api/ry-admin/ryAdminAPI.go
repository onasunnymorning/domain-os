package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/cmd/api/ry-admin/config"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/broker/rabbitmq"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/snowflakeidgenerator"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/web/ianaregistrars"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/web/icannspec5"
	"github.com/onasunnymorning/domain-os/internal/interface/rest"
	"go.uber.org/zap"

	"os"

	"github.com/apex/gateway"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	ginprometheus "github.com/zsais/go-gin-prometheus"

	docs "github.com/onasunnymorning/domain-os/docs" // Import docs pkg to be able to access docs.json https://github.com/swaggo/swag/issues/830#issuecomment-725587162
	swaggerFiles "github.com/swaggo/files"           // swagger embed files
	ginSwagger "github.com/swaggo/gin-swagger"       // gin-swagger middleware

	// NeW Relic APM
	"github.com/newrelic/go-agent/v3/newrelic"
)

const (
	APP_NAME  = entities.AppAdminAPI
	JWT_TOKEN = "the-brave-may-not-live-forever-but-the-cautious-do-not-live-at-all"
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
	docs.SwaggerInfo.Version = fmt.Sprintf("%s-%s", cfg.Version, cfg.GitSHA)
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

var (
	GitSHA string // GitSHA is the git commit hash set by the build process Ref. https://stackoverflow.com/a/1132237
)

// @title Domain OS Admin API
// @license.name Geoffrey De Prins All rights reserved
func main() {
	// create a new logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to create logger: %s", err)
	}

	// Load the APP configuration and log it
	cfg := config.LoadConfig(GitSHA)
	logger.Info("Starting Admin API with following config", zap.Any("config", cfg))

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

	// Set up the GORM DB connection
	gormDB, err := postgres.NewConnection(
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
	if err != nil {
		logger.Panic("Failed to connect to the database", zap.Error(err))
	}

	// Set up Eventservice if enabled
	var eventSvc *services.EventService
	if cfg.EventStreamEnabled {
		logger.Debug("Setting up Event Stream")
		portStr := os.Getenv("RMQ_PORT")
		port, err := strconv.Atoi(portStr)
		if err != nil {
			logger.Error("Failed to convert envar RMQ_PORT to int", zap.Error(err))
		}

		eventRepo, err := rabbitmq.NewEventRepository(&rabbitmq.RabbitConfig{
			Host:     os.Getenv("RMQ_HOST"),
			Port:     port,
			Username: os.Getenv("RMQ_USER"),
			Password: os.Getenv("RMQ_PASS"),
			Topic:    os.Getenv("EVENT_STREAM_TOPIC"),
		})
		if err != nil {
			logger.Error("Failed to create Event Repository", zap.Error(err))
		}
		eventSvc = services.NewEventService(eventRepo)
		err = eventSvc.SendStream(&entities.Event{
			Source: APP_NAME,
			User:   "system",
			Action: "startup",
			Details: entities.EventDetails{
				Result: entities.EventResultSuccess,
			},
			Timestamp: time.Now().UTC(),
		})
		if err != nil {
			logger.Error("Failed to send startup event", zap.Error(err))
		}
	}

	// SET UP SERVICES
	// Roid
	idGenerator, err := snowflakeidgenerator.NewIDGenerator()
	if err != nil {
		logger.Panic("Failed to create ID Generator", zap.Error(err))
	}
	roidService := services.NewRoidService(idGenerator)
	// TODO: Register the Node ID in Redis or something. Then we can add a check to avoid the unlikely scenario of a duplicate Node ID.
	log.Printf("Snowflake Node ID: %d", roidService.ListNode())
	// Registry Operators
	registryOperatorRepo := postgres.NewGORMRegistryOperatorRepository(gormDB)
	registryOperatorService := services.NewRegistryOperatorService(registryOperatorRepo)
	// TLDs
	tldRepo := postgres.NewGormTLDRepo(gormDB)
	dnsRecRepo := postgres.NewGormDNSRecordRepository(gormDB)
	tldService := services.NewTLDService(tldRepo, dnsRecRepo)
	// Phases
	phaseRepo := postgres.NewGormPhaseRepository(gormDB)
	phaseService := services.NewPhaseService(phaseRepo, tldRepo)
	// Fees
	feeRepo := postgres.NewFeeRepository(gormDB)
	feeService := services.NewFeeService(phaseRepo, feeRepo)
	// Prices
	priceRepo := postgres.NewGormPriceRepository(gormDB)
	priceService := services.NewPriceService(phaseRepo, priceRepo)
	// Premium Lists
	premiumListRepo := postgres.NewGORMPremiumListRepository(gormDB)
	premiumListService := services.NewPremiumListService(premiumListRepo)
	// Premium Labels
	premiumLabelRepo := postgres.NewGORMPremiumLabelRepository(gormDB)
	premiumLabelService := services.NewPremiumLabelService(premiumLabelRepo)
	// NNDNs
	nndnRepo := postgres.NewGormNNDNRepository(gormDB)
	nndnService := services.NewNNDNService(nndnRepo)
	// FX
	fxRepo := postgres.NewFXRepository(gormDB)
	fxService := services.NewFXService(fxRepo)
	// Sync
	ianaRepo := ianaregistrars.NewIANARRepository()
	icannRepo := icannspec5.NewICANNRepo()
	spec5Repo := postgres.NewSpec5Repository(gormDB)
	iregistrarRepo := postgres.NewIANARegistrarRepository(gormDB)
	syncService := services.NewSyncService(iregistrarRepo, spec5Repo, icannRepo, ianaRepo, fxRepo)
	// Spec5
	spec5Service := services.NewSpec5Service(spec5Repo)
	// IANA Registrars
	ianaRegistrarService := services.NewIANARegistrarService(iregistrarRepo)
	// Registrars
	registrarRepo := postgres.NewGormRegistrarRepository(gormDB)
	registrarService := services.NewRegistrarService(registrarRepo)
	// Accreditations
	accreditationRepo := postgres.NewAccreditationRepository(gormDB)
	accreditationService := services.NewAccreditationService(accreditationRepo, registrarRepo, tldRepo)
	// Contacts
	contactRepo := postgres.NewContactRepository(gormDB)
	contactService := services.NewContactService(contactRepo, *roidService)
	// Hosts
	hostRepo := postgres.NewGormHostRepository(gormDB)
	hostAddressRepo := postgres.NewGormHostAddressRepository(gormDB)
	hostService := services.NewHostService(hostRepo, hostAddressRepo, roidService)
	// Domains
	domainRepo := postgres.NewDomainRepository(gormDB)
	domainService := services.NewDomainService(domainRepo, hostRepo, *roidService, nndnRepo, tldRepo, phaseRepo, premiumLabelRepo, fxRepo, registrarRepo)

	// REMOVEME:
	// Quotes
	// quoteService := services.NewQuoteService(tldRepo, domainRepo, premiumLabelRepo, fxRepo)
	// FIXME: How to do better dependecy injection on this without the risk of a nil pointer
	// Possibly merge the domainservice and quoteservice
	// domainService.QuoteService = *quoteService

	// Whois
	whoisService := services.NewWhoisService(domainRepo, registrarRepo)

	// Create Gin Engine/Router
	r := gin.Default()
	// Configure CORS middleware
	config := cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"}, // Add your frontend URL here
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	r.Use(cors.New(config))

	// Attach the Stream Middleware
	if cfg.EventStreamEnabled && eventSvc != nil {
		r.Use(rest.StreamMiddleWare(eventSvc))
	}

	// Attach the Prometheus Middleware
	if cfg.PrometheusEnabled {
		initPrometheusMetrics(r)
	}

	rest.NewPingController(r)
	rest.NewRegistryOperatorController(r, registryOperatorService, TokenAuthMiddleware())
	rest.NewTLDController(r, tldService, domainService, TokenAuthMiddleware())
	rest.NewNNDNController(r, nndnService, TokenAuthMiddleware())
	rest.NewSyncController(r, syncService, TokenAuthMiddleware())
	rest.NewSpec5Controller(r, spec5Service, TokenAuthMiddleware())
	rest.NewIANARegistrarController(r, ianaRegistrarService, TokenAuthMiddleware())
	rest.NewRegistrarController(r, registrarService, ianaRegistrarService, TokenAuthMiddleware())
	rest.NewContactController(r, contactService, TokenAuthMiddleware())
	rest.NewHostController(r, hostService, TokenAuthMiddleware())
	rest.NewDomainController(r, domainService, TokenAuthMiddleware())
	rest.NewPhaseController(r, phaseService, TokenAuthMiddleware())
	rest.NewFeeController(r, feeService, TokenAuthMiddleware())
	rest.NewPriceController(r, priceService, TokenAuthMiddleware())
	rest.NewAccreditationController(r, accreditationService, TokenAuthMiddleware())
	rest.NewPremiumController(r, premiumListService, premiumLabelService, TokenAuthMiddleware())
	rest.NewFXController(r, fxService, TokenAuthMiddleware())
	// rest.NewQuoteController(r, quoteService, TokenAuthMiddleware())
	rest.NewWhoisController(r, whoisService, TokenAuthMiddleware())

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
