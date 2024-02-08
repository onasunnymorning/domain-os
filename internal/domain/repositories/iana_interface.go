package repositories

import "github.com/onasunnymorning/domain-os/internal/domain/entities"

// IANARepository is the interface for the IANARepository
type IANARepository interface {
	// List the registrars from the IANA Registrar List
	ListRegistrars() ([]*entities.IANARegistrar, error)
}
