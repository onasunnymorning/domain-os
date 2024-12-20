package main

import (
	"fmt"

	"github.com/onasunnymorning/domain-os/internal/application/actions"
)

func main() {
	domCount, err := actions.GetExpiredDomainCount()
	if err != nil {
		panic(err)
	}

	fmt.Println("Total domains to renew: ", domCount)
}
