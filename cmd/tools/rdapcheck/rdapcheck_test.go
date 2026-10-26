package main

import (
	"bytes"
	"context"
	"crypto/x509"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/tlsconfig"
)

// servers stands two RDAP servers in for one host's IPv4 and IPv6 addresses.
func servers(t *testing.T, v4, v6 http.HandlerFunc) options {
	t.Helper()
	cert, err := tlsconfig.SelfSigned("rdap.example.test")
	require.NoError(t, err)
	roots := x509.NewCertPool()
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	require.NoError(t, err)
	roots.AddCert(leaf)

	listen := func(h http.Handler) string {
		s := httptest.NewUnstartedServer(h)
		certs, err := tlsconfig.NewStaticCertSource(cert, tlsconfig.Hooks{})
		require.NoError(t, err)
		s.TLS = tlsconfig.Server(certs)
		s.StartTLS()
		t.Cleanup(s.Close)
		return s.Listener.Addr().String()
	}
	byAddr := map[netip.Addr]string{
		netip.MustParseAddr("192.0.2.1"):   listen(v4),
		netip.MustParseAddr("2001:db8::1"): listen(v6),
	}
	base, _ := url.Parse("https://rdap.example.test/example/")
	return options{
		Base: base, Objects: []string{"domain/a.example", "entity/9995"}, Roots: roots, Timeout: 5 * time.Second,
		resolve: func(context.Context, string) ([]netip.Addr, error) {
			return []netip.Addr{netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("2001:db8::1")}, nil
		},
		dial: func(ctx context.Context, a netip.Addr) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "tcp", byAddr[a])
		},
	}
}

// answer serves a body whose last-update event and array orders vary.
func answer(order int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rdap+json")
		events := `[{"eventAction":"registration","eventDate":"2020-01-01T00:00:00Z"},{"eventAction":"last update of RDAP database","eventDate":"` + time.Now().Format(time.RFC3339Nano) + `"}]`
		status := `["active","client transfer prohibited"]`
		if order == 1 {
			status = `["client transfer prohibited","active"]`
		}
		_, _ = w.Write([]byte(`{"objectClassName":"domain","path":"` + r.URL.Path + `","status":` + status + `,"events":` + events + `}`)) // #nosec G705 -- test server echoing its own fixed paths
	}
}

func TestConsistentServicePortsPass(t *testing.T) {
	r := check(context.Background(), servers(t, answer(0), answer(1)))
	var out bytes.Buffer
	r.write(&out)
	assert.True(t, r.ok(), out.String())
	assert.Contains(t, out.String(), "PASS  rdap-91/2 192.0.2.1 refuses TLS 1.1")
	assert.Contains(t, out.String(), "PASS  rdap-92 domain/a.example is the same on 2 addresses")
}

func TestInconsistentServicePortsFail(t *testing.T) {
	other := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rdap+json")
		_, _ = w.Write([]byte(`{"objectClassName":"domain","status":["inactive"]}`))
	}
	r := check(context.Background(), servers(t, answer(0), other))
	var out bytes.Buffer
	r.write(&out)
	assert.False(t, r.ok())
	assert.Contains(t, out.String(), "FAIL  rdap-92 domain/a.example is the same on 2 addresses")
}

func TestAnUntrustedCertificateFails(t *testing.T) {
	o := servers(t, answer(0), answer(0))
	o.Roots = x509.NewCertPool()
	r := check(context.Background(), o)
	var out bytes.Buffer
	r.write(&out)
	assert.False(t, r.ok())
	assert.True(t, strings.Contains(out.String(), "FAIL  rdap-91/1"), out.String())
}

func TestCanonical(t *testing.T) {
	a, err := canonical([]byte(`{"b":1,"links":[{"rel":"self"},{"rel":"about"}],"events":[{"eventAction":"last update of RDAP database","eventDate":"x"}]}`))
	require.NoError(t, err)
	b, err := canonical([]byte(`{"links":[{"rel":"about"},{"rel":"self"}],"b":1,"events":[{"eventAction":"last update of RDAP database","eventDate":"y"}]}`))
	require.NoError(t, err)
	assert.Equal(t, a, b)
}

func TestCLIValidates(t *testing.T) {
	var out bytes.Buffer
	_, err := cli([]string{"-base", "http://x.example/"}, &out)
	assert.Error(t, err)
	_, err = cli([]string{"-base", "https://x.example/"}, &out)
	assert.ErrorContains(t, err, "at least one")
}

func TestPort(t *testing.T) {
	withPort, _ := url.Parse("https://rdap.example.test:9443/")
	without, _ := url.Parse("https://rdap.example.test/")
	assert.Equal(t, "9443", options{Base: withPort}.port())
	assert.Equal(t, "443", options{Base: without}.port(), "RST requires 443")
}
