package epp

import (
	"crypto/tls"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/tlsconfig"
)

// SelfSignedCertificate generates a throwaway server certificate, valid for
// one day, for local development (EPP_TLS_SELF_SIGNED) and tests.
func SelfSignedCertificate() (tls.Certificate, error) {
	return tlsconfig.SelfSigned("epp.example.test")
}
