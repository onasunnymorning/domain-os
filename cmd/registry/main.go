package main

import (
	"log"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/web/iana"
	"github.com/onasunnymorning/domain-os/internal/interface/rest"

	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

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

	r := gin.Default()

	rest.NewTLDController(r, tldService)
	rest.NewNNDNController(r, nndnService)

	r.Run(":" + os.Getenv("API_PORT"))

}
