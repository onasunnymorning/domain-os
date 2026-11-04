// Package arch holds the architecture tests that `make ci-arch` runs beside
// .golangci.arch.yml: rules a linter cannot express but a parser can. Each
// test names the invariant it enforces (docs/INVARIANTS.md).
package arch

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// INV-09 — the domain declares sentinel errors; outer layers wrap rather
// than declare.
//
// A `var Err… = errors.New(...)` (or fmt.Errorf) at package level outside
// pkg/domain/entities is a sentinel an outer layer minted. The files that
// still hold one are allowlisted below with their counts, so the backlog
// cannot grow: a new sentinel in any other file, or one more in a listed
// file, fails this test. Delete a file's line when its sentinels move to
// the domain (or are replaced by wrapping a domain sentinel); the test
// fails on a stale line too, so the list stays exact.
var sentinelBacklog = map[string]int{
	// Infrastructure adapters whose refusals nothing above them matches by
	// sentinel yet. Each becomes a domain sentinel the day a caller needs
	// errors.Is on it.
	"internal/infrastructure/dns/resolver.go":                       4,
	"internal/infrastructure/db/postgres/schema_version.go":         3,
	"internal/infrastructure/api/mosapi/mosapiClient.go":            3,
	"internal/infrastructure/web/ianaregistrars/iana_repository.go": 1,
	"internal/infrastructure/passwordhash/argon2id.go":              1,
	"internal/infrastructure/db/postgres/errors.go":                 1,
	// Application services: escrow and RDE tooling, sync, transfers.
	"internal/application/services/escrow_service.go":          4,
	"internal/application/services/sync_service.go":            1,
	"internal/application/services/domain_service_transfer.go": 1,
	"internal/application/rdevalidate/pgp.go":                  2,
	"internal/application/rdeschema/engine.go":                 1,
	"internal/application/rdesanitize/token.go":                1,
	"internal/application/rdesanitize/fqdn.go":                 1,
	"internal/application/rdereport/report.go":                 1,
	// Commands and queries: input validation sentinels, which INV-13 keeps
	// beside the inputs they validate; moving them is a judgement call.
	"internal/application/commands/domain_commands.go":  1,
	"internal/application/commands/contact_commands.go": 1,
	"pkg/domain/queries/whoisQuery.go":                  2,
	"pkg/domain/queries/expiringDomainsQuery.go":        1,
	// REST adapter: request-shape refusals (page size, cursor, tenant
	// header) that never leave the adapter.
	"internal/interface/rest/pagination.go":               3,
	"internal/interface/rest/tenant.go":                   2,
	"internal/interface/rest/ianaRegistrar_controller.go": 1,
}

func TestSentinelsAreDeclaredInTheDomain(t *testing.T) {
	root := repoRoot(t)
	found := map[string]int{}
	for _, dir := range []string{"internal", "cmd", "pkg"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				switch d.Name() {
				case "vendor", "node_modules", "testdata":
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if strings.HasPrefix(rel, "pkg/domain/entities/") {
				return nil // where sentinels belong
			}
			if n := sentinelDeclarations(t, path); n > 0 {
				found[rel] = n
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	var files []string
	for f := range found {
		files = append(files, f)
	}
	sort.Strings(files)
	for _, f := range files {
		allowed, listed := sentinelBacklog[f]
		switch {
		case !listed:
			t.Errorf("%s declares %d sentinel error(s) outside pkg/domain/entities (INV-09): declare them in the domain and wrap them here, or wrap an existing domain sentinel", f, found[f])
		case found[f] > allowed:
			t.Errorf("%s declares %d sentinel error(s), up from the %d allowlisted (INV-09): declare the new one in pkg/domain/entities", f, found[f], allowed)
		case found[f] < allowed:
			t.Errorf("%s declares %d sentinel error(s), down from the %d allowlisted: update sentinelBacklog in internal/arch/sentinels_test.go", f, found[f], allowed)
		}
	}
	for f := range sentinelBacklog {
		if _, ok := found[f]; !ok {
			t.Errorf("%s is allowlisted but declares no sentinel error now: delete its line from sentinelBacklog", f)
		}
	}
}

// sentinelDeclarations counts the package-level `Err… = errors.New(...)`
// and `Err… = fmt.Errorf(...)` declarations in the file.
func sentinelDeclarations(t *testing.T, path string) int {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return countSentinels(f)
}

// countSentinels is sentinelDeclarations on a parsed file. The constructor
// is resolved through the file's import specs, so an aliased import
// (`import stderrors "errors"`) counts the same as the plain one.
func countSentinels(f *ast.File) int {
	imports := importsByLocalName(f)
	n := 0
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, spec := range gd.Specs {
			vs := spec.(*ast.ValueSpec)
			for i, name := range vs.Names {
				if !strings.HasPrefix(name.Name, "Err") || i >= len(vs.Values) {
					continue
				}
				if isErrorConstructor(vs.Values[i], imports) {
					n++
				}
			}
		}
	}
	return n
}

// importsByLocalName maps each import's name in the file (its alias, or the
// package name the path ends in) to its import path.
func importsByLocalName(f *ast.File) map[string]string {
	m := map[string]string{}
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		name := path[strings.LastIndex(path, "/")+1:]
		if imp.Name != nil {
			name = imp.Name.Name
		}
		m[name] = path
	}
	return m
}

func isErrorConstructor(e ast.Expr, imports map[string]string) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	switch imports[pkg.Name] {
	case "errors":
		return sel.Sel.Name == "New"
	case "fmt":
		return sel.Sel.Name == "Errorf"
	}
	return false
}

// TestCountSentinels pins what the INV-09 ratchet counts: the two standard
// constructors, however the package is named in the file, and nothing
// else.
func TestCountSentinels(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int
	}{
		{"errors.New", `package p; import "errors"; var ErrA = errors.New("a")`, 1},
		{"fmt.Errorf", `package p; import "fmt"; var ErrA = fmt.Errorf("a")`, 1},
		{"aliased errors", `package p; import stderrors "errors"; var ErrA = stderrors.New("a")`, 1},
		{"aliased fmt", `package p; import f "fmt"; var ErrA = f.Errorf("a")`, 1},
		{"both aliased in one block", `package p
import (
	e "errors"
	f "fmt"
)
var (
	ErrA = e.New("a")
	ErrB = f.Errorf("b")
)`, 2},
		{"local package named errors", `package p; import errors "example.com/x/errors"; var ErrA = errors.New("a")`, 0},
		{"not a sentinel name", `package p; import "errors"; var oops = errors.New("a")`, 0},
		{"a wrapped domain sentinel", `package p; import "fmt"; func f() error { return fmt.Errorf("x: %w", ErrA) }; var ErrA error`, 0},
		{"errors.New at package level without Err prefix", `package p; import "errors"; var NotFound = errors.New("a")`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := parser.ParseFile(token.NewFileSet(), "x.go", tc.src, 0)
			if err != nil {
				t.Fatal(err)
			}
			if got := countSentinels(f); got != tc.want {
				t.Errorf("got %d sentinel(s), want %d", got, tc.want)
			}
		})
	}
}

// repoRoot is the directory holding go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test")
		}
		dir = parent
	}
}
