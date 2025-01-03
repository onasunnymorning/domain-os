package entities

type ContactDataPolicy struct {
	RegistrantContactPolicy ContactDataPolicyType `json:"registrantContactPolicy,omitempty" example:"required"`
	TechContactPolicy       ContactDataPolicyType `json:"techContactPolicy,omitempty" example:"required"`
	AdminContactPolicy      ContactDataPolicyType `json:"adminContactPolicy,omitempty" example:"optional"`
	BillingContactPolicy    ContactDataPolicyType `json:"billingContactPolicy,omitempty" example:"prohibited"`
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
		RegistrantContactPolicy: ContactDataPolicyTypeRequired,
		TechContactPolicy:       ContactDataPolicyTypeRequired,
		AdminContactPolicy:      ContactDataPolicyTypeOptional,
		BillingContactPolicy:    ContactDataPolicyTypeOptional,
	}
}
