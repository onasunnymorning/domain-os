package main

import (
	"log"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
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

	gormDB, err := postgres.NewConnection()
	if err != nil {
		log.Println(err)
	}

	tldRepo := postgres.NewGormTLDRepo(gormDB)
	tldService := services.NewTLDService(tldRepo)

	nndnRepo := postgres.NewGormNNDNRepository(gormDB)
	nndnService := services.NewNNDNService(nndnRepo)

	r := gin.Default()

	rest.NewTLDController(r, tldService)
	rest.NewNNDNController(r, nndnService)

	r.Run(":" + os.Getenv("API_PORT"))

}
