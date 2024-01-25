package main

import (
	"fmt"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/web/iana"
)

func main() {
	repo := iana.NewIANARegistrarRepo()
	svc := services.NewIANAXMLService(repo)

	ianaRegistrars, err := svc.ListIANARegistrars()
	if err != nil {
		panic(err)
	}

	for _, registrar := range ianaRegistrars {
		fmt.Println(registrar.GurID, registrar.Name, registrar.Status, registrar.RdapURL)
	}
}
