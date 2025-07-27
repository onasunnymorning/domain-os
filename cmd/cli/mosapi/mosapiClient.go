package main

import (
	"fmt"
	"log"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/api/mosapi"
)

func main() {
	// Get a MosapiClientConfig
	mc := mosapi.NewMosapiClientConfig()
	// Create a mosapiClient
	mosapiClient, err := mosapi.NewMosapiClient(mc)
	if err != nil {
		log.Fatal(err)
	}

	days, err := mosapiClient.QueryAvailableMeasurementDays("DNS", "2023", "06")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(days)

}
