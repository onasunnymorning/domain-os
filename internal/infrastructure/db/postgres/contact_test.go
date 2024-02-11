package postgres

import (
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/require"
)

func getValidContactEntity() *entities.Contact {
	c := &entities.Contact{
		ID:       entities.ClIDType("test"),
		RoID:     entities.RoidType("123123_CONT-APEX"),
		Voice:    entities.E164Type("+1.12345678"),
		Fax:      entities.E164Type("+1.987654321"),
		Email:    "g@me.com",
		ClID:     entities.ClIDType("my-registrar"),
		CrRr:     entities.ClIDType("my-registrar"),
		UpRr:     entities.ClIDType("my-registrar"),
		AuthInfo: entities.AuthInfoType("str0ngAUTH1nf*"),
	}

	a0 := &entities.Address{
		Street1:       entities.OptPostalLineType("Playa blanca"),
		Street2:       entities.OptPostalLineType("Calle 1"),
		Street3:       entities.OptPostalLineType("Casa 2"),
		City:          entities.PostalLineType("El Cuyo"),
		StateProvince: entities.OptPostalLineType("Yucatan"),
		PostalCode:    entities.PCType("12345"),
		CountryCode:   entities.CCType("MX"),
	}

	p0 := &entities.ContactPostalInfo{
		Name:    entities.PostalLineType("Felipe"),
		Org:     entities.OptPostalLineType("Rebeccas"),
		Type:    entities.PostalInfoEnumType("int"),
		Address: a0,
	}

	c.AddPostalInfo(p0)

	a1 := &entities.Address{
		Street1:       entities.OptPostalLineType("Plaüa blanca"),
		Street2:       entities.OptPostalLineType("Calle 1"),
		Street3:       entities.OptPostalLineType("Casa 2"),
		City:          entities.PostalLineType("El Cuyo"),
		StateProvince: entities.OptPostalLineType("Yucatan"),
		PostalCode:    entities.PCType("12345"),
		CountryCode:   entities.CCType("MX"),
	}

	p1 := &entities.ContactPostalInfo{
		Name:    entities.PostalLineType("Felipé"),
		Org:     entities.OptPostalLineType("Rébeccas"),
		Type:    entities.PostalInfoEnumType("loc"),
		Address: a1,
	}

	c.AddPostalInfo(p1)

	return c
}

func TestToDBContact(t *testing.T) {
	c := getValidContactEntity()

	dbContact := ToDBContact(c)

	c2 := FromDBContact(dbContact)

	require.Equal(t, c, c2)

}
