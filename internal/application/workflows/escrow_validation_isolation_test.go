package workflows

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEscrowValidationDoesNotTouchImport is the structural half of issue
// #412's "validation is not import" proof: none of the EVE code may import
// the import services or reference the import activities. The behavioural
// half is TestEscrowValidation_NoRegistryWriteSideEffect in the activities
// package.
func TestEscrowValidationDoesNotTouchImport(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	var files []string
	for _, pattern := range []string{
		"internal/application/rdevalidate/*.go",
		"internal/application/rdevalidate/rdetest/*.go",
		"internal/application/rdereport/*.go",
		"internal/application/activities/escrow_validation*.go",
		"internal/application/workflows/escrowValidation.go",
		"internal/infrastructure/secrets/*.go",
	} {
		matches, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, matches...)
	}
	if len(files) < 10 {
		t.Fatalf("expected the EVE source set, found %d files", len(files))
	}

	deniedImports := []string{
		"github.com/onasunnymorning/domain-os/internal/application/services",
		"github.com/onasunnymorning/domain-os/internal/application/commands",
		"github.com/onasunnymorning/domain-os/internal/interface/api",
		"gorm.io/driver/sqlite",
		"modernc.org/sqlite",
	}
	deniedIdents := []string{
		"EscrowImportActivities", "EscrowImportWorkflow", "BuildStagingDatabase", "ResolveRegistrars",
		"ApplyRegistrarMappings", "IngestContacts", "IngestHosts", "IngestDomains", "IngestNNDNs",
		"LinkDomainHosts", "AccreditRegistrars", "DirectDBImporter", "ImportFromSQLite", "StreamingAnalysis",
		"XMLEscrowService", "StreamingXMLEscrowService",
	}

	fset := token.NewFileSet()
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, path, src, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			for _, d := range deniedImports {
				if p == d || strings.HasPrefix(p, d+"/") {
					t.Errorf("%s imports %s: validation must not depend on the import pipeline", path, p)
				}
			}
		}
		full, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		ast.Inspect(full, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			for _, d := range deniedIdents {
				if id.Name == d {
					t.Errorf("%s references %s: validation must not reach import code", path, d)
				}
			}
			return true
		})
	}
}
