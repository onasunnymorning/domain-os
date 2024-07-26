package config

import (
	"os"
)

// AdminApiConfig contains the configuration for the admin api
type AdminApiConfig struct {
	UseNewRelic bool
	AutoMigrate bool
}

func LoadConfig() *AdminApiConfig {
	autoMigrate := os.Getenv("AUTO_MIGRATE") == "true"
	useNewRelic := os.Getenv("USE_NEW_RELIC") == "true"

	return &AdminApiConfig{
		UseNewRelic: useNewRelic,
		AutoMigrate: autoMigrate,
	}
}
