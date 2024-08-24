package postgres

import (
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/require"
)

func getValidRegistrar() *entities.Registrar {
	cr, _ := time.Parse(time.RFC3339, "2021-01-01T00:00:00Z")
	r := &entities.Registrar{
		ClID:        entities.ClIDType("my-registrar-id"),
		Name:        "My Registrar's Name",
		NickName:    "MyReg",
		GurID:       119,
		Email:       "g@me.com",
		Status:      entities.RegistrarStatus(entities.RegistrarStatusOK),
		Autorenew:   true,
		Voice:       entities.E164Type("+1.12345678"),
		Fax:         entities.E164Type("+1.987654321"),
		URL:         entities.URL("http://myregistrar.com"),
		RdapBaseURL: entities.URL("http://myregistrar.com/rdap"),
		WhoisInfo: entities.WhoisInfo{
			Name: entities.DomainName("whois.myregistrar.com"),
			URL:  entities.URL("http://whois.myregistrar.com"),
		},
		CreatedAt: cr,
		UpdatedAt: cr,
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

	r.AccreditFor(&entities.TLD{
		Name: entities.DomainName("com"),
	})
	r.AccreditFor(&entities.TLD{
		Name: entities.DomainName("net"),
	})

	return r
}

func TestToDBRegistrar(t *testing.T) {
	r := getValidRegistrar()

	dbRar := ToDBRegistrar(r)

	r2 := FromDBRegistrar(dbRar)

	require.Equal(t, r, r2)

}
