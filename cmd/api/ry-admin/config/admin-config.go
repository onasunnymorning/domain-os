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
	return &AdminApiConfig{
		UseNewRelic: os.Getenv("USE_NEW_RELIC") == "true",
		AutoMigrate: os.Getenv("AUTO_MIGRATE") == "true",
	}
}
