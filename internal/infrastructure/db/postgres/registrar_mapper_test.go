package postgres

import (
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/require"
)

func getValidRegistrar() *entities.Registrar {
	r := &entities.Registrar{
		ClID:        entities.ClIDType("my-registrar-id"),
		Name:        "My Registrar's Name",
		NickName:    "MyReg",
		GurID:       119,
		Email:       "g@me.com",
		Status:      entities.RegistrarStatus("active"),
		Voice:       entities.E164Type("+1.12345678"),
		Fax:         entities.E164Type("+1.987654321"),
		URL:         entities.URL("http://myregistrar.com"),
		RdapBaseURL: entities.URL("http://myregistrar.com/rdap"),
		WhoisInfo: entities.WhoisInfo{
			Name: entities.DomainName("whois.myregistrar.com"),
			URL:  entities.URL("http://whois.myregistrar.com"),
		},
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

	p0 := &entities.RegistrarPostalInfo{
		Type:    entities.PostalInfoEnumType("int"),
		Address: a0,
	}

	r.AddPostalInfo(p0)

	a1 := &entities.Address{
		Street1:       entities.OptPostalLineType("Plaüa blanca"),
		Street2:       entities.OptPostalLineType("Calle 1"),
		Street3:       entities.OptPostalLineType("Casa 2"),
		City:          entities.PostalLineType("El Cuyo"),
		StateProvince: entities.OptPostalLineType("Yucatan"),
		PostalCode:    entities.PCType("12345"),
		CountryCode:   entities.CCType("MX"),
	}

	p1 := &entities.RegistrarPostalInfo{
		Type:    entities.PostalInfoEnumType("loc"),
		Address: a1,
	}

	r.AddPostalInfo(p1)

	return r
}

func TestToDBRegistrar(t *testing.T) {
	r := getValidRegistrar()

	dbRar := ToDBRegistrar(r)

	r2 := FromDBRegistrar(dbRar)

	require.Equal(t, r, r2)

}
