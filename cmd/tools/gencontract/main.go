// gencontract generates deploy/contract.json from the env var registry.
//
// Usage:
//
//	go run ./cmd/tools/gencontract > deploy/contract.json
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/onasunnymorning/domain-os/internal/config"
)

func main() {
	// Find the project root by walking up to find VERSION
	root, err := findProjectRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gencontract: could not find project root: %v\n", err)
		os.Exit(1)
	}

	versionFile := filepath.Join(root, "VERSION")
	contract, err := config.GenerateContract(versionFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gencontract: failed to generate contract: %v\n", err)
		os.Exit(1)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(contract); err != nil {
		fmt.Fprintf(os.Stderr, "gencontract: failed to encode JSON: %v\n", err)
		os.Exit(1)
	}
}

// findProjectRoot walks up from the current working directory to find go.mod.
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found in any parent directory")
		}
		dir = parent
	}
}
