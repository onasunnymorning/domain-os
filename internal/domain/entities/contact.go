package entities

import (
	"net/mail"
	"time"

	"errors"
)

// ContactStatusType is a type for contact status
type ContactStatusType string

// String returns the string value of the ContactStatusType
func (c ContactStatusType) String() string {
	return string(c)
}

const (
	// ContactStatusOK is the status of the contact when it has no restrictions. Can only be combined with linked
	ContactStatusOK ContactStatusType = "ok"
	// ContactStatusLinked is the status of the contact when it is linked to a domain. Can be combined with any other status
	ContactStatusLinked ContactStatusType = "linked"
	// ContactStatusPendingCreate is the status of the contact when it is pending creation. Only one pendingX status can be set at a time
	ContactStatusPendingCreate ContactStatusType = "pendingCreate"
	// ContactStatusPendingUpdate is the status of the contact when it is pending update. Only one pendingX status can be set at a time
	ContactStatusPendingUpdate ContactStatusType = "pendingUpdate"
	// ContactStatusPendingTransfer is the status of the contact when it is pending transfer. Only one pendingX status can be set at a time
	ContactStatusPendingTransfer ContactStatusType = "pendingTransfer"
	// ContactStatusPendingDelete is the status of the contact when it is pending deletion. Only one pendingX status can be set at a time
	ContactStatusPendingDelete ContactStatusType = "pendingDelete"
	// ContactStatusClientDeleteProhibited is the status of the contact when it is prohibited from being deleted by the client. Cannot be combined with pendingDelete
	ContactStatusClientDeleteProhibited ContactStatusType = "clientDeleteProhibited"
	// ContactStatusClientUpdateProhibited is the status of the contact when it is prohibited from being updated by the client. Cannot be combined with pendingUpdate
	ContactStatusClientUpdateProhibited ContactStatusType = "clientUpdateProhibited"
	// ContactStatusClientTransferProhibited is the status of the contact when it is prohibited from being transferred by the client. Cannot be combined with pendingTransfer
	ContactStatusClientTransferProhibited ContactStatusType = "clientTransferProhibited"
	// ContactStatusServerDeleteProhibited is the status of the contact when it is prohibited from being deleted by the server. Cannot be combined with pendingDelete
	ContactStatusServerDeleteProhibited ContactStatusType = "serverDeleteProhibited"
	// ContactStatusServerUpdateProhibited is the status of the contact when it is prohibited from being updated by the server. Cannot be combined with pendingUpdate
	ContactStatusServerUpdateProhibited ContactStatusType = "serverUpdateProhibited"
	// ContactStatusServerTransferProhibited is the status of the contact when it is prohibited from being transferred by the server. Cannot be combined with pendingTransfer
	ContactStatusServerTransferProhibited ContactStatusType = "serverTransferProhibited"
)

var (
	ErrContactNotFound                 = errors.New("contact not found")
	ErrInvalidContact                  = errors.New("invalid contact")
	ErrContactAlreadyExists            = errors.New("contact already exists")
	ErrInvalidContactStatusCombination = errors.New("invalid combination of contact statuses")
	ErrContactUpdateNotAllowed         = errors.New("contact status prohibits update")
	ErrPostalInfoTypeExistsAlready     = errors.New("postalinfo of this type already exists")
)

// Contact is the contact Entity struct Based on https://www.rfc-editor.org/rfc/rfc5733#section-3.1.2
type Contact struct {
	ID         ClIDType              `json:"ID" example:"sh8013" extensions:"x-order=0"`                          // The contact identifier as supplied by the registrar, this should be used by all references to the contact
	RoID       RoidType              `json:"RoID" example:"1729468286778740736_CONT-APEX" extensions:"x-order=1"` // The generated id for the contact, has to be unique within the registry
	PostalInfo [2]*ContactPostalInfo `json:"PostalInfo"`                                                          // 1 required, 2 maximum. I prefer the approach to have an array of two optional items over a map because it makes manipulating (updating) easier
	Voice      E164Type              `json:"Voice" example:"+1.9567345623"`
	Fax        E164Type              `json:"Fax" example:"+1.9567345623"`
	Email      string                `json:"Email" example:"solutions@apex.domains"` // Required
	ClID       ClIDType              `json:"ClID" example:"sh8013"`                  // Required
	CrRr       ClIDType              `json:"CrRr" example:"sh8013"`
	CreatedAt  time.Time             `json:"CrDate" example:"2023-04-03T22:00:00.0Z"`
	UpRr       ClIDType              `json:"UpRr" example:"sh8013"`
	UpdatedAt  time.Time             `json:"UpDate" example:"2023-04-03T22:00:00.0Z"`
	AuthInfo   AuthInfoType          `json:"AuthInfo" example:"sTr0N5p@zzWqRD"` // Required
	Status     ContactStatus         `json:"Status"`
	Disclose   ContactDisclose       `json:"Disclose"`
}

// ContactStatus substruct of Contact
// https://www.rfc-editor.org/rfc/rfc5733#section-2.2
type ContactStatus struct {
	OK                       bool `json:"OK" example:"true"`
	PendingCreate            bool `json:"PendingCreate" example:"false"`
	PendingUpdate            bool `json:"PendingUpdate" example:"false"`
	PendingTransfer          bool `json:"PendingTransfer" example:"false"`
	PendingDelete            bool `json:"PendingDelete" example:"false"`
	ClientDeleteProhibited   bool `json:"ClientDeleteProhibited" example:"false"`
	ClientUpdateProhibited   bool `json:"ClientUpdatProhibited" example:"false"`
	ClientTransferProhibited bool `json:"ClientTransferProhibited" example:"false"`
	ServerDeleteProhibited   bool `json:"ServerDeleteProhibited" example:"false"`
	ServerUpdateProhibited   bool `json:"ServerUpdateProhibited" example:"false"`
	ServerTransferProhibited bool `json:"ServerTransferProhibited" example:"false"`
	Linked                   bool `json:"Linked" example:"true"`
}

// IsNil checks if the ContactStatus is nil (all fields are false)
func (s *ContactStatus) IsNil() bool {
	return !s.ClientDeleteProhibited && !s.ClientTransferProhibited && !s.ClientUpdateProhibited && !s.ServerDeleteProhibited && !s.ServerTransferProhibited && !s.ServerUpdateProhibited && !s.PendingCreate && !s.PendingDelete && !s.PendingTransfer && !s.PendingUpdate && !s.OK
}

// SetFullStatus sets the ContactStatus equal to the received ContactStatus and returns an error if the status is invalid
func (c *Contact) SetFullStatus(status ContactStatus) error {
	if !status.IsNil() && !status.IsValidContactStatus() {
		return ErrInvalidContactStatusCombination
	}
	c.Status = status
	c.SetOKStatusIfNeeded()
	c.UnSetOKStatusIfNeeded()

	return nil
}

// ContactDisclose substruct of Contact describes the flags for disclosure of certain fields in accordance with https://datatracker.ietf.org/doc/html/rfc5733#section-2.9
// True means disclose, false means don't disclose.
type ContactDisclose struct {
	NameInt bool `json:"NameInt" example:"false"`
	NameLoc bool `json:"NameLoc" example:"false"`
	OrgInt  bool `json:"OrgInt" example:"false"`
	OrgLoc  bool `json:"OrgLoc" example:"false"`
	AddrInt bool `json:"AddrInt" example:"false"`
	AddrLoc bool `json:"AddrLoc" example:"false"`
	Voice   bool `json:"Voice" example:"false"`
	Fax     bool `json:"Fax" example:"false"`
	Email   bool `json:"Email" example:"false"`
}

// IsNil checks if the ContactDisclose is nil (all fields are false)
func (d *ContactDisclose) IsNil() bool {
	return !d.NameInt && !d.NameLoc && !d.OrgInt && !d.OrgLoc && !d.AddrInt && !d.AddrLoc && !d.Voice && !d.Fax && !d.Email
}

// NewDiscloseStruct creates a new Disclose struct with default values
func NewDiscloseStruct(v bool) *ContactDisclose {
	return &ContactDisclose{
		NameInt: v,
		NameLoc: v,
		OrgInt:  v,
		OrgLoc:  v,
		AddrInt: v,
		AddrLoc: v,
		Voice:   v,
		Fax:     v,
		Email:   v,
	}
}

// NewContact creates a new Contact with required parameteres. It will normalize string values and return a pointer to the new Contact or an error
// Calling code will have to supply a RoID. We can either generate on according to SnowflakeID + ROID_ID or use the one supplied if we are importing an escrow
func NewContact(id, roid, email, authInfo, rarClid string) (*Contact, error) {
	// Normalize strings and create the Contact
	c := &Contact{
		ID:       ClIDType(NormalizeString(id)),
		RoID:     RoidType(NormalizeString(roid)),
		Email:    NormalizeString(email),
		AuthInfo: AuthInfoType(NormalizeString(authInfo)),
		ClID:     ClIDType(NormalizeString(rarClid)),
	}
	// Set OK status
	c.SetOKStatusIfNeeded()
	// Test if all fields are valid
	if ok, err := c.IsValid(); !ok {
		return nil, errors.Join(ErrInvalidContact, err)
	}
	// By default the Disclose struct is set to FALSE (don't disclose)
	// TODO: make this configurable
	// if DEFAULT_DISCLOSE {
	// 	c.ContactDisclose = *NewDiscloseStruct(true)
	// }

	c.Disclose = *NewDiscloseStruct(false)

	return c, nil
}

// IsValidContactStatus checks if the combination of statuses is valid according to https://www.rfc-editor.org/rfc/rfc5733#section-2.2
func (s *ContactStatus) IsValidContactStatus() bool {
	if (s.ClientDeleteProhibited || s.ClientTransferProhibited || s.ClientUpdateProhibited || s.ServerDeleteProhibited || s.ServerTransferProhibited || s.ServerUpdateProhibited || s.PendingCreate || s.PendingDelete || s.PendingTransfer || s.PendingUpdate) && s.OK {
		return false
	}
	if !(s.ClientDeleteProhibited || s.ClientTransferProhibited || s.ClientUpdateProhibited || s.ServerDeleteProhibited || s.ServerTransferProhibited || s.ServerUpdateProhibited || s.PendingCreate || s.PendingDelete || s.PendingTransfer || s.PendingUpdate) && !s.OK {
		return false
	}

	return true
}

// SetStatus Sets the status of the contact respecting RFC rules described here https://www.rfc-editor.org/rfc/rfc5733#section-2.2
func (c *Contact) SetStatus(s ContactStatusType) error {
	// TODO: add a testcase for the first two checks

	// Ensure idempotence when setting and update prohibition that is already set
	if (s == ContactStatusClientUpdateProhibited && c.Status.ClientUpdateProhibited) || (s == ContactStatusServerUpdateProhibited && c.Status.ServerUpdateProhibited) {
		return nil
	}

	// Disalow setting statuses other than linked when an update prohibition is present
	if !c.CanBeUpdated() && s != ContactStatusLinked {
		return ErrContactUpdateNotAllowed
	}

	// Main switch statement
	switch s {
	case ContactStatusOK:
		if c.Status.ClientDeleteProhibited || c.Status.ClientTransferProhibited || c.Status.ClientUpdateProhibited || c.Status.ServerDeleteProhibited || c.Status.ServerTransferProhibited || c.Status.ServerUpdateProhibited || c.Status.PendingCreate || c.Status.PendingDelete || c.Status.PendingTransfer || c.Status.PendingUpdate {
			return ErrInvalidContactStatusCombination
		}
		c.Status.OK = true
	case ContactStatusLinked:
		c.Status.Linked = true
	case ContactStatusPendingCreate:
		if c.Status.PendingDelete || c.Status.PendingTransfer || c.Status.PendingUpdate {
			return ErrInvalidContactStatusCombination
		}
		c.Status.PendingCreate = true
		c.UnSetOKStatusIfNeeded()
	case ContactStatusPendingUpdate:
		if c.Status.ClientUpdateProhibited || c.Status.ServerUpdateProhibited {
			return ErrInvalidContactStatusCombination // Untestable / unreachable because CanBeUpdated() will return false.
		}
		if c.Status.PendingDelete || c.Status.PendingTransfer || c.Status.PendingCreate {
			return ErrInvalidContactStatusCombination
		}
		c.Status.PendingUpdate = true
		c.UnSetOKStatusIfNeeded()
	case ContactStatusPendingTransfer:
		if c.Status.ClientTransferProhibited || c.Status.ServerTransferProhibited {
			return ErrInvalidContactStatusCombination
		}
		if c.Status.PendingDelete || c.Status.PendingUpdate || c.Status.PendingCreate {
			return ErrInvalidContactStatusCombination
		}
		c.Status.PendingTransfer = true
		c.UnSetOKStatusIfNeeded()
	case ContactStatusPendingDelete:
		if c.Status.ClientDeleteProhibited || c.Status.ServerDeleteProhibited {
			return ErrInvalidContactStatusCombination
		}
		if c.Status.PendingUpdate || c.Status.PendingTransfer || c.Status.PendingCreate {
			return ErrInvalidContactStatusCombination
		}
		c.Status.PendingDelete = true
		c.UnSetOKStatusIfNeeded()
	case ContactStatusClientDeleteProhibited:
		if c.Status.PendingDelete {
			return ErrInvalidContactStatusCombination
		}
		c.Status.ClientDeleteProhibited = true
		c.UnSetOKStatusIfNeeded()
	case ContactStatusClientUpdateProhibited:
		if c.Status.PendingUpdate {
			return ErrInvalidContactStatusCombination
		}
		c.Status.ClientUpdateProhibited = true
		c.UnSetOKStatusIfNeeded()
	case ContactStatusClientTransferProhibited:
		if c.Status.PendingTransfer {
			return ErrInvalidContactStatusCombination
		}
		c.Status.ClientTransferProhibited = true
		c.UnSetOKStatusIfNeeded()
	case ContactStatusServerDeleteProhibited:
		if c.Status.PendingDelete {
			return ErrInvalidContactStatusCombination
		}
		c.Status.ServerDeleteProhibited = true
		c.UnSetOKStatusIfNeeded()
	case ContactStatusServerUpdateProhibited:
		if c.Status.PendingUpdate {
			return ErrInvalidContactStatusCombination
		}
		c.Status.ServerUpdateProhibited = true
		c.UnSetOKStatusIfNeeded()
	case ContactStatusServerTransferProhibited:
		if c.Status.PendingTransfer {
			return ErrInvalidContactStatusCombination
		}
		c.Status.ServerTransferProhibited = true
		c.UnSetOKStatusIfNeeded()
	default:
		return ErrInvalidContactStatusCombination // Untestable / unreachable because the switch statement will return before this line.
	}

	// Check if the status is valid after setting it
	if !c.Status.IsValidContactStatus() {
		return ErrInvalidContactStatusCombination // Untestable / unreachable because the switch statement will return before this line.
	}
	return nil
}

// UnSetStatus Unsets the status of the contact respecting RFC rules described here https://www.rfc-editor.org/rfc/rfc5733#section-2.2
func (c *Contact) UnSetStatus(s string) error {
	// Refuse to unset the status if the host has restrictions that prevent it from being updated
	if !c.CanBeUpdated() {
		// Unless the status to be UnSet is clientUpdateProhibited or serverUpdateProhibited
		// In this case we must allow the unset to happen to avoid a deadlock
		// https://datatracker.ietf.org/doc/html/rfc5733#section-2.2:~:text=Requests%20to%20update%20the%20object%20(other%20than%20to%20remove%20this%20status)%0A%20%20%20%20%20%20MUST%20be%20rejected.
		if s != "clientUpdateProhibited" && s != "serverUpdateProhibited" {
			return ErrContactUpdateNotAllowed
		}
	}
	switch s {
	case "ok":
		if !(c.Status.ClientDeleteProhibited || c.Status.ClientTransferProhibited || c.Status.ClientUpdateProhibited || c.Status.ServerDeleteProhibited || c.Status.ServerTransferProhibited || c.Status.ServerUpdateProhibited || c.Status.PendingCreate || c.Status.PendingDelete || c.Status.PendingTransfer || c.Status.PendingUpdate) {
			return ErrInvalidContactStatusCombination
		}
		c.Status.OK = false
	case "linked":
		c.Status.Linked = false
	case "pendingCreate":
		c.Status.PendingCreate = false
		c.SetOKStatusIfNeeded()
	case "pendingUpdate":
		c.Status.PendingUpdate = false
		c.SetOKStatusIfNeeded()
	case "pendingTransfer":
		c.Status.PendingTransfer = false
		c.SetOKStatusIfNeeded()
	case "pendingDelete":
		c.Status.PendingDelete = false
		c.SetOKStatusIfNeeded()
	case "clientDeleteProhibited":
		c.Status.ClientDeleteProhibited = false
		c.SetOKStatusIfNeeded()
	case "clientUpdateProhibited":
		c.Status.ClientUpdateProhibited = false
		c.SetOKStatusIfNeeded()
	case "clientTransferProhibited":
		c.Status.ClientTransferProhibited = false
		c.SetOKStatusIfNeeded()
	case "serverDeleteProhibited":
		c.Status.ServerDeleteProhibited = false
		c.SetOKStatusIfNeeded()
	case "serverUpdateProhibited":
		c.Status.ServerUpdateProhibited = false
		c.SetOKStatusIfNeeded()
	case "serverTransferProhibited":
		c.Status.ServerTransferProhibited = false
		c.SetOKStatusIfNeeded()
	default:
		return ErrInvalidContactStatusCombination // Untestable / unreachable because the switch statement will return before this line.
	}
	return nil
}

// SetOKStatusIfNeeded Sets the OK status should be set
func (c *Contact) SetOKStatusIfNeeded() {
	if !(c.Status.ClientDeleteProhibited || c.Status.ClientTransferProhibited || c.Status.ClientUpdateProhibited || c.Status.ServerDeleteProhibited || c.Status.ServerTransferProhibited || c.Status.ServerUpdateProhibited || c.Status.PendingCreate || c.Status.PendingDelete || c.Status.PendingTransfer || c.Status.PendingUpdate) {
		c.Status.OK = true
	}
}

// UnSetOKStatusIfNeeded Unsets the OK status if needed
func (c *Contact) UnSetOKStatusIfNeeded() {
	if c.Status.ClientDeleteProhibited || c.Status.ClientTransferProhibited || c.Status.ClientUpdateProhibited || c.Status.ServerDeleteProhibited || c.Status.ServerTransferProhibited || c.Status.ServerUpdateProhibited || c.Status.PendingCreate || c.Status.PendingDelete || c.Status.PendingTransfer || c.Status.PendingUpdate {
		c.Status.OK = false
	}
}

// CanBeDeleted returns true if the host can be deleted and returns false if a status is set that prevents deletion (ServerDeleteProhibited or ClientDeleteProhibited)
func (c *Contact) CanBeDeleted() bool {
	if c.Status.ServerDeleteProhibited || c.Status.ClientDeleteProhibited {
		return false
	}
	return true
}

// CanBeUpdated returns true if the host can be updated and returns false if a status is set that prevents update (ServerUpdateProhibited or ClientUpdateProhibited)
func (c *Contact) CanBeUpdated() bool {
	if c.Status.ServerUpdateProhibited || c.Status.ClientUpdateProhibited {
		return false
	}
	return true
}

// CanBeTransferred returns true if the host can be transferred and returns false if a status is set that prevents transfer (ServerTransferProhibited or ClientTransferProhibited)
func (c *Contact) CanBeTransferred() bool {
	if c.Status.ServerTransferProhibited || c.Status.ClientTransferProhibited {
		return false
	}
	return true
}

// IsValid checks if the Contact is valid according to RFC5733 and tests all fields
func (c *Contact) IsValid() (bool, error) {
	if err := c.ID.Validate(); err != nil {
		return false, err
	}
	if _, err := mail.ParseAddress(c.Email); err != nil {
		return false, ErrInvalidEmail
	}
	if err := c.AuthInfo.Validate(); err != nil {
		return false, err
	}
	if err := c.RoID.Validate(); err != nil {
		return false, err
	}
	if !c.Status.IsValidContactStatus() {
		return false, ErrInvalidContactStatusCombination
	}
	if err := c.Voice.Validate(); err != nil {
		return false, err
	}
	if err := c.Fax.Validate(); err != nil {
		return false, err
	}
	for _, pi := range c.PostalInfo {
		if pi != nil {
			if !pi.IsValid() {
				return false, ErrInvalidContactPostalInfo
			}
		}
	}

	// Removing this as its maybe a little too strict.
	// if c.PostalInfo[0] != nil && c.PostalInfo[1] != nil {
	// 	if c.PostalInfo[0].Address.CountryCode != c.PostalInfo[1].Address.CountryCode {
	// 		return false, ErrPostalInfoCountryCodeMismatch
	// 	}
	// }

	return true, nil
}

// AddPostalInfo Adds Postal Info to Contact. It checks validtiy of the PostalInfo object and returns an error if it is invalid
// INT postalinfo are stored in the first position, LOC postalinfo in second position
// If a postalinfo of the same type already exists, it returns an error
// RemovePostalInfo can be used to remove a postalinfo prior to adding a new one of the same type
func (c *Contact) AddPostalInfo(pi *ContactPostalInfo) error {
	// Fail fast if we get an  invalid PostalInfo object
	if !pi.IsValid() {
		return ErrInvalidContactPostalInfo
	}
	// Store the 'int' postalinfo first, the 'loc' postalinfo in second position
	if pi.Type == "int" {
		if c.PostalInfo[0] != nil {
			return ErrPostalInfoTypeExistsAlready
		}
		c.PostalInfo[0] = pi
	}
	if pi.Type == "loc" {
		if c.PostalInfo[1] != nil {
			return ErrPostalInfoTypeExistsAlready
		}
		c.PostalInfo[1] = pi
	}
	return nil
}

// RemovePostalInfo Removes Postal Info from Contact by type.
func (c *Contact) RemovePostalInfo(t string) error {
	// Make this idempotent
	// The 'int' postalinfo is stored in the first position, the 'loc' postalinfor in second position
	if t == "int" {
		c.PostalInfo[0] = nil
	}
	if t == "loc" {
		c.PostalInfo[1] = nil
	}
	return nil
}
