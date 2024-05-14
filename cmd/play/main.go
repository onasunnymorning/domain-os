package main

import (
	"encoding/json"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
)

func main() {
	domCommand := commands.RegisterDomainCommand{
		Name:         "example.com",
		ClID:         "cl1234",
		AuthInfo:     "1234",
		RegistrantID: "reg1234",
		AdminID:      "admin1234",
		TechID:       "tech1234",
		BillingID:    "bill1234",
		Years:        1,
		HostNames:    []string{"ns1.example.com", "ns2.example.com"},
	}

	jsondata, err := json.Marshal(domCommand)
	if err != nil {
		panic(err)
	}

	println(string(jsondata))
}
