package rdap

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/rdappolicy"
	"github.com/onasunnymorning/domain-os/internal/config"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// fakeLookups is a registry holding the records, names and IANA IDs it is
// given.
type fakeLookups struct {
	domains     map[string]*interfaces.RDAPDomain
	nameservers map[string]*interfaces.RDAPNameserver
	registrars  map[int]*entities.Registrar
	err         error // returned by every lookup when set

	mu         sync.Mutex
	refreshes  int
	refreshErr error
}

func (f *fakeLookups) Domain(_ context.Context, name string) (*interfaces.RDAPDomain, error) {
	if f.err != nil {
		return nil, f.err
	}
	rec, ok := f.domains[name]
	if !ok {
		return nil, entities.ErrDomainNotFound
	}
	return rec, nil
}

// minimalDomain is a registered domain with nothing but its name.
func minimalDomain(name, tld string) *interfaces.RDAPDomain {
	return &interfaces.RDAPDomain{
		Domain:      &entities.Domain{Name: entities.DomainName(name), TLDName: entities.DomainName(tld), RoID: "1_DOM-TEST"},
		TLDType:     entities.TLDTypeGTLD,
		Publication: rdappolicy.GTLDRegistrationDataPolicy2024(),
	}
}

func (f *fakeLookups) Nameserver(_ context.Context, name string) (*interfaces.RDAPNameserver, error) {
	if f.err != nil {
		return nil, f.err
	}
	rec, ok := f.nameservers[name]
	if !ok {
		return nil, entities.ErrHostNotFound
	}
	return rec, nil
}

func (f *fakeLookups) Registrar(_ context.Context, ianaID int) (*entities.Registrar, error) {
	if f.err != nil {
		return nil, f.err
	}
	rr, ok := f.registrars[ianaID]
	if !ok {
		return nil, entities.ErrRegistrarNotFound
	}
	return rr, nil
}

func (f *fakeLookups) RefreshPublicationPolicies(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refreshes++
	return f.refreshErr
}

// testRegistrar is an accredited registrar with every contact field.
func testRegistrar() *entities.Registrar {
	return &entities.Registrar{
		ClID: "rar-1", Name: "Example Registrar, Inc.", GurID: 9995,
		Voice: "+1.5555550000", Email: "abuse@registrar.example", URL: "https://registrar.example",
		PostalInfo: [2]*entities.RegistrarPostalInfo{{Type: "int", Address: &entities.Address{Street1: "1 Registrar Way", City: "Los Angeles", StateProvince: "CA", PostalCode: "90001", CountryCode: "US"}}},
	}
}

// testNameserver is a host named name, sponsored by testRegistrar.
func testNameserver(name string) *interfaces.RDAPNameserver {
	return &interfaces.RDAPNameserver{
		Host: &entities.Host{
			RoID: "200_HOST-TEST", Name: entities.DomainName(name), ClID: "rar-1",
			Addresses: []netip.Addr{netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("2001:db8::1")},
			Status:    entities.HostStatus{Linked: true, OK: true},
		},
		Registrar: testRegistrar(),
	}
}

// testConfig is a server on a self-signed certificate under base.
func testConfig(base string) config.RDAP {
	return config.RDAP{
		BaseURL:  base,
		TLS:      config.RDAPTLS{SelfSigned: true, ServerName: "rdap.example.test", ReloadInterval: time.Minute},
		Timeouts: config.RDAPTimeouts{Read: 5 * time.Second, Write: 5 * time.Second, Idle: 30 * time.Second, Drain: 5 * time.Second},
		Notices:  config.RDAPNotices{TermsOfServiceURL: "https://registry.example.test/rdap-terms"},
	}
}

// testServer is a running server and a client for it.
type testServer struct {
	*Server
	url    string // base URL on the loopback listener
	client *http.Client
}

func startServer(t *testing.T, cfg config.RDAP, lookups *fakeLookups) *testServer {
	t.Helper()
	srv, err := NewServer(Options{Config: cfg, Lookups: lookups})
	require.NoError(t, err)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	served := make(chan error, 1)
	go func() { served <- srv.Serve(l) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		require.NoError(t, <-served)
	})
	require.Eventually(t, srv.Ready, 2*time.Second, 5*time.Millisecond)
	return &testServer{
		Server: srv,
		url:    "https://" + l.Addr().String() + srv.basePath,
		client: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				// #nosec G402 -- test-only: the server presents a certificate generated for this test
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12},
			},
		},
	}
}

// do sends method to path (relative to the base URL) and returns the
// response with its body read.
func (ts *testServer) do(t *testing.T, method, path string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, ts.url+path, nil)
	require.NoError(t, err)
	req.Header.Set("Accept", contentType)
	resp, err := ts.client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, body
}

// decodeError reads an RDAP error object, failing the test if body is not
// one.
func decodeError(t *testing.T, body []byte) errorResponse {
	t.Helper()
	var e errorResponse
	require.NoError(t, json.Unmarshal(body, &e), "body is an RDAP error object: %s", body)
	return e
}

// opsGet fetches path from the server's ops handler.
func opsGet(s *Server, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	s.OpsHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}
