// rdapload drives an RDAP server with lookups, to measure it against the
// SLA's 400 ms round-trip time and to prove its rate limits hold (RDAP
// Phase 6, docs/rdap/RUNBOOK.md § Load test).
//
// Each worker looks up names from -names in turn (or -name over and over),
// and a share of -miss lookups goes to names that do not exist, as an
// enumerator's would. The report gives the status codes (429 is a client
// limit, 503 the node shedding), latency percentiles and how many requests
// were over 400 ms.
//
// Run it against a test environment. From one address it meets that
// address's own limits first: assign its network a generous access policy
// to measure the server rather than the limiter.
//
// Usage:
//
//	go run ./cmd/tools/rdapload -base https://localhost:8443/example/ \
//	  -names domains.txt -workers 8 -duration 1m -insecure
package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"time"
)

func main() {
	if err := cli(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "rdapload:", err)
		os.Exit(1)
	}
}

// options are a run's settings.
type options struct {
	Base     string        // base URL, ending in "/"
	Names    []string      // domain names to look up
	Miss     float64       // share of lookups for names that do not exist
	Workers  int           // concurrent workers
	Rate     float64       // requests per second per worker; 0 is as fast as possible
	Duration time.Duration // how long to run
	Insecure bool          // skip certificate verification (local development)
}

func cli(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("rdapload", flag.ContinueOnError)
	var (
		o      options
		names  = fs.String("names", "", "file of domain names to look up, one per line")
		name   = fs.String("name", "", "a single domain name to look up (instead of -names)")
		asJSON = fs.Bool("json", false, "write the report as JSON")
	)
	fs.StringVar(&o.Base, "base", "https://localhost:8443/", "the TLD's RDAP base URL")
	fs.Float64Var(&o.Miss, "miss", 0, "share of lookups (0 to 1) for names that do not exist")
	fs.IntVar(&o.Workers, "workers", 4, "concurrent workers")
	fs.Float64Var(&o.Rate, "rate", 0, "requests per second per worker (0: as fast as possible)")
	fs.DurationVar(&o.Duration, "duration", 30*time.Second, "how long to run")
	fs.BoolVar(&o.Insecure, "insecure", false, "do not verify the server certificate (local development)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	switch {
	case *names != "":
		list, err := readNames(*names)
		if err != nil {
			return err
		}
		o.Names = list
	case *name != "":
		o.Names = []string{*name}
	default:
		return errors.New("give -names or -name")
	}
	if !strings.HasSuffix(o.Base, "/") {
		o.Base += "/"
	}
	if o.Workers < 1 || o.Miss < 0 || o.Miss > 1 || o.Duration <= 0 {
		return errors.New("-workers must be at least 1, -miss between 0 and 1, -duration positive")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	rep := run(ctx, o)
	if *asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	}
	rep.write(out)
	return nil
}

func readNames(path string) ([]string, error) {
	f, err := os.Open(path) // #nosec G304 -- the operator's own list of names
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if n := strings.TrimSpace(sc.Text()); n != "" && !strings.HasPrefix(n, "#") {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s has no names", path)
	}
	return out, sc.Err()
}

// sample is one request's outcome.
type sample struct {
	code    int // 0 for a transport error
	latency time.Duration
}

// run drives the server until o.Duration passes or ctx is done.
func run(ctx context.Context, o options) report {
	ctx, cancel := context.WithTimeout(ctx, o.Duration)
	defer cancel()
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: o.Workers,
			// #nosec G402 -- opt-in for local development against a self-signed certificate
			TLSClientConfig: &tls.Config{InsecureSkipVerify: o.Insecure, MinVersion: tls.VersionTLS12},
		},
	}
	tld := ""
	if i := strings.LastIndex(strings.TrimSuffix(o.Names[0], "."), "."); i >= 0 {
		tld = o.Names[0][i+1:]
	}

	var (
		mu      sync.Mutex
		samples []sample
		wg      sync.WaitGroup
	)
	start := time.Now()
	for w := 0; w < o.Workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			var tick <-chan time.Time
			if o.Rate > 0 {
				t := time.NewTicker(time.Duration(float64(time.Second) / o.Rate))
				defer t.Stop()
				tick = t.C
			}
			for i := w; ; i += o.Workers {
				if tick != nil {
					select {
					case <-ctx.Done():
						return
					case <-tick:
					}
				} else if ctx.Err() != nil {
					return
				}
				name := o.Names[i%len(o.Names)]
				if o.Miss > 0 && fraction() < o.Miss {
					name = missName(tld)
				}
				s := lookup(ctx, client, o.Base+"domain/"+name)
				if ctx.Err() != nil && s.code == 0 {
					return // cut off by the end of the run, not a failure
				}
				mu.Lock()
				samples = append(samples, s)
				mu.Unlock()
			}
		}(w)
	}
	wg.Wait()
	return summarise(samples, time.Since(start))
}

func lookup(ctx context.Context, client *http.Client, url string) sample {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return sample{}
	}
	req.Header.Set("Accept", "application/rdap+json")
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return sample{latency: time.Since(start)}
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return sample{code: resp.StatusCode, latency: time.Since(start)}
}

// fraction is a uniform number in [0, 1).
func fraction() float64 {
	var b [8]byte
	_, _ = rand.Read(b[:])
	var n uint64
	for _, x := range b {
		n = n<<8 | uint64(x)
	}
	return float64(n>>11) / float64(1<<53)
}

// missName is a name under tld that no one has registered.
func missName(tld string) string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "rdapload-" + hex.EncodeToString(b[:]) + "." + tld
}

// slaRTT is the SLA's round-trip limit for an RDAP query.
const slaRTT = 400 * time.Millisecond

// report is what a run measured.
type report struct {
	Requests     int            `json:"requests"`
	PerSecond    float64        `json:"perSecond"`
	Codes        map[string]int `json:"codes"`
	Errors       int            `json:"transportErrors"`
	P50          time.Duration  `json:"p50"`
	P95          time.Duration  `json:"p95"`
	P99          time.Duration  `json:"p99"`
	Max          time.Duration  `json:"max"`
	OverSLA      int            `json:"overSLA"`
	RateLimited  int            `json:"rateLimited"`
	Shed         int            `json:"shed"`
	ElapsedSecs  float64        `json:"elapsedSeconds"`
	OverSLAShare float64        `json:"overSLAShare"`
}

func summarise(samples []sample, elapsed time.Duration) report {
	r := report{Requests: len(samples), Codes: map[string]int{}, ElapsedSecs: elapsed.Seconds()}
	if elapsed > 0 {
		r.PerSecond = float64(len(samples)) / elapsed.Seconds()
	}
	lat := make([]time.Duration, 0, len(samples))
	for _, s := range samples {
		switch {
		case s.code == 0:
			r.Errors++
		default:
			r.Codes[fmt.Sprint(s.code)]++
			lat = append(lat, s.latency)
		}
		if s.code == http.StatusTooManyRequests {
			r.RateLimited++
		}
		if s.code == http.StatusServiceUnavailable {
			r.Shed++
		}
		if s.latency > slaRTT {
			r.OverSLA++
		}
	}
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	at := func(q float64) time.Duration {
		if len(lat) == 0 {
			return 0
		}
		return lat[min(len(lat)-1, int(q*float64(len(lat))))]
	}
	r.P50, r.P95, r.P99 = at(0.50), at(0.95), at(0.99)
	if len(lat) > 0 {
		r.Max = lat[len(lat)-1]
	}
	if r.Requests > 0 {
		r.OverSLAShare = float64(r.OverSLA) / float64(r.Requests)
	}
	return r
}

func (r report) write(w io.Writer) {
	fmt.Fprintf(w, "requests      %d in %.1fs (%.1f/s)\n", r.Requests, r.ElapsedSecs, r.PerSecond)
	codes := make([]string, 0, len(r.Codes))
	for c := range r.Codes {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	for _, c := range codes {
		fmt.Fprintf(w, "  %s          %d\n", c, r.Codes[c])
	}
	if r.Errors > 0 {
		fmt.Fprintf(w, "  errors       %d (no response)\n", r.Errors)
	}
	fmt.Fprintf(w, "latency       p50 %s  p95 %s  p99 %s  max %s\n", r.P50.Round(time.Millisecond), r.P95.Round(time.Millisecond), r.P99.Round(time.Millisecond), r.Max.Round(time.Millisecond))
	fmt.Fprintf(w, "over 400 ms   %d (%.2f%%)\n", r.OverSLA, 100*r.OverSLAShare)
	fmt.Fprintf(w, "rate limited  %d (429)   shed %d (503)\n", r.RateLimited, r.Shed)
}
