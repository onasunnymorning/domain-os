package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/likexian/whois"
	whoisparser "github.com/likexian/whois-parser"
)

func performWhoisQuery(domain string, wg *sync.WaitGroup) {
	defer wg.Done()

	conn, err := net.DialTimeout("tcp", "whois.iana.org:43", 10*time.Second)
	if err != nil {
		fmt.Println("Error connecting:", err)
		return
	}
	defer conn.Close()

	conn.Write([]byte(domain + "\r\n"))

	// Read the response
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading from WHOIS server: %v", err)
	}
}

// performWhoisQuery performs a WHOIS query for the given domain using
// Perform the WHOIS query
func performWhoisQueryParsed(domain string, wg *sync.WaitGroup) {
	defer wg.Done()
	result, err := whois.Whois(domain)
	if err != nil {
		fmt.Printf("Error fetching WHOIS information: %v", err)
	}

	// Parse the WHOIS result
	parsedResult, err := whoisparser.Parse(result)
	if err != nil {
		fmt.Printf("Error parsing WHOIS information: %v", err)
	}

	fmt.Println(parsedResult.Domain.ID)
}

func main() {
	var wg sync.WaitGroup
	domains := []string{"deprins.net"}

	for _, domain := range domains {
		wg.Add(1)
		// go performWhoisQuery(domain, &wg)
		go performWhoisQueryParsed(domain, &wg)
	}

	wg.Wait()
}
