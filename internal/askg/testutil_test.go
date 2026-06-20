package askg

import (
	"io"
	"log/slog"
)

// newDiscardLogger returns a slog.Logger that discards all output.
// Used in tests to keep output clean.
func newDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
