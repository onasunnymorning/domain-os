package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/config"
)

// TestEnvExampleDrift verifies that .env.example is in sync with the env var
// registry. If this fails, run:
//
//	make generate-env-example
//
// The file is generated from config.GenerateEnvExample() — never edit it by hand.
//
// This is the guard that makes the onboarding promise hold: a variable added in
// code but missing from .env.example is a newcomer's nil panic three minutes
// into their first run. Here it is a build failure instead.
func TestEnvExampleDrift(t *testing.T) {
	root := findProjectRoot(t)
	path := filepath.Join(root, ".env.example")

	committed, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read .env.example: %v\n\nFix: run 'make generate-env-example'.", err)
	}

	expected, err := config.GenerateEnvExample()
	if err != nil {
		t.Fatalf("GenerateEnvExample failed: %v", err)
	}

	if string(committed) != expected {
		t.Errorf(".env.example is out of date.\n\nFix: run 'make generate-env-example' and commit the result.")
	}
}

// TestEnvExampleHasNoRealCredentials asserts the constraint that makes this file
// safe to commit: every value in it is a local default, never a real secret.
//
// The check is deliberately shallow — it catches the realistic accident (someone
// pastes a working key into the generator's override map) rather than trying to
// prove a negative.
func TestEnvExampleHasNoRealCredentials(t *testing.T) {
	generated, err := config.GenerateEnvExample()
	if err != nil {
		t.Fatalf("GenerateEnvExample failed: %v", err)
	}

	// Prefixes that identify a real credential from the providers this project
	// actually integrates with.
	bannedPrefixes := []string{
		"sk-ant-",    // Anthropic
		"AKIA",       // AWS access key ID
		"ASIA",       // AWS temporary access key ID
		"dp.pt.",     // Doppler personal token
		"dp.st.",     // Doppler service token
		"glc_",       // Grafana Cloud
		"phc_",       // PostHog project key
		"eyJhbGciOi", // a JWT header — base64 of {"alg":"...
	}

	for _, line := range strings.Split(generated, "\n") {
		if strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		_, value, _ := strings.Cut(line, "=")
		value = strings.Trim(value, `"`)
		if value == "" {
			continue
		}
		for _, banned := range bannedPrefixes {
			if strings.HasPrefix(value, banned) {
				t.Errorf("line %q carries what looks like a real credential (prefix %q). "+
					".env.example is committed — replace it with a local default.", line, banned)
			}
		}
	}
}

// TestEnvExampleCoversRegistry asserts that every registry variable reaches the
// generated file, except the ones deliberately skipped as platform-injected.
func TestEnvExampleCoversRegistry(t *testing.T) {
	generated, err := config.GenerateEnvExample()
	if err != nil {
		t.Fatalf("GenerateEnvExample failed: %v", err)
	}

	// Injected by the runtime (Lambda, Kubernetes); setting them by hand breaks
	// the environment detection that reads them.
	skipped := map[string]bool{
		"LAMBDA_TASK_ROOT":        true,
		"KUBERNETES_SERVICE_HOST": true,
	}

	for _, v := range config.Registry {
		if skipped[v.Name] {
			continue
		}
		if !strings.Contains(generated, "\n"+v.Name+"=") {
			t.Errorf("registry variable %q is missing from .env.example", v.Name)
		}
	}
}
