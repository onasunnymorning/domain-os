package dns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"

	mdns "github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startTestDNSServer starts a DNS server on 127.0.0.1:0 (random port) with
// the given handler function. Returns the address and a cleanup function.
func startTestDNSServer(t *testing.T, handler mdns.HandlerFunc) (string, func()) {
	t.Helper()

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := &mdns.Server{
		PacketConn: pc,
		Handler:    handler,
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		srv.ActivateAndServe() //nolint:errcheck
	}()

	addr := pc.LocalAddr().String()
	return addr, func() {
		srv.Shutdown() //nolint:errcheck
		wg.Wait()
	}
}

func TestQuerySOA_HappyPath_AnswerSection(t *testing.T) {
	handler := func(w mdns.ResponseWriter, r *mdns.Msg) {
		resp := new(mdns.Msg)
		resp.SetReply(r)
		resp.Answer = append(resp.Answer, &mdns.SOA{
			Hdr:     mdns.RR_Header{Name: "example.com.", Rrtype: mdns.TypeSOA, Class: mdns.ClassINET, Ttl: 3600},
			Ns:      "ns1.example.com.",
			Mbox:    "admin.example.com.",
			Serial:  2024010101,
			Refresh: 3600,
			Retry:   900,
			Expire:  604800,
			Minttl:  86400,
		})
		w.WriteMsg(resp) //nolint:errcheck
	}

	addr, cleanup := startTestDNSServer(t, handler)
	defer cleanup()

	resolver := NewResolver(0)
	result, err := resolver.QuerySOA(context.Background(), "example.com.", addr)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, uint32(2024010101), result.Serial)
	assert.Equal(t, uint32(3600), result.Refresh)
	assert.Equal(t, uint32(900), result.Retry)
	assert.Equal(t, uint32(604800), result.Expire)
	assert.Equal(t, uint32(86400), result.Minttl)
}

func TestQuerySOA_AuthoritySection(t *testing.T) {
	handler := func(w mdns.ResponseWriter, r *mdns.Msg) {
		resp := new(mdns.Msg)
		resp.SetReply(r)
		// SOA in authority section (common for NS queries)
		resp.Ns = append(resp.Ns, &mdns.SOA{
			Hdr:     mdns.RR_Header{Name: "example.com.", Rrtype: mdns.TypeSOA, Class: mdns.ClassINET, Ttl: 3600},
			Ns:      "ns1.example.com.",
			Mbox:    "admin.example.com.",
			Serial:  2024020202,
			Refresh: 7200,
			Retry:   1800,
			Expire:  1209600,
			Minttl:  43200,
		})
		w.WriteMsg(resp) //nolint:errcheck
	}

	addr, cleanup := startTestDNSServer(t, handler)
	defer cleanup()

	resolver := NewResolver(0)
	result, err := resolver.QuerySOA(context.Background(), "example.com.", addr)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, uint32(2024020202), result.Serial)
	assert.Equal(t, uint32(7200), result.Refresh)
}

func TestQuerySOA_NXDOMAIN(t *testing.T) {
	handler := func(w mdns.ResponseWriter, r *mdns.Msg) {
		resp := new(mdns.Msg)
		resp.SetReply(r)
		resp.Rcode = mdns.RcodeNameError
		w.WriteMsg(resp) //nolint:errcheck
	}

	addr, cleanup := startTestDNSServer(t, handler)
	defer cleanup()

	resolver := NewResolver(0)
	result, err := resolver.QuerySOA(context.Background(), "nonexistent.example.com.", addr)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, ErrNXDomain), "expected ErrNXDomain, got: %v", err)
}

func TestQuerySOA_Refused(t *testing.T) {
	handler := func(w mdns.ResponseWriter, r *mdns.Msg) {
		resp := new(mdns.Msg)
		resp.SetReply(r)
		resp.Rcode = mdns.RcodeRefused
		w.WriteMsg(resp) //nolint:errcheck
	}

	addr, cleanup := startTestDNSServer(t, handler)
	defer cleanup()

	resolver := NewResolver(0)
	result, err := resolver.QuerySOA(context.Background(), "example.com.", addr)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, ErrRefused), "expected ErrRefused, got: %v", err)
}

func TestQuerySOA_NoSOA(t *testing.T) {
	handler := func(w mdns.ResponseWriter, r *mdns.Msg) {
		resp := new(mdns.Msg)
		resp.SetReply(r)
		// Return an A record instead of SOA
		resp.Answer = append(resp.Answer, &mdns.A{
			Hdr: mdns.RR_Header{Name: "example.com.", Rrtype: mdns.TypeA, Class: mdns.ClassINET, Ttl: 3600},
			A:   net.ParseIP("192.0.2.1"),
		})
		w.WriteMsg(resp) //nolint:errcheck
	}

	addr, cleanup := startTestDNSServer(t, handler)
	defer cleanup()

	resolver := NewResolver(0)
	result, err := resolver.QuerySOA(context.Background(), "example.com.", addr)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, ErrNoSOA), "expected ErrNoSOA, got: %v", err)
}

func TestQuerySOA_NameserverPortHandling(t *testing.T) {
	handler := func(w mdns.ResponseWriter, r *mdns.Msg) {
		resp := new(mdns.Msg)
		resp.SetReply(r)
		resp.Answer = append(resp.Answer, &mdns.SOA{
			Hdr:     mdns.RR_Header{Name: "example.com.", Rrtype: mdns.TypeSOA, Class: mdns.ClassINET, Ttl: 3600},
			Ns:      "ns1.example.com.",
			Mbox:    "admin.example.com.",
			Serial:  1,
			Refresh: 3600,
			Retry:   900,
			Expire:  604800,
			Minttl:  86400,
		})
		w.WriteMsg(resp) //nolint:errcheck
	}

	addr, cleanup := startTestDNSServer(t, handler)
	defer cleanup()

	// Extract host and port to test that addr with port works directly
	host, port, err := net.SplitHostPort(addr)
	require.NoError(t, err)

	resolver := NewResolver(0)

	// Test with explicit port
	result, err := resolver.QuerySOA(context.Background(), "example.com.", fmt.Sprintf("%s:%s", host, port))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, uint32(1), result.Serial)
}
