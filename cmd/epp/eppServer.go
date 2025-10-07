package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"os"
	"time"

	"github.com/beevik/etree"
	epp "gitlab.com/internetstiftelsen-oss/epp-lib"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const connectionIDKey contextKey = "cid"

func main() {
	// Create a structured logger using slog
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	commandMux := &epp.CommandMux{}

	commandMux.BindGreeting(sendGreeting)
	commandMux.Bind(epp.NewXMLPathBuilder().AddOrphan("//hello", epp.NamespaceIETFEPP10.String()).String(), sendGreeting)
	// Login is a direct child of command, not in a namespace-specific element
	commandMux.Bind(epp.NewXMLPathBuilder().
		AddOrphan("//command", epp.NamespaceIETFEPP10.String()).
		Add("login", epp.NamespaceIETFEPP10.String()).String(), respondToLoginCommand)
	// Logout is a direct child of command, not in a namespace-specific element
	commandMux.Bind(epp.NewXMLPathBuilder().
		AddOrphan("//command", epp.NamespaceIETFEPP10.String()).
		Add("logout", epp.NamespaceIETFEPP10.String()).String(), respondToLogoutCommand)
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
		MaxMessageSize: 1000, // uint32 type in new library
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
	return `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <greeting>
    <svID>EPP Server</svID>
    <svDate>` + time.Now().UTC().Format(time.RFC3339) + `</svDate>
    <svcMenu>
      <version>1.0</version>
      <lang>en</lang>
      <objURI>urn:ietf:params:xml:ns:domain-1.0</objURI>
      <objURI>urn:ietf:params:xml:ns:contact-1.0</objURI>
      <objURI>urn:ietf:params:xml:ns:host-1.0</objURI>
    </svcMenu>
  </greeting>
</epp>`
}

// logConnection implements the
// ConnContext func(ctx context.Context, conn *tls.Conn) (context.Context, error)
// interface and is a placeholder for connection management.
// We simply log to the console that a connection has been established.
func logConnection(ctx context.Context, conn *tls.Conn) (context.Context, error) {
	// add the connection ID to the context
	ctx = context.WithValue(ctx, connectionIDKey, "12345")
	fmt.Printf("Connection with id %s established\n", ctx.Value(connectionIDKey))
	return ctx, nil
}

// respondToLoginCommand handles EPP login commands.
func respondToLoginCommand(ctx context.Context, rw epp.Writer, doc *etree.Document) {
	fmt.Println("Login command received")

	// Extract login information from the command (optional - for logging/validation)
	clID := doc.FindElement("//clID")
	if clID != nil {
		fmt.Printf("Login attempt from client: %s\n", clID.Text())
	}

	// For now, accept any login and return success
	// In a real implementation, you would validate credentials here
	rw.Write([]byte(getLoginResponseXML()))
}

// getLoginResponseXML returns a successful login response.
func getLoginResponseXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1000">
      <msg>Command completed successfully</msg>
    </result>
    <trID>
      <clTRID>ABC-12345</clTRID>
      <svTRID>APEX-LOGIN-123</svTRID>
    </trID>
  </response>
</epp>`
}

// respondToLogoutCommand handles EPP logout commands.
func respondToLogoutCommand(ctx context.Context, rw epp.Writer, doc *etree.Document) {
	fmt.Println("Client logout")
	rw.Write([]byte(getLogoutResponseXML()))
	// Close the connection after writing the response
	if respWriter, ok := rw.(*epp.ResponseWriter); ok {
		respWriter.CloseAfterWrite()
	}
}

// getLogoutResponseXML returns a successful logout response.
func getLogoutResponseXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1500">
      <msg>Command completed successfully; ending session</msg>
    </result>
    <trID>
      <clTRID>ABC-12345</clTRID>
      <svTRID>APEX-LOGOUT-123</svTRID>
    </trID>
  </response>
</epp>`
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
