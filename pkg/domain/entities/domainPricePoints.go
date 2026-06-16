package entities

// DomainPricePoints represents the all the price points for a domain.
type DomainPricePoints struct {
	Price          *Price
	Fees           []Fee
	PremiumPrice   *PremiumLabel
	GrandFathering *DomainGrandFathering
	FX             *FX
}
