package config

import (
	"os"
)

// AdminApiConfig contains the configuration for the admin api
type AdminApiConfig struct {
	Version           string
	GitSHA            string
	NewRelicEnabled   bool
	AutoMigrate       bool
	LogEvents         bool
	GinMode           string
	PrometheusEnabled bool
	ApiName           string
	ApiHost           string
	ApiPort           string
	Auth0Domain       string
	Auth0Audience     string
	Auth0Enabled      bool
	// Redis holds the EPP session registry, which the admin API lists and
	// ends sessions in.
	RedisHost     string
	RedisPort     string
	RedisPassword string
}

func LoadConfig(GitSHA string) *AdminApiConfig {
	return &AdminApiConfig{
		GitSHA:            GitSHA,
		NewRelicEnabled:   os.Getenv("NEW_RELIC_ENABLED") == "true",
		AutoMigrate:       os.Getenv("AUTO_MIGRATE") == "true",
		LogEvents:         os.Getenv("LOG_DOMAIN_EVENTS") != "false",
		GinMode:           os.Getenv("GIN_MODE"),
		PrometheusEnabled: os.Getenv("PROMETHEUS_ENABLED") == "true",
		ApiName:           os.Getenv("API_NAME"),
		Version:           os.Getenv("API_VERSION"),
		ApiHost:           os.Getenv("API_HOST"),
		ApiPort:           os.Getenv("API_PORT"),
		Auth0Domain:       os.Getenv("AUTH0_DOMAIN"),
		Auth0Audience:     os.Getenv("AUTH0_AUDIENCE"),
		Auth0Enabled:      os.Getenv("AUTH0_ENABLED") == "true",
		RedisHost:         valueOr(os.Getenv("REDIS_HOST"), "localhost"),
		RedisPort:         valueOr(os.Getenv("REDIS_PORT"), "6379"),
		RedisPassword:     os.Getenv("REDIS_PASSWORD"),
	}
}

func valueOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
