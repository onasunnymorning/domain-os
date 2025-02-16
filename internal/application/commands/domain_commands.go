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

// ApplyContactDataPolicy modifies the command’s registrant, admin, tech, and billing
// contact IDs according to the provided contact data policy. It returns an error if
// the operation fails due to invalid or missing data.
func (cmd *RegisterDomainCommand) ApplyContactDataPolicy(policy entities.ContactDataPolicy) error {
	return applyContactDataPolicy(
		policy,
		&cmd.RegistrantID,
		&cmd.AdminID,
		&cmd.TechID,
		&cmd.BillingID,
	)
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
	Currency string `json:"Currency"`
	Amount   int64  `json:"Amount"`
}

// IsZero checks if the FeeExtension instance is equal to the zero value of FeeExtension.
// It returns true if the instance is a zero value, otherwise false.
func (f FeeExtension) IsZero() bool {
	return f == FeeExtension{}
}

// CreateDomainCommand is a command to create a domain. This is intended for admin or import purposes. Normal Registrar transactions should use the RegisterDomainCommand and RenewDomainCommand ...
type CreateDomainCommand struct {
	RoID               string                        `json:"RoID"` // if not provided, it will be generated
	Name               string                        `json:"Name" binding:"required"`
	OriginalName       string                        `json:"OriginalName"`
	UName              string                        `json:"UName"`
	RegistrantID       string                        `json:"RegistrantID"`
	AdminID            string                        `json:"AdminID"`
	TechID             string                        `json:"TechID"`
	BillingID          string                        `json:"BillingID"`
	ClID               string                        `json:"ClID" binding:"required"`
	CrRr               string                        `json:"CrRr"`
	UpRr               string                        `json:"UpRr"`
	ExpiryDate         time.Time                     `json:"ExpiryDate" binding:"required"`
	DropCatch          bool                          `json:"DropCatch"`
	RenewedYears       int                           `json:"RenewedYears"`
	AuthInfo           string                        `json:"AuthInfo"  binding:"required"`
	CreatedAt          time.Time                     `json:"CreatedAt"`
	UpdatedAt          time.Time                     `json:"UpdatedAt"`
	Status             entities.DomainStatus         `json:"Status"`
	RGPStatus          entities.DomainRGPStatus      `json:"RGPStatus"`
	GrandFathering     entities.DomainGrandFathering `json:"GrandFathering"`
	EnforcePhasePolicy bool                          `json:"EnforcePhasePolicy"`
}

// ApplyContactDataPolicy modifies the command’s registrant, admin, tech, and billing
// contact IDs according to the provided contact data policy. It returns an error if
// the operation fails due to invalid or missing data.
func (cmd *CreateDomainCommand) ApplyContactDataPolicy(policy entities.ContactDataPolicy) error {
	return applyContactDataPolicy(
		policy,
		&cmd.RegistrantID,
		&cmd.AdminID,
		&cmd.TechID,
		&cmd.BillingID,
	)
}

type FromRdeDomainResult struct {
	Warnings []string
}

// FromRdeDomain creates a CreateDomainCommand from an RdeDomain
func (cmd *CreateDomainCommand) FromRdeDomain(rdeDomain *entities.RDEDomain) (*FromRdeDomainResult, error) {
	// Check if we have a valid RoID (this will only be the case if we are importing our own escrows).
	// If the Roid is invalid, use a valid one to pass through domain validation and unset it in the final command to have one generated.
	roid := entities.RoidType(rdeDomain.RoID)
	if roid.Validate() != nil || roid.ObjectIdentifier() != "DOM" {
		// set a dummy valid RoID to pass through domain validation
		rdeDomain.RoID = "1_DOM-APEX"
	}

	// Create the domain entity from our RDEDomain, this will validate the domain
	result, err := rdeDomain.ToEntity()
	if err != nil {
		return nil, err
	}
	if result.Domain == nil {
		return nil, errors.New("RdeDomain.ToEmtitY() returned no error and nil Domain")
	}

	var finalResult = FromRdeDomainResult{}
	if result.Warnings != nil {
		for _, warning := range result.Warnings {
			finalResult.Warnings = append(finalResult.Warnings, warning.Error())
		}
	}

	dom := result.Domain

	// Now that we have a a valid domain, convert it to a command
	// Only set the RoID if it is not the dummy RoID
	if dom.RoID.String() != "1_DOM-APEX" {
		cmd.RoID = dom.RoID.String()
	}
	cmd.Name = dom.Name.String()
	cmd.OriginalName = dom.OriginalName.String()
	cmd.UName = dom.UName.String()
	if dom.RegistrantID == "" {
		return nil, ErrRegistrantIDNotSet
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

	return &finalResult, nil
}

// UpdateDomainCommand is a command to update a domain. RoID and Name are not updatable, please delete and create a new domain if you need to change these fields
type UpdateDomainCommand struct {
	OriginalName       string                        `json:"OriginalName"`
	UName              string                        `json:"UName"`
	RegistrantID       string                        `json:"RegistrantID"`
	AdminID            string                        `json:"AdminID"`
	TechID             string                        `json:"TechID"`
	BillingID          string                        `json:"BillingID"`
	ClID               string                        `json:"ClID" binding:"required"`
	CrRr               string                        `json:"CrRr"`
	UpRr               string                        `json:"UpRr"`
	ExpiryDate         time.Time                     `json:"ExpiryDate" binding:"required"`
	DropCatch          bool                          `json:"DropCatch"`
	AuthInfo           string                        `json:"AuthInfo"  binding:"required"`
	CreatedAt          time.Time                     `json:"CreatedAt"`
	UpdatedAt          time.Time                     `json:"UpdatedAt"`
	Status             entities.DomainStatus         `json:"Status"`
	RGPStatus          entities.DomainRGPStatus      `json:"RGPStatus"`
	GrandFathering     entities.DomainGrandFathering `json:"GrandFathering"`
	EnforcePhasePolicy bool                          `json:"EnforcePhasePolicy"`
}

// ApplyContactDataPolicy modifies the command’s registrant, admin, tech, and billing
// contact IDs according to the provided contact data policy. It returns an error if
// the operation fails due to invalid or missing data.
func (cmd *UpdateDomainCommand) ApplyContactDataPolicy(policy entities.ContactDataPolicy) error {
	return applyContactDataPolicy(
		policy,
		&cmd.RegistrantID,
		&cmd.AdminID,
		&cmd.TechID,
		&cmd.BillingID,
	)
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

// applyContactDataPolicy enforces the appropriate contact data policy rules for the
// specified registrantID, adminID, techID, and billingID fields. It ensures that
// mandatory fields are set, returning an error if any mandatory field is empty,
// and clears prohibited fields according to the provided policy.
func applyContactDataPolicy(
	policy entities.ContactDataPolicy,
	registrantID, adminID, techID, billingID *string,
) error {
	// -- Fail fast for mandatory fields --
	if policy.RegistrantContactDataPolicy == entities.ContactDataPolicyTypeMandatory && *registrantID == "" {
		return entities.ErrRegistrantIDRequiredButNotSet
	}
	if policy.AdminContactDataPolicy == entities.ContactDataPolicyTypeMandatory && *adminID == "" {
		return entities.ErrAdminIDRequiredButNotSet
	}
	if policy.TechContactDataPolicy == entities.ContactDataPolicyTypeMandatory && *techID == "" {
		return entities.ErrTechIDRequiredButNotSet
	}
	if policy.BillingContactDataPolicy == entities.ContactDataPolicyTypeMandatory && *billingID == "" {
		return entities.ErrBillingIDRequiredButNotSet
	}

	// -- Empty out prohibited fields --
	if policy.RegistrantContactDataPolicy == entities.ContactDataPolicyTypeProhibited {
		*registrantID = ""
	}
	if policy.AdminContactDataPolicy == entities.ContactDataPolicyTypeProhibited {
		*adminID = ""
	}
	if policy.TechContactDataPolicy == entities.ContactDataPolicyTypeProhibited {
		*techID = ""
	}
	if policy.BillingContactDataPolicy == entities.ContactDataPolicyTypeProhibited {
		*billingID = ""
	}

	return nil
}
