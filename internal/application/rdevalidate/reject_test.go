package rdevalidate

import (
	"testing"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/assert"
)

// A contact that fails to build comes back from NewContact wrapped in
// ErrInvalidContact, so the generic rule must not shadow the specific one
// that says what was wrong with it.
func TestEntityRuleContact(t *testing.T) {
	validContact := func() entities.RDEContact {
		return entities.RDEContact{
			ID:    "RSADMIN",
			RoID:  "C50983139-CO",
			Email: "regops@registry.godaddy",
			ClID:  "H1625",
			Status: []entities.RDEContactStatus{
				{S: "ok"},
			},
			PostalInfo: []entities.RDEContactPostalInfo{{
				Type: "int",
				Name: "RS LLC Marketing Administrator",
				Address: entities.RDEAddress{
					Street: []string{"100 S. Mill Ave"}, City: "Tempe", CountryCode: "US",
				},
			}},
		}
	}

	t.Run("a deposit's valid contact is not rejected", func(t *testing.T) {
		c := validContact()
		_, err := c.ToEntity()
		assert.NoError(t, err)
	})

	t.Run("two addresses in one email element name the email rule", func(t *testing.T) {
		c := validContact()
		c.Email = "regops@registry.godaddy,regops@reigstry.godaddy"
		_, err := c.ToEntity()
		assert.Equal(t, "email: is not a valid address", entityRule(err))
	})

	t.Run("a bad domain contact type still names the type rule", func(t *testing.T) {
		assert.Equal(t, "contact: type attribute must be admin, tech or billing", entityRule(entities.ErrInvalidDomainContactType))
	})

	t.Run("a contact failure nothing more specific names falls to the object catch-all", func(t *testing.T) {
		assert.Equal(t, "the assembled contact failed its final validation", entityRule(entities.ErrInvalidContact))
	})
}
