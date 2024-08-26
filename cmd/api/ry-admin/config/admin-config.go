package config

import (
	"os"
)

// AdminApiConfig contains the configuration for the admin api
type AdminApiConfig struct {
	NewRelicEnabled    bool
	AutoMigrate        bool
	EventStreamEnabled bool
	EventStreamTopic   string
	GinMode            string
}

func LoadConfig() *AdminApiConfig {
	return &AdminApiConfig{
		NewRelicEnabled:    os.Getenv("NEW_RELIC_ENABLED") == "true",
		AutoMigrate:        os.Getenv("AUTO_MIGRATE") == "true",
		EventStreamEnabled: os.Getenv("EVENT_STREAM_ENABLED") == "true",
		EventStreamTopic:   os.Getenv("EVENT_STREAM_TOPIC"),
		GinMode:            os.Getenv("GIN_MODE"),
	}
}
