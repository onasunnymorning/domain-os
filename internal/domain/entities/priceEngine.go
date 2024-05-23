package entities

import (
	"errors"
	"fmt"

	"github.com/Rhymond/go-money"
)

var (
	ErrInvalidGrandFatheringPrice = errors.New("invalid grand fathering price")
)

// PriceEngine is the entity that calculates the price of a domain name.
type PriceEngine struct {
	Phase          Phase
	PremiumEntries []*PremiumLabel
	FXRate         FX
	Domain         Domain
	QuoteRequest   QuoteRequest
	Quote          *Quote
}

// NewPriceEngine creates a new PriceEngine. It needs to be instantiated with a Phase, Domain, FX, and a slice of optional PremiumLabels (for that specific Domain.Label)
func NewPriceEngine(phase Phase, dom Domain, fx FX, pe []*PremiumLabel) *PriceEngine {
	// if phase.Policy.BaseCurrency != fx.BaseCurrency {
	// 	panic(ErrBaseCurrencyMismatch)
	// }
	return &PriceEngine{
		Phase:          phase,
		PremiumEntries: pe,
		FXRate:         fx,
		Domain:         dom,
	}
}

// setQuoteParams sets the quote parameters that need to be copied from the price engine.
func (pe *PriceEngine) setQuoteParams() {
	pe.Quote.FXRate = &pe.FXRate
	pe.Quote.DomainName = pe.Domain.Name
	pe.Quote.Phase = &pe.Phase
}

// addPhaseFees gets the applicable fees from the phase and copies them tot he Quote and updates the Quote's total price
func (pe *PriceEngine) addPhaseFees() error {
	if pe.Phase.Fees != nil {
		// Get the fees in the target currency
		fees := pe.Phase.GetFees(pe.QuoteRequest.Currency)
		for _, fee := range fees {
			err := pe.Quote.AddFeeAndUpdatePrice(&fee, false)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// addGrandFatheringFees sets the grand fathering fees on the quote.
func (pe *PriceEngine) addGrandFatheringFees() error {
	refundable := true // Renew fees are refundable
	// Only apply grand fathering fees if the domain is grandfathered and the transaction is renew
	if pe.Domain.IsGrandFathered() && pe.QuoteRequest.TransactionType == "renew" {
		err := pe.Quote.AddFeeAndUpdatePrice(
			&Fee{
				Name:       "GrandFathering",
				Amount:     uint64(pe.Domain.GrandFathering.GFAmount),
				Currency:   pe.Domain.GrandFathering.GFCurrency,
				Refundable: &refundable,
			}, true,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// addPremiumFees sets the premium fees on the quote.
func (pe *PriceEngine) addPremiumFees() error {
	refundable := true // Premium fees are refundable
	if len(pe.PremiumEntries) > 0 {
		premiumfees := []*Fee{}
		for _, pl := range pe.PremiumEntries {
			// Try and find the entry for the domain in the target currency
			if pl.Label == Label(pe.Domain.Name.Label()) && pl.Currency == pe.QuoteRequest.Currency {
				money, _ := pl.GetMoney(pe.QuoteRequest.TransactionType)
				premiumfees = append(premiumfees, &Fee{
					Name:       "premium fee",
					Amount:     uint64(money.Amount()),
					Currency:   money.Currency().Code,
					Refundable: &refundable,
				})
			}
		}
		// If we have no entries, look for matches using the phase's base currency
		for _, pl := range pe.PremiumEntries {
			if pl.Label == Label(pe.Domain.Name.Label()) && pl.Currency == pe.Phase.Policy.BaseCurrency {
				money, _ := pl.GetMoney(pe.QuoteRequest.TransactionType)
				money, _ = pe.FXRate.Convert(money)
				premiumfees = append(premiumfees, &Fee{
					Name:       "premium fee",
					Amount:     uint64(money.Amount()),
					Currency:   money.Currency().Code,
					Refundable: &refundable,
				})
			}
		}
		// If we still have no entries, we have an issue because there is a premium price, but neither in the target or base currency
		if len(premiumfees) == 0 {
			return errors.New("expected premium pricing but none found")
		}
		// Add the fees to the quote
		for _, fee := range premiumfees {
			err := pe.Quote.AddFeeAndUpdatePrice(fee, true)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// addPhasePrice sets the phase price on the quote.
func (pe *PriceEngine) addPhasePrice() error {
	refundable := true // Phase fees are refundable
	// If the phase has prices, try and find the price in the target currency
	if pe.Phase.Prices != nil {
		price, err := pe.Phase.GetPrice(pe.QuoteRequest.Currency)
		if err != nil {
			// If we can't find the price in the target currency, try the phase's base currency
			price, err = pe.Phase.GetPrice(pe.Phase.Policy.BaseCurrency)
			if err != nil {
				// If we can't find the price in the base currency, we have no price so no need to continue
				return nil
			}
		}
		// Get the price for the transaction type
		priceMoney, _ := price.GetMoney(pe.QuoteRequest.TransactionType)
		// Add the fee to the quote
		err = pe.Quote.AddFeeAndUpdatePrice(&Fee{
			Name:       ClIDType(fmt.Sprintf("%s fee", pe.QuoteRequest.TransactionType)),
			Amount:     uint64(priceMoney.Amount()),
			Currency:   price.Currency,
			Refundable: &refundable,
		}, true)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetQuoteSimplified calculates the price for a transaction and returns a Quote entity.
func (pe *PriceEngine) GetQuoteSimplified(qr QuoteRequest) (*Quote, error) {
	if qr.PhaseName != pe.Phase.Name.String() {
		return nil, ErrInvalidPhaseName
	}
	pe.QuoteRequest = qr

	var err error
	pe.Quote, err = NewQuoteFromQuoteRequest(qr)
	if err != nil {
		return nil, err
	}
	pe.setQuoteParams()

	// First look at optional fees that need to be applied on top of pricing
	err = pe.addPhaseFees()
	if err != nil {
		return nil, err
	}

	// If the domain is grandfathered, apply the grand fathering fees and return
	if pe.Domain.IsGrandFathered() && qr.TransactionType == "renew" {
		err = pe.addGrandFatheringFees()
		if err != nil {
			return nil, err
		}
		return pe.Quote, nil
	}

	// If there are premium entries, apply the premium fees and return
	if len(pe.PremiumEntries) > 0 {
		err = pe.addPremiumFees()
		if err != nil {
			return nil, err
		}
		return pe.Quote, nil
	}

	err = pe.addPhasePrice()
	if err != nil {
		return nil, err
	}

	return pe.Quote, nil
}

// GetQuote calculates the price for a transaction and returns a Quote entity.
func (pe *PriceEngine) GetQuote(qr QuoteRequest) (*Quote, error) {
	if qr.PhaseName != pe.Phase.Name.String() {
		return nil, ErrInvalidPhaseName
	}
	pe.QuoteRequest = qr

	var err error
	pe.Quote, err = NewQuoteFromQuoteRequest(qr)
	if err != nil {
		return nil, err
	}
	pe.setQuoteParams()

	// Add any additional fees for the phase
	if pe.Phase.Fees != nil {
		needsFX := false
		fees := pe.Phase.GetFees(qr.Currency)
		if fees == nil {
			fees = pe.Phase.GetFees(pe.Phase.Policy.BaseCurrency)
			needsFX = true
		}
		for _, fee := range fees {
			pe.Quote.Fees = append(pe.Quote.Fees, &Fee{
				Name:       fee.Name,
				Amount:     uint64(fee.Amount),
				Currency:   fee.Currency,
				Refundable: fee.Refundable,
			})
			if needsFX {
				feeMoney, err := pe.FXRate.Convert(money.New(int64(fee.Amount), fee.Currency))
				if err != nil {
					return nil, err
				}
				pe.Quote.Price, err = pe.Quote.Price.Add(feeMoney)
				if err != nil {
					return nil, err
				}
			} else {
				pe.Quote.Price, err = pe.Quote.Price.Add(money.New(int64(fee.Amount), fee.Currency))
				if err != nil {
					return nil, err
				}
			}

		}

	}

	// The rest of the items will be refundable
	refundable := true

	// If the domain has grandfathering and the transaction is renew, then that is our price
	if pe.Domain.IsGrandFathered() && qr.TransactionType == "renew" {
		if pe.Domain.GrandFathering.GFCurrency == qr.Currency {
			gf := money.New(int64(pe.Domain.GrandFathering.GFAmount), qr.Currency)
			if gf == nil {
				return nil, ErrInvalidGrandFatheringPrice
			}
			for i := 0; i < qr.Years; i++ {
				pe.Quote.Fees = append(pe.Quote.Fees, &Fee{
					Name:       "GrandFathering",
					Amount:     uint64(gf.Amount()),
					Currency:   gf.Currency().Code,
					Refundable: &refundable,
				})
			}
			gf = gf.Multiply(int64(qr.Years))
			pe.Quote.Price, err = pe.Quote.Price.Add(gf)
			if err != nil {
				return nil, err
			}
		} else {
			gf, err := pe.FXRate.Convert(money.New(int64(pe.Domain.GrandFathering.GFAmount), qr.Currency))
			if err != nil {
				return nil, err
			}
			for i := 0; i < qr.Years; i++ {
				pe.Quote.Fees = append(pe.Quote.Fees, &Fee{
					Name:       "GrandFathering",
					Amount:     uint64(gf.Amount()),
					Currency:   gf.Currency().Code,
					Refundable: &refundable,
				})
			}
			gf = gf.Multiply(int64(qr.Years))
			pe.Quote.Price, err = pe.Quote.Price.Add(gf)
			if err != nil {
				return nil, err
			}
		}
		// if the domain is not grandfathered, we can return the price at this point
		return pe.Quote, nil
	}

	// If the domain is not grandfathered, check if there is a premium price
	if len(pe.PremiumEntries) > 0 {
		for _, pl := range pe.PremiumEntries {
			if pl.Label == Label(pe.Domain.Name.Label()) {
				moneyToAdd, err := pl.GetMoney(qr.TransactionType)
				if err != nil {
					return nil, err
				}
				for i := 0; i < qr.Years; i++ {
					pe.Quote.Fees = append(pe.Quote.Fees, &Fee{
						Name:       "premium fee",
						Amount:     uint64(moneyToAdd.Amount()),
						Currency:   moneyToAdd.Currency().Code,
						Refundable: &refundable,
					})
				}
				moneyToAdd = moneyToAdd.Multiply(int64(qr.Years))
				pe.Quote.Class = pl.Class
				if pl.Currency == qr.Currency {
					pe.Quote.Price, err = pe.Quote.Price.Add(moneyToAdd)
					if err != nil {
						return nil, err
					}
				} else {
					moneyToAddInCorrectCurrency, err := pe.FXRate.Convert(moneyToAdd)
					if err != nil {
						return nil, err
					}
					pe.Quote.Price, err = pe.Quote.Price.Add(moneyToAddInCorrectCurrency)
					if err != nil {
						return nil, err
					}
				}
			}
		}
		// if we found a premium price, we can return the price at this point
		return pe.Quote, nil
	}

	// Fall back on the phase price
	needsFX := false
	if pe.Phase.Prices != nil {
		// See if we have a price in the requested currency
		price, err := pe.Phase.GetPrice(qr.Currency)
		if err != nil {
			if !errors.Is(err, ErrPriceNotFound) {
				return nil, err
			}
			// Try and find it in the base currency
			needsFX = true
			price, err = pe.Phase.GetPrice(pe.Phase.Policy.BaseCurrency)
			if err != nil {
				return nil, err
			}
		}
		priceMoneyToAdd, err := price.GetMoney(qr.TransactionType)
		if err != nil {
			return nil, err
		}
		// Add the fee to the quote, as many times as years
		for i := 0; i < qr.Years; i++ {
			pe.Quote.Fees = append(pe.Quote.Fees, &Fee{
				Name:       ClIDType(fmt.Sprintf("%s fee", qr.TransactionType)),
				Amount:     uint64(priceMoneyToAdd.Amount()),
				Currency:   price.Currency,
				Refundable: &refundable,
			})
		}
		priceMoneyToAdd = priceMoneyToAdd.Multiply(int64(qr.Years))
		if needsFX {
			priceMoneyToAdd, err = pe.FXRate.Convert(priceMoneyToAdd)
			if err != nil {
				return nil, err
			}
		}
		pe.Quote.Price, err = pe.Quote.Price.Add(priceMoneyToAdd)
		if err != nil {
			return nil, err
		}
		// if we found a price, we can return the price at this point
		return pe.Quote, nil
	}

	// If we haven't got a price at this point, none is available so we're assuming free (0)
	// we can just retutn the quote as the fees are already applied
	return pe.Quote, nil
}
