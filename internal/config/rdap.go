package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"
)

// DefaultRDAPPort is HTTPS. RST requires RDAP base URLs on port 443.
const DefaultRDAPPort = 443

// DefaultRDAPOpsPort serves health, readiness and metrics over plain HTTP.
const DefaultRDAPOpsPort = 9701

// RDAP is the RDAP server's configuration, read once at start (INV-17).
// Every setting, its default and how to change it is listed in
// docs/rdap/POLICY_REFERENCE.md.
type RDAP struct {
	Port     int
	OpsPort  int
	LogLevel slog.Level
	// BaseURL is the public base every TLD is served under, ending in "/":
	// a TLD's base URL is BaseURL + tld + "/". Defaults to
	// https://ServerName/.
	BaseURL  string
	TLS      RDAPTLS
	Proxy    RDAPProxy
	Timeouts RDAPTimeouts
	Notices  RDAPNotices
	DB       RDAPDatabase
	// PolicyRefresh is how often the node reloads the publication and
	// access policies.
	PolicyRefresh time.Duration
	Limits        RDAPLimits
	Redis         EPPRedis
}

// RDAPLimits protect the node, whoever is asking (ADR 0022 decision 7).
// The limits on clients are access policies, in the database.
type RDAPLimits struct {
	// MaxInFlight is how many requests the node serves at once. More are
	// answered 503 at once rather than queued.
	MaxInFlight int
	// MaxConnections is how many connections the node holds open. More
	// are closed at accept.
	MaxConnections int
}

// RDAPTLS says where the server certificate comes from. Exactly one of a
// certificate/key pair or SelfSigned is in effect.
type RDAPTLS struct {
	CertPath       string
	KeyPath        string
	SelfSigned     bool          // local development only
	ReloadInterval time.Duration // how often the files are checked for a new certificate
	// ServerName is the RDAP hostname. When set, a certificate from file
	// must cover it (rdap-91) or it is refused.
	ServerName string
}

// RDAPProxy configures PROXY protocol on the listener, as for EPP.
type RDAPProxy struct {
	Enabled      bool
	TrustedCIDRs []string
}

// RDAPTimeouts bound each request and the drain.
type RDAPTimeouts struct {
	Read  time.Duration // reading a request, headers and any PROXY header included
	Write time.Duration // writing a response
	Idle  time.Duration // a keep-alive connection waiting for its next request
	Drain time.Duration // on SIGTERM, time allowed for requests in flight
}

// RDAPNotices are the links the gTLD RDAP Profile's notices carry. They
// are the RDAP service's own, the same for every TLD.
type RDAPNotices struct {
	// TermsOfServiceURL is the registry's RDAP terms of use. Empty leaves
	// the "Terms of Use" notice out, which the gTLD profile does not allow.
	TermsOfServiceURL string
}

// RDAPDatabase locates the registry database. It reads the same DB_*
// variables as the other services.
type RDAPDatabase struct {
	MaxOpenConns int
	User         string
	Pass         string
	Host         string
	Port         string
	Name         string
	SSLMode      string
}

// rdapDefaults are the values used when a variable is unset. They are the
// defaults documented in docs/rdap/POLICY_REFERENCE.md.
var rdapDefaults = RDAP{
	Port:     DefaultRDAPPort,
	OpsPort:  DefaultRDAPOpsPort,
	LogLevel: slog.LevelInfo,
	TLS:      RDAPTLS{ReloadInterval: time.Minute},
	Timeouts: RDAPTimeouts{
		Read:  5 * time.Second,
		Write: 10 * time.Second,
		Idle:  60 * time.Second,
		Drain: 20 * time.Second,
	},
	PolicyRefresh: 30 * time.Second,
	Limits:        RDAPLimits{MaxInFlight: 200, MaxConnections: 5000},
	Redis:         EPPRedis{Host: "localhost", Port: "6379"},
	DB:            RDAPDatabase{MaxOpenConns: 20, User: "postgres", Pass: "postgres", Host: "localhost", Port: "5432", Name: "domain_os", SSLMode: "disable"},
}

// LoadRDAP reads the RDAP server's environment. A value that is set but
// unusable is an error, so a typo stops the server at start instead of
// running it on a default nobody chose.
func LoadRDAP() (RDAP, error) {
	cfg := rdapDefaults
	var errs []error
	collect := func(err error) {
		if err != nil {
			errs = append(errs, err)
		}
	}

	collect(setPort(&cfg.Port, "RDAP_PORT", os.Getenv("RDAP_PORT")))
	collect(setPort(&cfg.OpsPort, "RDAP_OPS_PORT", os.Getenv("RDAP_OPS_PORT")))
	level, err := parseLogLevel(os.Getenv("LOG_LEVEL"))
	collect(err)
	cfg.LogLevel = level

	cfg.TLS.CertPath = strings.TrimSpace(os.Getenv("RDAP_TLS_CERT_PATH"))
	cfg.TLS.KeyPath = strings.TrimSpace(os.Getenv("RDAP_TLS_KEY_PATH"))
	cfg.TLS.SelfSigned, err = parseBoolDefaultFalse("RDAP_TLS_SELF_SIGNED", os.Getenv("RDAP_TLS_SELF_SIGNED"))
	collect(err)
	collect(setDuration(&cfg.TLS.ReloadInterval, "RDAP_TLS_RELOAD_INTERVAL", os.Getenv("RDAP_TLS_RELOAD_INTERVAL")))
	cfg.TLS.ServerName = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(os.Getenv("RDAP_SERVER_NAME")), "."))

	cfg.Proxy.Enabled, err = parseBoolDefaultFalse("RDAP_PROXY_PROTOCOL", os.Getenv("RDAP_PROXY_PROTOCOL"))
	collect(err)
	cidrs, err := parseCIDRList("RDAP_PROXY_TRUSTED_CIDRS", os.Getenv("RDAP_PROXY_TRUSTED_CIDRS"))
	collect(err)
	cfg.Proxy.TrustedCIDRs = cidrs

	collect(setDuration(&cfg.Timeouts.Read, "RDAP_READ_TIMEOUT", os.Getenv("RDAP_READ_TIMEOUT")))
	collect(setDuration(&cfg.Timeouts.Write, "RDAP_WRITE_TIMEOUT", os.Getenv("RDAP_WRITE_TIMEOUT")))
	collect(setDuration(&cfg.Timeouts.Idle, "RDAP_IDLE_TIMEOUT", os.Getenv("RDAP_IDLE_TIMEOUT")))
	collect(setDuration(&cfg.Timeouts.Drain, "RDAP_DRAIN_TIMEOUT", os.Getenv("RDAP_DRAIN_TIMEOUT")))
	collect(setDuration(&cfg.PolicyRefresh, "RDAP_POLICY_REFRESH_INTERVAL", os.Getenv("RDAP_POLICY_REFRESH_INTERVAL")))
	collect(setPositiveInt(&cfg.Limits.MaxInFlight, "RDAP_MAX_IN_FLIGHT", os.Getenv("RDAP_MAX_IN_FLIGHT")))
	collect(setPositiveInt(&cfg.Limits.MaxConnections, "RDAP_MAX_CONNECTIONS", os.Getenv("RDAP_MAX_CONNECTIONS")))
	cfg.Redis.Host = valueOr(os.Getenv("REDIS_HOST"), cfg.Redis.Host)
	cfg.Redis.Port = valueOr(os.Getenv("REDIS_PORT"), cfg.Redis.Port)
	cfg.Redis.Password = os.Getenv("REDIS_PASSWORD")

	cfg.BaseURL, err = parseRDAPBaseURL(os.Getenv("RDAP_BASE_URL"), cfg.TLS.ServerName)
	collect(err)
	cfg.Notices.TermsOfServiceURL, err = parseOptionalHTTPSURL("RDAP_TERMS_OF_SERVICE_URL", os.Getenv("RDAP_TERMS_OF_SERVICE_URL"))
	collect(err)

	collect(setPositiveInt(&cfg.DB.MaxOpenConns, "RDAP_DB_MAX_OPEN_CONNS", os.Getenv("RDAP_DB_MAX_OPEN_CONNS")))
	cfg.DB.User = valueOr(os.Getenv("DB_USER"), cfg.DB.User)
	cfg.DB.Pass = valueOr(os.Getenv("DB_PASS"), cfg.DB.Pass)
	cfg.DB.Host = valueOr(os.Getenv("DB_HOST"), cfg.DB.Host)
	cfg.DB.Port = valueOr(os.Getenv("DB_PORT"), cfg.DB.Port)
	cfg.DB.Name = valueOr(os.Getenv("DB_NAME"), cfg.DB.Name)
	cfg.DB.SSLMode = valueOr(os.Getenv("DB_SSLMODE"), cfg.DB.SSLMode)

	collect(cfg.validate())
	if len(errs) > 0 {
		return RDAP{}, errors.Join(errs...)
	}
	return cfg, nil
}

// validate checks the rules that span more than one variable.
func (c RDAP) validate() error {
	var errs []error
	hasFiles := c.TLS.CertPath != "" || c.TLS.KeyPath != ""
	switch {
	case c.TLS.SelfSigned && hasFiles:
		errs = append(errs, errors.New("RDAP_TLS_SELF_SIGNED=true conflicts with RDAP_TLS_CERT_PATH/RDAP_TLS_KEY_PATH; set one or the other"))
	case !c.TLS.SelfSigned && (c.TLS.CertPath == "" || c.TLS.KeyPath == ""):
		errs = append(errs, errors.New("RDAP_TLS_CERT_PATH and RDAP_TLS_KEY_PATH are both required, unless RDAP_TLS_SELF_SIGNED=true (local development only)"))
	}
	if c.Proxy.Enabled && len(c.Proxy.TrustedCIDRs) == 0 {
		errs = append(errs, errors.New("RDAP_PROXY_PROTOCOL=true requires RDAP_PROXY_TRUSTED_CIDRS: a PROXY header is only believed from the load balancer"))
	}
	if c.Port == c.OpsPort {
		errs = append(errs, fmt.Errorf("RDAP_PORT and RDAP_OPS_PORT are both %d", c.Port))
	}
	if c.Limits.MaxInFlight > c.Limits.MaxConnections {
		errs = append(errs, fmt.Errorf("RDAP_MAX_IN_FLIGHT (%d) exceeds RDAP_MAX_CONNECTIONS (%d)", c.Limits.MaxInFlight, c.Limits.MaxConnections))
	}
	return errors.Join(errs...)
}

// parseRDAPBaseURL reads RDAP_BASE_URL, or derives it from the server name.
// It must be an absolute https URL with no query or fragment, and is
// returned ending in "/" (RST's rdap.baseURLs require the trailing slash).
func parseRDAPBaseURL(raw, serverName string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if serverName == "" {
			return "", errors.New("RDAP_BASE_URL is unset and so is RDAP_SERVER_NAME to derive it from; set one")
		}
		return "https://" + serverName + "/", nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("RDAP_BASE_URL=%q is not an absolute https URL without query or fragment", raw)
	}
	if !strings.HasSuffix(raw, "/") {
		raw += "/"
	}
	return raw, nil
}

func parseOptionalHTTPSURL(name, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return "", fmt.Errorf("%s=%q is not an absolute https URL", name, raw)
	}
	return raw, nil
}
