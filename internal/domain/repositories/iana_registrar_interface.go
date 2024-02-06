package repositories

import "github.com/onasunnymorning/domain-os/internal/domain/entities"

type IANARegistrarRepository interface {
	ListIANARegistrars() ([]*entities.IANARegistrar, error)
}
