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

	// Create a tls.Certificate from the client cert and key
	cert, err := tls.X509KeyPair([]byte(strings.ReplaceAll(cfg.ClientCert, `\n`, "\n")), []byte(strings.ReplaceAll(cfg.ClientKey, `\n`, "\n")))
	if err != nil {
		return nil, err
	}

	// Create the Temporal client object
	c, err := client.Dial(client.Options{
		HostPort:  cfg.HostPort,
		Namespace: cfg.Namespace,
		ConnectionOptions: client.ConnectionOptions{
			TLS: &tls.Config{
				Certificates: []tls.Certificate{cert},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	return c, nil
}
