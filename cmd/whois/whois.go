package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"

	"gorm.io/gorm"
)

const (
	// WHOIS_PORT is the default port for WHOIS servers.
	WHOIS_PORT = 43
)

func main() {
	// Set up the database connection.
	db, err := setupDB()
	if err != nil {
		log.Fatalf("Error setting up database: %v", err)
	}
	domRepo := postgres.NewDomainRepository(db)
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

	for {
		// Accept incoming connections.
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}

		// Handle each connection in a new goroutine to allow concurrent clients.
		go handleConnection(conn, WhoisSvc)
	}
}

func handleConnection(conn net.Conn, svc *services.WhoisService) {
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
	response := getWHOISResponse(query, svc)

	// Write the response back to the client.
	conn.Write([]byte(response))
}

// getWHOISResponse determines the type of WHOIS query and returns a response.
func getWHOISResponse(query string, svc *services.WhoisService) string {
	t, err := queries.ClassifyWhoisQuery(query)
	if err != nil {
		return err.Error()
	}
	switch t {
	case queries.WhoisQueryTypeDomainName:
		resp, err := svc.GetDomainWhois(context.Background(), query)
		if err != nil {
			return fmt.Sprintf("Error getting WHOIS information for domain: %v\n", err)
		}
		return resp.String()
	case queries.WhoisQueryTypeIP:
		return fmt.Sprintf("WHOIS IP query for: %s\n", query)
	case queries.WhoisQueryTypeRegistrar:
		return fmt.Sprintf("WHOIS registrar query for: %s\n", query)
	default:
		return fmt.Sprintf("Unknown WHOIS query type for: %s\n", query)
	}
}

func setupDB() (*gorm.DB, error) {
	return postgres.NewConnection(
		postgres.Config{
			User:    os.Getenv("DB_USER"),
			Pass:    os.Getenv("DB_PASS"),
			Host:    os.Getenv("DB_HOST"),
			Port:    os.Getenv("DB_PORT"),
			DBName:  os.Getenv("DB_NAME"),
			SSLmode: os.Getenv("DB_SSLMODE"),
		},
	)
}
