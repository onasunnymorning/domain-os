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

// GetQuote calculates the price for a transaction and returns a Quote entity.
func (pe *PriceEngine) GetQuote(qr QuoteRequest) (*Quote, error) {
	err := qr.Validate()
	if err != nil {
		return nil, err
	}
	if qr.PhaseName != pe.Phase.Name.String() {
		return nil, ErrInvalidPhaseName
	}
	if qr.DomainName != pe.Domain.Name.String() {
		return nil, ErrInvalidDomainName
	}
	clid, err := NewClIDType(qr.ClID)
	if err != nil {
		return nil, err
	}
	q := NewQuote()
	q.Class = "standard"
	q.FXRate = &pe.FXRate
	q.Price = money.New(0, qr.Currency)
	q.DomainName = pe.Domain.Name
	q.Years = qr.Years
	q.TransactionType = qr.TransactionType
	q.Clid = clid
	q.Phase = &pe.Phase

	// Add any additional fees for the phase
	if pe.Phase.Fees != nil {
		needsFX := false
		fees := pe.Phase.GetFees(qr.Currency)
		if fees == nil {
			fees = pe.Phase.GetFees(pe.Phase.Policy.BaseCurrency)
			needsFX = true
		}
		for _, fee := range fees {
			q.Fees = append(q.Fees, &Fee{
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
				q.Price, err = q.Price.Add(feeMoney)
				if err != nil {
					return nil, err
				}
			} else {
				q.Price, err = q.Price.Add(money.New(int64(fee.Amount), fee.Currency))
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
			q.Fees = append(q.Fees, &Fee{
				Name:       "GrandFathering",
				Amount:     uint64(gf.Amount()),
				Currency:   gf.Currency().Code,
				Refundable: &refundable,
			})
			gf = gf.Multiply(int64(qr.Years))
			q.Price, err = q.Price.Add(gf)
			if err != nil {
				return nil, err
			}
		} else {
			gf, err := pe.FXRate.Convert(money.New(int64(pe.Domain.GrandFathering.GFAmount), qr.Currency))
			if err != nil {
				return nil, err
			}
			q.Fees = append(q.Fees, &Fee{
				Name:       "GrandFathering",
				Amount:     uint64(gf.Amount()),
				Currency:   gf.Currency().Code,
				Refundable: &refundable,
			})
			gf = gf.Multiply(int64(qr.Years))
			q.Price, err = q.Price.Add(gf)
			if err != nil {
				return nil, err
			}
		}
		// if the domain is not grandfathered, we can return the price at this point
		return q, nil
	}

	// If the domain is not grandfathered, check if there is a premium price
	if len(pe.PremiumEntries) > 0 {
		for _, pl := range pe.PremiumEntries {
			if pl.Label == Label(pe.Domain.Name.Label()) {
				moneyToAdd, err := pl.GetMoney(qr.TransactionType)
				if err != nil {
					return nil, err
				}
				q.Fees = append(q.Fees, &Fee{
					Name:       "premium fee",
					Amount:     uint64(moneyToAdd.Amount()),
					Currency:   moneyToAdd.Currency().Code,
					Refundable: &refundable,
				})
				moneyToAdd = moneyToAdd.Multiply(int64(qr.Years))
				q.Class = pl.Class
				if pl.Currency == qr.Currency {
					q.Price, err = q.Price.Add(moneyToAdd)
					if err != nil {
						return nil, err
					}
				} else {
					moneyToAddInCorrectCurrency, err := pe.FXRate.Convert(moneyToAdd)
					if err != nil {
						return nil, err
					}
					q.Price, err = q.Price.Add(moneyToAddInCorrectCurrency)
					if err != nil {
						return nil, err
					}
				}
			}
		}
		// if we found a premium price, we can return the price at this point
		return q, nil
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
		priceMoneyToAdd = priceMoneyToAdd.Multiply(int64(qr.Years))
		q.Fees = append(q.Fees, &Fee{
			Name:       ClIDType(fmt.Sprintf("%s fee", qr.TransactionType)),
			Amount:     uint64(priceMoneyToAdd.Amount()),
			Currency:   price.Currency,
			Refundable: &refundable,
		})
		if needsFX {
			priceMoneyToAdd, err = pe.FXRate.Convert(priceMoneyToAdd)
			if err != nil {
				return nil, err
			}
		}
		q.Price, err = q.Price.Add(priceMoneyToAdd)
		if err != nil {
			return nil, err
		}
		// if we found a price, we can return the price at this point
		return q, nil
	}

	// If we haven't got a price at this point, none is available so we're assuming free (0)
	// we can just retutn the quote as the fees are already applied
	return q, nil
}
