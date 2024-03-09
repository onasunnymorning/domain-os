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

	_ "github.com/onasunnymorning/domain-os/docs" // Import docs pkg to be able to access docs.json https://github.com/swaggo/swag/issues/830#issuecomment-725587162
	swaggerFiles "github.com/swaggo/files"        // swagger embed files
	ginSwagger "github.com/swaggo/gin-swagger"    // gin-swagger middleware
)

// inLambda returns true if the code is running in AWS Lambda
func inLambda() bool {
	if lambdaTaskRoot := os.Getenv("LAMBDA_TASK_ROOT"); lambdaTaskRoot != "" {
		return true
	}
	return false
}

// @title APEX RegistryOS
// @version 0.5.1
// @license.name APEX all rights reserved
func main() {
	godotenv.Load()

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

	r := gin.Default()

	rest.NewPingController(r)
	rest.NewTLDController(r, tldService)
	rest.NewNNDNController(r, nndnService)
	rest.NewSyncController(r, syncService)
	rest.NewSpec5Controller(r, spec5Service)
	rest.NewIANARegistrarController(r, ianaRegistrarService)
	rest.NewRegistrarController(r, registrarService, ianaRegistrarService)
	rest.NewContactController(r, contactService)

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
