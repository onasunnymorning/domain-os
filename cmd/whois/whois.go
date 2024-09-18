package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
)

func main() {
	// Listen on port 43 for incoming WHOIS requests.
	listener, err := net.Listen("tcp", ":43")
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
	defer listener.Close()

	fmt.Println("WHOIS server running on port 43")

	for {
		// Accept incoming connections.
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}

		// Handle each connection in a new goroutine to allow concurrent clients.
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
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
	response := getWHOISResponse(query)

	// Write the response back to the client.
	conn.Write([]byte(response))
}

func getWHOISResponse(query string) string {
	t, err := queries.ClassifyWhoisQuery(query)
	if err != nil {
		return err.Error()
	}
	switch t {
	case queries.WhoisQueryTypeDomainName:
		return fmt.Sprintf("WHOIS domain query for: %s\n", query)
	case queries.WhoisQueryTypeIP:
		return fmt.Sprintf("WHOIS IP query for: %s\n", query)
	case queries.WhoisQueryTypeRegistrar:
		return fmt.Sprintf("WHOIS registrar query for: %s\n", query)
	default:
		return fmt.Sprintf("Unknown WHOIS query type for: %s\n", query)
	}
	// For demonstration purposes, we'll just handle a few hardcoded domains and IPs.
	// 	switch strings.ToLower(query) {
	// 	case "example.com":
	// 		return `Domain Name: EXAMPLE.COM
	// Registrar: RESERVED DOMAINS REGISTRY
	// Creation Date: 1995-08-14T04:00:00Z
	// Registry Expiry Date: 2024-08-13T04:00:00Z
	// Name Server: A.IANA-SERVERS.NET
	// Name Server: B.IANA-SERVERS.NET
	// Domain Status: clientDeleteProhibited
	// >>> Last update of WHOIS database: 2023-08-01T00:00:00Z <<<
	// `
	// 	case "192.0.2.1":
	// 		return `NetRange: 192.0.2.0 - 192.0.2.255
	// CIDR: 192.0.2.0/24
	// NetName: EXAMPLE-NET
	// NetHandle: NET-192-0-2-0-1
	// Organization: Example Organization
	// Updated: 2023-01-01
	// `
	// 	case "2001:db8::1":
	// 		return `NetRange: 2001:db8::/32
	// CIDR: 2001:db8::/32
	// NetName: EXAMPLE-V6
	// Organization: Example IPv6 Org
	// Updated: 2023-01-01
	// `
	// 	default:
	// 		return fmt.Sprintf("No WHOIS data found for: %s\n", query)
	// 	}
}
