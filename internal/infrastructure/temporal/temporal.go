package temporal

import (
	"crypto/tls"
	"strings"

	"go.temporal.io/sdk/client"
)

type TemporalClientconfig struct {
	HostPort    string
	Namespace   string
	ClientKey   string
	ClientCert  string
	APIKey      string
	WorkerQueue string
}

func GetTemporalClient(cfg TemporalClientconfig) (client.Client, error) {
	// 1. API Key auth (Temporal Cloud recommended)
	if strings.TrimSpace(cfg.APIKey) != "" {
		return client.Dial(client.Options{
			HostPort:    cfg.HostPort,
			Namespace:   cfg.Namespace,
			Credentials: client.NewAPIKeyStaticCredentials(cfg.APIKey),
			ConnectionOptions: client.ConnectionOptions{
				TLS: &tls.Config{},
			},
		})
	}

	// 2. mTLS (legacy Temporal Cloud)
	if strings.TrimSpace(cfg.ClientCert) != "" && strings.TrimSpace(cfg.ClientKey) != "" {
		cert, err := tls.X509KeyPair([]byte(strings.ReplaceAll(cfg.ClientCert, `\n`, "\n")), []byte(strings.ReplaceAll(cfg.ClientKey, `\n`, "\n")))
		if err != nil {
			return nil, err
		}

		return client.Dial(client.Options{
			HostPort:  cfg.HostPort,
			Namespace: cfg.Namespace,
			ConnectionOptions: client.ConnectionOptions{
				TLS: &tls.Config{
					Certificates: []tls.Certificate{cert},
				},
			},
		})
	}

	// 3. No auth (local dev / temporalite)
	return client.Dial(client.Options{
		HostPort:  cfg.HostPort,
		Namespace: cfg.Namespace,
	})
}
