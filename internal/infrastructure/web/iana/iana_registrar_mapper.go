package iana

import (
	"strconv"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// FromIANAXMLRecord converts a IANAXMLRegistryRecord struct to a IANARegistrar struct
func FromIANAXMLRegistrarRecord(record *RegistrarRecord) *entities.IANARegistrar {
	gurid, _ := strconv.Atoi(record.Value)
	return &entities.IANARegistrar{
		GurID:   gurid,
		Name:    record.Name,
		Status:  entities.IANARegistrarStatus(record.Status),
		RdapURL: record.RdapURL.Server,
	}
}

// ToIANAXMLRegistrarRecord converts a IANARegistrar struct to a IANAXMLRegistryRecord struct
func ToIANAXMLRegistrarRecord(registrar *entities.IANARegistrar) *RegistrarRecord {
	return &RegistrarRecord{
		Value:  strconv.Itoa(registrar.GurID),
		Name:   registrar.Name,
		Status: string(registrar.Status),
		RdapURL: RdapURL{
			Server: registrar.RdapURL,
		},
	}
}
