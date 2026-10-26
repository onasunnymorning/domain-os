// Package tlsconfig is the TLS the registry's public protocol servers
// present: the RFC 9325 cipher suites, a server certificate loaded from file
// and reloaded when it is renewed, and PROXY protocol on the listener. The
// EPP and RDAP servers both terminate TLS on the node (ADR 0013, ADR 0020),
// so they share this code rather than keep two copies of it.
package tlsconfig

import (
	"crypto/tls"
	"net"
	"time"

	"github.com/pires/go-proxyproto"
)

// RFC9325CipherSuites are the TLS 1.2 cipher suites RFC 9325 §4.2
// recommends, and the only ones offered (RST epp-01 #10, rdap-91). Go's
// defaults also include CBC-mode suites. TLS 1.3 suites are not configurable
// in Go and are all AEAD.
var RFC9325CipherSuites = []uint16{
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
}

// Server returns a server configuration presenting certs' certificate, with
// TLS 1.2 as the minimum and only the RFC 9325 suites. Callers add what is
// particular to their protocol (client certificates, ALPN).
func Server(certs *CertSource) *tls.Config {
	return &tls.Config{
		GetCertificate: certs.GetCertificate,
		MinVersion:     tls.VersionTLS12,
		CipherSuites:   RFC9325CipherSuites,
	}
}

// ProxyListener wraps l so the client address comes from a PROXY protocol
// header (v1 or v2). Only peers in trusted may connect, and each must send a
// header; anything else is closed before the caller sees it.
func ProxyListener(l net.Listener, trusted []string, headerTimeout time.Duration) (net.Listener, error) {
	policy, err := proxyproto.TrustProxyHeaderFromRanges(trusted)
	if err != nil {
		return nil, err
	}
	return &proxyproto.Listener{
		Listener:          l,
		ConnPolicy:        policy,
		ReadHeaderTimeout: headerTimeout,
	}, nil
}
