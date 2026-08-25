package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

const (
	// WHOIS_PORT is the default port for WHOIS servers.
	WHOIS_PORT = 43
	// WHOIS_SERVER is the default WHOIS server.
	// WHOIS_SERVER = "whois.iana.org"
	WHOIS_SERVER = "localhost"
)

func performWhoisQuery(domain string, wg *sync.WaitGroup) {
	defer wg.Done()

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", WHOIS_SERVER, WHOIS_PORT), 10*time.Second)
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
		log.Printf("Error reading from WHOIS server: %v", err)
	}
}

func main() {
	var wg sync.WaitGroup
	domains := []string{"claire.melisa", "florida.melisa", "rashad.melisa"}

	for _, domain := range domains {
		wg.Add(1)
		go performWhoisQuery(domain, &wg)
	}

	wg.Wait()
}
