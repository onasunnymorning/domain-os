package main

import (
	"fmt"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
)

func main() {
	domCount, err := activities.GetExpiredDomainCount()
	if err != nil {
		panic(err)
	}

	fmt.Println("Total domains to renew: ", domCount)
}
