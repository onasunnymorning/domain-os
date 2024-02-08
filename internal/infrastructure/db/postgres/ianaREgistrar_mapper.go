package postgres

import "github.com/onasunnymorning/domain-os/internal/domain/entities"

// ToIanaRegistrar converts a DB IANARegistrar to a domain IANARegistrar
func ToIanaRegistrar(dbRegistrar *IANARegistrar) *entities.IANARegistrar {
	return &entities.IANARegistrar{
		GurID:     dbRegistrar.GurID,
		Name:      dbRegistrar.Name,
		Status:    entities.IANARegistrarStatus(dbRegistrar.Status),
		RdapURL:   dbRegistrar.RdapURL,
		CreatedAt: dbRegistrar.CreatedAt,
	}
}

// ToDBIANARegistrar converts a domain IANARegistrar to a DB IANARegistrar
func ToDBIANARegistrar(registrar *entities.IANARegistrar) *IANARegistrar {
	return &IANARegistrar{
		GurID:     registrar.GurID,
		Name:      registrar.Name,
		Status:    string(registrar.Status),
		RdapURL:   registrar.RdapURL,
		CreatedAt: registrar.CreatedAt,
	}
}
