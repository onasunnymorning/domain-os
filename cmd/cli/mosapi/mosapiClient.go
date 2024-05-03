package main

import (
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
	// Login
	// err = mosapiClient.Login()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// Get the status of the TLD
	status, err := mosapiClient.GetState()
	if err != nil {
		log.Fatal(err)
	}
	log.Println(status.TLD, status.Status)

	// Get the DNS status of the TLD
	alarmResponse, err := mosapiClient.GetAlarm("DNS")
	if err != nil {
		log.Fatal(err)
	}
	log.Println(alarmResponse)

	// Get the RDDS Downtime status of the TLD
	downtimeResponse, err := mosapiClient.GetDowntime("RDDS")
	if err != nil {
		log.Fatal(err)
	}
	log.Println(downtimeResponse)

	// Logout
	// err = mosapiClient.Logout()
	// if err != nil {
	// 	log.Fatal(err)
	// }

}
