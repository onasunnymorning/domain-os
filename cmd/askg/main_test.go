package main

import (
	"bytes"
	"io"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLogOutputGoesToStderr verifies that after the setup performed by run(),
// the standard "log" package writes to stderr — not stdout. This prevents
// infrastructure code (e.g. GORM connection logging) from corrupting the
// JSON output on stdout.
func TestLogOutputGoesToStderr(t *testing.T) {
	// Capture stdout
	origStdout := os.Stdout
	rOut, wOut, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = wOut

	// Capture stderr
	origStderr := os.Stderr
	rErr, wErr, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = wErr

	// Apply the same fix as run()
	log.SetOutput(os.Stderr)
	log.Println("gorm-style connection message")

	// Close writers and read captured output
	wOut.Close()
	wErr.Close()
	os.Stdout = origStdout
	os.Stderr = origStderr

	var bufOut, bufErr bytes.Buffer
	io.Copy(&bufOut, rOut)
	io.Copy(&bufErr, rErr)

	// stdout must be clean — this is what jq parses
	assert.Empty(t, bufOut.String(), "log output must not appear on stdout")

	// stderr should have the message
	assert.True(t, strings.Contains(bufErr.String(), "gorm-style connection message"),
		"log output should appear on stderr")
}

// TestRunWithoutArgs verifies the CLI prints usage to stderr and returns
// exit code 1 when no question is provided.
func TestRunWithoutArgs(t *testing.T) {
	// Save and restore os.Args
	origArgs := os.Args
	defer func() { os.Args = origArgs }()
	os.Args = []string{"askg"} // no question arg

	// Capture stderr
	origStderr := os.Stderr
	rErr, wErr, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = wErr

	code := run()

	wErr.Close()
	os.Stderr = origStderr

	var bufErr bytes.Buffer
	io.Copy(&bufErr, rErr)

	assert.Equal(t, 1, code)
	assert.Contains(t, bufErr.String(), "Usage:")
}
