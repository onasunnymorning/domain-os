package config_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/config"
)

// TestEnvRegistryDrift scans Go source files for os.Getenv() calls and
// validates that every env var used in code is documented in the registry.
//
// Run with: go test -run TestEnvRegistryDrift ./internal/config/...
//
// This test catches env var drift — when someone adds a new os.Getenv("FOO")
// call without updating the registry, CI will fail.
func TestEnvRegistryDrift(t *testing.T) {
	// Build lookup from registry
	registered := make(map[string]bool)
	for _, e := range config.Registry {
		registered[e.Name] = true
	}

	// Find project root (walk up from this test file until we find go.mod)
	root := findProjectRoot(t)

	// Scan Go source files for os.Getenv calls
	codeVars := extractGetenvCalls(t, root)

	// Check: every var in code must be in registry
	var unregistered []string
	for varName := range codeVars {
		if !registered[varName] {
			unregistered = append(unregistered, varName)
		}
	}

	sort.Strings(unregistered)
	if len(unregistered) > 0 {
		t.Errorf("Found %d env var(s) in code but NOT in registry (internal/config/env_registry.go):\n", len(unregistered))
		for _, v := range unregistered {
			locations := codeVars[v]
			t.Errorf("  %s (used in %s)", v, strings.Join(locations, ", "))
		}
		t.Error("\nFix: add these variables to config.Registry in internal/config/env_registry.go")
	}

	// Info: registry entries not found in code (may be frontend-only or platform-set)
	var unused []string
	for _, e := range config.Registry {
		if _, found := codeVars[e.Name]; !found {
			// Skip frontend, CLI, and runtime-detection vars — they won't appear in Go source
			if containsService(e.Services, config.ServiceFrontend) ||
				containsService(e.Services, config.ServiceCLI) ||
				e.Name == "LAMBDA_TASK_ROOT" ||
				e.Name == "KUBERNETES_SERVICE_HOST" {
				continue
			}
			unused = append(unused, e.Name)
		}
	}
	sort.Strings(unused)
	if len(unused) > 0 {
		t.Logf("INFO: %d registry entries have no os.Getenv() call in Go code (may be used via config structs or docker-compose only):", len(unused))
		for _, v := range unused {
			t.Logf("  %s", v)
		}
	}
}

// TestRegistryNoDuplicates ensures no env var name appears twice in the registry.
func TestRegistryNoDuplicates(t *testing.T) {
	seen := make(map[string]bool)
	for _, e := range config.Registry {
		if seen[e.Name] {
			t.Errorf("Duplicate registry entry: %s", e.Name)
		}
		seen[e.Name] = true
	}
}

// TestRegistryHasDescriptions ensures every entry has a non-empty description.
func TestRegistryHasDescriptions(t *testing.T) {
	for _, e := range config.Registry {
		if e.Description == "" {
			t.Errorf("Registry entry %q has no description", e.Name)
		}
		if len(e.Services) == 0 {
			t.Errorf("Registry entry %q has no services listed", e.Name)
		}
	}
}

// --- helpers ---

// findProjectRoot walks up from the current working dir to find go.mod.
func findProjectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (no go.mod found)")
		}
		dir = parent
	}
}

// extractGetenvCalls uses go/ast to find all os.Getenv("...") calls in Go files,
// excluding test files and vendor directories.
// Returns map[varName][]locations.
func extractGetenvCalls(t *testing.T, root string) map[string][]string {
	t.Helper()
	result := make(map[string][]string)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable dirs
		}

		// Skip vendor, .git, node_modules, frontend (TypeScript)
		base := info.Name()
		if info.IsDir() {
			switch base {
			case "vendor", ".git", "node_modules", "frontend", ".github":
				return filepath.SkipDir
			}
			return nil
		}

		// Only .go files, skip tests
		if !strings.HasSuffix(base, ".go") || strings.HasSuffix(base, "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		f, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return nil // skip unparseable files
		}

		relPath, _ := filepath.Rel(root, path)

		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			ident, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}

			// Match os.Getenv("...")
			if ident.Name == "os" && sel.Sel.Name == "Getenv" && len(call.Args) == 1 {
				lit, ok := call.Args[0].(*ast.BasicLit)
				if ok && lit.Kind == token.STRING {
					// Strip quotes
					varName := strings.Trim(lit.Value, `"`)
					if varName != "" {
						pos := fset.Position(call.Pos())
						loc := relPath + ":" + strings.TrimPrefix(filepath.Base(relPath), "") + ":" + itoa(pos.Line)
						result[varName] = appendUnique(result[varName], loc)
					}
				}
			}
			return true
		})

		return nil
	})

	if err != nil {
		t.Fatalf("filepath.Walk: %v", err)
	}
	return result
}

func containsService(services []config.Service, target config.Service) bool {
	for _, s := range services {
		if s == target {
			return true
		}
	}
	return false
}

func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
