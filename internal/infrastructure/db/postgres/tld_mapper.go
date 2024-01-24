package postgres

import (
	"github.com/onasunnymorning/registry-os/internal/domain/entities"
)

// ToDBTLD converts a TLD struct to a DBTLD struct
func ToDBTLD(tld *entities.TLD) *TLD {
	return &TLD{
		Name:      tld.Name.String(),
		Type:      tld.Type.String(),
		UName:     tld.UName,
		CreatedAt: tld.CreatedAt,
		UpdatedAt: tld.UpdatedAt,
	}
}

// FromDBTLD converts a DBTLD struct to a TLD struct
func FromDBTLD(dbtld *TLD) *entities.TLD {
	tld := &entities.TLD{
		Name:      entities.DomainName(dbtld.Name),
		Type:      entities.TLDType(dbtld.Type),
		UName:     dbtld.UName,
		CreatedAt: dbtld.CreatedAt.UTC(),
		UpdatedAt: dbtld.UpdatedAt.UTC(),
	}
	return tld
}
