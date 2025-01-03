package entities

type ContactDataPolicy struct {
	RegistrantContactDataPolicy ContactDataPolicyType `json:"registrantDataContactPolicy,omitempty" example:"required"`
	TechContactDataPolicy       ContactDataPolicyType `json:"techContactDataPolicy,omitempty" example:"required"`
	AdminContactDataPolicy      ContactDataPolicyType `json:"adminContactDataPolicy,omitempty" example:"optional"`
	BillingContactDataPolicy    ContactDataPolicyType `json:"billingContactDataPolicy,omitempty" example:"prohibited"`
}

// ContactDataPolicyType is a type for the contact policy of a TLD phase
type ContactDataPolicyType string

// ContactPolicy value object consists of all the settings of a TLD that can be changed in a phase roll
const (
	ContactDataPolicyTypeRequired   = ContactDataPolicyType("required")
	ContactDataPolicyTypeOptional   = ContactDataPolicyType("optional")
	ContactDataPolicyTypeProhibited = ContactDataPolicyType("prohibited")
)

// ContactPolicy factory. This returns a new ContactPolicy object with default values (Registrant and Tech are required, Admin and Billing are optional)
func NewContactPolicy() ContactDataPolicy {
	return ContactDataPolicy{
		RegistrantContactDataPolicy: ContactDataPolicyTypeRequired,
		TechContactDataPolicy:       ContactDataPolicyTypeRequired,
		AdminContactDataPolicy:      ContactDataPolicyTypeOptional,
		BillingContactDataPolicy:    ContactDataPolicyTypeOptional,
	}
}
