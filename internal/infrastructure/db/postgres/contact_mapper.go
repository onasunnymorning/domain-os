package postgres

import "github.com/onasunnymorning/domain-os/internal/domain/entities"

// ToDBContact converts a domain entities.Contact to a database Contact
func ToDBContact(c *entities.Contact) *Contact {
	dbContact := &Contact{}
	roidInt, _ := c.RoID.Int64() // We don't handle errors here we just transform the domain entity to the database entity

	dbContact.ID = c.ID.String()
	dbContact.RoID = roidInt
	dbContact.Voice = c.Voice.String()
	dbContact.Fax = c.Fax.String()
	dbContact.Email = c.Email
	dbContact.ClID = c.ClID.String()
	dbContact.CrRr = c.CrRr.String()
	dbContact.UpRr = c.UpRr.String()
	dbContact.AuthInfo = c.AuthInfo.String()
	dbContact.CreatedAt = c.CreatedAt
	dbContact.UpdatedAt = c.UpdatedAt
	dbContact.ContactDisclose = c.ContactDisclose
	dbContact.ContactStatus = c.ContactStatus

	if c.PostalInfo[0] != nil {
		dbContact.NameInt = c.PostalInfo[0].Name.String()
		dbContact.OrgInt = c.PostalInfo[0].Org.String()
		if c.PostalInfo[0].Address != nil {
			dbContact.Street1Int = c.PostalInfo[0].Address.Street1.String()
			dbContact.Street2Int = c.PostalInfo[0].Address.Street2.String()
			dbContact.Street3Int = c.PostalInfo[0].Address.Street3.String()
			dbContact.CityInt = c.PostalInfo[0].Address.City.String()
			dbContact.SPInt = c.PostalInfo[0].Address.StateProvince.String()
			dbContact.PCInt = c.PostalInfo[0].Address.PostalCode.String()
			dbContact.CCInt = c.PostalInfo[0].Address.CountryCode.String()
		}
	}

	if c.PostalInfo[1] != nil {
		dbContact.NameLoc = c.PostalInfo[1].Name.String()
		dbContact.OrgLoc = c.PostalInfo[1].Org.String()
		if c.PostalInfo[1].Address != nil {
			dbContact.Street1Loc = c.PostalInfo[1].Address.Street1.String()
			dbContact.Street2Loc = c.PostalInfo[1].Address.Street2.String()
			dbContact.Street3Loc = c.PostalInfo[1].Address.Street3.String()
			dbContact.CityLoc = c.PostalInfo[1].Address.City.String()
			dbContact.SPLoc = c.PostalInfo[1].Address.StateProvince.String()
			dbContact.PCLoc = c.PostalInfo[1].Address.PostalCode.String()
			dbContact.CCLoc = c.PostalInfo[1].Address.CountryCode.String()
		}
	}

	return dbContact
}

// FromDBContact converts a database Contact to a domain entities.Contact
func FromDBContact(c *Contact) *entities.Contact {
	domainContact := &entities.Contact{}
	roidString, _ := entities.NewRoidType(c.RoID, entities.RoidTypeContact) // We don't handle errors here we just transform the database entity to the domain entity

	domainContact.ID = entities.ClIDType(c.ID)
	domainContact.RoID = roidString
	domainContact.Voice = entities.E164Type(c.Voice)
	domainContact.Fax = entities.E164Type(c.Fax)
	domainContact.Email = c.Email
	domainContact.ClID = entities.ClIDType(c.ClID)
	domainContact.CrRr = entities.ClIDType(c.CrRr)
	domainContact.UpRr = entities.ClIDType(c.UpRr)
	domainContact.AuthInfo = entities.AuthInfoType(c.AuthInfo)
	domainContact.CreatedAt = c.CreatedAt
	domainContact.UpdatedAt = c.UpdatedAt
	domainContact.ContactDisclose = c.ContactDisclose
	domainContact.ContactStatus = c.ContactStatus

	a0 := &entities.Address{
		Street1:       entities.OptPostalLineType(c.Street1Int),
		Street2:       entities.OptPostalLineType(c.Street2Int),
		Street3:       entities.OptPostalLineType(c.Street3Int),
		City:          entities.PostalLineType(c.CityInt),
		StateProvince: entities.OptPostalLineType(c.SPInt),
		PostalCode:    entities.PCType(c.PCInt),
		CountryCode:   entities.CCType(c.CCInt),
	}

	p0 := &entities.ContactPostalInfo{
		Name:    entities.PostalLineType(c.NameInt),
		Org:     entities.OptPostalLineType(c.OrgInt),
		Type:    entities.PostalInfoEnumType("int"),
		Address: a0,
	}

	domainContact.AddPostalInfo(p0)

	a1 := &entities.Address{
		Street1:       entities.OptPostalLineType(c.Street1Loc),
		Street2:       entities.OptPostalLineType(c.Street2Loc),
		Street3:       entities.OptPostalLineType(c.Street3Loc),
		City:          entities.PostalLineType(c.CityLoc),
		StateProvince: entities.OptPostalLineType(c.SPLoc),
		PostalCode:    entities.PCType(c.PCLoc),
		CountryCode:   entities.CCType(c.CCLoc),
	}

	p1 := &entities.ContactPostalInfo{
		Name:    entities.PostalLineType(c.NameLoc),
		Org:     entities.OptPostalLineType(c.OrgLoc),
		Type:    entities.PostalInfoEnumType("loc"),
		Address: a1,
	}

	domainContact.AddPostalInfo(p1)

	return domainContact
}
