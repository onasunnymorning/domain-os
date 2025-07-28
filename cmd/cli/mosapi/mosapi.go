package main

import (
	"fmt"
	"log"

	"github.com/onasunnymorning/domain-os/cmd/cli/mosapi/cmd"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/api/mosapi"
)

func main() {
	cmd.Execute()
}

// runMeasurementDetailsTest demonstrates how to use the MosapiClient to get measurement details
func runMeasurementDetailsTest() {
	// Get a MosapiClientConfig
	mc := mosapi.NewMosapiClientConfig()
	// Create a mosapiClient
	mosapiClient, err := mosapi.NewMosapiClient(mc)
	if err != nil {
		log.Fatal(err)
	}

	measurements, err := mosapiClient.GetMeasurementDetails("dns", "2025", "07", "28", "1753733280.json")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(measurements.PrettyPrint())
}
