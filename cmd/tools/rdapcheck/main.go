// rdapcheck checks a deployed RDAP service the way RST's rdap-91 and
// rdap-92 do (RDAP Phase 7, docs/rdap/ARCHITECTURE.md § Pre-test
// checklist). These two depend on the deployment — its DNS, its addresses,
// the certificate installed — so the suite cannot check them; run this
// against the real base URL before an RST run.
//
// For every IPv4 and IPv6 address the RDAP host name resolves to:
//
//	rdap-91  TLS 1.2 is accepted and TLS 1.1 refused; only RFC 9325 cipher
//	         suites are offered; the certificate chains to a trusted CA,
//	         has not expired and covers the host name.
//	rdap-92  each test object answers 200, and the answers are the same on
//	         every address once parsed and canonicalised, the "last update
//	         of RDAP database" event aside.
//
// Usage:
//
//	go run ./cmd/tools/rdapcheck -base https://rdap.example/example/ \
//	  -domain nic.example -nameserver ns1.nic.example -entity 9995
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/tlsconfig"
)

func main() {
	ok, err := cli(os.Args[1:], os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rdapcheck:", err)
		os.Exit(2)
	}
	if !ok {
		os.Exit(1)
	}
}

// options are what to check.
type options struct {
	Base    *url.URL
	Objects []string // paths under Base: domain/x, nameserver/y, entity/z
	Roots   *x509.CertPool
	Timeout time.Duration
	// dial connects to addr on the base URL's port. Tests replace it.
	dial func(ctx context.Context, addr netip.Addr) (net.Conn, error)
	// resolve returns the host name's addresses. Tests replace it.
	resolve func(ctx context.Context, host string) ([]netip.Addr, error)
}

type list []string

func (l *list) String() string     { return strings.Join(*l, ",") }
func (l *list) Set(v string) error { *l = append(*l, v); return nil }

func cli(args []string, out io.Writer) (bool, error) {
	fs := flag.NewFlagSet("rdapcheck", flag.ContinueOnError)
	var (
		base                         = fs.String("base", "", "the TLD's RDAP base URL, as given to RST (rdap.baseURLs)")
		domains, nameservers, entity list
		caFile                       = fs.String("ca", "", "extra trusted CA certificates, PEM (testing only: RST trusts browser CAs)")
		timeout                      = fs.Duration("timeout", 10*time.Second, "per connection and request")
	)
	fs.Var(&domains, "domain", "a test domain (rdap.testDomains); repeatable")
	fs.Var(&nameservers, "nameserver", "a test nameserver (rdap.testNameservers); repeatable")
	fs.Var(&entity, "entity", "a test entity handle (rdap.testEntities); repeatable")
	if err := fs.Parse(args); err != nil {
		return false, err
	}
	u, err := url.Parse(*base)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return false, errors.New("-base must be an https URL")
	}
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}
	o := options{Base: u, Timeout: *timeout}
	for _, d := range domains {
		o.Objects = append(o.Objects, "domain/"+d)
	}
	for _, n := range nameservers {
		o.Objects = append(o.Objects, "nameserver/"+n)
	}
	for _, e := range entity {
		o.Objects = append(o.Objects, "entity/"+e)
	}
	if len(o.Objects) == 0 {
		return false, errors.New("give at least one -domain, -nameserver or -entity")
	}
	o.Roots, err = x509.SystemCertPool()
	if err != nil {
		o.Roots = x509.NewCertPool()
	}
	if *caFile != "" {
		pem, err := os.ReadFile(*caFile) // #nosec G304 -- the operator's own CA file
		if err != nil {
			return false, err
		}
		if !o.Roots.AppendCertsFromPEM(pem) {
			return false, fmt.Errorf("%s has no PEM certificates", *caFile)
		}
	}
	res := check(context.Background(), o)
	res.write(out)
	return res.ok(), nil
}

// result is every check's outcome, in order.
type result struct {
	lines []line
}

type line struct {
	ok   bool
	text string
}

func (r *result) add(ok bool, format string, args ...any) {
	r.lines = append(r.lines, line{ok: ok, text: fmt.Sprintf(format, args...)})
}

func (r *result) ok() bool {
	for _, l := range r.lines {
		if !l.ok {
			return false
		}
	}
	return len(r.lines) > 0
}

func (r *result) write(w io.Writer) {
	for _, l := range r.lines {
		mark := "PASS"
		if !l.ok {
			mark = "FAIL"
		}
		fmt.Fprintf(w, "%s  %s\n", mark, l.text)
	}
}

func (o options) port() string {
	if p := o.Base.Port(); p != "" {
		return p
	}
	return "443"
}

// check runs rdap-91 and rdap-92 against every address of the base URL's
// host.
func check(ctx context.Context, o options) *result {
	r := &result{}
	host := o.Base.Hostname()
	if o.resolve == nil {
		o.resolve = func(ctx context.Context, host string) ([]netip.Addr, error) {
			ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
			for i := range ips {
				ips[i] = ips[i].Unmap()
			}
			return ips, err
		}
	}
	if o.dial == nil {
		o.dial = func(ctx context.Context, addr netip.Addr) (net.Conn, error) {
			d := net.Dialer{Timeout: o.Timeout}
			return d.DialContext(ctx, "tcp", net.JoinHostPort(addr.String(), o.port()))
		}
	}
	addrs, err := o.resolve(ctx, host)
	if err != nil || len(addrs) == 0 {
		r.add(false, "rdap-92 %s resolves (RDAP_TLS_DNS_RESOLUTION_ERROR): %v", host, err)
		return r
	}
	var v4, v6 int
	for _, a := range addrs {
		if a.Is4() {
			v4++
		} else {
			v6++
		}
	}
	r.add(v4 > 0, "rdap-92 %s has an IPv4 address (%d)", host, v4)
	r.add(v6 > 0, "rdap-92 %s has an IPv6 address (%d)", host, v6)

	answers := map[string]map[netip.Addr]string{} // object -> address -> canonical body
	for _, a := range addrs {
		checkTLS(ctx, o, r, host, a)
		client := o.client(host, a)
		for _, obj := range o.Objects {
			body, err := fetch(ctx, client, o.Base.String()+obj, o.Timeout)
			if err != nil {
				r.add(false, "rdap-92 %s on %s (RDAP_QUERY_FAILED): %v", obj, a, err)
				continue
			}
			if answers[obj] == nil {
				answers[obj] = map[netip.Addr]string{}
			}
			answers[obj][a] = body
		}
	}
	for _, obj := range o.Objects {
		var first string
		consistent, seen := true, 0
		for _, a := range addrs {
			body, ok := answers[obj][a]
			if !ok {
				continue
			}
			if seen == 0 {
				first = body
			} else if body != first {
				consistent = false
			}
			seen++
		}
		if seen > 0 {
			r.add(consistent, "rdap-92 %s is the same on %d addresses (RDAP_SERVICE_PORT_NOT_CONSISTENT if not)", obj, seen)
		}
	}
	return r
}

// client fetches over a connection to addr only, verifying the certificate
// against host.
func (o options) client(host string, addr netip.Addr) *http.Client {
	return &http.Client{
		Timeout: o.Timeout,
		// RST grades the answer at the URL it asked for: a redirect to a
		// 200 is not a 200.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Transport: &http.Transport{
			DialContext:     func(ctx context.Context, _, _ string) (net.Conn, error) { return o.dial(ctx, addr) },
			TLSClientConfig: &tls.Config{ServerName: host, RootCAs: o.Roots, MinVersion: tls.VersionTLS12},
		},
	}
}

// checkTLS is rdap-91 on one address.
func checkTLS(ctx context.Context, o options, r *result, host string, addr netip.Addr) {
	handshake := func(cfg *tls.Config) (*tls.ConnectionState, error) {
		cfg.ServerName, cfg.RootCAs = host, o.Roots
		raw, err := o.dial(ctx, addr)
		if err != nil {
			return nil, err
		}
		defer func() { _ = raw.Close() }()
		_ = raw.SetDeadline(time.Now().Add(o.Timeout))
		c := tls.Client(raw, cfg)
		if err := c.HandshakeContext(ctx); err != nil {
			return nil, err
		}
		st := c.ConnectionState()
		return &st, nil
	}

	st, err := handshake(&tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12}) // #nosec G402 -- pinning TLS 1.2 to check it is offered (rdap-91/1)
	r.add(err == nil, "rdap-91/1 %s accepts TLS 1.2 with a trusted, current certificate for %s: %v", addr, host, errOrOK(err))
	if err == nil {
		r.add(rfc9325(st.CipherSuite), "rdap-91/6 %s negotiates an RFC 9325 suite (%s)", addr, tls.CipherSuiteName(st.CipherSuite))
		leaf := st.PeerCertificates[0]
		r.add(time.Until(leaf.NotAfter) > 0, "rdap-91/4 %s certificate expires %s", addr, leaf.NotAfter.UTC().Format(time.RFC3339))
	}
	_, err = handshake(&tls.Config{MinVersion: tls.VersionTLS10, MaxVersion: tls.VersionTLS11, InsecureSkipVerify: true}) // #nosec G402 -- deliberately offering TLS 1.1 to check it is refused
	r.add(err != nil, "rdap-91/2 %s refuses TLS 1.1 and earlier", addr)
	var weak []uint16
	for _, s := range tls.CipherSuites() {
		if !rfc9325(s.ID) && slicesContains(s.SupportedVersions, tls.VersionTLS12) {
			weak = append(weak, s.ID)
		}
	}
	for _, s := range tls.InsecureCipherSuites() {
		if slicesContains(s.SupportedVersions, tls.VersionTLS12) {
			weak = append(weak, s.ID)
		}
	}
	_, err = handshake(&tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, CipherSuites: weak, InsecureSkipVerify: true}) // #nosec G402 -- deliberately offering only weak suites to check they are refused
	// Go can only offer the suites it implements, so this cannot see a
	// server that also accepts, say, a DHE or CCM suite; say so, and leave
	// the full scan to a tool that offers every suite.
	r.add(err != nil, "rdap-91/6 %s refuses the %d TLS 1.2 suites outside RFC 9325 that Go can offer (not DHE, CCM, ARIA or Camellia suites: scan those with testssl.sh)", addr, len(weak))
}

func rfc9325(id uint16) bool { return slicesContains(tlsconfig.RFC9325CipherSuites, id) }

func slicesContains[T comparable](s []T, v T) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func errOrOK(err error) string {
	if err != nil {
		return err.Error()
	}
	return "ok"
}

// fetch GETs url and returns its body, canonicalised, or an error for any
// status but 200.
func fetch(ctx context.Context, client *http.Client, url string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/rdap+json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	return canonical(body)
}

// unordered are the members whose arrays rdap-92 compares in any order.
var unordered = map[string]bool{
	"entities": true, "events": true, "notices": true, "remarks": true, "links": true, "rdapConformance": true,
	"publicIds": true, "status": true, "ipAddresses": true, "nameservers": true, "redacted": true, "v4": true, "v6": true,
}

// canonical is body parsed and re-encoded with sorted keys, the unordered
// arrays sorted, and the "last update of RDAP database" event dropped.
func canonical(body []byte) (string, error) {
	var v any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return "", fmt.Errorf("not JSON: %w", err)
	}
	// One JSON value and nothing after it: trailing data would otherwise
	// compare equal to a clean answer.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return "", errors.New("not JSON: data after the first value")
	}
	out, err := json.Marshal(normalise(v, ""))
	return string(out), err
}

func normalise(v any, key string) any {
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			t[k] = normalise(x, k)
		}
		return t
	case []any:
		// Never nil: an empty array must not marshal as null and compare
		// equal to a null member.
		kept := make([]any, 0, len(t))
		for _, x := range t {
			if key == "events" {
				if m, ok := x.(map[string]any); ok && m["eventAction"] == "last update of RDAP database" {
					continue
				}
			}
			kept = append(kept, normalise(x, ""))
		}
		if unordered[key] {
			sort.Slice(kept, func(i, j int) bool {
				a, _ := json.Marshal(kept[i])
				b, _ := json.Marshal(kept[j])
				return string(a) < string(b)
			})
		}
		return kept
	default:
		return v
	}
}
