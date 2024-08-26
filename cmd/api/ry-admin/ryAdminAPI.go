package main

import (
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/onasunnymorning/domain-os/cmd/api/ry-admin/config"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/broker/rabbitmq"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/snowflakeidgenerator"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/web/iana"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/web/icann"
	"github.com/onasunnymorning/domain-os/internal/interface/rest"

	"os"

	"github.com/apex/gateway"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	docs "github.com/onasunnymorning/domain-os/docs" // Import docs pkg to be able to access docs.json https://github.com/swaggo/swag/issues/830#issuecomment-725587162
	swaggerFiles "github.com/swaggo/files"           // swagger embed files
	ginSwagger "github.com/swaggo/gin-swagger"       // gin-swagger middleware

	// NeW Relic APM

	"github.com/newrelic/go-agent/v3/newrelic"
)

const (
	AppName = entities.AppAdminAPI
)

// inLambda returns true if the code is running in AWS Lambda
func inLambda() bool {
	if lambdaTaskRoot := os.Getenv("LAMBDA_TASK_ROOT"); lambdaTaskRoot != "" {
		return true
	}
	return false
}

// setSwaggerInfo sets the swagger API documentation variables based on the environment variables. These are used to generate the swagger documentation, such as version, host, etc.
func setSwaggerInfo() {
	docs.SwaggerInfo.Version = os.Getenv("API_VERSION")
	docs.SwaggerInfo.Host = os.Getenv("API_HOST") + ":" + os.Getenv("API_PORT")
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
		newrelic.ConfigAppName(AppName),
		newrelic.ConfigLicense(os.Getenv("NEW_RELIC_LICENSE_KEY")),
		newrelic.ConfigAppLogForwardingEnabled(true),
	)

}

// @title APEX Domain OS ADMIN API
// @license.name APEX all rights reserved
func main() {
	cfg := config.LoadConfig()
	log.Println("Starting Admin API with following config:")
	jBytes, err := json.Marshal(cfg)
	if err != nil {
		log.Fatalf("Failed to marshal config: %s", err)
	}
	log.Println(string(jBytes))
	// Load environment variables when not running in Docker
	if !runningInDocker() {
		if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
			log.Println("Running in Kubernetes")
		} else {
			log.Println("Could not determine runtime environment")
		}
	} else {
		log.Println("Running in Docker runtime")
	}
	if cfg.NewRelicEnabled {
		log.Println("Initializing New Relic APM - remove/setFalse environment variable 'NEW_RELIC_ENABED' to disable")
		app, err := initNewRelicAPM()
		if err != nil {
			log.Fatalf("Failed to initialize New Relic APM: %s", err)
		}
		defer app.Shutdown(0)
	}

	setSwaggerInfo()

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
		log.Fatalln(err)
	}

	// Roid
	idGenerator, err := snowflakeidgenerator.NewIDGenerator()
	if err != nil {
		panic(err)
	}
	roidService := services.NewRoidService(idGenerator)
	// TODO: Register the Node ID in Redis or something. Then we can add a check to avoid the unlikely scenario of a duplicate Node ID.
	log.Printf("Snowflake Node ID: %d", roidService.ListNode())

	// SET UP SERVICES

	// Events
	var eventSvc *services.EventService
	if cfg.EventStreamEnabled {
		log.Println("Setting up Event Stream")
		portStr := os.Getenv("RMQ_PORT")
		port, err := strconv.Atoi(portStr)
		if err != nil {
			log.Fatalf("Failed to convert RMQ_PORT to int: %s", err)
		}

		eventRepo, err := rabbitmq.NewEventRepository(&rabbitmq.RabbitConfig{
			Host:     os.Getenv("RMQ_HOST"),
			Port:     port,
			Username: os.Getenv("RMQ_USER"),
			Password: os.Getenv("RMQ_PASS"),
			Topic:    os.Getenv("EVENT_STREAM_TOPIC"),
		})
		if err != nil {
			log.Fatalf("Failed to create Event Repository: %s", err)
		}
		eventSvc = services.NewEventService(eventRepo)
		err = eventSvc.SendStream(&entities.Event{
			Source: AppName,
			User:   "system",
			Action: "startup",
			Details: entities.EventDetails{
				Result: entities.EventResultSuccess,
			},
			Timestamp: time.Now().UTC(),
		})
		if err != nil {
			log.Fatalf("Failed to send event: %s", err)
		}
	}

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
	ianaRepo := iana.NewIANARRepository()
	icannRepo := icann.NewICANNRepo()
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
	hostService := services.NewHostService(hostRepo, hostAddressRepo, *roidService)
	// Domains
	domainRepo := postgres.NewDomainRepository(gormDB)
	domainService := services.NewDomainService(domainRepo, hostRepo, *roidService, nndnRepo, tldRepo, phaseRepo, premiumLabelRepo, fxRepo)
	// Quotes
	quoteService := services.NewQuoteService(tldRepo, domainRepo, premiumLabelRepo, fxRepo)

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

	rest.NewPingController(r)
	rest.NewRegistryOperatorController(r, registryOperatorService)
	rest.NewTLDController(r, tldService, domainService)
	rest.NewNNDNController(r, nndnService)
	rest.NewSyncController(r, syncService)
	rest.NewSpec5Controller(r, spec5Service)
	rest.NewIANARegistrarController(r, ianaRegistrarService)
	rest.NewRegistrarController(r, registrarService, ianaRegistrarService)
	rest.NewContactController(r, contactService)
	rest.NewHostController(r, hostService)
	rest.NewDomainController(r, domainService)
	rest.NewPhaseController(r, phaseService)
	rest.NewFeeController(r, feeService)
	rest.NewPriceController(r, priceService)
	rest.NewAccreditationController(r, accreditationService)
	rest.NewPremiumController(r, premiumListService, premiumLabelService)
	rest.NewFXController(r, fxService)
	rest.NewQuoteController(r, quoteService)

	// Serve the swagger documentation
	r.GET("/swagger/*any", ginSwagger.WrapHandler(
		swaggerFiles.Handler,
		ginSwagger.DocExpansion("none"))) // collapse all endpoints by default

	if inLambda() {
		log.Println("Running in AWS Lambda")
		// Start the server using the AWS Lambda proxy
		log.Fatal(gateway.ListenAndServe(os.Getenv("API_PORT"), r))
	} else {
		// Start the server using the standard HTTP server
		r.Run(":" + os.Getenv("API_PORT"))
	}

}
