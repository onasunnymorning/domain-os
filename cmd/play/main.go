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

	fmt.Printf("Total domains expiring before %v: %d\n", domCount.Timestamp, domCount.Count)
}
