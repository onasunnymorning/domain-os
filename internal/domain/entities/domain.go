package entities

import (
	"fmt"
	"time"
)

var (
	ErrInvalidDomainRoID              = fmt.Errorf("invalid Domain.RoID.ObjectIdentifier(), expecing '%s'", DOMAIN_ROID_ID)
	ErrInvalidDomainStatusCombination = fmt.Errorf("invalid Domain status combination")
)

// Domain is the domain object in a domain Name registry inspired by the EPP Domain object.
// Ref: https://datatracker.ietf.org/doc/html/rfc5731
type Domain struct {
	RoID         RoidType        `json:"RoID"`
	Name         DomainName      `json:"Name"`
	OriginalName string          `json:"OriginalName"`
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
	OK                       bool `json:"ok"`
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

// NewDomain creates a new Domain object. It should only be used directly in internal code (e.g. importing data). When Registering and Renewing domains, use the appropriate methods.
func NewDomain(roid, name, clid, authInfo string) (*Domain, error) {
	var err error

	d := &Domain{
		RoID:     RoidType(roid),
		Name:     DomainName(name),
		ClID:     ClIDType(clid),
		AuthInfo: AuthInfoType(authInfo),
	}

	d.TLDName = DomainName(d.Name.ParentDomain())

	d.UName, err = d.Name.ToUnicode()
	if err != nil {
		return nil, err
	}

	d.Status = NewDomainStatus()

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
