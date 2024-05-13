package commands

import (
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// RegisterDomainCommand is a command to register a domain
type RegisterDomainCommand struct {
	Name         string       `json:"Name" binding:"required"`
	ClID         string       `json:"ClID" binding:"required"`
	AuthInfo     string       `json:"AuthInfo"  binding:"required"`
	RegistrantID string       `json:"RegistrantID" binding:"required"` // Contacts must exist before registering a domain
	AdminID      string       `json:"AdminID" binding:"required"`      // Contacts must exist before registering a domain
	TechID       string       `json:"TechID" binding:"required"`       // Contacts must exist before registering a domain
	BillingID    string       `json:"BillingID" binding:"required"`    // Contacts must exist before registering a domain
	Years        int          `json:"Years"`                           // if not provided, it will be 1
	HostNames    []string     `json:"HostNames"`                       // HostNames must exist before registering a domain
	PhaseName    string       `json:"PhaseName"`                       // Optional, if provided the domain will be registered (and validated) in this phase, if omitted the active GA phase will be used
	Fee          FeeExtension `json:"Fee"`                             // Optional, if provided must match the calculated fee, if not provided the fee calculated fee will be used regardless of the amount or class
}

// RenewDomainCommand is a command to renew a domain
type RenewDomainCommand struct {
	Name  string       `json:"Name" binding:"required"`
	ClID  string       `json:"ClID" binding:"required"`
	Years int          `json:"Years"` // if not provided, it will be 1
	Fee   FeeExtension `json:"Fee"`   // Optional, if provided must match the calculated fee, if not provided the fee calculated fee will be used regardless of the amount or class
}

// FeeExtension is a struct that can optionally be included in commands to provide information about the price
type FeeExtension struct {
	Currency string  `json:"Currency"`
	Amount   float64 `json:"Amount"`
}

// CreateDomainCommand is a command to create a domain. This is intended for admin or import purposes. Normal Registrar operations should use the RegisterDomainCommand and RenewDomainCommand ...
type CreateDomainCommand struct {
	RoID         string                   `json:"RoID"` // if not provided, it will be generated
	Name         string                   `json:"Name" binding:"required"`
	OriginalName string                   `json:"OriginalName"`
	UName        string                   `json:"UName"`
	RegistrantID string                   `json:"RegistrantID" binding:"required"`
	AdminID      string                   `json:"AdminID" binding:"required"`
	TechID       string                   `json:"TechID" binding:"required"`
	BillingID    string                   `json:"BillingID" binding:"required"`
	ClID         string                   `json:"ClID" binding:"required"`
	CrRr         string                   `json:"CrRr"`
	UpRr         string                   `json:"UpRr"`
	ExpiryDate   time.Time                `json:"ExpiryDate" binding:"required"`
	AuthInfo     string                   `json:"AuthInfo"  binding:"required"`
	CreatedAt    time.Time                `json:"CreatedAt"`
	UpdatedAt    time.Time                `json:"UpdatedAt"`
	Status       entities.DomainStatus    `json:"Status"`
	RGPStatus    entities.DomainRGPStatus `json:"RGPStatus"`
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
	cmd.RegistrantID = dom.RegistrantID.String()
	cmd.AdminID = dom.AdminID.String()
	cmd.TechID = dom.TechID.String()
	cmd.BillingID = dom.BillingID.String()
	cmd.ClID = dom.ClID.String()
	cmd.CrRr = dom.CrRr.String()
	cmd.UpRr = dom.UpRr.String()
	cmd.ExpiryDate = dom.ExpiryDate
	cmd.AuthInfo = dom.AuthInfo.String()
	cmd.CreatedAt = dom.CreatedAt
	cmd.UpdatedAt = dom.UpdatedAt
	cmd.Status = dom.Status
	cmd.RGPStatus = dom.RGPStatus

	return nil
}

// UpdateDomainCommand is a command to update a domain. RoID and Name are not updatable, please delete and create a new domain if you need to change these fields
type UpdateDomainCommand struct {
	OriginalName string                   `json:"OriginalName"`
	UName        string                   `json:"UName"`
	RegistrantID string                   `json:"RegistrantID" binding:"required"`
	AdminID      string                   `json:"AdminID" binding:"required"`
	TechID       string                   `json:"TechID" binding:"required"`
	BillingID    string                   `json:"BillingID" binding:"required"`
	ClID         string                   `json:"ClID" binding:"required"`
	CrRr         string                   `json:"CrRr"`
	UpRr         string                   `json:"UpRr"`
	ExpiryDate   time.Time                `json:"ExpiryDate" binding:"required"`
	AuthInfo     string                   `json:"AuthInfo"  binding:"required"`
	CreatedAt    time.Time                `json:"CreatedAt"`
	UpdatedAt    time.Time                `json:"UpdatedAt"`
	Status       entities.DomainStatus    `json:"Status"`
	RGPStatus    entities.DomainRGPStatus `json:"RGPStatus"`
}
