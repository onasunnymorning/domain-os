package repositories

import "github.com/onasunnymorning/domain-os/internal/domain/entities"

type IANARepository interface {
	ListIANARegistrars() ([]*entities.IANARegistrar, error)
}
