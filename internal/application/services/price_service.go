package services

import "github.com/onasunnymorning/domain-os/internal/domain/repositories"

// PriceService is the implementation of the PriceService interface
type PriceService struct {
	priceRepo repositories.PriceRepository
}

// NewPriceService returns a new instance of PriceService
func NewPriceService(priceRepo repositories.PriceRepository) *PriceService {
	return &PriceService{
		priceRepo: priceRepo,
	}
}
