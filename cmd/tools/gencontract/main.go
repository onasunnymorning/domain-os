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

	"github.com/onasunnymorning/domain-os/internal/config"
)

func main() {
	contract, err := config.GenerateContract()
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
