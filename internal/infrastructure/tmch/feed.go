package tmch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// Config is where the Clearinghouse publishes each set and the registry's
// credentials for the two lists (the CA bundle is public). The values are
// deployment settings (ADR 0018: environment, read at the composition
// root); the pilot and production Clearinghouses have different URLs and
// credentials.
type Config struct {
	CertificatesURL string
	SMDRLURL        string
	DNLURL          string
	Username        string
	Password        string
	// Timeout bounds one download; zero means DefaultTimeout.
	Timeout time.Duration
	// MaxBytes bounds one download; zero means DefaultMaxBytes. The DNL list
	// is the largest set, tens of megabytes.
	MaxBytes int64
}

const (
	DefaultTimeout = 5 * time.Minute
	// DefaultMaxBytes bounds one download. The DNL list, the largest set, is
	// tens of megabytes; the body is held in memory while it is parsed.
	DefaultMaxBytes int64 = 64 << 20
)

// The refusals are the domain's sentinels (INV-09).
var (
	ErrNotConfigured = entities.ErrTMCHNotConfigured
	ErrFetch         = entities.ErrTMCHFetch
	ErrTooLarge      = entities.ErrTMCHResponseLarge
)

// Feed downloads the three sets over HTTPS. It implements the application
// layer's TMCH fetcher port.
type Feed struct {
	cfg  Config
	http *http.Client
	now  func() time.Time
}

// NewFeed builds a feed; client may be nil for a default client with the
// configured timeout that follows a redirect only to the same host and
// scheme, so the credentials never go to another host or over plain HTTP.
func NewFeed(cfg Config, client *http.Client) *Feed {
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}
	if cfg.MaxBytes == 0 {
		cfg.MaxBytes = DefaultMaxBytes
	}
	if client == nil {
		client = &http.Client{Timeout: cfg.Timeout, CheckRedirect: sameOrigin}
	}
	return &Feed{cfg: cfg, http: client, now: func() time.Time { return time.Now().UTC() }}
}

// sameOrigin refuses a redirect that leaves the request's host or scheme.
func sameOrigin(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return errors.New("too many redirects")
	}
	first := via[0].URL
	if req.URL.Scheme != first.Scheme || req.URL.Host != first.Host {
		return fmt.Errorf("redirect from %s to %s refused", redacted(first.String()), redacted(req.URL.String()))
	}
	return nil
}

// Configured reports whether the sets can be fetched: every URL and the
// credentials the Clearinghouse requires for the lists. The URLs have
// defaults, so credentials are what makes an installation a TMCH client; a
// deployment without them runs no refresh.
func (f *Feed) Configured() bool {
	return f.cfg.CertificatesURL != "" && f.cfg.SMDRLURL != "" && f.cfg.DNLURL != "" && f.cfg.Username != "" && f.cfg.Password != ""
}

// redacted is a URL as the status, the events and the errors show it:
// without any userinfo it carries.
func redacted(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.User = nil
	return u.String()
}

// utf8BOM is what some tools prepend to a CSV file; the Clearinghouse's
// files have none, but a mirror or a re-save may add it.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// FetchCertificates downloads and parses the CA bundle.
func (f *Feed) FetchCertificates(ctx context.Context) (*entities.TMCHCertificates, error) {
	body, err := f.get(ctx, f.cfg.CertificatesURL, false)
	if err != nil {
		return nil, err
	}
	return entities.NewTMCHCertificates(body, redacted(f.cfg.CertificatesURL), f.now())
}

// FetchSMDRL downloads and parses the SMD revocation list.
func (f *Feed) FetchSMDRL(ctx context.Context) (*entities.SMDRevocationList, error) {
	body, err := f.get(ctx, f.cfg.SMDRLURL, true)
	if err != nil {
		return nil, err
	}
	return ParseSMDRL(bytesReader(body), redacted(f.cfg.SMDRLURL), f.now())
}

// FetchDNL downloads and parses the DNL list.
func (f *Feed) FetchDNL(ctx context.Context) (*entities.DNLList, error) {
	body, err := f.get(ctx, f.cfg.DNLURL, true)
	if err != nil {
		return nil, err
	}
	return ParseDNL(bytesReader(body), redacted(f.cfg.DNLURL), f.now())
}

func (f *Feed) get(ctx context.Context, target string, authenticated bool) ([]byte, error) {
	if target == "" {
		return nil, fmt.Errorf("%w: no URL for this data set", ErrNotConfigured)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFetch, err)
	}
	if authenticated && f.cfg.Username != "" {
		req.SetBasicAuth(f.cfg.Username, f.cfg.Password)
	}
	req.Header.Set("Accept", "text/csv, application/x-pem-file, */*")
	resp, err := f.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFetch, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %s answered %s", ErrFetch, redacted(target), resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, f.cfg.MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFetch, err)
	}
	if int64(len(body)) > f.cfg.MaxBytes {
		return nil, fmt.Errorf("%w: %s", ErrTooLarge, redacted(target))
	}
	return bytes.TrimPrefix(body, utf8BOM), nil
}
