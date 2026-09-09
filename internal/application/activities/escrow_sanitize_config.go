package activities

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/rdesanitize"
)

// loadEscrowSanitizeLimits reads the sanitisation limits from the environment.
// Every variable is registered in internal/config/env_registry.go with the same
// default.
func loadEscrowSanitizeLimits() (rdesanitize.Limits, error) {
	lim := rdesanitize.DefaultLimits()
	depth, err := envInt64("ESCROW_SANITIZE_MAX_XML_DEPTH", os.Getenv("ESCROW_SANITIZE_MAX_XML_DEPTH"), int64(lim.MaxXMLDepth))
	if err != nil {
		return lim, err
	}
	lim.MaxXMLDepth = int(depth)
	if lim.MaxElements, err = envInt64("ESCROW_SANITIZE_MAX_ELEMENTS", os.Getenv("ESCROW_SANITIZE_MAX_ELEMENTS"), lim.MaxElements); err != nil {
		return lim, err
	}
	field, err := envInt64("ESCROW_SANITIZE_MAX_FIELD_BYTES", os.Getenv("ESCROW_SANITIZE_MAX_FIELD_BYTES"), int64(lim.MaxFieldBytes))
	if err != nil {
		return lim, err
	}
	lim.MaxFieldBytes = int(field)
	if raw := strings.TrimSpace(os.Getenv("ESCROW_SANITIZE_TIMEOUT")); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return lim, fmt.Errorf("ESCROW_SANITIZE_TIMEOUT: %w", err)
		}
		lim.Timeout = d
	}
	if err := lim.Validate(); err != nil {
		return lim, err
	}
	return lim, nil
}

// escrowSanitizeSuffix returns the configured synthetic registry suffix.
func escrowSanitizeSuffix() string {
	if v := strings.TrimSpace(os.Getenv("ESCROW_SANITIZE_SUFFIX")); v != "" {
		return v
	}
	return "artful-dodger"
}
