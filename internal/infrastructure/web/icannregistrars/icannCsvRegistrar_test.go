package icannregistrars

import (
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/assert"
)

func TestCSVRegistrar_ContactName(t *testing.T) {
	tests := []struct {
		name     string
		contact  string
		expected string
	}{
		{"With plus sign", "John Doe +1234567890", "John Doe"},
		{"Without plus sign", "John Doe null", "John Doe"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := CSVRegistrar{PublicContact: tt.contact}
			assert.Equal(t, tt.expected, r.ContactName())
		})
	}
}

func TestCSVRegistrar_CreateSlug(t *testing.T) {
	tests := []struct {
		name      string
		registrar CSVRegistrar
		expected  string
	}{
		{"Basic case", CSVRegistrar{Name: "Example Registrar, Inc.", IANAID: 1234}, "1234-example-reg"},
		{"With special characters", CSVRegistrar{Name: "Example! Registrar, Inc.", IANAID: 1234}, "1234-example-reg"},
		{"With spaces", CSVRegistrar{Name: "Example Registrar", IANAID: 1234}, "1234-example-reg"},
		{"Long name", CSVRegistrar{Name: "Example Registrar with a very long name", IANAID: 1234}, "1234-example-reg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slug, err := tt.registrar.CreateSlug()
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, slug)
		})
	}
}

func TestCSVRegistrar_ContactPhone(t *testing.T) {
	tests := []struct {
		name     string
		contact  string
		expected string
	}{
		{"With phone number", "John Doe +123 4567890 me@my.com", "+123.4567890"},
		{"Without phone number", "John Doe me@my.com", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := CSVRegistrar{PublicContact: tt.contact}
			assert.Equal(t, tt.expected, r.ContactPhone())
		})
	}
}

func TestCSVRegistrar_ContactEmail(t *testing.T) {
	tests := []struct {
		name     string
		contact  string
		expected string
	}{
		{"With email", "John Doe +1234567890 john.doe@example.com", "john.doe@example.com"},
		{"Without email", "John Doe +1234567890", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := CSVRegistrar{PublicContact: tt.contact}
			assert.Equal(t, tt.expected, r.ContactEmail())
		})
	}
}

func TestCSVRegistrar_Address(t *testing.T) {
	tests := []struct {
		name      string
		registrar CSVRegistrar
		expected  *entities.Address
		expectErr bool
	}{
		{"Valid country", CSVRegistrar{Country: "United States"}, &entities.Address{City: "Washington", CountryCode: "US"}, false},
		{"Invalid country", CSVRegistrar{Country: "Invalid Country"}, nil, true},
		{"Special case - United Kingdom", CSVRegistrar{Country: "United Kingdom"}, &entities.Address{City: "London", CountryCode: "GB"}, false},
		{"Special case - Hong Kong", CSVRegistrar{Country: "Hong Kong"}, &entities.Address{City: "Hong Kong", CountryCode: "HK"}, false},
		{"Special case - Marshall Islands", CSVRegistrar{Country: "Marshall Islands"}, &entities.Address{City: "Majuro", CountryCode: "MH"}, false},
		{"Special case - Panama", CSVRegistrar{Country: "Panama"}, &entities.Address{City: "Panama City", CountryCode: "PA"}, false},
		{"Special case - Taipei", CSVRegistrar{Country: "Taipei"}, &entities.Address{City: "Taipei", CountryCode: "TW"}, false},
		{"Special case - IANAID 3874", CSVRegistrar{IANAID: 3874}, &entities.Address{City: "Singapore", CountryCode: "SG"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			address, err := tt.registrar.Address()
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, address)
			}
		})
	}
}
