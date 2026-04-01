package entities

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewAddress(t *testing.T) {
	tests := []struct {
		name     string
		city     string
		cc       string
		expected *Address
		err      error
	}{
		{"uppercase CC", "Buenos Aires", "AR", &Address{City: PostalLineType("Buenos Aires"), CountryCode: CCType("AR")}, nil},
		{"lowercase CC", "Buenos Aires", "ar", &Address{City: PostalLineType("Buenos Aires"), CountryCode: CCType("AR")}, nil},
		{"mixed case CC", "Buenos Aires", "aR", &Address{City: PostalLineType("Buenos Aires"), CountryCode: CCType("AR")}, nil},
		{"mixed case CC", "Buenos Aires", "Ar", &Address{City: PostalLineType("Buenos Aires"), CountryCode: CCType("AR")}, nil},
		{"single letter CC", "Buenos Aires", "a", nil, ErrInvalidCountryCode},
		{"no CC", "Buenos Aires", "", nil, ErrInvalidCountryCode},
		{"three letter CC", "Buenos Aires", "USA", nil, ErrInvalidCountryCode},
		{"non existing CC", "Buenos Aires", "PP", nil, ErrInvalidCountryCode},
		{"missing name", "", "AR", nil, ErrInvalidPostalLineType},
		{"name too long", "12345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890", "AR", nil, ErrInvalidPostalLineType},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a, err := NewAddress(test.city, test.cc)
			require.Equal(t, test.err, err)
			require.Equal(t, test.expected, a)
		})
	}
}

func TestAddress_IsValid(t *testing.T) {
	tests := []struct {
		name  string
		a     *Address
		t     PostalInfoEnumType
		valid error
	}{
		{"empty address", &Address{}, PostalInfoEnumTypeINT, ErrInvalidPostalLineType},
		{"non existing CC", &Address{City: PostalLineType("El Cuyo"), CountryCode: CCType("PP")}, PostalInfoEnumTypeINT, ErrInvalidCountryCode},
		{"missing city", &Address{City: PostalLineType(""), CountryCode: CCType("MX")}, PostalInfoEnumTypeINT, ErrInvalidPostalLineType},
		{"valid", &Address{City: PostalLineType("El Cuyo"), CountryCode: CCType("MX")}, PostalInfoEnumTypeINT, nil},
		{"pctype too long", &Address{City: PostalLineType("El Cuyo"), CountryCode: CCType("MX"), PostalCode: PCType("12345678901234567890")}, PostalInfoEnumTypeINT, ErrInvalidPCType},
		{"street1 too long", &Address{City: PostalLineType("El Cuyo"), CountryCode: CCType("MX"), Street1: OptPostalLineType("12345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890")}, PostalInfoEnumTypeINT, ErrInvalidOptPostalLineType},
		{"street2 too long", &Address{City: PostalLineType("El Cuyo"), CountryCode: CCType("MX"), Street2: OptPostalLineType("12345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890")}, PostalInfoEnumTypeINT, ErrInvalidOptPostalLineType},
		{"street3 too long", &Address{City: PostalLineType("El Cuyo"), CountryCode: CCType("MX"), Street3: OptPostalLineType("12345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890")}, PostalInfoEnumTypeINT, ErrInvalidOptPostalLineType},
		{"state too long", &Address{City: PostalLineType("El Cuyo"), CountryCode: CCType("MX"), StateProvince: OptPostalLineType("12345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890")}, PostalInfoEnumTypeINT, ErrInvalidOptPostalLineType},
		{"valid LOC", &Address{City: PostalLineType("El Cüyo"), CountryCode: CCType("MX")}, PostalInfoEnumTypeLOC, nil},
		{"invalid INT street1", &Address{City: PostalLineType("El Cuyo"), CountryCode: CCType("MX"), Street1: OptPostalLineType("çavapas é")}, PostalInfoEnumTypeINT, ErrInvalidASCIIInIntAddress},
		{"invalid INT street2", &Address{City: PostalLineType("El Cuyo"), CountryCode: CCType("MX"), Street2: OptPostalLineType("çavapas é")}, PostalInfoEnumTypeINT, ErrInvalidASCIIInIntAddress},
		{"invalid INT street3", &Address{City: PostalLineType("El Cuyo"), CountryCode: CCType("MX"), Street3: OptPostalLineType("çavapas é")}, PostalInfoEnumTypeINT, ErrInvalidASCIIInIntAddress},
		{"invalid INT SP", &Address{City: PostalLineType("El Cuyo"), CountryCode: CCType("MX"), StateProvince: OptPostalLineType("çavapas é")}, PostalInfoEnumTypeINT, ErrInvalidASCIIInIntAddress},
		{"invalid INT PC", &Address{City: PostalLineType("El Cuyo"), CountryCode: CCType("MX"), PostalCode: PCType("çavapas é")}, PostalInfoEnumTypeINT, ErrInvalidASCIIInIntAddress},
		{"invalid INT CC", &Address{City: PostalLineType("El Cuyo"), CountryCode: CCType("çavapas é")}, PostalInfoEnumTypeINT, ErrInvalidCountryCode},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.valid, test.a.Validate(test.t))
		})
	}
}

func TestAddress_IsASCII(t *testing.T) {
	a := Address{
		City:        "El Cuyo",
		CountryCode: "Mü",
	}
	_, err := a.IsASCII()
	require.Equal(t, ErrInvalidASCIIInIntAddress, err, "Expected IsASCII to return ErrInvalidASCIIInIntAddress, but got %s", err)
}
func TestAddress_DeepCopy(t *testing.T) {
	original := Address{
		Street1:       OptPostalLineType("Boulnes 2545"),
		Street2:       OptPostalLineType("Piso8"),
		Street3:       OptPostalLineType("Portero"),
		City:          PostalLineType("Buenos Aires"),
		StateProvince: OptPostalLineType("Palermo SOHO"),
		PostalCode:    PCType("EN234Z"),
		CountryCode:   CCType("AR"),
	}

	copy := original.DeepCopy()

	require.Equal(t, original, copy, "Expected DeepCopy to return an identical Address")
	require.NotSame(t, &original, &copy, "Expected DeepCopy to return a different Address instance")
}
