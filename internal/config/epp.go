package config

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
)

// DefaultEPPPort is the IANA-assigned EPP port (RFC 5734).
const DefaultEPPPort = 700

// EPP is the EPP server's configuration, read once at start (INV-17). Every
// setting, its default and how to change it is listed in
// docs/epp/POLICY_REFERENCE.md.
type EPP struct {
	Port     int
	LogLevel slog.Level
	Redis    EPPRedis
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

// LoadEPP reads EPP_PORT, LOG_LEVEL, REDIS_HOST, REDIS_PORT and
// REDIS_PASSWORD. A value that is set but unusable is an error, so a typo
// stops the server at start instead of running it on a default nobody chose.
func LoadEPP() (EPP, error) {
	port, err := parseEPPPort(os.Getenv("EPP_PORT"))
	if err != nil {
		return EPP{}, err
	}
	level, err := parseLogLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		return EPP{}, err
	}
	return EPP{
		Port:     port,
		LogLevel: level,
		Redis: EPPRedis{
			Host:     valueOr(os.Getenv("REDIS_HOST"), "localhost"),
			Port:     valueOr(os.Getenv("REDIS_PORT"), "6379"),
			Password: os.Getenv("REDIS_PASSWORD"),
		},
	}, nil
}

func parseEPPPort(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultEPPPort, nil
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("EPP_PORT=%q is not a TCP port", raw)
	}
	return port, nil
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
