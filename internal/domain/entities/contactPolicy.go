package entities

type ContactPolicy struct {
	RegistrantContactPolicy ContactPolicyType `json:"registrantContactPolicy,omitempty" example:"required"`
	TechContactPolicy       ContactPolicyType `json:"techContactPolicy,omitempty" example:"required"`
	AdminContactPolicy      ContactPolicyType `json:"adminContactPolicy,omitempty" example:"optional"`
	BillingContactPolicy    ContactPolicyType `json:"billingContactPolicy,omitempty" example:"prohibited"`
}

// ContactPolicyType is a type for the contact policy of a TLD phase
type ContactPolicyType string

// ContactPolicy value object consists of all the settings of a TLD that can be changed in a phase roll
const (
	ContactPolicyTypeRequired   = ContactPolicyType("required")
	ContactPolicyTypeOptional   = ContactPolicyType("optional")
	ContactPolicyTypeProhibited = ContactPolicyType("prohibited")
)

// ContactPolicy factory. This returns a new ContactPolicy object with default values (Registrant and Tech are required, Admin and Billing are optional)
func NewContactPolicy() ContactPolicy {
	return ContactPolicy{
		RegistrantContactPolicy: ContactPolicyTypeRequired,
		TechContactPolicy:       ContactPolicyTypeRequired,
		AdminContactPolicy:      ContactPolicyTypeOptional,
		BillingContactPolicy:    ContactPolicyTypeOptional,
	}
}
