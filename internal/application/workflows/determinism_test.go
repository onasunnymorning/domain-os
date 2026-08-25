package workflows

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestWorkflowDeterminism enforces INV-06: workflow code must be deterministic,
// so clock, randomness, network and IO belong in activities rather than here.
//
// Temporal replays workflow code against its event history. A wall-clock read
// or a network call in workflow scope produces a nondeterminism panic on
// replay — typically in production, during an incident, on the workflow you
// least want to lose. Because the failure only appears on replay, tests that
// exercise the happy path will not catch it; this check will.
//
// Use workflow.Now(ctx) for time, workflow.SideEffect for randomness, and
// workflow.ExecuteActivity for anything that touches the network or disk.
//
// See docs/INVARIANTS.md, INV-06.
func TestWorkflowDeterminism(t *testing.T) {
	// Nondeterminism proper. Applied to every file in the package, tests
	// included: these diverge on replay wherever they sit, and the invariant
	// is documented as holding across tests too.
	nondeterministic := map[string]string{
		"net/http":                "network IO belongs in an activity",
		"math/rand":               "use workflow.SideEffect for randomness",
		"math/rand/v2":            "use workflow.SideEffect for randomness",
		"crypto/rand":             "use workflow.SideEffect for randomness",
		"github.com/google/uuid":  "use workflow.SideEffect to generate IDs",
		"github.com/pborman/uuid": "use workflow.SideEffect to generate IDs",
	}

	// IO that belongs in an activity. Applied to workflow code only — test
	// files legitimately open databases and read the environment to build
	// fixtures, and Temporal never replays them.
	ioOnlyInActivities := map[string]string{
		"database/sql": "database IO belongs in an activity",
		"os":           "filesystem and environment access belong in an activity",
		"gorm.io/gorm": "database access belongs in an activity",
	}

	// Package-qualified calls that must never appear in workflow scope.
	// time is importable here — it is needed for time.Duration and time.Time —
	// but reading the clock is not.
	forbiddenCalls := map[string]string{
		"time.Now":   "use workflow.Now(ctx)",
		"time.Since": "use workflow.Now(ctx) and subtract",
	}

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("globbing workflow files: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no workflow files found — has this test moved out of the workflows package?")
	}

	fset := token.NewFileSet()
	for _, file := range files {
		// This test declares the forbidden names as data; exempt it from itself.
		if file == "determinism_test.go" {
			continue
		}

		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("reading %s: %v", file, err)
		}
		f, err := parser.ParseFile(fset, file, src, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", file, err)
		}

		isTest := strings.HasSuffix(file, "_test.go")

		for _, imp := range f.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}
			if reason, bad := nondeterministic[path]; bad {
				t.Errorf("%s:%d: workflow code must not import %q — %s (INV-06)",
					file, fset.Position(imp.Pos()).Line, path, reason)
			}
			if reason, bad := ioOnlyInActivities[path]; bad && !isTest {
				t.Errorf("%s:%d: workflow code must not import %q — %s (INV-06)",
					file, fset.Position(imp.Pos()).Line, path, reason)
			}
		}

		// Clock reads diverge on replay in workflow code; test files may
		// legitimately stamp fixtures with the wall clock.
		if isTest {
			continue
		}

		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			qualified := pkg.Name + "." + sel.Sel.Name
			if reason, bad := forbiddenCalls[qualified]; bad {
				t.Errorf("%s:%d: workflow code must not call %s — %s (INV-06)",
					file, fset.Position(call.Pos()).Line, qualified, reason)
			}
			return true
		})
	}

	if !t.Failed() {
		t.Logf("checked %d files in %s", len(files), mustCwd(t))
	}
}

func mustCwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		return "?"
	}
	return strings.TrimPrefix(wd, filepath.Dir(filepath.Dir(filepath.Dir(wd)))+string(filepath.Separator))
}
