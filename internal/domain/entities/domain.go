package entities

import (
	"fmt"
	"time"

	"errors"
)

var (
	ErrDomainNotFound                  = errors.New("domain not found")
	ErrInvalidDomain                   = errors.New("invalid domain")
	ErrTLDAsDomain                     = errors.New("can't create a TLD as a domain")
	ErrInvalidDomainRoID               = fmt.Errorf("invalid Domain.RoID.ObjectIdentifier(), expecing '%s'", DOMAIN_ROID_ID)
	ErrUNameFieldReservedForIDNDomains = errors.New("UName field is reserved for IDN domains")
	ErrOriginalNameFieldReservedForIDN = errors.New("OriginalName field is reserved for IDN domains")
	ErrOriginalNameShouldBeAlabel      = errors.New("OriginalName field should be an A-label")
	ErrOriginalNameEqualToDomain       = errors.New("OriginalName field should not be equal to the domain name, it should point to the a-label of which this domain is a variant")
	ErrNoUNameProvidedForIDNDomain     = errors.New("UName field must be provided for IDN domains")
	ErrUNameDoesNotMatchDomain         = errors.New("UName must be the unicode version of the the domain name (a-label)")
)

// Domain is the domain object in a domain Name registry inspired by the EPP Domain object.
// Ref: https://datatracker.ietf.org/doc/html/rfc5731
type Domain struct {
	RoID         RoidType        `json:"RoID"`
	Name         DomainName      `json:"Name"`         // in case of IDN, this contains the A-label
	OriginalName DomainName      `json:"OriginalName"` // is used to indicate that the domain name is an IDN variant. This element contains the domain name (A-label) used to generate the IDN variant.
	UName        DomainName      `json:"UName"`        // is used in case the domain is an IDN domain. This element contains the Unicode representation of the domain name (aka U-label).
	RegistrantID ClIDType        `json:"RegistrantID"`
	AdminID      ClIDType        `json:"AdminID"`
	TechID       ClIDType        `json:"TechID"`
	BillingID    ClIDType        `json:"BillingID"`
	ClID         ClIDType        `json:"ClID"`
	CrRr         ClIDType        `json:"CrRr"`
	UpRr         ClIDType        `json:"UpRr"`
	TLDName      DomainName      `json:"TLDName"`
	ExpiryDate   time.Time       `json:"ExpiryDate"`
	RenewedYears int             `json:"RenewedYears"`
	AuthInfo     AuthInfoType    `json:"AuthInfo"`
	CreatedAt    time.Time       `json:"CreatedAt"`
	UpdatedAt    time.Time       `json:"UpdatedAt"`
	Status       DomainStatus    `json:"Status"`
	RGPStatus    DomainRGPStatus `json:"RGPStatus"`
	Hosts        []*Host         `json:"Hosts"`
}

// SetOKStatusIfNeeded sets Domain.Status.OK = true if no other prohibition or pendings are present on the DomainStatus
func (d *Domain) SetOKStatusIfNeeded() {
	// if nil, we set OK
	if d.Status.IsNil() {
		d.Status.OK = true
		return
	}
	// if no prohibitions and no pending exists, we set OK
	if !d.Status.HasProhibitions() && !d.Status.HasPendings() {
		d.Status.OK = true
		return
	}
}

// SetUnsetInactiveStatus is to be triggered when making changes to the hosts. Sets Domain.Status.Inactive = true if the domain has no hosts associated with it. It will set Domain.Status.Inactive = false otherwise.
func (d *Domain) SetUnsetInactiveStatus() {
	d.Status.Inactive = !d.HasHosts()
}

// HasHosts checks if the domain has hosts associated with it
func (d *Domain) HasHosts() bool {
	return len(d.Hosts) > 0
}

// UnSetOKStatusIfNeeded unsets the Domain.Status.OK flag if a prohibition or pending action is present on the DomainStatus
func (d *Domain) UnSetOKStatusIfNeeded() {
	if d.Status.HasPendings() || d.Status.HasProhibitions() {
		d.Status.OK = false
	}
}

// NewDomain creates a new Domain object. It returns an error if the Domain object is invalid.
// This function is intended to admin functionality such as importing domains from an escrow file.
// It does not set RGP statuses. If creating a domain in the context of a registration, see RegisterDomain instead
func NewDomain(roid, name, clid, authInfo string) (*Domain, error) {
	var err error

	n, err := NewDomainName(name)
	if err != nil {
		return nil, err
	}

	if n.ParentDomain() == "" {
		return nil, ErrTLDAsDomain
	}

	c, err := NewClIDType(clid)
	if err != nil {
		return nil, err
	}

	a, err := NewAuthInfoType(authInfo)
	if err != nil {
		return nil, err
	}

	d := &Domain{
		RoID:     RoidType(roid),
		Name:     *n,
		ClID:     c,
		AuthInfo: a,
	}

	d.TLDName = DomainName(d.Name.ParentDomain())

	if isIDN, _ := d.Name.IsIDN(); isIDN {
		uName, _ := d.Name.ToUnicode() // Error is already checked in NewDomainName
		d.UName = DomainName(uName)
	}

	d.Status = NewDomainStatus() // set the default statuses

	if err := d.Validate(); err != nil {
		return nil, err
	}

	return d, nil
}

// Validate checks if the Domain object is valid
func (d *Domain) Validate() error {
	if err := d.RoID.Validate(); err != nil {
		return err
	}
	if d.RoID.ObjectIdentifier() != DOMAIN_ROID_ID {
		return ErrInvalidDomainRoID
	}
	if err := d.Name.Validate(); err != nil {
		return err
	}
	if err := d.ClID.Validate(); err != nil {
		return err
	}
	if err := d.AuthInfo.Validate(); err != nil {
		return err
	}
	if err := d.Status.Validate(); err != nil {
		return err
	}
	if isIDN, _ := d.Name.IsIDN(); !isIDN {
		// if the domain is not an IDN domain, the OriginalName and UName fields must be empty
		if d.OriginalName != "" {
			return ErrOriginalNameFieldReservedForIDN
		}
		if d.UName != "" {
			return ErrUNameFieldReservedForIDNDomains
		}
	} else {
		// the domain is an IDN domain
		// the UName field must not be empty
		if d.UName == "" {
			return ErrNoUNameProvidedForIDNDomain
		}
		// the UName field must be the unicode version of the domain name (a-label)
		if uLabel, _ := d.Name.ToUnicode(); uLabel != string(d.UName) {
			return ErrUNameDoesNotMatchDomain
		}
		// if the OriginalName field is not empty, it should be an A-label (contain only ASCII characters)
		if d.OriginalName != "" && !IsASCII(d.OriginalName.String()) {
			return ErrOriginalNameShouldBeAlabel
		}
		// if the OriginalName field is not empty, it should not be equal to the domain name (it should point to the orignal A-label of which this domain is a variant)
		if d.OriginalName != "" && d.Name == d.OriginalName {
			return ErrOriginalNameEqualToDomain
		}
	}
	return nil
}

// CanBeDeleted checks if the Domain can be deleted (e.g. no delete prohibition is present in its status object: ClientDeleteProhibited or ServerDeleteProhibited). If the domain is alread in pending Delete status, it can't be deleted
func (d *Domain) CanBeDeleted() bool {
	return !d.Status.ClientDeleteProhibited && !d.Status.ServerDeleteProhibited && !d.Status.PendingDelete
}

// CanBeRenewed checks if the Domain can be renewed (e.g. no renew prohibition is present in its status object: ClientRenewProhibited or ServerRenewProhibited). If the domain is alread in pending Renew status, it can't be renewed
func (d *Domain) CanBeRenewed() bool {
	return !d.Status.ClientRenewProhibited && !d.Status.ServerRenewProhibited && !d.Status.PendingRenew
}

// CanBeTransferred checks if the Domain can be transferred (e.g. no transfer prohibition is present in its status object: ClientTransferProhibited or ServerTransferProhibited). If the domain is alread in pending Transfer status, it can't be transferred
func (d *Domain) CanBeTransferred() bool {
	return !d.Status.ClientTransferProhibited && !d.Status.ServerTransferProhibited && !d.Status.PendingTransfer
}

// CanBeUpdated checks if the Domain can be updated (e.g. no update prohibition is present in its status object: ClientUpdateProhibited or ServerUpdateProhibited). If the domain is alread in pending Update status, it can't be updated
func (d *Domain) CanBeUpdated() bool {
	return !d.Status.ClientUpdateProhibited && !d.Status.ServerUpdateProhibited && !d.Status.PendingUpdate
}
