package main

import (
	"fmt"
	"time"

	"github.com/Rhymond/go-money"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

func main() {

	dollar := money.New(1, "USD")
	eur := money.New(1, "EUR")

	fx := entities.FX{
		Date: time.Now(),
		From: "USD",
		To:   "EUR",
		Rate: 0.92884,
	}

	result, _ := fx.Convert(dollar)

	fmt.Println(result.Display())

	equal, _ := eur.Equals(result)

	fmt.Println(equal)

}
