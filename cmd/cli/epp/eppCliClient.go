package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"log"
	"math/big"
	"net"
	"os"
	"time"

	"github.com/dotse/epp-client/pkg"
	"github.com/dotse/epp-client/pkg/prompt"
	"github.com/dotse/epp-client/pkg/validation"

	"github.com/alecthomas/kingpin"
)

func main() {
	var (
		port = kingpin.
			Flag("port", "the port to send requests to").
			Short('p').
			Default("700").
			String()

		host = kingpin.
			Flag("host", "the host to send requests to").
			Short('h').
			Default("epp-ote.centralnic.com").
			String()

		// cert = kingpin.
		// 	Flag("cert", "path to the cert to use for tls").
		// 	Short('c').
		// 	Default("some-cert-path.cert").
		// 	String()

		// key = kingpin.
		// 	Flag("key", "path to the key to use for tls").
		// 	Short('k').
		// 	Default("some-key-path.key").
		// 	String()

		keepAlive = kingpin.
				Flag("keep-alive", "keep connection to the epp server alive").
				Short('a').
				Default("true").
				Bool()

		validateResponses = kingpin.
					Flag("validate-responses", "validate responses from epp server").
					Short('v').
					Default("false").
					Bool()
	)

	kingpin.Parse()

	logger := log.New(os.Stdout, "", 0)
	ctx := context.Background()

	cl, err := connect(ctx, *host, *port)
	if err != nil {
		logger.Fatalf("Failed to connect to EPP server: %v", err)
	}
	logger.Println(cl.Greeting)

	if *keepAlive {
		cl.KeepAlive(ctx)
	}

	scnr := bufio.NewScanner(os.Stdin)
	scnr.Split(func(data []byte, _ bool) (int, []byte, error) {
		if i := bytes.Index(data, []byte{'\n', '\n'}); i >= 0 {
			return i + 2, data[0:i], nil
		}

		return 0, nil, nil
	})

	p := prompt.Prompt{
		Client:           cl,
		MultilineScanner: scnr,
		XMLValidator: &validation.XMLValidator{
			XSDIndexFile: "pkg/validation/xsd/index.xsd",
		},
		Cli: logger,
	}

	p.Run(ctx, *validateResponses)
}

func connect(ctx context.Context, host, port string) (*pkg.Client, error) {
	log.Println("Connecting to EPP server...")
	cl, err := pkg.Connect(net.JoinHostPort(host, port), &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // should only be used for testing..
		Certificates:       []tls.Certificate{generateCertificate()},
	})
	if err != nil {
		log.Printf("Failed to connect to EPP server: %v", err)
		return nil, err
	}

	log.Println("Connected to EPP server successfully")

	cl.KeepAlive(ctx)

	log.Println("Keepalive activated")

	// Create a login data object
	loginData := &pkg.LoginData{
		Username: "H1056502248-OTE",
		Password: "m8u5:}PKy[C1}dBJ",
		Namespaces: []string{
			"urn:ietf:params:xml:ns:host-1.0",
			"urn:ietf:params:xml:ns:contact-1.0",
			"urn:ietf:params:xml:ns:domain-1.0",
		},
		ExtensionNamespaces: []string{},
		Version:             "1.0",
		Lang:                "en",
		ClTrID:              "ABC-12345",
	}

	// Send the login command
	_, err = cl.SendCommandUsingTemplate("login.xml", loginData)
	if err != nil {
		return nil, err
	}

	return cl, nil
}

// generateCertificate generates a self-signed certificate in case client side certificates are not provided.
func generateCertificate() tls.Certificate {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)

	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1653),
		Subject: pkix.Name{
			CommonName:   "epp.example.test",
			Organization: []string{"Simple Server Test"},
			Country:      []string{"SE"},
			Locality:     []string{"Stockholm"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 0, 1),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	certificate, _ := x509.CreateCertificate(rand.Reader, cert, cert, key.Public(), key)

	return tls.Certificate{
		Certificate: [][]byte{certificate},
		PrivateKey:  key,
	}
}
