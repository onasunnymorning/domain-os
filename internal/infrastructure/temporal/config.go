package temporal

import "os"

// NewClientConfigFromEnv reads Temporal client configuration from
// standard environment variables. Queue must be specified explicitly
// to avoid accidental cross-queue submissions.
func NewClientConfigFromEnv(queue string) TemporalClientconfig {
	return TemporalClientconfig{
		HostPort:    os.Getenv("TEMPORAL_HOST_PORT"),
		Namespace:   os.Getenv("TEMPORAL_NAMESPACE"),
		ClientKey:   os.Getenv("TEMPORAL_CLIENT_KEY"),
		ClientCert:  os.Getenv("TEMPORAL_CLIENT_CERT"),
		APIKey:      os.Getenv("TEMPORAL_API_KEY"),
		WorkerQueue: queue,
	}
}
