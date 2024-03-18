package entities

import (
	"fmt"
	"time"
)

var (
	ErrInvalidDomainRoID = fmt.Errorf("invalid Domain.RoID.ObjectIdentifier(), expecing '%s'", DOMAIN_ROID_ID)
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

// Valid checks if the DomainStatus object is valid
func (ds *DomainStatus) Validate() error {
	// TODO: Implement validation
	return nil
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
func NewDomain(roid, name, authInfo string) (*Domain, error) {
	var err error

	d := &Domain{
		RoID:     RoidType(roid),
		Name:     DomainName(name),
		AuthInfo: AuthInfoType(authInfo),
	}

	d.TLDName = DomainName(d.Name.ParentDomain())

	d.UName, err = d.Name.ToUnicode()
	if err != nil {
		return nil, err
	}

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
	if err := d.TLDName.Validate(); err != nil {
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
