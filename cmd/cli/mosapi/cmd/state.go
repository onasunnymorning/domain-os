package cmd

import (
	"fmt"
	"log"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/api/mosapi"
	"github.com/spf13/cobra"
)

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "Show overall service state",
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := getState()
		if err != nil {
			return err
		}
		fmt.Println(out)
		return nil
	},
}

func init() {
	getCmd.AddCommand(stateCmd)
}

// getState is a stub you can replace with an actual MOSAPI call.
func getState() (string, error) {
	// Get a MosapiClientConfig
	mc := mosapi.NewMosapiClientConfig()
	// Create a mosapiClient
	mosapiClient, err := mosapi.NewMosapiClient(mc)
	if err != nil {
		log.Fatal(err)
	}

	status, err := mosapiClient.GetState()
	if err != nil {
		return "", err
	}
	return status.PrettyPrint(), nil
}
