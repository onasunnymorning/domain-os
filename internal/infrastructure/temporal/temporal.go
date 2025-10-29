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
	WorkerQueue string
}

func GetTemporalClient(cfg TemporalClientconfig) (client.Client, error) {
	// If no cert/key provided, connect without TLS (useful for dev/temporalite)
	if strings.TrimSpace(cfg.ClientCert) == "" || strings.TrimSpace(cfg.ClientKey) == "" {
		return client.Dial(client.Options{
			HostPort:  cfg.HostPort,
			Namespace: cfg.Namespace,
		})
	}

	// Otherwise, use TLS with provided certs
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
