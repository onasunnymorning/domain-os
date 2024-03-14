package postgres

import "github.com/onasunnymorning/domain-os/internal/domain/entities"

func ToDBRegistrar(r *entities.Registrar) *Registrar {
	rar := &Registrar{
		ClID:        r.ClID.String(),
		Name:        r.Name,
		NickName:    r.NickName,
		GurID:       r.GurID,
		Email:       r.Email,
		Status:      r.Status.String(),
		Voice:       r.Voice.String(),
		Fax:         r.Fax.String(),
		URL:         r.URL.String(),
		Whois43:     r.WhoisInfo.Name.String(),
		Whois80:     r.WhoisInfo.URL.String(),
		RdapBaseUrl: r.RdapBaseURL.String(),
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}

	if r.PostalInfo[0] != nil {
		if r.PostalInfo[0].Address != nil {
			rar.Street1Int = r.PostalInfo[0].Address.Street1.String()
			rar.Street2Int = r.PostalInfo[0].Address.Street2.String()
			rar.Street3Int = r.PostalInfo[0].Address.Street3.String()
			rar.CityInt = r.PostalInfo[0].Address.City.String()
			rar.SPInt = r.PostalInfo[0].Address.StateProvince.String()
			rar.PCInt = r.PostalInfo[0].Address.PostalCode.String()
			rar.CCInt = r.PostalInfo[0].Address.CountryCode.String()
		}
	}

	if r.PostalInfo[1] != nil {
		if r.PostalInfo[1].Address != nil {
			rar.Street1Loc = r.PostalInfo[1].Address.Street1.String()
			rar.Street2Loc = r.PostalInfo[1].Address.Street2.String()
			rar.Street3Loc = r.PostalInfo[1].Address.Street3.String()
			rar.CityLoc = r.PostalInfo[1].Address.City.String()
			rar.SPLoc = r.PostalInfo[1].Address.StateProvince.String()
			rar.PCLoc = r.PostalInfo[1].Address.PostalCode.String()
			rar.CCLoc = r.PostalInfo[1].Address.CountryCode.String()
		}
	}

	return rar
}

func FromDBRegistrar(dbr *Registrar) *entities.Registrar {
	registrar := &entities.Registrar{
		ClID:     entities.ClIDType(dbr.ClID),
		Name:     dbr.Name,
		NickName: dbr.NickName,
		GurID:    dbr.GurID,
		Status:   entities.RegistrarStatus(dbr.Status),
		Voice:    entities.E164Type(dbr.Voice),
		Fax:      entities.E164Type(dbr.Fax),
		Email:    dbr.Email,
		WhoisInfo: entities.WhoisInfo{
			Name: entities.DomainName(dbr.Whois43),
			URL:  entities.URL(dbr.Whois80),
		},
		URL:         entities.URL(dbr.URL),
		RdapBaseURL: entities.URL(dbr.RdapBaseUrl),
		CreatedAt:   dbr.CreatedAt,
		UpdatedAt:   dbr.UpdatedAt,
	}

	a0 := &entities.Address{
		Street1:       entities.OptPostalLineType(dbr.Street1Int),
		Street2:       entities.OptPostalLineType(dbr.Street2Int),
		Street3:       entities.OptPostalLineType(dbr.Street3Int),
		City:          entities.PostalLineType(dbr.CityInt),
		StateProvince: entities.OptPostalLineType(dbr.SPInt),
		PostalCode:    entities.PCType(dbr.PCInt),
		CountryCode:   entities.CCType(dbr.CCInt),
	}

	pi0 := &entities.RegistrarPostalInfo{
		Type:    entities.PostalInfoEnumType("int"),
		Address: a0,
	}

	registrar.AddPostalInfo(pi0)

	a1 := &entities.Address{
		Street1:       entities.OptPostalLineType(dbr.Street1Loc),
		Street2:       entities.OptPostalLineType(dbr.Street2Loc),
		Street3:       entities.OptPostalLineType(dbr.Street3Loc),
		City:          entities.PostalLineType(dbr.CityLoc),
		StateProvince: entities.OptPostalLineType(dbr.SPLoc),
		PostalCode:    entities.PCType(dbr.PCLoc),
		CountryCode:   entities.CCType(dbr.CCLoc),
	}

	pi1 := &entities.RegistrarPostalInfo{
		Type:    entities.PostalInfoEnumType("loc"),
		Address: a1,
	}

	registrar.AddPostalInfo(pi1)

	return registrar
}
