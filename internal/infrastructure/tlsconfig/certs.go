package tlsconfig

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"time"
)

// Hooks report a certificate source's state to its server's metrics. Either
// may be nil.
type Hooks struct {
	// Loaded is called with the expiry of each certificate put in use.
	Loaded func(notAfter time.Time)
	// ReloadFailed is called when a renewed certificate could not be read
	// or was refused, and the current one stays in use.
	ReloadFailed func()
}

// FileOptions say where a certificate source reads from and what it refuses.
type FileOptions struct {
	CertPath, KeyPath string
	// ServerName, when set, must be covered by every certificate loaded.
	ServerName string
	// ServerNameSetting names the setting ServerName came from, for errors
	// (e.g. EPP_SERVER_NAME).
	ServerNameSetting string
	// Interval is how often the files are checked for a renewal.
	Interval time.Duration
	// Service names the server in log messages ("EPP", "RDAP").
	Service string
	Logger  *slog.Logger
	Hooks   Hooks
}

// CertSource supplies the server certificate for each handshake. Swapping
// the certificate affects new handshakes only; open connections keep the one
// they negotiated.
type CertSource struct {
	current atomic.Pointer[tls.Certificate]

	opts         FileOptions
	lastModified [2]time.Time
}

// NewFileCertSource loads the certificate and key from disk. A failure here
// should stop the server: there is no previous certificate to fall back to.
//
// A certificate is refused, at start and on reload, if it is outside its
// validity window or, when ServerName is set, does not cover it (RST epp-01
// #7 and #9, rdap-91): better a server that will not start, or keeps its
// current certificate, than one presenting a certificate every client will
// reject.
func NewFileCertSource(opts FileOptions) (*CertSource, error) {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Service == "" {
		opts.Service = "TLS"
	}
	cs := &CertSource{opts: opts}
	if err := cs.load(); err != nil {
		return nil, err
	}
	return cs, nil
}

// NewStaticCertSource presents one certificate for the life of the process.
func NewStaticCertSource(cert tls.Certificate, hooks Hooks) (*CertSource, error) {
	cs := &CertSource{opts: FileOptions{Hooks: hooks}}
	if err := cs.set(&cert); err != nil {
		return nil, err
	}
	return cs, nil
}

// GetCertificate is tls.Config.GetCertificate.
func (cs *CertSource) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return cs.current.Load(), nil
}

// NotAfter is the expiry of the certificate currently presented.
func (cs *CertSource) NotAfter() time.Time {
	return cs.current.Load().Leaf.NotAfter
}

// Watch reloads the certificate whenever either file changes, until ctx is
// done. A file that fails to load is reported and the previous certificate
// stays in use, so a half-written renewal cannot take the server down.
func (cs *CertSource) Watch(ctx context.Context) {
	if cs.opts.CertPath == "" {
		return
	}
	ticker := time.NewTicker(cs.opts.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		changed, err := cs.changed()
		if err != nil {
			cs.reloadFailed()
			cs.opts.Logger.Error(cs.opts.Service+" server certificate files unreadable; keeping the current certificate", "error", err)
			continue
		}
		if !changed {
			continue
		}
		if err := cs.load(); err != nil {
			cs.reloadFailed()
			cs.opts.Logger.Error(cs.opts.Service+" server certificate not reloaded; keeping the current certificate", "error", err)
			continue
		}
		cs.opts.Logger.Info(cs.opts.Service+" server certificate reloaded", "not_after", cs.NotAfter().UTC().Format(time.RFC3339))
	}
}

func (cs *CertSource) reloadFailed() {
	if cs.opts.Hooks.ReloadFailed != nil {
		cs.opts.Hooks.ReloadFailed()
	}
}

func (cs *CertSource) modTimes() ([2]time.Time, error) {
	var mod [2]time.Time
	for i, path := range []string{cs.opts.CertPath, cs.opts.KeyPath} {
		info, err := os.Stat(path)
		if err != nil {
			return mod, fmt.Errorf("stat %s: %w", path, err)
		}
		mod[i] = info.ModTime()
	}
	return mod, nil
}

func (cs *CertSource) changed() (bool, error) {
	mod, err := cs.modTimes()
	if err != nil {
		return false, err
	}
	return mod != cs.lastModified, nil
}

func (cs *CertSource) load() error {
	mod, err := cs.modTimes()
	if err != nil {
		return err
	}
	cert, err := tls.LoadX509KeyPair(cs.opts.CertPath, cs.opts.KeyPath)
	if err != nil {
		return fmt.Errorf("load server certificate %s: %w", cs.opts.CertPath, err)
	}
	if err := cs.check(cert.Leaf, time.Now()); err != nil {
		return fmt.Errorf("server certificate %s: %w", cs.opts.CertPath, err)
	}
	if err := cs.set(&cert); err != nil {
		return err
	}
	cs.lastModified = mod
	return nil
}

func (cs *CertSource) set(cert *tls.Certificate) error {
	if cert.Leaf == nil {
		leaf, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return fmt.Errorf("parse server certificate: %w", err)
		}
		cert.Leaf = leaf
	}
	cs.current.Store(cert)
	if cs.opts.Hooks.Loaded != nil {
		cs.opts.Hooks.Loaded(cert.Leaf.NotAfter)
	}
	return nil
}

// check refuses a certificate no client should accept.
func (cs *CertSource) check(leaf *x509.Certificate, now time.Time) error {
	if leaf == nil {
		return errors.New("no leaf certificate")
	}
	if now.Before(leaf.NotBefore) {
		return fmt.Errorf("not valid until %s", leaf.NotBefore.UTC().Format(time.RFC3339))
	}
	if now.After(leaf.NotAfter) {
		return fmt.Errorf("expired %s", leaf.NotAfter.UTC().Format(time.RFC3339))
	}
	if cs.opts.ServerName != "" {
		if err := leaf.VerifyHostname(cs.opts.ServerName); err != nil {
			return fmt.Errorf("does not cover %s %q: %w", cs.opts.ServerNameSetting, cs.opts.ServerName, err)
		}
	}
	return nil
}
