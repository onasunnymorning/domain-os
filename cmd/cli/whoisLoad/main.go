package main

import (
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
)

// Target WHOIS server and WOIS_PORT
const (
	WHOIS_SERVER   = "localhost"
	WOIS_PORT      = 43
	TOTAL_REQUESTS = 10000
	THREADS        = 50
)

// List of domain names to pick randomly from
var domains = []string{
	"claire.melisa",
	"florida.melisa",
	"rashad.melisa",
}

// Metrics structure
type Metrics struct {
	mu             sync.Mutex
	connectionTime []time.Duration
	ttfb           []time.Duration
	ttlb           []time.Duration
	failures       int
	timeouts       int
	errorMessages  map[string]int
}

func (m *Metrics) recordConnectionTime(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connectionTime = append(m.connectionTime, d)
}

func (m *Metrics) recordTTFB(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ttfb = append(m.ttfb, d)
}

func (m *Metrics) recordTTLB(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ttlb = append(m.ttlb, d)
}

func (m *Metrics) recordFailure(errorMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failures++
	m.errorMessages[errorMsg]++
}

func (m *Metrics) recordTimeout() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.timeouts++
	m.errorMessages["Timeout"]++
}

func (m *Metrics) summarize() {
	m.mu.Lock()
	defer m.mu.Unlock()

	fmt.Printf("Summary:\n")
	fmt.Printf("Total Requests: %d\n", TOTAL_REQUESTS)
	fmt.Printf("Number of Threads: %d\n", THREADS)
	fmt.Printf("Failures: %d\n", m.failures)
	fmt.Printf("Timeouts: %d\n", m.timeouts)

	// Summarize connection time
	m.summarizeDurations("Connection Time", m.connectionTime)

	// Summarize TTFB
	m.summarizeDurations("TTFB", m.ttfb)

	// Summarize TTLB
	m.summarizeDurations("TTLB", m.ttlb)

	// Error Messages Summary
	fmt.Printf("Error Messages:\n")
	for msg, count := range m.errorMessages {
		fmt.Printf("  %s: %d\n", msg, count)
	}
}

func (m *Metrics) summarizeDurations(name string, durations []time.Duration) {
	if len(durations) == 0 {
		fmt.Printf("%s: No successful measurements\n", name)
		return
	}

	min, max, avg := minMaxAvgDuration(durations)
	fmt.Printf("%s:\n", name)
	fmt.Printf("  Min: %v\n", min)
	fmt.Printf("  Max: %v\n", max)
	fmt.Printf("  Avg: %v\n", avg)
}

func minMaxAvgDuration(durations []time.Duration) (min time.Duration, max time.Duration, avg time.Duration) {
	if len(durations) == 0 {
		return 0, 0, 0
	}

	min = durations[0]
	max = durations[0]
	var total time.Duration

	for _, d := range durations {
		if d < min {
			min = d
		}
		if d > max {
			max = d
		}
		total += d
	}

	avg = total / time.Duration(len(durations))
	return min, max, avg
}

func sendQuery(query string, metrics *Metrics) {
	start := time.Now()

	// Establish a TCP connection to the WHOIS server
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", WHOIS_SERVER, WOIS_PORT), 10*time.Second)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			metrics.recordTimeout()
		} else {
			metrics.recordFailure(err.Error())
		}
		fmt.Printf("Connection error occurred for query '%s': %v\n", query, err)
		return
	}
	defer conn.Close()

	connectionTime := time.Since(start)
	metrics.recordConnectionTime(connectionTime)

	// Send the WHOIS query
	_, err = fmt.Fprintf(conn, "%s\r\n", query)
	if err != nil {
		metrics.recordFailure(err.Error())
		fmt.Printf("Error occurred for query '%s': %v\n", query, err)
		return
	}

	ttfbStart := time.Now()
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	ttfb := time.Since(ttfbStart)
	metrics.recordTTFB(ttfb)

	if err != nil {
		metrics.recordFailure(err.Error())
		fmt.Printf("Error occurred for query '%s': %v\n", query, err)
		return
	}

	ttlb := time.Since(start)
	metrics.recordTTLB(ttlb)

	response := string(buf[:n])
	fmt.Printf("Response for query '%s':\n%s\n", query, response)

	// Check for specific error in response
	if strings.Contains(response, "Query rate exceeded, please try again later.") {
		metrics.recordFailure("Query rate exceeded")
	}
}

func runLoadTest() {
	// Seed random number generator
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Create channels to synchronize goroutines
	done := make(chan bool)
	requests := make(chan string)

	// Metrics collection
	metrics := &Metrics{
		errorMessages: make(map[string]int),
	}

	// Distribute requests among goroutines
	for i := 0; i < THREADS; i++ {
		go func() {
			for query := range requests {
				sendQuery(query, metrics)
			}
			done <- true
		}()
	}

	// Generate random domain names and send requests
	for i := 0; i < TOTAL_REQUESTS; i++ {
		randomDomain := domains[r.Intn(len(domains))]
		requests <- randomDomain
	}

	// Close the requests channel to signal goroutines to stop
	close(requests)

	// Wait for all goroutines to finish
	for i := 0; i < THREADS; i++ {
		<-done
	}

	// Summarize the metrics
	metrics.summarize()
}

func main() {
	runLoadTest()
}
