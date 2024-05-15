package main

import (
	"fmt"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/api/openfx"
)

func main() {

	client := openfx.NewFxClient()

	rates, err := client.GetLatestRates("USD", []string{"EUR", "PEN"})
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(rates)

}
