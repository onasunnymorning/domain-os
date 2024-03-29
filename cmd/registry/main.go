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
	"github.com/joho/godotenv"

	docs "github.com/onasunnymorning/domain-os/docs" // Import docs pkg to be able to access docs.json https://github.com/swaggo/swag/issues/830#issuecomment-725587162
	swaggerFiles "github.com/swaggo/files"           // swagger embed files
	ginSwagger "github.com/swaggo/gin-swagger"       // gin-swagger middleware
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

// @title APEX Domain OS ADMIN API
// @license.name APEX all rights reserved
func main() {
	// Load environment variables when not running in Docker
	if !runningInDocker() {
		log.Println("Running outside of Docker")
		err := godotenv.Load()
		if err != nil {
			log.Println("Error loading .env file")
		}
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

	tldRepo := postgres.NewGormTLDRepo(gormDB)
	tldService := services.NewTLDService(tldRepo)

	nndnRepo := postgres.NewGormNNDNRepository(gormDB)
	nndnService := services.NewNNDNService(nndnRepo)

	// Sync
	ianaRepo := iana.NewIANARRepository()
	icannRepo := icann.NewICANNRepo()
	spec5Repo := postgres.NewSpec5Repository(gormDB)
	iregistrarRepo := postgres.NewIANARegistrarRepository(gormDB)
	syncService := services.NewSyncService(iregistrarRepo, spec5Repo, icannRepo, ianaRepo)

	// Spec5
	spec5Service := services.NewSpec5Service(spec5Repo)

	// IANA Registrar
	ianaRegistrarService := services.NewIANARegistrarService(iregistrarRepo)

	// Registrars
	registrarRepo := postgres.NewGormRegistrarRepository(gormDB)
	registrarService := services.NewRegistrarService(registrarRepo)

	// Contacts
	contactRepo := postgres.NewContactRepository(gormDB)
	contactService := services.NewContactService(contactRepo, *roidService)

	// Hosts
	hostRepo := postgres.NewGormHostRepository(gormDB)
	hostAddressRepo := postgres.NewGormHostAddressRepository(gormDB)
	hostService := services.NewHostService(hostRepo, hostAddressRepo, *roidService)

	// Domains
	domainRepo := postgres.NewDomainRepository(gormDB)
	domainService := services.NewDomainService(domainRepo, *roidService)

	// Gin router
	r := gin.Default()

	rest.NewPingController(r)
	rest.NewTLDController(r, tldService)
	rest.NewNNDNController(r, nndnService)
	rest.NewSyncController(r, syncService)
	rest.NewSpec5Controller(r, spec5Service)
	rest.NewIANARegistrarController(r, ianaRegistrarService)
	rest.NewRegistrarController(r, registrarService, ianaRegistrarService)
	rest.NewContactController(r, contactService)
	rest.NewHostController(r, hostService)
	rest.NewDomainController(r, domainService)

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
