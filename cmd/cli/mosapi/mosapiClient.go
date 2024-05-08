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
	if status.Status == "Down" {
		for s, service := range status.TestedServices {
			log.Printf("Service %s for TLD %s is %s - emergency threshold percent : %v\n", s, status.TLD, service.Status, service.EmergencyThreshold)
		}
	} else {
		log.Printf("TLD %s is %s\n", status.TLD, status.Status)
	}

	// jsondata, err := json.Marshal(status)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// log.Println(string(jsondata))

	// Get the DNS status of the TLD
	// alarmResponse, err := mosapiClient.GetAlarm("DNS")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// log.Println(alarmResponse.IsAlarmed())

	// // Get the RDDS Downtime status of the TLD
	// downtimeResponse, err := mosapiClient.GetDowntime("RDDS")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// log.Println(downtimeResponse.Downtime)

	// Logout
	// err = mosapiClient.Logout()
	// if err != nil {
	// 	log.Fatal(err)
	// }

}
