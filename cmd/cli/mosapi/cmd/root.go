package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mosapi",
	Short: "CLI client for MOSAPI measurements",
	Long:  "MOSAPI command-line interface for querying measurement data and service status.",
}

// Execute is called by main().
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Global persistent flags go here, e.g. --config
}
