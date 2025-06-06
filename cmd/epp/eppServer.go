package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"time"

	"github.com/beevik/etree"
	epp "github.com/dotse/epp-lib"
	"github.com/sirupsen/logrus"
)

// LogrusLogger for the logger interface using logrus.
type LogrusLogger struct {
	logger *logrus.Logger
}

// NewLogrusLogger creates a new instance of LogrusLogger.
func NewLogrusLogger() *LogrusLogger {
	logger := logrus.New()
	// Set logger configuration here if needed
	return &LogrusLogger{logger: logger}
}

// Errorf logs an error message.
func (l *LogrusLogger) Errorf(format string, args ...interface{}) {
	l.logger.Errorf(format, args...)
}

// Infof logs an info message.
func (l *LogrusLogger) Infof(format string, args ...interface{}) {
	l.logger.Infof(format, args...)
}

// Debugf logs a debug message.
func (l *LogrusLogger) Debugf(format string, args ...interface{}) {
	l.logger.Debugf(format, args...)
}

func main() {
	logger := NewLogrusLogger()

	commandMux := &epp.CommandMux{}

	commandMux.BindGreeting(sendGreeting)
	commandMux.Bind(epp.NewXMLPathBuilder().AddOrphan("//hello", epp.NamespaceIETFEPP10.String()).String(), sendGreeting)
	commandMux.BindCommand("check", epp.NamespaceIETFDomain10.String(), respondToDomainCheckCommand)
	// commandMux.BindCommand("info", epp.NamespaceIETFContact10.String(),
	// 	funcTharHandlesContactInfoCommand,
	// )

	server := &epp.Server{
		HandleCommand: commandMux.Handle,
		Greeting:      commandMux.GetGreeting,
		TLSConfig: tls.Config{
			Certificates: []tls.Certificate{generateCertificate()},
			ClientAuth:   tls.RequireAnyClientCert,
			MinVersion:   tls.VersionTLS12,
		},
		ConnContext:    logConnection,
		Timeout:        time.Hour,
		IdleTimeout:    350 * time.Second,
		WriteTimeout:   2 * time.Minute,
		ReadTimeout:    10 * time.Second,
		Logger:         logger,
		MaxMessageSize: 1000,
	}

	listener, err := net.ListenTCP("tcp", &net.TCPAddr{
		Port: 700,
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("Listening on port 700")

	if err := server.Serve(listener); err != nil {
		panic(err)
	}
}

// generateCertificate generates a self-signed certificate in case client side certificates are not provided.
func generateCertificate() tls.Certificate {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)

	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1653),
		Subject: pkix.Name{
			CommonName:   "epp.example.test",
			Organization: []string{"Simple Server Test"},
			Country:      []string{"AR"},
			Locality:     []string{"Buenos Aires"},
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

// sendGreeting adheres to the CommandFunc signature and sends a greeting.
func sendGreeting(ctx context.Context, rw epp.Writer, _ *etree.Document) {
	rw.Write([]byte(getGreetingXML()))
}

// getGreetingXML returns the XML for a greeting.
func getGreetingXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="no"?><epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><response><result code="1000"><msg>Welcome Stranger</msg></result><trID><clTRID>ABC-12345</clTRID><svTRID>APEX-123</svTRID></trID></response></epp>`
}

// logConnection implements the
// ConnContext func(ctx context.Context, conn *tls.Conn) (context.Context, error)
// interface and is a placeholder for connection management.
// We simply log to the console that a connection has been established.
func logConnection(ctx context.Context, conn *tls.Conn) (context.Context, error) {
	// add the connection ID to the context
	ctx = context.WithValue(ctx, "cid", "12345")
	fmt.Printf("Connection with id %s established\n", ctx.Value("cid"))
	return ctx, nil
}

// respondToDomainCheckCommand is a placeholder function that responds to a domain check command.
func respondToDomainCheckCommand(ctx context.Context, rw epp.Writer, doc *etree.Document) {
	// SETUP
	// Domain service
	// gormDB, err := postgres.NewConnection(
	// 	postgres.Config{
	// 		User:    os.Getenv("DB_USER"),
	// 		Pass:    os.Getenv("DB_PASS"),
	// 		Host:    "localhost",
	// 		Port:    os.Getenv("DB_PORT"),
	// 		DBName:  os.Getenv("DB_NAME"),
	// 		SSLmode: "require",
	// 	},
	// )
	// if err != nil {
	// 	log.Fatalln(err)
	// }
	// idGenerator, err := snowflakeidgenerator.NewIDGenerator()
	// if err != nil {
	// 	panic(err)
	// }
	// roidService := services.NewRoidService(idGenerator)
	// hostRepo := postgres.NewGormHostRepository(gormDB)
	// nndnRepo := postgres.NewGormNNDNRepository(gormDB)
	// tldRepo := postgres.NewGormTLDRepo(gormDB)
	// phaseRepo := postgres.NewGormPhaseRepository(gormDB)
	// premiumLabelRepo := postgres.NewGORMPremiumLabelRepository(gormDB)
	// fxRepo := postgres.NewFXRepository(gormDB)
	// domainRepo := postgres.NewDomainRepository(gormDB)
	// ds := services.NewDomainService(domainRepo, hostRepo, *roidService, nndnRepo, tldRepo, phaseRepo, premiumLabelRepo, fxRepo)
	// // Get a list of domain names to check
	// domainNames := doc.FindElements("//domain:name")
	// results := make([]*queries.DomainCheckResult, len(domainNames))
	// for i, domainName := range domainNames {
	// 	fmt.Printf("Checking domain: %s\n", domainName.Text())
	// 	result, err := ds.CheckDomain(ctx, &queries.DomainCheckQuery{
	// 		DomainName: entities.DomainName(domainName.Text()),
	// 	})
	// 	if err != nil {
	// 		log.Fatalf("Error checking domain: %s\n", err)
	// 	}
	// 	results[i] = result
	// 	fmt.Println(results)
	// }
	rw.Write([]byte(dummyDomainCheckResponse()))
}

// dummyDomainCheckResponse returns a dummy domain check response.
func dummyDomainCheckResponse() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="no"?><epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><response><result code="1000"><msg>Welcome Stranger</msg></result><resData><domain:chkData xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"><domain:cd><domain:name avail="1">geoff.smoketestcnic</domain:name></domain:cd></domain:chkData></resData> <trID><clTRID>ABC-12345</clTRID><svTRID>APEX-123</svTRID></trID></response></epp>`
}
