package tmch

import (
	"context"
	"fmt"
	"io"
	"net/http"
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
	DefaultTimeout        = 5 * time.Minute
	DefaultMaxBytes int64 = 512 << 20
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
// configured timeout.
func NewFeed(cfg Config, client *http.Client) *Feed {
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}
	if cfg.MaxBytes == 0 {
		cfg.MaxBytes = DefaultMaxBytes
	}
	if client == nil {
		client = &http.Client{Timeout: cfg.Timeout}
	}
	return &Feed{cfg: cfg, http: client, now: func() time.Time { return time.Now().UTC() }}
}

// Configured reports whether every URL is set; a deployment without TMCH
// credentials runs no refresh.
func (f *Feed) Configured() bool {
	return f.cfg.CertificatesURL != "" && f.cfg.SMDRLURL != "" && f.cfg.DNLURL != ""
}

// FetchCertificates downloads and parses the CA bundle.
func (f *Feed) FetchCertificates(ctx context.Context) (*entities.TMCHCertificates, error) {
	body, err := f.get(ctx, f.cfg.CertificatesURL, false)
	if err != nil {
		return nil, err
	}
	return entities.NewTMCHCertificates(body, f.cfg.CertificatesURL, f.now())
}

// FetchSMDRL downloads and parses the SMD revocation list.
func (f *Feed) FetchSMDRL(ctx context.Context) (*entities.SMDRevocationList, error) {
	body, err := f.get(ctx, f.cfg.SMDRLURL, true)
	if err != nil {
		return nil, err
	}
	return ParseSMDRL(bytesReader(body), f.cfg.SMDRLURL, f.now())
}

// FetchDNL downloads and parses the DNL list.
func (f *Feed) FetchDNL(ctx context.Context) (*entities.DNLList, error) {
	body, err := f.get(ctx, f.cfg.DNLURL, true)
	if err != nil {
		return nil, err
	}
	return ParseDNL(bytesReader(body), f.cfg.DNLURL, f.now())
}

func (f *Feed) get(ctx context.Context, url string, authenticated bool) ([]byte, error) {
	if url == "" {
		return nil, fmt.Errorf("%w: no URL for this data set", ErrNotConfigured)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
		return nil, fmt.Errorf("%w: %s answered %s", ErrFetch, url, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, f.cfg.MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFetch, err)
	}
	if int64(len(body)) > f.cfg.MaxBytes {
		return nil, fmt.Errorf("%w: %s", ErrTooLarge, url)
	}
	return body, nil
}
