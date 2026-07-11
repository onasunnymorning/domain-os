package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	expected, err := config.GenerateContract()
	if err != nil {
		t.Fatalf("GenerateContract failed: %v", err)
	}

	// Compare everything except the "generated" timestamp
	// (which changes on every generation)

	// Schema version
	if committed.SchemaVersion != expected.SchemaVersion {
		t.Errorf("schema_version: committed=%q expected=%q", committed.SchemaVersion, expected.SchemaVersion)
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

// TestSecretsAreExplicit is the guard that would have caught DB_PASS.
//
// EnvVar.Secret is declared by hand, so the risk is forgetting it on a new
// credential. The old substring heuristic is kept as a *lower bound*: any var
// whose name contains SECRET/PASSWORD/TOKEN/CERT/KEY/LICENSE must be marked
// Secret. It cannot be the upper bound — DB_PASS and DATABASE_URL are both
// credentials that the heuristic misses — so known misses are asserted directly.
func TestSecretsAreExplicit(t *testing.T) {
	// Lower bound: an obviously-credential name may never ship with Secret unset.
	for _, v := range config.Registry {
		if config.NameLooksSecret(v.Name) && !v.Secret {
			t.Errorf("%s: name matches the secret heuristic but Secret is false.\n"+
				"Either mark it Secret: true, or if it is genuinely not credential "+
				"material, say so in the description and add it to the exemptions here.", v.Name)
		}
	}

	// Names the heuristic cannot catch. These are the ones that fail unsafely:
	// a deploy tool trusting `secret: false` would put them in a plaintext env var.
	mustBeSecret := []string{
		"DB_PASS",                  // "PASSWORD" is not a substring of "DB_PASS"
		"DATABASE_URL",             // embeds the password; matches no pattern
		"OPENEXCHANGERATES_APP_ID", // an API key; matches no pattern
	}
	reg := config.RegistryMap()
	for _, name := range mustBeSecret {
		v, ok := reg[name]
		if !ok {
			t.Errorf("%s is missing from the registry", name)
			continue
		}
		if !v.Secret {
			t.Errorf("%s must be Secret: true — it is credential material that the "+
				"name heuristic does not catch", name)
		}
	}

	// NEXT_PUBLIC_* is served to every browser and can never be secret, however
	// it is declared. This is why NEXT_PUBLIC_API_TOKEN emits secret:false.
	for _, v := range config.Registry {
		if strings.HasPrefix(v.Name, "NEXT_PUBLIC_") && v.Secret {
			t.Errorf("%s: NEXT_PUBLIC_* values are readable by any browser and must "+
				"never be declared Secret", v.Name)
		}
	}
}

// TestInfraUsedByIsDerived pins the facts that infra tooling keys on, so a
// change to a service's DB access shows up here rather than in a deploy.
func TestInfraUsedByIsDerived(t *testing.T) {
	c, err := config.GenerateContract()
	if err != nil {
		t.Fatalf("GenerateContract failed: %v", err)
	}

	got := map[string][]string{}
	for _, comp := range c.Infrastructure {
		got[comp.Name] = comp.UsedBy
	}

	// mcp-server and whois both build a DSN from DB_* (cmd/mcp/main.go,
	// cmd/whois/whois.go) — they are Postgres consumers and need DB credentials.
	want := map[string][]string{
		"postgresql": {"admin-api", "unified-worker", "whois", "mcp-server"},
		"redis":      {"epp-server"},
		"temporal":   {"admin-api", "unified-worker"},
		"s3":         {"admin-api", "unified-worker"},
	}
	for name, wantUsers := range want {
		gotUsers := got[name]
		if len(gotUsers) != len(wantUsers) {
			t.Errorf("%s.used_by = %v, want %v", name, gotUsers, wantUsers)
			continue
		}
		for i := range wantUsers {
			if gotUsers[i] != wantUsers[i] {
				t.Errorf("%s.used_by = %v, want %v", name, gotUsers, wantUsers)
				break
			}
		}
	}
}

// TestEveryDeployableServiceHasAHealthCheck — an ALB/ECS target group needs a
// probe target for every service it fronts.
func TestEveryDeployableServiceHasAHealthCheck(t *testing.T) {
	c, err := config.GenerateContract()
	if err != nil {
		t.Fatalf("GenerateContract failed: %v", err)
	}
	for name, svc := range c.Services {
		if svc.HealthCheck == nil {
			t.Errorf("service %q has no health_check; infra cannot build a target group for it", name)
		}
	}
}
