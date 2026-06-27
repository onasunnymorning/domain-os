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
	"strconv"
	"time"

	"github.com/beevik/etree"
	"github.com/redis/go-redis/v9"
	epp "gitlab.com/internetstiftelsen-oss/epp-lib"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/epp/middleware"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	connectionIDKey contextKey = "cid"
	clientIPKey     contextKey = "clientIP"
	registrarIDKey  contextKey = "registrarID"
)

// Global rate limiter (will be initialized in main)
var rateLimiter *middleware.RateLimiter

func main() {
	// Get log level from environment (default: INFO for production)
	logLevel := slog.LevelInfo
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		switch level {
		case "debug", "DEBUG":
			logLevel = slog.LevelDebug
		case "info", "INFO":
			logLevel = slog.LevelInfo
		case "warn", "WARN", "warning", "WARNING":
			logLevel = slog.LevelWarn
		case "error", "ERROR":
			logLevel = slog.LevelError
		}
	}

	// Create a structured logger using slog
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	// Initialize Redis client
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}
	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", redisHost, redisPort),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	// Test Redis connection
	ctx := context.Background()
	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		logger.Error("Failed to connect to Redis", "error", err)
		panic(err)
	}
	logger.Info("Connected to Redis", "addr", fmt.Sprintf("%s:%s", redisHost, redisPort))

	// Initialize rate limiter with custom config
	rateLimitConfig := &middleware.RateLimitConfig{
		MaxConnPerIP:        10,
		MaxConnPerRegistrar: 100,
		ConnTTL:             5 * time.Minute,
		RequestsPerSecond:   100,
		BurstSize:           200,
		RequestWindow:       time.Second,
		MaxFailedLogins:     5,
		LockoutDuration:     15 * time.Minute,
	}
	rateLimiter = middleware.NewRateLimiter(redisClient, rateLimitConfig, logger)

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

	eppPort := 700
	if p := os.Getenv("EPP_PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			eppPort = parsed
		}
	}
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{
		Port: eppPort,
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Listening on port %d\n", eppPort)

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
// interface and handles connection rate limiting and tracking.
func logConnection(ctx context.Context, conn *tls.Conn) (context.Context, error) {
	// Extract client IP
	clientIP := conn.RemoteAddr().String()
	if tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		clientIP = tcpAddr.IP.String()
	}

	// Check connection limit before accepting (only if rateLimiter is initialized)
	if rateLimiter != nil {
		err := rateLimiter.CheckConnectionLimit(ctx, clientIP, "")
		if err != nil {
			fmt.Printf("Connection from %s rejected: %v\n", clientIP, err)
			return nil, err
		}
	}

	// Generate connection ID
	connectionID := fmt.Sprintf("conn-%d", time.Now().UnixNano())

	// Add connection info to context
	ctx = context.WithValue(ctx, connectionIDKey, connectionID)
	ctx = context.WithValue(ctx, clientIPKey, clientIP)

	// Increment connection counter (only if rateLimiter is initialized)
	if rateLimiter != nil {
		err := rateLimiter.IncrementConnection(ctx, clientIP, "")
		if err != nil {
			fmt.Printf("Failed to track connection: %v\n", err)
		}
	}

	fmt.Printf("Connection %s from %s established\n", connectionID, clientIP)

	// Set up cleanup when connection closes
	go func() {
		<-ctx.Done()
		// Get registrar ID from context if available
		registrarID, _ := ctx.Value(registrarIDKey).(string)

		// Decrement connection counter (only if rateLimiter is initialized)
		if rateLimiter != nil {
			cleanupCtx := context.Background()
			if err := rateLimiter.DecrementConnection(cleanupCtx, clientIP, registrarID); err != nil {
				fmt.Printf("Failed to cleanup connection: %v\n", err)
			}
		}
		fmt.Printf("Connection %s from %s closed\n", connectionID, clientIP)
	}()

	return ctx, nil
}

// respondToLoginCommand handles EPP login commands.
func respondToLoginCommand(ctx context.Context, rw epp.Writer, doc *etree.Document) {
	fmt.Println("Login command received")

	// Extract login information from the command
	clID := doc.FindElement("//clID")
	pw := doc.FindElement("//pw")

	var username string
	if clID != nil {
		username = clID.Text()
		fmt.Printf("Login attempt from client: %s\n", username)
	}

	// Get client IP from context
	clientIP, _ := ctx.Value(clientIPKey).(string)

	// Check if account is locked (only if rateLimiter is initialized)
	if rateLimiter != nil && username != "" {
		locked, err := rateLimiter.IsAccountLocked(ctx, username)
		if err != nil {
			fmt.Printf("Error checking account lock status: %v\n", err)
		}
		if locked {
			fmt.Printf("Login attempt for locked account: %s\n", username)
			// Send error response
			rw.Write([]byte(getAuthErrorResponseXML("account locked")))
			return
		}
	}

	// TODO: Implement actual authentication logic here
	// For now, we'll simulate successful login for demonstration
	authSuccess := true // This should be replaced with real authentication

	if username == "" || pw == nil {
		authSuccess = false
	}

	if !authSuccess {
		// Record failed login (only if rateLimiter is initialized)
		if rateLimiter != nil && username != "" && clientIP != "" {
			if err := rateLimiter.RecordFailedLogin(ctx, username, clientIP); err != nil {
				fmt.Printf("Failed login recorded, account may be locked: %v\n", err)
			}
		}

		// Send authentication error response
		rw.Write([]byte(getAuthErrorResponseXML("invalid credentials")))
		return
	}

	// Successful login - clear failed login attempts (only if rateLimiter is initialized)
	if rateLimiter != nil && username != "" && clientIP != "" {
		if err := rateLimiter.ClearFailedLogins(ctx, username, clientIP); err != nil {
			fmt.Printf("Error clearing failed logins: %v\n", err)
		}

		// Store registrar ID in context for connection tracking
		// In a real implementation, you'd extract this from your auth system
		ctx = context.WithValue(ctx, registrarIDKey, username)
	}

	// For now, accept any login and return success
	// In a real implementation, you would validate credentials here
	rw.Write([]byte(getLoginResponseXML()))
}

// getAuthErrorResponseXML returns an authentication error response.
func getAuthErrorResponseXML(reason string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="2200">
      <msg>Authentication error - ` + reason + `</msg>
    </result>
    <trID>
      <svTRID>` + fmt.Sprintf("srv-%d", time.Now().UnixNano()) + `</svTRID>
    </trID>
  </response>
</epp>`
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
