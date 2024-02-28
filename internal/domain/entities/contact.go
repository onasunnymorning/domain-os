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
	ID              ClIDType              `json:"id" example:"sh8013" extensions:"x-order=0"`                          // The contact identifier as supplied by the registrar, this should be used by all references to the contact
	RoID            RoidType              `json:"roId" example:"1729468286778740736_CONT-APEX" extensions:"x-order=1"` // The generated id for the contact, has to be unique within the registry
	PostalInfo      [2]*ContactPostalInfo `json:"postalInfo"`                                                          // 1 required, 2 maximum. I prefer the approach to have an array of two optional items over a map because it makes manipulating (updating) easier
	Voice           E164Type              `json:"voice" example:"+1.9567345623"`
	Fax             E164Type              `json:"fax" example:"+1.9567345623"`
	Email           string                `json:"email" example:"solutions@apex.domains"` // Required
	ClID            ClIDType              `json:"clID" example:"sh8013"`                  // Required
	CrRr            ClIDType              `json:"crRR" example:"sh8013"`
	CreatedAt       time.Time             `json:"crDate" example:"2023-04-03T22:00:00.0Z"`
	UpRr            ClIDType              `json:"upRR" example:"sh8013"`
	UpdatedAt       time.Time             `json:"upDate" example:"2023-04-03T22:00:00.0Z"`
	AuthInfo        AuthInfoType          `json:"authInfo" example:"sTr0N5p@zzWqRD"` // Required
	ContactStatus                         // Embedded struct
	ContactDisclose                       // Embedded struct
}

// ContactStatus substruct of Contact
// https://www.rfc-editor.org/rfc/rfc5733#section-2.2
type ContactStatus struct {
	OK                       bool `json:"ok" example:"true"`
	PendingCreate            bool `json:"pendinCreate" example:"false"`
	PendingUpdate            bool `json:"pendinUpdate" example:"false"`
	PendingTransfer          bool `json:"pendinTransfer" example:"false"`
	PendingDelete            bool `json:"pendinDelete" example:"false"`
	ClientDeleteProhibited   bool `json:"clienDeleteProhibited" example:"false"`
	ClientUpdateProhibited   bool `json:"clientUpdatProhibited" example:"false"`
	ClientTransferProhibited bool `json:"clientTransferProhibited" example:"false"`
	ServerDeleteProhibited   bool `json:"serverDeleteProhibited" example:"false"`
	ServerUpdateProhibited   bool `json:"serverUpdateProhibited" example:"false"`
	ServerTransferProhibited bool `json:"serverTransferProhibited" example:"false"`
	Linked                   bool `json:"linked" example:"true"`
}

// ContactDisclose substruct of Contact describes the flags for disclosure of certain fields in accordance with https://datatracker.ietf.org/doc/html/rfc5733#section-2.9
// True means disclose, false means don't disclose.
type ContactDisclose struct {
	DiscloseNameInt bool `json:"DiscloseNameInt" example:"false"`
	DiscloseNameLoc bool `json:"DiscloseNameLoc" example:"false"`
	DiscloseOrgInt  bool `json:"DiscloseOrgInt" example:"false"`
	DiscloseOrgLoc  bool `json:"DiscloseOrgLoc" example:"false"`
	DiscloseAddrInt bool `json:"DiscloseAddrInt" example:"false"`
	DiscloseAddrLoc bool `json:"DiscloseAddrLoc" example:"false"`
	DiscloseVoice   bool `json:"DiscloseVoice" example:"false"`
	DiscloseFax     bool `json:"DiscloseFax" example:"false"`
	DiscloseEmail   bool `json:"DiscloseEmail" example:"false"`
}

// NewDiscloseStruct creates a new Disclose struct with default values
func NewDiscloseStruct(v bool) *ContactDisclose {
	return &ContactDisclose{
		DiscloseNameInt: v,
		DiscloseNameLoc: v,
		DiscloseOrgInt:  v,
		DiscloseOrgLoc:  v,
		DiscloseAddrInt: v,
		DiscloseAddrLoc: v,
		DiscloseVoice:   v,
		DiscloseFax:     v,
		DiscloseEmail:   v,
	}
}

// NewContact creates a new Contact with required parameteres. It will normalize string values and return a pointer to the new Contact or an error
// Calling code will have to supply a RoID. We can either generate on according to SnowflakeID + ROID_ID or use the one supplied if we are importing an escrow
func NewContact(id, roid, email, authInfo, rarClid string, postalInfo [2]*ContactPostalInfo) (*Contact, error) {
	// Normalize strings and create the Contact
	c := &Contact{
		ID:       ClIDType(NormalizeString(id)),
		RoID:     RoidType(NormalizeString(roid)),
		Email:    NormalizeString(email),
		AuthInfo: AuthInfoType(NormalizeString(authInfo)),
		ClID:     ClIDType(NormalizeString(rarClid)),
	}
	// Set OK status
	c.setOKStatusIfNeeded()

	// Add postal info if it is not nil
	for _, pi := range postalInfo {
		if pi != nil {
			err := c.AddPostalInfo(pi)
			if err != nil {
				return nil, err
			}
		}
	}

	// Test if all fields are valid
	if ok, err := c.IsValid(); !ok {
		return nil, errors.Join(ErrInvalidContact, err)
	}
	// By default the Disclose struct is set to FALSE (don't disclose)
	// TODO: make this configurable
	// if DEFAULT_DISCLOSE {
	// 	c.ContactDisclose = *NewDiscloseStruct(true)
	// }

	c.ContactDisclose = *NewDiscloseStruct(false)

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
	if (s == ContactStatusClientUpdateProhibited && c.ClientUpdateProhibited) || (s == ContactStatusServerUpdateProhibited && c.ServerUpdateProhibited) {
		return nil
	}

	// Disalow setting statuses other than linked when an update prohibition is present
	if !c.CanBeUpdated() && s != ContactStatusLinked {
		return ErrContactUpdateNotAllowed
	}

	// Main switch statement
	switch s {
	case ContactStatusOK:
		if c.ClientDeleteProhibited || c.ClientTransferProhibited || c.ClientUpdateProhibited || c.ServerDeleteProhibited || c.ServerTransferProhibited || c.ServerUpdateProhibited || c.PendingCreate || c.PendingDelete || c.PendingTransfer || c.PendingUpdate {
			return ErrInvalidContactStatusCombination
		}
		c.OK = true
	case ContactStatusLinked:
		c.Linked = true
	case ContactStatusPendingCreate:
		if c.PendingDelete || c.PendingTransfer || c.PendingUpdate {
			return ErrInvalidContactStatusCombination
		}
		c.PendingCreate = true
		c.OK = false
	case ContactStatusPendingUpdate:
		if c.ClientUpdateProhibited || c.ServerUpdateProhibited {
			return ErrInvalidContactStatusCombination // Untestable / unreachable because CanBeUpdated() will return false.
		}
		if c.PendingDelete || c.PendingTransfer || c.PendingCreate {
			return ErrInvalidContactStatusCombination
		}
		c.PendingUpdate = true
		c.OK = false
	case ContactStatusPendingTransfer:
		if c.ClientTransferProhibited || c.ServerTransferProhibited {
			return ErrInvalidContactStatusCombination
		}
		if c.PendingDelete || c.PendingUpdate || c.PendingCreate {
			return ErrInvalidContactStatusCombination
		}
		c.PendingTransfer = true
		c.OK = false
	case ContactStatusPendingDelete:
		if c.ClientDeleteProhibited || c.ServerDeleteProhibited {
			return ErrInvalidContactStatusCombination
		}
		if c.PendingUpdate || c.PendingTransfer || c.PendingCreate {
			return ErrInvalidContactStatusCombination
		}
		c.PendingDelete = true
		c.OK = false
	case ContactStatusClientDeleteProhibited:
		if c.PendingDelete {
			return ErrInvalidContactStatusCombination
		}
		c.ClientDeleteProhibited = true
		c.OK = false
	case ContactStatusClientUpdateProhibited:
		if c.PendingUpdate {
			return ErrInvalidContactStatusCombination
		}
		c.ClientUpdateProhibited = true
		c.OK = false
	case ContactStatusClientTransferProhibited:
		if c.PendingTransfer {
			return ErrInvalidContactStatusCombination
		}
		c.ClientTransferProhibited = true
		c.OK = false
	case ContactStatusServerDeleteProhibited:
		if c.PendingDelete {
			return ErrInvalidContactStatusCombination
		}
		c.ServerDeleteProhibited = true
		c.OK = false
	case ContactStatusServerUpdateProhibited:
		if c.PendingUpdate {
			return ErrInvalidContactStatusCombination
		}
		c.ServerUpdateProhibited = true
		c.OK = false
	case ContactStatusServerTransferProhibited:
		if c.PendingTransfer {
			return ErrInvalidContactStatusCombination
		}
		c.ServerTransferProhibited = true
		c.OK = false
	default:
		return ErrInvalidContactStatusCombination // Untestable / unreachable because the switch statement will return before this line.
	}

	// Check if the status is valid after setting it
	if !c.ContactStatus.IsValidContactStatus() {
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
		if !(c.ClientDeleteProhibited || c.ClientTransferProhibited || c.ClientUpdateProhibited || c.ServerDeleteProhibited || c.ServerTransferProhibited || c.ServerUpdateProhibited || c.PendingCreate || c.PendingDelete || c.PendingTransfer || c.PendingUpdate) {
			return ErrInvalidContactStatusCombination
		}
		c.OK = false
	case "linked":
		c.Linked = false
	case "pendingCreate":
		c.PendingCreate = false
		c.setOKStatusIfNeeded()
	case "pendingUpdate":
		c.PendingUpdate = false
		c.setOKStatusIfNeeded()
	case "pendingTransfer":
		c.PendingTransfer = false
		c.setOKStatusIfNeeded()
	case "pendingDelete":
		c.PendingDelete = false
		c.setOKStatusIfNeeded()
	case "clientDeleteProhibited":
		c.ClientDeleteProhibited = false
		c.setOKStatusIfNeeded()
	case "clientUpdateProhibited":
		c.ClientUpdateProhibited = false
		c.setOKStatusIfNeeded()
	case "clientTransferProhibited":
		c.ClientTransferProhibited = false
		c.setOKStatusIfNeeded()
	case "serverDeleteProhibited":
		c.ServerDeleteProhibited = false
		c.setOKStatusIfNeeded()
	case "serverUpdateProhibited":
		c.ServerUpdateProhibited = false
		c.setOKStatusIfNeeded()
	case "serverTransferProhibited":
		c.ServerTransferProhibited = false
		c.setOKStatusIfNeeded()
	default:
		return ErrInvalidContactStatusCombination // Untestable / unreachable because the switch statement will return before this line.
	}
	return nil
}

// setOKStatusIfNeeded Sets the OK status should be set
func (c *Contact) setOKStatusIfNeeded() {
	if !(c.ClientDeleteProhibited || c.ClientTransferProhibited || c.ClientUpdateProhibited || c.ServerDeleteProhibited || c.ServerTransferProhibited || c.ServerUpdateProhibited || c.PendingCreate || c.PendingDelete || c.PendingTransfer || c.PendingUpdate) {
		c.OK = true
	}
}

// CanBeDeleted returns true if the host can be deleted and returns false if a status is set that prevents deletion (ServerDeleteProhibited or ClientDeleteProhibited)
func (c *Contact) CanBeDeleted() bool {
	if c.ServerDeleteProhibited || c.ClientDeleteProhibited {
		return false
	}
	return true
}

// CanBeUpdated returns true if the host can be updated and returns false if a status is set that prevents update (ServerUpdateProhibited or ClientUpdateProhibited)
func (c *Contact) CanBeUpdated() bool {
	if c.ServerUpdateProhibited || c.ClientUpdateProhibited {
		return false
	}
	return true
}

// CanBeTransferred returns true if the host can be transferred and returns false if a status is set that prevents transfer (ServerTransferProhibited or ClientTransferProhibited)
func (c *Contact) CanBeTransferred() bool {
	if c.ServerTransferProhibited || c.ClientTransferProhibited {
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
	if !c.ContactStatus.IsValidContactStatus() {
		return false, ErrInvalidContactStatusCombination
	}
	if err := c.Voice.Validate(); err != nil {
		return false, err
	}
	if err := c.Fax.Validate(); err != nil {
		return false, err
	}
	validPostalInfoCount := 0
	for _, pi := range c.PostalInfo {
		if pi != nil {
			if !pi.IsValid() {
				return false, ErrInvalidContactPostalInfo
			}
			validPostalInfoCount++
		}
	}
	if validPostalInfoCount == 0 {
		return false, ErrInvalidContactPostalInfo
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
