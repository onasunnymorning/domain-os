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
	if c.Limits.MaxHandshakes > c.Limits.MaxConnections {
		errs = append(errs, fmt.Errorf("EPP_MAX_HANDSHAKES (%d) exceeds EPP_MAX_CONNECTIONS (%d)", c.Limits.MaxHandshakes, c.Limits.MaxConnections))
	}
	return errors.Join(errs...)
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
