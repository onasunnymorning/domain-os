package interfaces

import "github.com/onasunnymorning/domain-os/internal/domain/entities"

type IANAXMLService interface {
	ListIANARegistrars() ([]*entities.IANARegistrar, error)
}
