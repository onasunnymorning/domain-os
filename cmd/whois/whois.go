package main

import (
	"bufio"
	"context"
	"fmt"

	"github.com/onasunnymorning/domain-os/internal/buildinfo"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/config"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"

	"gorm.io/gorm"
)

const (
	// WHOIS_PORT is the default port for WHOIS servers.
	WHOIS_PORT = 43
)

func main() {
	log.Printf("whois-server version=%s sha=%s", buildinfo.Version, buildinfo.GitSHA)
	// Set up a context to handle signals for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	// Set up the database connection.
	db, err := setupDB()
	if err != nil {
		log.Printf("Error setting up database: %v", err)
		return
	}
	// Responses show the domain's ROID ("Registry Domain ID"), which ends in
	// the installation's repository ID (ADR 0017).
	repositoryID, err := config.LoadRepositoryID()
	if err == nil {
		err = postgres.CheckRepositoryID(ctx, db, repositoryID)
	}
	if err != nil {
		// Exit non-zero, so a supervisor sees a failed start, not a clean one.
		log.Printf("Refusing to start: %v", err)
		os.Exit(1) //nolint:gocritic // exitAfterDefer: nothing deferred so far needs to run; the signal context is released by exit
	}
	domRepo := postgres.NewDomainRepository(db, repositoryID)
	rarRepo := postgres.NewGormRegistrarRepository(db)

	// Set up the Whois Service
	WhoisSvc := services.NewWhoisService(domRepo, rarRepo)

	// Listen on port 43 for incoming WHOIS requests.
	listener, err := net.Listen("tcp", ":"+fmt.Sprint(WHOIS_PORT))
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
	defer listener.Close()

	fmt.Printf("WHOIS server running on port %d\n", WHOIS_PORT)

	// Accept connections in a goroutine so we can select on ctx.Done().
	connChan := make(chan net.Conn)
	errChan := make(chan error)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				errChan <- err
				return
			}
			connChan <- conn
		}
	}()

	for {
		select {
		case conn := <-connChan:
			// Handle each connection in a new goroutine to allow concurrent clients.
			go handleConnection(ctx, conn, WhoisSvc)
		case err := <-errChan:
			fmt.Println("Error accepting connection:", err)
		case <-ctx.Done():
			fmt.Println("Shutting down gracefully...")
			return
		}
	}
}

func handleConnection(ctx context.Context, conn net.Conn, svc *services.WhoisService) {
	defer conn.Close()

	// Read the query from the connection.
	reader := bufio.NewReader(conn)
	query, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading query:", err)
		return
	}

	// Trim the newline characters from the query.
	query = strings.TrimSpace(query)

	// Determine the type of query and respond accordingly.
	response := getWHOISResponse(ctx, query, svc)

	// Write the response back to the client.
	_, _ = conn.Write([]byte(response)) // the connection is gone if this fails, and the handler has no error path
}

// getWHOISResponse determines the type of WHOIS query and returns a response.
func getWHOISResponse(ctx context.Context, query string, svc *services.WhoisService) string {
	t, err := queries.ClassifyWhoisQuery(query)
	if err != nil {
		return err.Error()
	}
	switch t {
	case queries.WhoisQueryTypeDomainName:
		resp, err := svc.GetDomainWhois(ctx, query)
		if err != nil {
			return fmt.Sprintf("Error getting WHOIS information for domain: %v\n", err)
		}
		return resp.String()
	case queries.WhoisQueryTypeIP:
		return fmt.Sprintf("Only Domain objects are supported for now - received WHOIS query for IP: %s\n", query)
	case queries.WhoisQueryTypeRegistrar:
		return fmt.Sprintf("Only Domain objects are supported for now - received WHOIS query for Registrar               s: %s\n", query)
	default:
		return fmt.Sprintf("Unknown WHOIS query type for: %s\n", query)
	}
}

func setupDB() (*gorm.DB, error) {
	dbCfg, err := config.LoadDatabase()
	if err != nil {
		return nil, err
	}
	return postgres.NewConnection(postgres.Config(dbCfg))
}
