package rdap

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

func registry() *fakeLookups {
	return &fakeLookups{
		domains:     map[string]*interfaces.RDAPDomain{"example.tld": minimalDomain("example.tld", "tld"), "xn--bcher-kva.tld": minimalDomain("xn--bcher-kva.tld", "tld")},
		nameservers: map[string]*interfaces.RDAPNameserver{"ns1.example.tld": testNameserver("ns1.example.tld")},
		registrars:  map[int]*entities.Registrar{9995: testRegistrar()},
	}
}

func TestParseQuery(t *testing.T) {
	for _, tc := range []struct {
		path string
		kind queryKind
		tld  string
		arg  string
	}{
		{"help", queryHelp, "", ""},
		{"tld/help", queryHelp, "tld", ""},
		{"domain/Example.TLD.", queryDomain, "", "example.tld"},
		{"domain/bücher.tld", queryDomain, "", "xn--bcher-kva.tld"},
		{"tld/domain/example.tld", queryDomain, "tld", "example.tld"},
		{"xn--p1ai/domain/xn--e1afmkfd.xn--p1ai", queryDomain, "xn--p1ai", "xn--e1afmkfd.xn--p1ai"},
		{"nameserver/NS1.example.tld", queryNameserver, "", "ns1.example.tld"},
		{"entity/9995", queryEntity, "", "9995"},
		{"domains", querySearch, "", ""},
		{"tld/nameservers", querySearch, "tld", ""},
		{"entities", querySearch, "", ""},
		{"", queryInvalid, "", ""},
		{"tld", queryInvalid, "", ""},
		{"tld/", queryInvalid, "", ""},
		{"autnum/1", queryInvalid, "", ""},
		{"domain/", queryInvalid, "", ""},
		{"domain/tld", queryInvalid, "", ""},
		{"domain/a/b", queryInvalid, "", ""},
		{"domain/under_score.tld", queryInvalid, "", ""},
		{"help/me", queryInvalid, "", ""},
		{"co.uk/help", queryHelp, "co.uk", ""},
		{"co.uk/domain/example.co.uk", queryDomain, "co.uk", "example.co.uk"},
		{"help/help", queryHelp, "help", ""},
		{"domains/help", queryHelp, "domains", ""},
		{"domain/domain/a.domain", queryDomain, "domain", "a.domain"},
		{"domain/a..tld", queryInvalid, "", ""},
		{"domain/" + strings.Repeat("a", 64) + ".tld", queryInvalid, "", ""},
		{"domain/" + strings.Repeat("é", 50) + "." + strings.Repeat("é", 50) + "." + strings.Repeat("é", 50) + ".tld", queryDomain, "", ""},
	} {
		t.Run(tc.path, func(t *testing.T) {
			q := parseQuery(tc.path)
			assert.Equal(t, tc.kind, q.kind)
			assert.Equal(t, tc.tld, q.tld)
			if tc.arg != "" || tc.kind != queryDomain {
				assert.Equal(t, tc.arg, q.arg)
			}
			if tc.kind == queryInvalid {
				assert.NotEmpty(t, q.problem)
			}
		})
	}
}

func TestRegistrarHandle(t *testing.T) {
	for handle, want := range map[string]int{"9995": 9995, "1": 1, "0": 0, "": 0, "-3": 0, "12AB": 0, "123_CON-LOCAL": 0, "99999999999": 0} {
		id, ok := registrarHandle(handle)
		assert.Equal(t, want, id, handle)
		assert.Equal(t, want != 0, ok, handle)
	}
}

func TestResponses(t *testing.T) {
	ts := startServer(t, testConfig("https://rdap.example.test/"), registry())

	for _, tc := range []struct {
		name, method, path string
		status             int
	}{
		{"help", http.MethodGet, "help", http.StatusOK},
		{"help under a TLD", http.MethodGet, "tld/help", http.StatusOK},
		{"missing domain", http.MethodGet, "domain/missing.tld", http.StatusNotFound},
		{"domain outside the base URL's TLD", http.MethodGet, "other/domain/example.tld", http.StatusNotFound},
		{"missing nameserver", http.MethodGet, "nameserver/ns9.example.tld", http.StatusNotFound},
		{"missing registrar", http.MethodGet, "entity/1234", http.StatusNotFound},
		{"contact handle", http.MethodGet, "entity/123_CON-LOCAL", http.StatusNotFound},
		{"existing domain", http.MethodGet, "domain/example.tld", http.StatusOK},
		{"existing domain under its TLD", http.MethodGet, "tld/domain/example.tld", http.StatusOK},
		{"existing IDN domain by U-label", http.MethodGet, "domain/b%C3%BCcher.tld", http.StatusOK},
		{"HEAD of a domain", http.MethodHead, "domain/example.tld", http.StatusOK},
		{"existing nameserver", http.MethodGet, "nameserver/ns1.example.tld", http.StatusOK},
		{"existing registrar", http.MethodGet, "entity/9995", http.StatusOK},
		{"search", http.MethodGet, "domains?name=ex*", http.StatusNotImplemented},
		{"unknown query", http.MethodGet, "ip/192.0.2.1", http.StatusBadRequest},
		{"invalid name", http.MethodGet, "domain/-bad-.tld", http.StatusBadRequest},
		{"POST", http.MethodPost, "help", http.StatusMethodNotAllowed},
		{"HEAD of a missing domain", http.MethodHead, "domain/missing.tld", http.StatusNotFound},
		{"HEAD of help", http.MethodHead, "help", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := ts.do(t, tc.method, tc.path)
			assert.Equal(t, tc.status, resp.StatusCode)
			assert.Equal(t, contentType, resp.Header.Get("Content-Type"))
			assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"), "every response, errors and HEAD included")
			if tc.method == http.MethodHead {
				assert.Empty(t, body, "HEAD has no body")
				return
			}
			if tc.status != http.StatusOK {
				e := decodeError(t, body)
				assert.Equal(t, tc.status, e.ErrorCode)
				assert.Contains(t, e.RDAPConformance, "rdap_level_0")
			}
		})
	}

	t.Run("help carries conformance and the terms of use", func(t *testing.T) {
		_, body := ts.do(t, http.MethodGet, "tld/help")
		var help helpResponse
		require.NoError(t, json.Unmarshal(body, &help))
		assert.Equal(t, conformance, help.RDAPConformance)
		require.Len(t, help.Notices, 1)
		assert.Equal(t, "Terms of Use", help.Notices[0].Title)
		assert.Equal(t, "https://registry.example.test/rdap-terms", help.Notices[0].Links[0].Href)
		assert.Equal(t, "https://rdap.example.test/tld/help", help.Notices[0].Links[0].Value, "the context is the public URL")
	})

	t.Run("a public answer may be cached, varying on authorization", func(t *testing.T) {
		resp, _ := ts.do(t, http.MethodGet, "help")
		assert.Equal(t, cachePublic, resp.Header.Get("Cache-Control"))
		assert.Equal(t, "Authorization", resp.Header.Get("Vary"))
		resp, _ = ts.do(t, http.MethodPost, "help")
		assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
	})
}

func TestBasePath(t *testing.T) {
	ts := startServer(t, testConfig("https://rdap.example.test/rdap/"), registry())

	resp, _ := ts.do(t, http.MethodGet, "help")
	assert.Equal(t, http.StatusOK, resp.StatusCode, "served under the base path")

	outside := strings.TrimSuffix(ts.url, "rdap/") + "help"
	resp, err := ts.client.Get(outside)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "nothing is served outside the base path")
}

func TestEscapedBasePath(t *testing.T) {
	ts := startServer(t, testConfig("https://rdap.example.test/rdap%20v1/"), registry())
	resp, body := ts.do(t, http.MethodGet, "tld/help")
	require.Equal(t, http.StatusOK, resp.StatusCode, "routed by the decoded base path")
	var help helpResponse
	require.NoError(t, json.Unmarshal(body, &help))
	assert.Equal(t, "https://rdap.example.test/rdap%20v1/tld/help", help.Notices[0].Links[0].Value, "links keep the escaped base")
}

func TestEveryErrorCarriesTheTermsOfUse(t *testing.T) {
	ts := startServer(t, testConfig("https://rdap.example.test/"), registry())
	for path, method := range map[string]string{
		"ip/192.0.2.1":       http.MethodGet,  // 400
		"domain/missing.tld": http.MethodGet,  // 404
		"help":               http.MethodPost, // 405
		"domains?name=a*":    http.MethodGet,  // 501
	} {
		_, body := ts.do(t, method, path)
		e := decodeError(t, body)
		require.NotEmpty(t, e.Notices, "%s %s (%d)", method, path, e.ErrorCode)
		assert.Equal(t, "Terms of Use", e.Notices[0].Title)
	}
	lookups := registry()
	lookups.err = errors.New("down")
	failing := startServer(t, testConfig("https://rdap.example.test/"), lookups)
	_, body := failing.do(t, http.MethodGet, "domain/example.tld")
	assert.NotEmpty(t, decodeError(t, body).Notices, "500")
}

func TestLinkValuesAreTheQueriedURI(t *testing.T) {
	ts := startServer(t, testConfig("https://rdap.example.test/rdap/"), registry())
	root := strings.TrimSuffix(ts.url, "rdap/")
	get := func(path string) []byte {
		resp, err := ts.client.Get(root + path)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return body
	}

	var help helpResponse
	require.NoError(t, json.Unmarshal(get("rd%61p/help"), &help))
	require.NotEmpty(t, help.Notices, "an escaped spelling of the base path still routes")
	assert.Equal(t, "https://rdap.example.test/rd%61p/help", help.Notices[0].Links[0].Value, "the path as it was queried")

	e := decodeError(t, get("elsewhere"))
	assert.Equal(t, "https://rdap.example.test/elsewhere", e.Notices[0].Links[0].Value, "outside the base path too")
}

func TestLinkValuesKeepTheQuery(t *testing.T) {
	ts := startServer(t, testConfig("https://rdap.example.test/"), registry())
	_, body := ts.do(t, http.MethodGet, "help?foo=bar")
	var help helpResponse
	require.NoError(t, json.Unmarshal(body, &help))
	assert.Equal(t, "https://rdap.example.test/help?foo=bar", help.Notices[0].Links[0].Value)
}

func TestStalledLookupsEndBeforeTheWriteDeadline(t *testing.T) {
	lookups := registry()
	lookups.stall = true
	cfg := testConfig("https://rdap.example.test/")
	cfg.Timeouts.Write = 400 * time.Millisecond
	ts := startServer(t, cfg, lookups)

	start := time.Now()
	resp, body := ts.do(t, http.MethodGet, "domain/example.tld")
	assert.Less(t, time.Since(start), cfg.Timeouts.Write, "answered within the write timeout")
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Equal(t, 500, decodeError(t, body).ErrorCode)
}

func TestLookupFailureIs500(t *testing.T) {
	lookups := registry()
	lookups.err = errors.New("database unavailable")
	ts := startServer(t, testConfig("https://rdap.example.test/"), lookups)

	resp, body := ts.do(t, http.MethodGet, "domain/example.tld")
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Equal(t, 500, decodeError(t, body).ErrorCode)
	assert.NotContains(t, string(body), "database unavailable", "internal errors are not shown")
}

func TestOpsAndMetrics(t *testing.T) {
	ts := startServer(t, testConfig("https://rdap.example.test/"), registry())
	_, _ = ts.do(t, http.MethodGet, "domain/missing.tld")

	assert.Equal(t, http.StatusOK, opsGet(ts.Server, "/healthz").Code)
	assert.Equal(t, http.StatusOK, opsGet(ts.Server, "/readyz").Code)
	metrics := opsGet(ts.Server, "/metrics").Body.String()
	assert.Contains(t, metrics, `rdap_requests_total{code="404",route="domain"} 1`)
	assert.Contains(t, metrics, "rdap_request_duration_seconds_bucket")
	assert.Contains(t, metrics, "rdap_server_cert_not_after_timestamp_seconds")

	t.Run("draining turns readiness off", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		require.NoError(t, ts.Shutdown(ctx))
		assert.Equal(t, http.StatusServiceUnavailable, opsGet(ts.Server, "/readyz").Code)
		assert.Contains(t, opsGet(ts.Server, "/metrics").Body.String(), "rdap_draining 1")
	})
}

func TestPublicationPoliciesAreReloaded(t *testing.T) {
	cfg := testConfig("https://rdap.example.test/")
	cfg.PolicyRefresh = 20 * time.Millisecond

	t.Run("on start and every interval", func(t *testing.T) {
		lookups := registry()
		ts := startServer(t, cfg, lookups)
		require.Eventually(t, func() bool {
			lookups.mu.Lock()
			defer lookups.mu.Unlock()
			return lookups.refreshes >= 2
		}, 2*time.Second, 5*time.Millisecond)
		assert.NotContains(t, opsGet(ts.Server, "/metrics").Body.String(), "rdap_policies_refreshed_timestamp_seconds 0")
	})

	t.Run("a failed reload is counted and the server keeps serving", func(t *testing.T) {
		lookups := registry()
		lookups.refreshErr, lookups.failAfter = errors.New("database unavailable"), 1
		ts := startServer(t, cfg, lookups)
		require.Eventually(t, func() bool {
			return strings.Contains(opsGet(ts.Server, "/metrics").Body.String(), "rdap_policy_refresh_errors_total 2")
		}, 2*time.Second, 5*time.Millisecond)
		resp, _ := ts.do(t, http.MethodGet, "domain/example.tld")
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("no first load, no serving", func(t *testing.T) {
		lookups := registry()
		lookups.refreshErr = errors.New("database unavailable")
		srv, err := NewServer(Options{Config: cfg, Lookups: lookups})
		require.NoError(t, err)
		l, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer func() { _ = l.Close() }()
		err = srv.Serve(l)
		assert.ErrorContains(t, err, "load the publication policies before serving")
		assert.False(t, srv.Ready(), "never ready")
	})

	t.Run("shutdown stops a first load that has stalled", func(t *testing.T) {
		slow := cfg
		slow.PolicyRefresh = time.Minute
		lookups := registry()
		lookups.stallRefresh = true
		srv, err := NewServer(Options{Config: slow, Lookups: lookups})
		require.NoError(t, err)
		l, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer func() { _ = l.Close() }()
		served := make(chan error, 1)
		go func() { served <- srv.Serve(l) }()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, srv.Shutdown(ctx))
		select {
		case err := <-served:
			assert.NoError(t, err, "a shutdown is not a failure to start")
		case <-time.After(2 * time.Second):
			t.Fatal("Serve still waiting on the policy load after Shutdown")
		}
		assert.False(t, srv.Ready())
	})
}
