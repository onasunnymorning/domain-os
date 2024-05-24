package main

import (
	"log"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/snowflakeidgenerator"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/web/iana"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/web/icann"
	"github.com/onasunnymorning/domain-os/internal/interface/rest"

	"os"

	"github.com/apex/gateway"
	"github.com/gin-gonic/gin"

	docs "github.com/onasunnymorning/domain-os/docs" // Import docs pkg to be able to access docs.json https://github.com/swaggo/swag/issues/830#issuecomment-725587162
	swaggerFiles "github.com/swaggo/files"           // swagger embed files
	ginSwagger "github.com/swaggo/gin-swagger"       // gin-swagger middleware

	// NeW Relic APM
	nrgin "github.com/newrelic/go-agent/v3/integrations/nrgin"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// inLambda returns true if the code is running in AWS Lambda
func inLambda() bool {
	if lambdaTaskRoot := os.Getenv("LAMBDA_TASK_ROOT"); lambdaTaskRoot != "" {
		return true
	}
	return false
}

// setSwaggerInfo sets the swagger info dynamically based on the environment variables
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
		newrelic.ConfigAppName("domain-os"),
		newrelic.ConfigLicense("07597dba536368f708cf36d68937d4bfFFFFNRAL"),
		newrelic.ConfigAppLogForwardingEnabled(true),
	)

}

// @title APEX Domain OS ADMIN API
// @license.name APEX all rights reserved
func main() {
	// Load environment variables when not running in Docker
	if !runningInDocker() {
		log.Println("Running outside of Docker")
	} else {
		log.Println("Running in Docker")
	}

	setSwaggerInfo()

	gormDB, err := postgres.NewConnection(
		postgres.Config{
			User:   os.Getenv("DB_USER"),
			Pass:   os.Getenv("DB_PASS"),
			Host:   os.Getenv("DB_HOST"),
			Port:   os.Getenv("DB_PORT"),
			DBName: os.Getenv("DB_NAME"),
		},
	)
	if err != nil {
		log.Println(err)
	}

	// Roid
	idGenerator, err := snowflakeidgenerator.NewIDGenerator()
	if err != nil {
		panic(err)
	}
	roidService := services.NewRoidService(idGenerator)
	// TODO: Register the Node ID in Redis or something. Then we can add a check to avoid the unlikely scenario of a duplicate Node ID.
	log.Printf("Snowflake Node ID: %d", roidService.ListNode())

	// Registry Operators
	registryOperatorRepo := postgres.NewGORMRegistryOperatorRepository(gormDB)
	registryOperatorService := services.NewRegistryOperatorService(registryOperatorRepo)

	// TLDs
	tldRepo := postgres.NewGormTLDRepo(gormDB)
	tldService := services.NewTLDService(tldRepo)

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

	// Gin router
	r := gin.Default()

	// Add New Relic APM middleware if we are in DEV environment
	log.Println("ENV: ", os.Getenv("ENV"))
	if os.Getenv("ENV") == "DEV" {
		log.Println("Running in DEV environment, initializing New Relic APM")
		newRelicApp, err := initNewRelicAPM()
		if err != nil {
			log.Fatal("Error initializing New Relic APM")
		}
		r.Use(nrgin.Middleware(newRelicApp))
	}

	rest.NewPingController(r)
	rest.NewRegistryOperatorController(r, registryOperatorService)
	rest.NewTLDController(r, tldService)
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
		log.Fatal(gateway.ListenAndServe(":8080", r))
	} else {
		r.Run(":" + os.Getenv("API_PORT"))
	}

}
