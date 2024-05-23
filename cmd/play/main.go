package main

import (
	"fmt"
	"log"
	"time"

	"encoding/json"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

func main() {

	// Create a priceEngine
	phase, err := entities.NewPhase("GA1", string(entities.PhaseTypeGA), time.Now().UTC())
	if err != nil {
		log.Fatalf("error creating phase: %v", err)
	}
	priceUSD, err := entities.NewPrice("USD", 10000, 10000, 10000, 10000)
	if err != nil {
		log.Fatalf("error creating price: %v", err)
	}
	pricePEN, err := entities.NewPrice("PEN", 40000, 40000, 40000, 40000)
	if err != nil {
		log.Fatalf("error creating price: %v", err)
	}
	_, err = phase.AddPrice(*priceUSD)
	if err != nil {
		log.Fatalf("error adding price: %v", err)
	}
	_, err = phase.AddPrice(*pricePEN)
	if err != nil {
		log.Fatalf("error adding price: %v", err)
	}
	refundable := false
	feeUSD, err := entities.NewFee("USD", "verification fee", 100000, &refundable)
	if err != nil {
		log.Fatalf("error creating fee: %v", err)
	}
	feePEN, err := entities.NewFee("PEN", "verification fee", 400000, &refundable)
	if err != nil {
		log.Fatalf("error creating fee: %v", err)
	}
	_, err = phase.AddFee(*feeUSD)
	if err != nil {
		log.Fatalf("error adding fee: %v", err)
	}
	_, err = phase.AddFee(*feePEN)
	if err != nil {
		log.Fatalf("error adding fee: %v", err)
	}

	domain, err := entities.NewDomain("122433_DOM-APEX", "example.com", "reg007", "st0ngP@azz§§w0rd")
	if err != nil {
		log.Fatalf("error creating domain: %v", err)
	}
	fx := entities.FX{
		BaseCurrency:   "USD",
		TargetCurrency: "EUR",
		Rate:           0.8,
	}
	pl := []*entities.PremiumLabel{}
	pe := entities.NewPriceEngine(*phase, *domain, fx, pl)

	// Make a QuoteRequest
	qr := entities.QuoteRequest{
		DomainName:      "example.com",
		TransactionType: entities.TransactionTypeRegistration,
		PhaseName:       "GA1",
		Currency:        "PEN",
		Years:           2,
		ClID:            "reg007",
	}

	// Get a Quote
	// q, err := pe.GetQuote(qr)
	// if err != nil {
	// 	log.Fatalf("error getting quote: %v", err)
	// }

	// Get a Quote Simplified
	q, err := pe.GetQuoteSimplified(qr)
	if err != nil {
		log.Fatalf("error getting quote: %v", err)
	}

	// Print the Quote Amount
	jsonBytes, err := json.Marshal(q.Price)
	if err != nil {
		log.Fatalf("error marshalling quote: %v", err)
	}

	fmt.Printf("Quote Amount: %+v\n", string(jsonBytes))

	// Print the Quote Fees
	jsonBytes, err = json.Marshal(q.Fees)
	if err != nil {
		log.Fatalf("error marshalling quote: %v", err)
	}

	fmt.Printf("Quote Fees: %+v\n", string(jsonBytes))

}
