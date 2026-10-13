package epp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"time"
)

// certSource supplies the server certificate for each handshake. Swapping
// the certificate affects new handshakes only; live sessions keep the one
// they negotiated.
type certSource struct {
	current atomic.Pointer[tls.Certificate]

	certFile, keyFile string
	interval          time.Duration
	lastModified      [2]time.Time
	metrics           *metrics
	logger            *slog.Logger
}

// newFileCertSource loads the certificate and key from disk. A failure here
// stops the server: there is no previous certificate to fall back to.
func newFileCertSource(certFile, keyFile string, interval time.Duration, m *metrics, logger *slog.Logger) (*certSource, error) {
	cs := &certSource{certFile: certFile, keyFile: keyFile, interval: interval, metrics: m, logger: logger}
	if err := cs.load(); err != nil {
		return nil, err
	}
	return cs, nil
}

// newStaticCertSource presents one certificate for the life of the process.
func newStaticCertSource(cert tls.Certificate, m *metrics) (*certSource, error) {
	cs := &certSource{metrics: m}
	if err := cs.set(&cert); err != nil {
		return nil, err
	}
	return cs, nil
}

func (cs *certSource) getCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return cs.current.Load(), nil
}

// notAfter is the expiry of the certificate currently presented.
func (cs *certSource) notAfter() time.Time {
	return cs.current.Load().Leaf.NotAfter
}

// watch reloads the certificate whenever either file changes, until ctx is
// done. A file that fails to load is reported and the previous certificate
// stays in use, so a half-written renewal cannot take the server down.
func (cs *certSource) watch(ctx context.Context) {
	if cs.certFile == "" {
		return
	}
	ticker := time.NewTicker(cs.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		changed, err := cs.changed()
		if err != nil {
			cs.metrics.certReloadErrors.Inc()
			cs.logger.Error("EPP server certificate files unreadable; keeping the current certificate", "error", err)
			continue
		}
		if !changed {
			continue
		}
		if err := cs.load(); err != nil {
			cs.metrics.certReloadErrors.Inc()
			cs.logger.Error("EPP server certificate not reloaded; keeping the current certificate", "error", err)
			continue
		}
		cs.logger.Info("EPP server certificate reloaded", "not_after", cs.notAfter().UTC().Format(time.RFC3339))
	}
}

func (cs *certSource) changed() (bool, error) {
	var mod [2]time.Time
	for i, path := range []string{cs.certFile, cs.keyFile} {
		info, err := os.Stat(path)
		if err != nil {
			return false, fmt.Errorf("stat %s: %w", path, err)
		}
		mod[i] = info.ModTime()
	}
	return mod != cs.lastModified, nil
}

func (cs *certSource) load() error {
	var mod [2]time.Time
	for i, path := range []string{cs.certFile, cs.keyFile} {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("stat %s: %w", path, err)
		}
		mod[i] = info.ModTime()
	}
	cert, err := tls.LoadX509KeyPair(cs.certFile, cs.keyFile)
	if err != nil {
		return fmt.Errorf("load server certificate %s: %w", cs.certFile, err)
	}
	if err := cs.set(&cert); err != nil {
		return err
	}
	cs.lastModified = mod
	return nil
}

func (cs *certSource) set(cert *tls.Certificate) error {
	if cert.Leaf == nil {
		leaf, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return fmt.Errorf("parse server certificate: %w", err)
		}
		cert.Leaf = leaf
	}
	cs.current.Store(cert)
	cs.metrics.certNotAfter.Set(float64(cert.Leaf.NotAfter.Unix()))
	return nil
}
