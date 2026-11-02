package config

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"
)

// DefaultEPPPort is the IANA-assigned EPP port (RFC 5734).
const DefaultEPPPort = 700

// DefaultEPPOpsPort serves health, readiness and metrics over plain HTTP.
const DefaultEPPOpsPort = 9700

// EPP is the EPP server's configuration, read once at start (INV-17). Every
// setting, its default and how to change it is listed in
// docs/epp/POLICY_REFERENCE.md.
type EPP struct {
	Port     int
	OpsPort  int
	LogLevel slog.Level
	Redis    EPPRedis
	TLS      EPPTLS
	Proxy    EPPProxy
	Limits   EPPLimits
	Timeouts EPPTimeouts
	Auth     EPPAuth
	Sessions EPPSessions
	Commands EPPCommands
	Audit    EPPAudit
	DB       EPPDatabase
}

// EPPCommands bounds what one command may ask of the registry.
type EPPCommands struct {
	// CreatesInFlight is how many <domain:create>s the node runs at once.
	// Creates waiting for a slot are let in fairly: one registrar (or
	// family) at a time, in turn.
	CreatesInFlight int
	// CreateQueueWait is the longest a create waits for a slot before it
	// is refused with 2400.
	CreateQueueWait time.Duration
}

// EPPAudit is the EPP audit log (docs/epp/ARCHITECTURE.md §6).
type EPPAudit struct {
	// Retention is how long a transaction is kept.
	Retention time.Duration
	// QueryResponses keeps the response frames of queries (<check>, <info>,
	// <poll op="req">) as well as their requests. Transforms always keep
	// both.
	QueryResponses bool
	// Buffer is how many query records may wait to be written in a batch.
	Buffer int
}

// EPPSessions configures the cluster-wide session registry and its caps
// (docs/epp/ARCHITECTURE.md §3). The caps move into tiered policy with the
// policy engine (Phase 5).
type EPPSessions struct {
	// NodeID names this node in the registry. Defaults to the hostname.
	NodeID string
	// Lease is how long a session stays in the registry without a renewal;
	// Heartbeat is how often the node renews its sessions.
	Lease     time.Duration
	Heartbeat time.Duration
	// PerSource caps live sessions from one source address, across every
	// node, before anyone has logged in. The caps per registrar, login and
	// family are the registrars' EPP policies' (ADR 0019).
	PerSource int
}

// EPPAuth governs how logins are authenticated against registrars'
// credentials (ADR 0014).
type EPPAuth struct {
	// RequireAllowlist drops connections from outside every registrar's
	// allowed networks before the TLS handshake, and refuses a login from
	// outside its own registrar's. Only local development turns it off.
	RequireAllowlist bool
	// RefreshInterval is how often the node reloads the union of allowed
	// networks and ends sessions whose credentials were retired as
	// compromised.
	RefreshInterval time.Duration
	// MaxConcurrentVerify bounds password verifications in progress on the
	// node; each Argon2id verification holds 19 MiB.
	MaxConcurrentVerify int
	// MaxLoginAttempts is how many failed logins a session may make before it
	// is closed.
	MaxLoginAttempts int
}

// EPPDatabase locates the database holding registrars' credentials. It reads
// the same DB_* variables as the other services.
type EPPDatabase struct {
	// MaxOpenConns bounds this node's database connections.
	MaxOpenConns int
	User         string
	Pass         string
	Host         string
	Port         string
	Name         string
	SSLMode      string
}

// EPPRedis locates the Redis instance the EPP server keeps its shared
// counters in.
type EPPRedis struct {
	Host     string
	Port     string
	Password string
}

// Addr returns host:port for the Redis client.
func (r EPPRedis) Addr() string {
	return net.JoinHostPort(r.Host, r.Port)
}

// EPPTLS says where the server certificate comes from. Exactly one of a
// certificate/key pair or SelfSigned is in effect.
type EPPTLS struct {
	CertPath       string
	KeyPath        string
	SelfSigned     bool          // local development only
	ReloadInterval time.Duration // how often the files are checked for a new certificate
	// ServerName is the EPP hostname registrars connect to. When set, a
	// certificate from file must cover it (RST epp-01 #9) or it is refused.
	ServerName string
}

// EPPProxy configures PROXY protocol (v1 or v2) on the listener. When
// enabled, every connection must come from a trusted CIDR and must open with
// a PROXY header; anything else is dropped.
type EPPProxy struct {
	Enabled      bool
	TrustedCIDRs []string
}

// EPPLimits are per-node caps that hold without any shared state.
type EPPLimits struct {
	MaxConnections int    // open connections on this node
	MaxHandshakes  int    // TLS handshakes in progress on this node
	MaxFrameBytes  uint32 // RFC 5734 data unit, including its 4-byte header
	XMLMaxDepth    int
	XMLMaxElements int
}

// EPPTimeouts bound each stage of a session.
type EPPTimeouts struct {
	Handshake  time.Duration // TCP accept to completed TLS handshake
	Login      time.Duration // greeting to successful <login>
	Idle       time.Duration // between commands
	Read       time.Duration // reading one frame once it has started
	Write      time.Duration // writing one response
	MaxSession time.Duration // total session lifetime
	Drain      time.Duration // on SIGTERM, time allowed for sessions to finish
}

// eppDefaults are the values used when a variable is unset. They are the
// defaults documented in docs/epp/POLICY_REFERENCE.md.
var eppDefaults = EPP{
	Port:     DefaultEPPPort,
	OpsPort:  DefaultEPPOpsPort,
	LogLevel: slog.LevelInfo,
	Redis:    EPPRedis{Host: "localhost", Port: "6379"},
	TLS:      EPPTLS{ReloadInterval: time.Minute},
	Limits: EPPLimits{
		MaxConnections: 2000,
		MaxHandshakes:  200,
		MaxFrameBytes:  64 * 1024,
		XMLMaxDepth:    32,
		XMLMaxElements: 2000,
	},
	Timeouts: EPPTimeouts{
		Handshake:  10 * time.Second,
		Login:      30 * time.Second,
		Idle:       10 * time.Minute,
		Read:       30 * time.Second,
		Write:      60 * time.Second,
		MaxSession: 24 * time.Hour,
		Drain:      30 * time.Second,
	},
	Auth: EPPAuth{
		RequireAllowlist:    true,
		RefreshInterval:     30 * time.Second,
		MaxConcurrentVerify: 8,
		MaxLoginAttempts:    3,
	},
	Sessions: EPPSessions{
		Lease:     90 * time.Second,
		Heartbeat: 30 * time.Second,
		PerSource: 20,
	},
	Commands: EPPCommands{
		CreatesInFlight: 10,
		CreateQueueWait: 5 * time.Second,
	},
	Audit: EPPAudit{
		Retention: 365 * 24 * time.Hour,
		Buffer:    10000,
	},
	DB: EPPDatabase{MaxOpenConns: 20, User: "postgres", Pass: "postgres", Host: "localhost", Port: "5432", Name: "domain_os", SSLMode: "disable"},
}

// LoadEPP reads the EPP server's environment. A value that is set but
// unusable is an error, so a typo stops the server at start instead of
// running it on a default nobody chose.
func LoadEPP() (EPP, error) {
	cfg := eppDefaults
	var errs []error
	collect := func(err error) {
		if err != nil {
			errs = append(errs, err)
		}
	}

	collect(setPort(&cfg.Port, "EPP_PORT", os.Getenv("EPP_PORT")))
	collect(setPort(&cfg.OpsPort, "EPP_OPS_PORT", os.Getenv("EPP_OPS_PORT")))
	level, err := parseLogLevel(os.Getenv("LOG_LEVEL"))
	collect(err)
	cfg.LogLevel = level

	cfg.Redis.Host = valueOr(os.Getenv("REDIS_HOST"), cfg.Redis.Host)
	cfg.Redis.Port = valueOr(os.Getenv("REDIS_PORT"), cfg.Redis.Port)
	cfg.Redis.Password = os.Getenv("REDIS_PASSWORD")

	cfg.TLS.CertPath = strings.TrimSpace(os.Getenv("EPP_TLS_CERT_PATH"))
	cfg.TLS.KeyPath = strings.TrimSpace(os.Getenv("EPP_TLS_KEY_PATH"))
	cfg.TLS.SelfSigned, err = parseBoolDefaultFalse("EPP_TLS_SELF_SIGNED", os.Getenv("EPP_TLS_SELF_SIGNED"))
	collect(err)
	collect(setDuration(&cfg.TLS.ReloadInterval, "EPP_TLS_RELOAD_INTERVAL", os.Getenv("EPP_TLS_RELOAD_INTERVAL")))
	cfg.TLS.ServerName = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(os.Getenv("EPP_SERVER_NAME")), "."))

	cfg.Proxy.Enabled, err = parseBoolDefaultFalse("EPP_PROXY_PROTOCOL", os.Getenv("EPP_PROXY_PROTOCOL"))
	collect(err)
	cidrs, err := parseCIDRList("EPP_PROXY_TRUSTED_CIDRS", os.Getenv("EPP_PROXY_TRUSTED_CIDRS"))
	collect(err)
	cfg.Proxy.TrustedCIDRs = cidrs

	collect(setPositiveInt(&cfg.Limits.MaxConnections, "EPP_MAX_CONNECTIONS", os.Getenv("EPP_MAX_CONNECTIONS")))
	collect(setPositiveInt(&cfg.Limits.MaxHandshakes, "EPP_MAX_HANDSHAKES", os.Getenv("EPP_MAX_HANDSHAKES")))
	collect(setFrameBytes(&cfg.Limits.MaxFrameBytes, os.Getenv("EPP_MAX_FRAME_BYTES")))
	collect(setPositiveInt(&cfg.Limits.XMLMaxDepth, "EPP_XML_MAX_DEPTH", os.Getenv("EPP_XML_MAX_DEPTH")))
	collect(setPositiveInt(&cfg.Limits.XMLMaxElements, "EPP_XML_MAX_ELEMENTS", os.Getenv("EPP_XML_MAX_ELEMENTS")))

	collect(setDuration(&cfg.Timeouts.Handshake, "EPP_HANDSHAKE_TIMEOUT", os.Getenv("EPP_HANDSHAKE_TIMEOUT")))
	collect(setDuration(&cfg.Timeouts.Login, "EPP_LOGIN_TIMEOUT", os.Getenv("EPP_LOGIN_TIMEOUT")))
	collect(setDuration(&cfg.Timeouts.Idle, "EPP_IDLE_TIMEOUT", os.Getenv("EPP_IDLE_TIMEOUT")))
	collect(setDuration(&cfg.Timeouts.Read, "EPP_READ_TIMEOUT", os.Getenv("EPP_READ_TIMEOUT")))
	collect(setDuration(&cfg.Timeouts.Write, "EPP_WRITE_TIMEOUT", os.Getenv("EPP_WRITE_TIMEOUT")))
	collect(setDuration(&cfg.Timeouts.MaxSession, "EPP_MAX_SESSION_DURATION", os.Getenv("EPP_MAX_SESSION_DURATION")))
	collect(setDuration(&cfg.Timeouts.Drain, "EPP_DRAIN_TIMEOUT", os.Getenv("EPP_DRAIN_TIMEOUT")))

	cfg.Auth.RequireAllowlist, err = parseBoolDefault("EPP_REQUIRE_ALLOWLIST", os.Getenv("EPP_REQUIRE_ALLOWLIST"), cfg.Auth.RequireAllowlist)
	collect(err)
	collect(setDuration(&cfg.Auth.RefreshInterval, "EPP_CREDENTIAL_REFRESH_INTERVAL", os.Getenv("EPP_CREDENTIAL_REFRESH_INTERVAL")))
	collect(setPositiveInt(&cfg.Auth.MaxConcurrentVerify, "EPP_MAX_CONCURRENT_LOGINS", os.Getenv("EPP_MAX_CONCURRENT_LOGINS")))
	collect(setPositiveInt(&cfg.Auth.MaxLoginAttempts, "EPP_MAX_LOGIN_ATTEMPTS", os.Getenv("EPP_MAX_LOGIN_ATTEMPTS")))

	cfg.Sessions.NodeID = strings.TrimSpace(os.Getenv("EPP_NODE_ID"))
	if cfg.Sessions.NodeID == "" {
		cfg.Sessions.NodeID, _ = os.Hostname()
	}
	collect(setDuration(&cfg.Sessions.Lease, "EPP_SESSION_LEASE", os.Getenv("EPP_SESSION_LEASE")))
	collect(setDuration(&cfg.Sessions.Heartbeat, "EPP_SESSION_HEARTBEAT", os.Getenv("EPP_SESSION_HEARTBEAT")))
	collect(setPositiveInt(&cfg.Sessions.PerSource, "EPP_MAX_SESSIONS_PER_SOURCE_IP", os.Getenv("EPP_MAX_SESSIONS_PER_SOURCE_IP")))

	collect(setPositiveInt(&cfg.Commands.CreatesInFlight, "EPP_MAX_CREATES_IN_FLIGHT", os.Getenv("EPP_MAX_CREATES_IN_FLIGHT")))
	collect(setDuration(&cfg.Commands.CreateQueueWait, "EPP_CREATE_QUEUE_WAIT", os.Getenv("EPP_CREATE_QUEUE_WAIT")))

	collect(setDuration(&cfg.Audit.Retention, "EPP_AUDIT_RETENTION", os.Getenv("EPP_AUDIT_RETENTION")))
	cfg.Audit.QueryResponses, err = parseBoolDefaultFalse("EPP_AUDIT_QUERY_RESPONSES", os.Getenv("EPP_AUDIT_QUERY_RESPONSES"))
	collect(err)
	collect(setPositiveInt(&cfg.Audit.Buffer, "EPP_AUDIT_BUFFER", os.Getenv("EPP_AUDIT_BUFFER")))

	collect(setPositiveInt(&cfg.DB.MaxOpenConns, "EPP_DB_MAX_OPEN_CONNS", os.Getenv("EPP_DB_MAX_OPEN_CONNS")))
	cfg.DB.User = valueOr(os.Getenv("DB_USER"), cfg.DB.User)
	cfg.DB.Pass = valueOr(os.Getenv("DB_PASS"), cfg.DB.Pass)
	cfg.DB.Host = valueOr(os.Getenv("DB_HOST"), cfg.DB.Host)
	cfg.DB.Port = valueOr(os.Getenv("DB_PORT"), cfg.DB.Port)
	cfg.DB.Name = valueOr(os.Getenv("DB_NAME"), cfg.DB.Name)
	cfg.DB.SSLMode = valueOr(os.Getenv("DB_SSLMODE"), cfg.DB.SSLMode)

	collect(cfg.validate())
	if len(errs) > 0 {
		return EPP{}, errors.Join(errs...)
	}
	return cfg, nil
}

// validate checks the rules that span more than one variable.
func (c EPP) validate() error {
	var errs []error
	hasFiles := c.TLS.CertPath != "" || c.TLS.KeyPath != ""
	switch {
	case c.TLS.SelfSigned && hasFiles:
		errs = append(errs, errors.New("EPP_TLS_SELF_SIGNED=true conflicts with EPP_TLS_CERT_PATH/EPP_TLS_KEY_PATH; set one or the other"))
	case !c.TLS.SelfSigned && (c.TLS.CertPath == "" || c.TLS.KeyPath == ""):
		errs = append(errs, errors.New("EPP_TLS_CERT_PATH and EPP_TLS_KEY_PATH are both required, unless EPP_TLS_SELF_SIGNED=true (local development only)"))
	}
	if c.Proxy.Enabled && len(c.Proxy.TrustedCIDRs) == 0 {
		errs = append(errs, errors.New("EPP_PROXY_PROTOCOL=true requires EPP_PROXY_TRUSTED_CIDRS: a PROXY header is only believed from the load balancer"))
	}
	if c.Port == c.OpsPort {
		errs = append(errs, fmt.Errorf("EPP_PORT and EPP_OPS_PORT are both %d", c.Port))
	}
	if c.Sessions.Heartbeat*2 > c.Sessions.Lease {
		errs = append(errs, fmt.Errorf("EPP_SESSION_HEARTBEAT (%s) must be at most half of EPP_SESSION_LEASE (%s), so one missed renewal does not drop a live session", c.Sessions.Heartbeat, c.Sessions.Lease))
	}
	if c.Sessions.NodeID == "" {
		errs = append(errs, errors.New("EPP_NODE_ID is unset and the hostname is unknown; set EPP_NODE_ID"))
	}
	if c.Commands.CreatesInFlight >= c.DB.MaxOpenConns {
		errs = append(errs, fmt.Errorf("EPP_MAX_CREATES_IN_FLIGHT (%d) must be below EPP_DB_MAX_OPEN_CONNS (%d), so creates cannot take every connection", c.Commands.CreatesInFlight, c.DB.MaxOpenConns))
	}
	if c.Limits.MaxHandshakes > c.Limits.MaxConnections {
		errs = append(errs, fmt.Errorf("EPP_MAX_HANDSHAKES (%d) exceeds EPP_MAX_CONNECTIONS (%d)", c.Limits.MaxHandshakes, c.Limits.MaxConnections))
	}
	return errors.Join(errs...)
}

// parseBoolDefault reads a boolean, with def when unset.
func parseBoolDefault(name, raw string, def bool) (bool, error) {
	if strings.TrimSpace(raw) == "" {
		return def, nil
	}
	return parseBoolDefaultFalse(name, raw)
}

func setPort(dst *int, name, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("%s=%q is not a TCP port", name, raw)
	}
	*dst = port
	return nil
}

func setPositiveInt(dst *int, name, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return fmt.Errorf("%s=%q is not a positive integer", name, raw)
	}
	*dst = v
	return nil
}

// setFrameBytes reads EPP_MAX_FRAME_BYTES. The floor is generous enough for
// a login with every standard object and extension URI; anything smaller
// would refuse ordinary sessions.
func setFrameBytes(dst *uint32, raw string) error {
	const floor = 4 * 1024
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	v, err := strconv.ParseUint(raw, 10, 32)
	if err != nil || v < floor || v > math.MaxUint32 {
		return fmt.Errorf("EPP_MAX_FRAME_BYTES=%q is not a byte count between %d and %d", raw, floor, uint32(math.MaxUint32))
	}
	*dst = uint32(v)
	return nil
}

func setDuration(dst *time.Duration, name, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return fmt.Errorf("%s=%q is not a positive duration (e.g. 30s, 10m)", name, raw)
	}
	*dst = d
	return nil
}

// parseCIDRList reads a comma-separated list of CIDRs or single addresses.
func parseCIDRList(name, raw string) ([]string, error) {
	var out []string
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, err := netip.ParsePrefix(item); err != nil {
			if _, addrErr := netip.ParseAddr(item); addrErr != nil {
				return nil, fmt.Errorf("%s: %q is neither a CIDR nor an IP address", name, item)
			}
		}
		out = append(out, item)
	}
	return out, nil
}

// parseLogLevel accepts debug, info, warn (or warning) and error in any case.
// Unset is info.
func parseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("LOG_LEVEL=%q is not one of debug, info, warn, error", raw)
	}
}

func valueOr(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
