package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// TMCH is the Trademark Clearinghouse feed (ADR 0024 §4, roadmap 9a), read
// once at start (INV-17). The URLs default to the production Clearinghouse;
// the pilot environment publishes the same files at other URLs, so a test
// deployment sets all three. The lists need the registry's TMDB
// credentials; the CA bundle is public. Without credentials the feed is
// not configured and the refresh schedule finds nothing to do.
type TMCH struct {
	CertificatesURL string
	SMDRLURL        string
	DNLURL          string
	Username        string
	Password        string
	RefreshInterval time.Duration
}

const (
	DefaultTMCHCertificatesURL = "https://ca.icann.org/tmch.crt"
	DefaultTMCHSMDRLURL        = "https://ry.marksdb.org/smdrl/smdrl-latest.csv"
	DefaultTMCHDNLURL          = "https://ry.marksdb.org/dnl/dnl-latest.csv"
	// DefaultTMCHRefreshInterval is how often the lists are fetched when
	// TMCH_REFRESH_INTERVAL is unset or invalid. The Clearinghouse
	// regenerates both lists at least daily; fetching every six hours keeps
	// the stored copy within hours of a revocation.
	DefaultTMCHRefreshInterval = 6 * time.Hour
	// MinTMCHRefreshInterval is the shortest interval accepted: the
	// Clearinghouse asks registries not to poll more often than hourly.
	MinTMCHRefreshInterval = time.Hour
)

// LoadTMCH reads TMCH_CA_URL, TMCH_SMDRL_URL, TMCH_DNL_URL, TMCH_USERNAME,
// TMCH_PASSWORD and TMCH_REFRESH_INTERVAL. The error, if any, says why a set
// interval was not used as written; the returned configuration is always
// usable.
func LoadTMCH() (TMCH, error) {
	cfg := TMCH{
		CertificatesURL: envOr("TMCH_CA_URL", DefaultTMCHCertificatesURL),
		SMDRLURL:        envOr("TMCH_SMDRL_URL", DefaultTMCHSMDRLURL),
		DNLURL:          envOr("TMCH_DNL_URL", DefaultTMCHDNLURL),
		Username:        strings.TrimSpace(os.Getenv("TMCH_USERNAME")),
		Password:        os.Getenv("TMCH_PASSWORD"),
	}
	interval, err := parseTMCHRefreshInterval(os.Getenv("TMCH_REFRESH_INTERVAL"))
	cfg.RefreshInterval = interval
	return cfg, err
}

// Configured reports whether the lists can be fetched: the Clearinghouse
// answers them only with credentials.
func (c TMCH) Configured() bool {
	return c.CertificatesURL != "" && c.SMDRLURL != "" && c.DNLURL != "" && c.Username != "" && c.Password != ""
}

func envOr(name, def string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return def
}

func parseTMCHRefreshInterval(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultTMCHRefreshInterval, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return DefaultTMCHRefreshInterval, fmt.Errorf("TMCH_REFRESH_INTERVAL=%q is not a positive duration; using %s", raw, DefaultTMCHRefreshInterval)
	}
	if d < MinTMCHRefreshInterval {
		return MinTMCHRefreshInterval, fmt.Errorf("TMCH_REFRESH_INTERVAL=%q is below the minimum; using %s", raw, MinTMCHRefreshInterval)
	}
	return d, nil
}
