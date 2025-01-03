package commands

import (
	"errors"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

var (
	ErrRegistrantIDNotSet = errors.New("RegistrantID must be set")
)

// RegisterDomainCommand is a command to register a domain
type RegisterDomainCommand struct {
	Name         string       `json:"Name" binding:"required"`
	ClID         string       `json:"ClID" binding:"required"`
	AuthInfo     string       `json:"AuthInfo"  binding:"required"`
	RegistrantID string       `json:"RegistrantID"` // Contacts must exist before registering a domain
	AdminID      string       `json:"AdminID"`      // Contacts must exist before registering a domain
	TechID       string       `json:"TechID"`       // Contacts must exist before registering a domain
	BillingID    string       `json:"BillingID"`    // Contacts must exist before registering a domain
	Years        int          `json:"Years"`        // if not provided, it will be 1
	HostNames    []string     `json:"HostNames"`    // HostNames must exist before registering a domain
	PhaseName    string       `json:"PhaseName"`    // Optional, if provided the domain will be registered (and validated) in this phase, if omitted the active GA phase will be used
	Fee          FeeExtension `json:"Fee"`          // Optional, if provided must match the calculated fee, if not provided the fee calculated fee will be used regardless of the amount or class
}

// ApplyContactDataPolicy applies the contact data policy to the command's properties:
// Prohibited contact IDs will be set to an empty string
// Mandatory contact IDs must not be empty strings or an error will be returned
// Optional contact IDs can be either empty strings or set to a valid contact ID
// NOTE: This function will not validate if the contact with the specified ID exists, this is the responsibility of the repository layer which will enforce a FK constraint
func (cmd *RegisterDomainCommand) ApplyContactDataPolicy(policy entities.ContactDataPolicy) error {
	// Fail first
	if policy.RegistrantContactDataPolicy == entities.ContactDataPolicyTypeMandatory && cmd.RegistrantID == "" {
		return entities.ErrRegistrantIDRequiredButNotSet
	}
	if policy.AdminContactDataPolicy == entities.ContactDataPolicyTypeMandatory && cmd.AdminID == "" {
		return entities.ErrAdminIDRequiredButNotSet
	}
	if policy.TechContactDataPolicy == entities.ContactDataPolicyTypeMandatory && cmd.TechID == "" {
		return entities.ErrTechIDRequiredButNotSet
	}
	if policy.BillingContactDataPolicy == entities.ContactDataPolicyTypeMandatory && cmd.BillingID == "" {
		return entities.ErrBillingIDRequiredButNotSet
	}

	// Empty the prohibited fields
	if policy.RegistrantContactDataPolicy == entities.ContactDataPolicyTypeProhibited {
		cmd.RegistrantID = ""
	}
	if policy.AdminContactDataPolicy == entities.ContactDataPolicyTypeProhibited {
		cmd.AdminID = ""
	}
	if policy.TechContactDataPolicy == entities.ContactDataPolicyTypeProhibited {
		cmd.TechID = ""
	}
	if policy.BillingContactDataPolicy == entities.ContactDataPolicyTypeProhibited {
		cmd.BillingID = ""
	}

	return nil
}

// RenewDomainCommand is a command to renew a domain
type RenewDomainCommand struct {
	Name  string       `json:"Name" binding:"required"`
	ClID  string       `json:"ClID" binding:"required"`
	Years int          `json:"Years"` // if not provided, it will be 1
	Fee   FeeExtension `json:"Fee"`   // Optional, if provided must match the calculated fee, if not provided, the renew is allowed and any cost
}

// FeeExtension is a struct that can optionally be included in commands to provide information about the price
type FeeExtension struct {
	Currency string  `json:"Currency"`
	Amount   float64 `json:"Amount"`
}

// CreateDomainCommand is a command to create a domain. This is intended for admin or import purposes. Normal Registrar operations should use the RegisterDomainCommand and RenewDomainCommand ...
type CreateDomainCommand struct {
	RoID           string                        `json:"RoID"` // if not provided, it will be generated
	Name           string                        `json:"Name" binding:"required"`
	OriginalName   string                        `json:"OriginalName"`
	UName          string                        `json:"UName"`
	RegistrantID   string                        `json:"RegistrantID"`
	AdminID        string                        `json:"AdminID"`
	TechID         string                        `json:"TechID"`
	BillingID      string                        `json:"BillingID"`
	ClID           string                        `json:"ClID" binding:"required"`
	CrRr           string                        `json:"CrRr"`
	UpRr           string                        `json:"UpRr"`
	ExpiryDate     time.Time                     `json:"ExpiryDate" binding:"required"`
	DropCatch      bool                          `json:"DropCatch"`
	RenewedYears   int                           `json:"RenewedYears"`
	AuthInfo       string                        `json:"AuthInfo"  binding:"required"`
	CreatedAt      time.Time                     `json:"CreatedAt"`
	UpdatedAt      time.Time                     `json:"UpdatedAt"`
	Status         entities.DomainStatus         `json:"Status"`
	RGPStatus      entities.DomainRGPStatus      `json:"RGPStatus"`
	GrandFathering entities.DomainGrandFathering `json:"GrandFathering"`
}

// FromRdeDomain creates a CreateDomainCommand from an RdeDomain
func (cmd *CreateDomainCommand) FromRdeDomain(rdeDomain *entities.RDEDomain) error {
	// Check if we have a valid RoID (this will only be the case if we are importing our own escrows).
	// If the Roid is invalid, use a valid one to pass through domain validation and unset it in the final command to have one generated.
	roid := entities.RoidType(rdeDomain.RoID)
	if roid.Validate() != nil || roid.ObjectIdentifier() != "DOM" {
		// set a dummy valid RoID to pass through domain validation
		rdeDomain.RoID = "1_DOM-APEX"
	}

	// Create the domain entity from our RDEDomain, this will validate the domain
	dom, err := rdeDomain.ToEntity()
	if err != nil {
		return err
	}

	// Now that we have a a valid domain, convert it to a command
	// Only set the RoID if it is not the dummy RoID
	if dom.RoID.String() != "1_DOM-APEX" {
		cmd.RoID = dom.RoID.String()
	}
	cmd.Name = dom.Name.String()
	cmd.OriginalName = dom.OriginalName.String()
	cmd.UName = dom.UName.String()
	if dom.RegistrantID == "" {
		return ErrRegistrantIDNotSet
	}
	cmd.RegistrantID = dom.RegistrantID.String()
	cmd.AdminID = dom.AdminID.String()
	if dom.AdminID == "" {
		// If we don't have an admin contact, use the registrant contact
		cmd.AdminID = dom.RegistrantID.String()
	}
	cmd.TechID = dom.TechID.String()
	if dom.TechID == "" {
		// If we don't have a tech contact, use the registrant contact
		cmd.TechID = dom.RegistrantID.String()
	}
	cmd.BillingID = dom.BillingID.String()
	if dom.BillingID == "" {
		// If we don't have a billing contact, use the registrant contact
		cmd.BillingID = dom.RegistrantID.String()
	}
	cmd.ClID = dom.ClID.String()
	cmd.CrRr = dom.CrRr.String()
	cmd.UpRr = dom.UpRr.String()
	cmd.ExpiryDate = dom.ExpiryDate
	cmd.AuthInfo = dom.AuthInfo.String()
	cmd.CreatedAt = dom.CreatedAt
	cmd.UpdatedAt = dom.UpdatedAt
	cmd.Status = dom.Status
	cmd.RGPStatus = dom.RGPStatus
	cmd.RenewedYears = dom.RenewedYears

	return nil
}

// UpdateDomainCommand is a command to update a domain. RoID and Name are not updatable, please delete and create a new domain if you need to change these fields
type UpdateDomainCommand struct {
	OriginalName   string                        `json:"OriginalName"`
	UName          string                        `json:"UName"`
	RegistrantID   string                        `json:"RegistrantID"`
	AdminID        string                        `json:"AdminID"`
	TechID         string                        `json:"TechID"`
	BillingID      string                        `json:"BillingID"`
	ClID           string                        `json:"ClID" binding:"required"`
	CrRr           string                        `json:"CrRr"`
	UpRr           string                        `json:"UpRr"`
	ExpiryDate     time.Time                     `json:"ExpiryDate" binding:"required"`
	DropCatch      bool                          `json:"DropCatch"`
	AuthInfo       string                        `json:"AuthInfo"  binding:"required"`
	CreatedAt      time.Time                     `json:"CreatedAt"`
	UpdatedAt      time.Time                     `json:"UpdatedAt"`
	Status         entities.DomainStatus         `json:"Status"`
	RGPStatus      entities.DomainRGPStatus      `json:"RGPStatus"`
	GrandFathering entities.DomainGrandFathering `json:"GrandFathering"`
}

// FromEntity converts a domain entity to an UpdateDomainCommand
func (cmd *UpdateDomainCommand) FromEntity(dom *entities.Domain) {
	cmd.OriginalName = dom.OriginalName.String()
	cmd.UName = dom.UName.String()
	cmd.RegistrantID = dom.RegistrantID.String()
	cmd.AdminID = dom.AdminID.String()
	cmd.TechID = dom.TechID.String()
	cmd.BillingID = dom.BillingID.String()
	cmd.ClID = dom.ClID.String()
	cmd.CrRr = dom.CrRr.String()
	cmd.UpRr = dom.UpRr.String()
	cmd.ExpiryDate = dom.ExpiryDate
	cmd.DropCatch = dom.DropCatch
	cmd.AuthInfo = dom.AuthInfo.String()
	cmd.CreatedAt = dom.CreatedAt
	cmd.UpdatedAt = dom.UpdatedAt
	cmd.Status = dom.Status
	cmd.RGPStatus = dom.RGPStatus
}
