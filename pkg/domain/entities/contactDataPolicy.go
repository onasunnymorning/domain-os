package entities

type ContactDataPolicy struct {
	RegistrantContactDataPolicy ContactDataPolicyType `json:"registrantContactDataPolicy,omitempty" example:"mandatory"`
	TechContactDataPolicy       ContactDataPolicyType `json:"techContactDataPolicy,omitempty" example:"mandatory"`
	AdminContactDataPolicy      ContactDataPolicyType `json:"adminContactDataPolicy,omitempty" example:"optional"`
	BillingContactDataPolicy    ContactDataPolicyType `json:"billingContactDataPolicy,omitempty" example:"prohibited"`
}

// ContactDataPolicyType is a type for the contact policy of a TLD phase
type ContactDataPolicyType string

// ContactPolicy value object consists of all the settings of a TLD that can be changed in a phase roll
const (
	ContactDataPolicyTypeMandatory  = ContactDataPolicyType("mandatory")
	ContactDataPolicyTypeOptional   = ContactDataPolicyType("optional")
	ContactDataPolicyTypeProhibited = ContactDataPolicyType("prohibited")
)

// ContactDataPolicy factory. This returns a new ContactPolicy object with default values (Registrant and Tech are required, Admin and Billing are optional, this aligns with the 2025 RDP for gTLDs)
func NewContactDataPolicy() ContactDataPolicy {
	return ContactDataPolicy{
		RegistrantContactDataPolicy: ContactDataPolicyTypeMandatory,
		TechContactDataPolicy:       ContactDataPolicyTypeMandatory,
		AdminContactDataPolicy:      ContactDataPolicyTypeOptional,
		BillingContactDataPolicy:    ContactDataPolicyTypeOptional,
	}
}
