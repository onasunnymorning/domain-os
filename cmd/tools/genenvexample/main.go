// genenvexample generates .env.example from the env var registry.
//
// Usage:
//
//	go run ./cmd/tools/genenvexample > .env.example
package main

import (
	"fmt"
	"os"

	"github.com/onasunnymorning/domain-os/internal/config"
)

func main() {
	out, err := config.GenerateEnvExample()
	if err != nil {
		fmt.Fprintf(os.Stderr, "genenvexample: failed to generate: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(out)
}
