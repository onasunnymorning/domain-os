package config

import (
	"os"
)

// AdminApiConfig contains the configuration for the admin api
type AdminApiConfig struct {
	Version            string
	GitSHA             string
	NewRelicEnabled    bool
	AutoMigrate        bool
	EventStreamEnabled bool
	EventStreamTopic   string
	GinMode            string
	PrometheusEnabled  bool
	ApiName            string
	ApiHost            string
	ApiPort            string
}

func LoadConfig(GitSHA string) *AdminApiConfig {
	return &AdminApiConfig{
		GitSHA:             GitSHA,
		NewRelicEnabled:    os.Getenv("NEW_RELIC_ENABLED") == "true",
		AutoMigrate:        os.Getenv("AUTO_MIGRATE") == "true",
		EventStreamEnabled: os.Getenv("EVENT_STREAM_ENABLED") == "true",
		EventStreamTopic:   os.Getenv("EVENT_STREAM_TOPIC"),
		GinMode:            os.Getenv("GIN_MODE"),
		PrometheusEnabled:  os.Getenv("PROMETHEUS_ENABLED") == "true",
		ApiName:            os.Getenv("API_NAME"),
		Version:            os.Getenv("API_VERSION"),
		ApiHost:            os.Getenv("API_HOST"),
		ApiPort:            os.Getenv("API_PORT"),
	}
}
