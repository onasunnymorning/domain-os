package entities

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
)

var (
	ErrDomainNotFound                 = errors.New("domain not found")
	ErrInvalidDomain                  = errors.New("invalid domain")
	ErrTLDAsDomain                    = errors.New("can't create a TLD as a domain")
	ErrInvalidDomainRoID              = fmt.Errorf("invalid Domain.RoID.ObjectIdentifier(), expecing '%s'", DOMAIN_ROID_ID)
	ErrInvalidDomainStatusCombination = errors.New("invalid Domain status combination")
)

// Domain is the domain object in a domain Name registry inspired by the EPP Domain object.
// Ref: https://datatracker.ietf.org/doc/html/rfc5731
type Domain struct {
	RoID         RoidType        `json:"RoID"`
	Name         DomainName      `json:"Name"`
	OriginalName string          `json:"OriginalName"` // is used to indicate that the domain name is an IDN variant. This element contains the domain name used to generate the IDN variant.
	UName        string          `json:"UName"`
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
}

const (
	DomainStatusOK                       = "OK"
	DomainStatusInactive                 = "Inactive"
	DomainStatusClientTransferProhibited = "clientTransferProhibited"
	DomainStatusClientUpdateProhibited   = "clientUpdateProhibited"
	DomainStatusClientDeleteProhibited   = "clientDeleteProhibited"
	DomainStatusClientRenewProhibited    = "clientRenewProhibited"
	DomainStatusClientHold               = "clientHold"
	DomainStatusServerTransferProhibited = "serverTransferProhibited"
	DomainStatusServerUpdateProhibited   = "serverUpdateProhibited"
	DomainStatusServerDeleteProhibited   = "serverDeleteProhibited"
	DomainStatusServerRenewProhibited    = "serverRenewProhibited"
	DomainStatusServerHold               = "serverHold"
	DomainStatusPendingCreate            = "pendingCreate"
	DomainStatusPendingRenew             = "pendingRenew"
	DomainStatusPendingTransfer          = "pendingTransfer"
	DomainStatusPendingUpdate            = "pendingUpdate"
	DomainStatusPendingRestore           = "pendingRestore"
	DomainStatusPendingDelete            = "pendingDelete"
)

// DomainStatus value object
// https://www.rfc-editor.org/rfc/rfc5731.html#section-2.3:~:text=%5D.%0A%0A2.3.-,Status%20Values,-A%20domain%20object
type DomainStatus struct {
	OK                       bool `json:"OK"`
	Inactive                 bool `json:"inactive"`
	ClientTransferProhibited bool `json:"ClientTransferProhibited"`
	ClientUpdateProhibited   bool `json:"ClientUpdateProhibited"`
	ClientDeleteProhibited   bool `json:"ClientDeleteProhibited"`
	ClientRenewProhibited    bool `json:"ClientRenewProhibited"`
	ClientHold               bool `json:"ClientHold"`
	ServerTransferProhibited bool `json:"ServerTransferProhibited"`
	ServerUpdateProhibited   bool `json:"ServerUpdateProhibited"`
	ServerDeleteProhibited   bool `json:"ServerDeleteProhibited"`
	ServerRenewProhibited    bool `json:"ServerPenewProhibited"`
	ServerHold               bool `json:"ServerHold"`
	PendingCreate            bool `json:"PendingCreate"`
	PendingRenew             bool `json:"PendingRenew"`
	PendingTransfer          bool `json:"PendingTransfer"`
	PendingUpdate            bool `json:"PendingUpdate"`
	PendingRestore           bool `json:"PendingRestore"`
	PendingDelete            bool `json:"PendingDelete"`
}

// NewDomainStatus returns a DomainStatus with default settings (Inactive and OK)
func NewDomainStatus() DomainStatus {
	return DomainStatus{
		OK:       true,
		Inactive: true,
	}
}

// Valid checks if the DomainStatus object is valid
func (ds *DomainStatus) Validate() error {
	if ds.IsNil() {
		return ErrInvalidDomainStatusCombination
	}
	if ds.HasPendings() && ds.OK {
		return ErrInvalidDomainStatusCombination
	}
	if ds.HasProhibitions() && ds.OK {
		return ErrInvalidDomainStatusCombination
	}
	if !ds.HasPendings() && !ds.HasProhibitions() && !ds.OK {
		return ErrInvalidDomainStatusCombination
	}
	return nil
}

// IsNil checks if the Domainstatus has all false values
func (ds *DomainStatus) IsNil() bool {
	return !ds.OK && !ds.Inactive && !ds.ClientTransferProhibited && !ds.ClientUpdateProhibited && !ds.ClientDeleteProhibited && !ds.ClientRenewProhibited && !ds.ClientHold && !ds.ServerTransferProhibited && !ds.ServerUpdateProhibited && !ds.ServerDeleteProhibited && !ds.ServerRenewProhibited && !ds.ServerHold && !ds.PendingCreate && !ds.PendingRenew && !ds.PendingTransfer && !ds.PendingUpdate && !ds.PendingRestore && !ds.PendingDelete
}

// HasProhibitions returns true if the DomainsStatus has any prohibitions set
func (ds *DomainStatus) HasProhibitions() bool {
	return ds.ClientTransferProhibited || ds.ClientUpdateProhibited || ds.ClientDeleteProhibited || ds.ClientRenewProhibited || ds.ClientHold || ds.ServerTransferProhibited || ds.ServerUpdateProhibited || ds.ServerDeleteProhibited || ds.ServerRenewProhibited || ds.ServerHold
}

// HasPendings returns true if the DomainStatus has any pending actions
func (ds *DomainStatus) HasPendings() bool {
	return ds.PendingCreate || ds.PendingRenew || ds.PendingTransfer || ds.PendingUpdate || ds.PendingRestore || ds.PendingDelete
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

// UnSetOKStatusIfNeeded unsets the Domain.Status.OK flag if a prohibition or pending action is present on the DomainStatus
func (d *Domain) UnSetOKStatusIfNeeded() {
	if d.Status.HasPendings() || d.Status.HasProhibitions() {
		d.Status.OK = false
	}
}

// DomainRGPStatus value object
type DomainRGPStatus struct {
	AddPeriodEnd           time.Time `json:"AddPeriodEnd"`
	RenewPeriodEnd         time.Time `json:"RenewPeriodEnd"`
	AutoRenewPeriodEnd     time.Time `json:"AutoRenewPeriodEnd"`
	TransferPeriodEnd      time.Time `json:"RransferPeriodEnd"`
	RedemptionPeriodEnd    time.Time `json:"RedemptionPeriodEnd"`
	PendingDeletePeriodEnd time.Time `json:"PendingDeletePeriodEnd"`
}

// IsNil checks if the DomainRGPStatus object is nil
func (d *DomainRGPStatus) IsNil() bool {
	return d.AddPeriodEnd.IsZero() && d.RenewPeriodEnd.IsZero() && d.AutoRenewPeriodEnd.IsZero() && d.TransferPeriodEnd.IsZero() && d.RedemptionPeriodEnd.IsZero() && d.PendingDeletePeriodEnd.IsZero()
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

	d.UName, _ = d.Name.ToUnicode() // Error is already checked in NewDomainName

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
