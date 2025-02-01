package entities

import (
	"net/mail"
	"strings"
	"time"

	"errors"
)

const (
	// Terminated means the registrar once had an ICANN accreditation but it has been terminated. This only affects the registrar's ability to register new domains in Generic TLDs. The registrar can still manage existing domains until we transfer them out (usually upon ICANN request)
	RegistrarStatusTerminated RegistrarStatus = "terminated"
	// OK means the registrar has an active ICANN accreditation (ok is mapped to 'accredited' in this list https://www.iana.org/assignments/registrar-ids/registrar-ids.xhtml )
	RegistrarStatusOK RegistrarStatus = "ok"
	// Readonly means the registrar has an active ICANN accreditation but is in a readonly state. Interpreted as: the registrar can't register new domains, but can manage existing ones
	RegistrarStatusReadonly RegistrarStatus = "readonly"

	RegistrarPostalInfoTypeINT = "int"
	RegistrarPostalInfoTypeLOC = "loc"
)

var (
	ErrInvalidRegistrar                                 = errors.New("invalid registrar")
	ErrRegistrarNotFound                                = errors.New("registrar not found")
	ErrRegistrarMissingEmail                            = errors.New("missing email: a valid email is required")
	ErrRegistrarMissingName                             = errors.New("missing name: a valid name and unique name is required")
	ErrInvalidRegistrarStatus                           = errors.New("invalid registrar status: status must be one of 'ok', 'readonly', 'terminated'")
	ErrInvalidRegistrarIANAStatus                       = errors.New("invalid registrar status: status must be one of 'Reserved', 'Terminated', 'Accredited', 'Unknown'")
	ErrRegistrarPostalInfoTypeExists                    = errors.New("postalinfo of this type already exists")
	ErrRegistrarStatusPreventsAccreditation             = errors.New("registrar status prevents accreditation")
	ErrOnlyICANNAccreditedRegistrarsCanAccreditForGTLDs = errors.New("only ICANN accredited registrars can accredit for gTLDs")

	VALID_RAR_STATUSES = []RegistrarStatus{RegistrarStatusOK, RegistrarStatusReadonly, RegistrarStatusTerminated}
)

// RegistrarStatus is a type for registrar status as defined in RFC 9022(https://datatracker.ietf.org/doc/html/rfc9022#name-registrar-object:~:text=5.4.1.1.-,%3CrdeRegistrar%3Aregistrar%3E%20Element,-The%20%3Cregistrar%3E%20element)
type RegistrarStatus string

// String returns the string value of the RegistrarStatus
func (r *RegistrarStatus) String() string {
	return string(*r)
}

// IsValid checks if the RegistrarStatus is valid
func (r *RegistrarStatus) IsValid() bool {
	for _, status := range VALID_RAR_STATUSES {
		if strings.EqualFold(string(*r), string(status)) { // use strings package to compare case-insensitive
			return true
		}
	}
	return false
}

// Registrar object represents the sponsoring client for other objects and is typically referred to as the sponsoring registrar.
// Ref: https://www.rfc-editor.org/rfc/rfc9022.html#name-registrar-object
type Registrar struct {
	// ClID is the client identifier of the registrar and is used throughout the Registry to identify the sponsoring registrar.
	ClID ClIDType `json:"ClID" example:"my-regisrar-007" extensions:"x-order:0"`
	// Must match the Legal entity name. For ICANN Accredited registrars, must match the entity registered with ICANN for the corresponding GurID.
	Name string
	// A Nickname for the regisrar, can be used if the registrar has multiple brands or it is know in the industry as a different name than their legal entity.
	NickName string
	// The IANA Registrar ID for the registrar. This is the ID that is attributed in the IANA Registrar ID Registry if the Registrar is accredited by ICANN. Ref: https://www.iana.org/assignments/registrar-ids/registrar-ids.xhtml
	GurID int
	// The status of the registrar. It can be one of the following: "ok", "readonly", "terminated"
	Status RegistrarStatus
	// IANAStatus is the status of the registrar as defined by IANA here Ref: https://www.iana.org/assignments/registrar-ids/registrar-ids.xhtml
	// can be one of the following: "Accredited", "Reserved", "Terminated"
	IANAStatus IANARegistrarStatus
	// A flag that indicates whether the registrar has opted-in to automatically renew domains that are eligible for auto-renewal.
	Autorenew  bool
	PostalInfo [2]*RegistrarPostalInfo
	// The ABUSE phone number for the registrar
	Voice E164Type
	Fax   E164Type
	// The ABUSE email address for the registrar
	Email       string
	URL         URL
	WhoisInfo   WhoisInfo
	RdapBaseURL URL
	CreatedAt   time.Time
	UpdatedAt   time.Time
	// The TLDs the registrar is accredited for
	TLDs []*TLD
}

// RegistrarListItem is a subset of the Registrar object that is used in lists (e.g. list all registrars) when the full object is not needed
type RegistrarListItem struct {
	ClID      ClIDType
	Name      string
	GurID     int
	Status    RegistrarStatus
	Autorenew bool
}

// GetListRegistrarItem returns a pointer to a ListRegistrarItem struct
// populated with the Registrar's ClID, Name, GurID, Status, and Autorenew fields.
func (r *Registrar) GetListRegistrarItem() *RegistrarListItem {
	return &RegistrarListItem{
		ClID:      r.ClID,
		Name:      r.Name,
		GurID:     r.GurID,
		Status:    r.Status,
		Autorenew: r.Autorenew,
	}
}

// NewRegistrar creates a new instance of Registrar
func NewRegistrar(clID, name, email string, gurid int, postalInfo [2]*RegistrarPostalInfo) (*Registrar, error) {
	r := &Registrar{
		ClID: ClIDType(NormalizeString(clID)),
		Name: NormalizeString(name),
		// By default, the nickname is the same as the name
		NickName: NormalizeString(name),
		GurID:    gurid,
		Email:    strings.ToLower(NormalizeString(email)),
		// Create the status as readonly by default
		Status: RegistrarStatusReadonly,
		// Create the IANAStatus as Unknown by default
		IANAStatus: IANARegistrarStatusUnknown,
	}

	// Add the postal info information
	for _, pi := range postalInfo {
		if pi != nil {
			err := r.AddPostalInfo(pi)
			if err != nil {
				return nil, err
			}
		}
	}

	if err := r.Validate(); err != nil {
		return nil, err
	}

	return r, nil
}

// Validate checks if the registrar object is valid
// It is valid when all of the following conditions are true:
// - ClID is valid
// - Name and Email are not empty
// - Status is one of the valid values
// - Email is valid
// - The postal info is valid
func (r *Registrar) Validate() error {
	if err := r.ClID.Validate(); err != nil {
		return err
	}
	if r.Name == "" {
		return ErrRegistrarMissingName
	}
	if r.Email == "" {
		return ErrRegistrarMissingEmail
	}
	// Status must be one of the valid values
	if r.Status != RegistrarStatusOK && r.Status != RegistrarStatusReadonly && r.Status != RegistrarStatusTerminated {
		return ErrInvalidRegistrarStatus
	}
	// This can be empty, but if it is not empty, it must be valid
	if r.IANAStatus != "" && !r.IANAStatus.IsValid() {
		return ErrInvalidRegistrarIANAStatus
	}
	_, err := mail.ParseAddress(r.Email)
	if err != nil {
		return ErrInvalidEmail
	}
	validPostalInfoCount := 0
	for _, pi := range r.PostalInfo {
		if pi != nil {
			if err := pi.IsValid(); err != nil {
				return ErrInvalidRegistrarPostalInfo
			}
			validPostalInfoCount++
		}
	}

	if validPostalInfoCount == 0 {
		return ErrInvalidRegistrarPostalInfo
	}

	return nil
}

// AddPostalInfo Adds Postal Info to a Registrar. It checks validtiy of the PostalInfo object and returns an error if it is invalid
// INT postalinfo are stored in the first position, LOC postalinfo in second position
// If a postalinfo of the same type already exists, it returns an error
// RemovePostalInfo can be used to remove a postalinfo prior to adding a new one of the same type
func (r *Registrar) AddPostalInfo(pi *RegistrarPostalInfo) error {
	// Fail fast if we get an  invalid PostalInfo object
	if err := pi.IsValid(); err != nil {
		return errors.Join(ErrInvalidRegistrarPostalInfo, err)

	}
	// In the 2-item array, store the 'int' postalinfo first, the 'loc' postalinfo in second position
	if pi.Type == "int" {
		if r.PostalInfo[0] != nil {
			return ErrRegistrarPostalInfoTypeExists
		}
		r.PostalInfo[0] = pi
	}
	if pi.Type == "loc" {
		if r.PostalInfo[1] != nil {
			return ErrRegistrarPostalInfoTypeExists
		}
		r.PostalInfo[1] = pi
	}
	return nil
}

// RemovePostalInfo Removes Postal Info from Registrar by specifying the type
func (r *Registrar) RemovePostalInfo(t string) error {
	if t != "int" && t != "loc" {
		return ErrInvalidPostalInfoEnumType
	}
	// Make this idempotent
	// The 'int' postalinfo is stored in the first position, the 'loc' postalinfor in second position
	if t == "int" {
		r.PostalInfo[0] = nil
	}
	if t == "loc" {
		r.PostalInfo[1] = nil
	}
	return nil
}

// Checks if a registrar is accredited for a particular TLD
func (r *Registrar) IsAccreditedFor(tld *TLD) (int, bool) {
	for i, a := range r.TLDs {
		if tld.Name == a.Name {
			return i, true
		}
	}
	return 0, false
}

// Accreditation is the process by which a registrar is granted the ability to register domain names in a particular TLD.
// In order for a Registrar to be able to accredit for a TLD, the Registrar.Status needs to be "ok".
// If the TLD is a Generic TLD, the Registrar.GurID must be set and the Registrar.IANAStatus must be "Accredited".
func (r *Registrar) AccreditFor(tld *TLD) error {
	_, isAccredited := r.IsAccreditedFor(tld)
	if isAccredited {
		return nil // Idempotent
	}
	if r.Status != "ok" {
		return ErrRegistrarStatusPreventsAccreditation
	}
	if (r.GurID == 0 || r.IANAStatus != IANARegistrarStatusAccredited) && tld.Type == TLDTypeGTLD {
		return ErrOnlyICANNAccreditedRegistrarsCanAccreditForGTLDs
	}

	r.TLDs = append(r.TLDs, tld)
	return nil
}

// DeAccreditation is the process by which a registrar is removed from the list of registrars that are allowed to register domain names in a particular TLD.
func (r *Registrar) DeAccreditFor(tld *TLD) error {
	index, isAccredited := r.IsAccreditedFor(tld)
	if !isAccredited {
		return nil // Idempotent
	}
	r.TLDs = append(r.TLDs[:index], r.TLDs[index+1:]...)
	return nil
}

// SetStatus sets the status of the registrar and returns an error if the status is invalid
func (r *Registrar) SetStatus(s RegistrarStatus) error {
	if !s.IsValid() {
		return ErrInvalidRegistrarStatus
	}
	r.Status = RegistrarStatus(strings.ToLower(string(s))) // When setting always use lowercase
	return nil
}
