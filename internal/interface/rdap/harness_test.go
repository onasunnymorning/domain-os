package rdap

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/config"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// fakeLookups is a registry holding the names and IANA IDs it is given.
type fakeLookups struct {
	domains     map[string]bool
	nameservers map[string]bool
	registrars  map[int]bool
	err         error // returned by every lookup when set
	// stall makes Domain wait until its context is done.
	stall bool
}

func (f *fakeLookups) Domain(ctx context.Context, name string) (*entities.Domain, error) {
	if f.stall {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if f.err != nil {
		return nil, f.err
	}
	if !f.domains[name] {
		return nil, entities.ErrDomainNotFound
	}
	return &entities.Domain{Name: entities.DomainName(name)}, nil
}

func (f *fakeLookups) NameserverExists(_ context.Context, name string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.nameservers[name], nil
}

func (f *fakeLookups) Registrar(_ context.Context, ianaID int) (*entities.Registrar, error) {
	if f.err != nil {
		return nil, f.err
	}
	if !f.registrars[ianaID] {
		return nil, entities.ErrRegistrarNotFound
	}
	return &entities.Registrar{GurID: ianaID}, nil
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
		url:    "https://" + l.Addr().String() + srv.escapedBasePath,
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
