package main

import (
	"log"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/api/mosapi"
)

func main() {
	// Get a MosapiClientConfig
	mc := mosapi.NewMosapiClientConfig()
	// Create a client
	client, err := mosapi.NewMosapiClient(mc)
	if err != nil {
		log.Fatal(err)
	}
	// Get the status of the TLD
	status, err := client.GetState()
	if err != nil {
		log.Fatal(err)
	}

	log.Println(status)
}
