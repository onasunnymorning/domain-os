package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/config"
)

// TestContractDrift verifies that deploy/contract.json is in sync with the
// env var registry and service metadata. If this test fails, run:
//
//	make generate-contract
//
// The contract is generated from config.GenerateContract() — never edit it by hand.
func TestContractDrift(t *testing.T) {
	root := findProjectRoot(t)
	contractPath := filepath.Join(root, "deploy", "contract.json")

	// Read the committed contract
	committedBytes, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("Could not read deploy/contract.json: %v\n\nFix: run 'make generate-contract' to create it.", err)
	}

	var committed config.Contract
	if err := json.Unmarshal(committedBytes, &committed); err != nil {
		t.Fatalf("deploy/contract.json is not valid JSON: %v", err)
	}

	// Generate the expected contract
	versionFile := filepath.Join(root, "VERSION")
	expected, err := config.GenerateContract(versionFile)
	if err != nil {
		t.Fatalf("GenerateContract failed: %v", err)
	}

	// Compare everything except the "generated" timestamp
	// (which changes on every generation)

	// Schema version
	if committed.SchemaVersion != expected.SchemaVersion {
		t.Errorf("schema_version: committed=%q expected=%q", committed.SchemaVersion, expected.SchemaVersion)
	}

	// App version
	if committed.AppVersion != expected.AppVersion {
		t.Errorf("app_version: committed=%q expected=%q\n\nThe VERSION file was bumped but contract.json was not regenerated.", committed.AppVersion, expected.AppVersion)
	}

	// Services — compare as normalized JSON for stable comparison
	committedSvcJSON := mustMarshal(t, committed.Services)
	expectedSvcJSON := mustMarshal(t, expected.Services)
	if committedSvcJSON != expectedSvcJSON {
		t.Errorf("services section is out of date.\n\ncommitted:\n%s\n\nexpected:\n%s\n\nFix: run 'make generate-contract' and commit the result.", committedSvcJSON, expectedSvcJSON)
	}

	// Infrastructure
	committedInfraJSON := mustMarshal(t, committed.Infrastructure)
	expectedInfraJSON := mustMarshal(t, expected.Infrastructure)
	if committedInfraJSON != expectedInfraJSON {
		t.Errorf("infrastructure section is out of date.\n\ncommitted:\n%s\n\nexpected:\n%s\n\nFix: run 'make generate-contract' and commit the result.", committedInfraJSON, expectedInfraJSON)
	}
}

// mustMarshal returns prettified JSON for comparison. Uses sorted map keys for stability.
func mustMarshal(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent failed: %v", err)
	}
	return string(b)
}
