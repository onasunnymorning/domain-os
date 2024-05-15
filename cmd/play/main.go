package main

import (
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/api/openfx"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
)

func main() {

	// Get an updated list of exchange rates from the OpenFX API
	client := openfx.NewFxClient()
	response, err := client.GetLatestRates("USD", []string{})
	if err != nil {
		fmt.Println(err)
	}

	// Convert the response to a slice of postgres.FX structs
	fxs := []postgres.FX{}
	for currency, rate := range response.Rates {
		fx := postgres.FX{
			Date:   time.Unix(response.Timestamp, 0).UTC(),
			Base:   response.Base,
			Target: currency,
			Rate:   rate,
		}
		fxs = append(fxs, fx)
	}

	// Bulk insert the exchange rates into the database

}
