package activities

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate"
)

// loadEscrowValidationLimits reads the resource limits from the environment
// (issue #412, design constraint 7: limits are configuration). Every variable
// is registered in internal/config/env_registry.go with the same default.
func loadEscrowValidationLimits() (rdevalidate.Limits, error) {
	lim := rdevalidate.DefaultLimits()
	var err error
	if lim.MaxCompressedBytes, err = envInt64("ESCROW_VALIDATION_MAX_COMPRESSED_BYTES", os.Getenv("ESCROW_VALIDATION_MAX_COMPRESSED_BYTES"), lim.MaxCompressedBytes); err != nil {
		return lim, err
	}
	if lim.MaxUnpackedBytes, err = envInt64("ESCROW_VALIDATION_MAX_UNPACKED_BYTES", os.Getenv("ESCROW_VALIDATION_MAX_UNPACKED_BYTES"), lim.MaxUnpackedBytes); err != nil {
		return lim, err
	}
	maxFiles, err := envInt64("ESCROW_VALIDATION_MAX_FILES", os.Getenv("ESCROW_VALIDATION_MAX_FILES"), int64(lim.MaxFiles))
	if err != nil {
		return lim, err
	}
	lim.MaxFiles = int(maxFiles)
	maxNesting, err := envInt64("ESCROW_VALIDATION_MAX_NESTING", os.Getenv("ESCROW_VALIDATION_MAX_NESTING"), int64(lim.MaxNesting))
	if err != nil {
		return lim, err
	}
	lim.MaxNesting = int(maxNesting)
	maxXrefObjects, err := envInt64("ESCROW_VALIDATION_MAX_CROSS_REFERENCE_OBJECTS", os.Getenv("ESCROW_VALIDATION_MAX_CROSS_REFERENCE_OBJECTS"), int64(lim.MaxCrossReferenceObjects))
	if err != nil {
		return lim, err
	}
	lim.MaxCrossReferenceObjects = int(maxXrefObjects)
	maxXrefNames, err := envInt64("ESCROW_VALIDATION_MAX_CROSS_REFERENCE_NAMES", os.Getenv("ESCROW_VALIDATION_MAX_CROSS_REFERENCE_NAMES"), int64(lim.MaxCrossReferenceNames))
	if err != nil {
		return lim, err
	}
	lim.MaxCrossReferenceNames = int(maxXrefNames)
	if raw := strings.TrimSpace(os.Getenv("ESCROW_VALIDATION_TIMEOUT")); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return lim, fmt.Errorf("ESCROW_VALIDATION_TIMEOUT: %w", err)
		}
		lim.Timeout = d
	}
	if err := lim.Validate(); err != nil {
		return lim, err
	}
	return lim, nil
}

func envInt64(name, raw string, def int64) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def, nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return v, nil
}

// escrowDEAName returns the Data Escrow Agent name for notifications.
func escrowDEAName() string {
	if v := strings.TrimSpace(os.Getenv("ESCROW_VALIDATION_DEA_NAME")); v != "" {
		return v
	}
	return "domain-os EVE"
}
