package entities

import "time"

// Domain is the domain object in a domain Name registry inspired by the EPP Domain object.
// Ref: https://datatracker.ietf.org/doc/html/rfc5731
type Domain struct {
	RoID         RoidType     `json:"roID"`
	Name         DomainName   `json:"name"`
	OriginalName string       `json:"originalName"`
	UName        string       `json:"uName"`
	RegistrantID ClIDType     `json:"registrantID"`
	AdminID      ClIDType     `json:"adminID"`
	TechID       ClIDType     `json:"techID"`
	BillingID    ClIDType     `json:"billingID"`
	ClID         ClIDType     `json:"clID"`
	CrRr         ClIDType     `json:"crRr"`
	UpRr         ClIDType     `json:"upRr"`
	TLDName      DomainName   `json:"tldName"`
	ExpiryDate   time.Time    `json:"expiryDate"`
	RenewedYears int          `json:"renewedYears"`
	AuthInfo     AuthInfoType `json:"authInfo"`
	CreatedAt    time.Time    `json:"createdAt"`
	UpdatedAt    time.Time    `json:"updatedAt"`

	DomainStatus
	DomainsRGPStatus
}

const (
	DomainStatusOK                       = "ok"
	DomainStatusInactive                 = "inactive"
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
	ClientTransferProhibited bool `json:"clientTransferProhibited"`
	ClientUpdateProhibited   bool `json:"clientUpdateProhibited"`
	ClientDeleteProhibited   bool `json:"clientDeleteProhibited"`
	ClientRenewProhibited    bool `json:"clientRenewProhibited"`
	ClientHold               bool `json:"clientHold"`
	ServerTransferProhibited bool `json:"serverTransferProhibited"`
	ServerUpdateProhibited   bool `json:"serverUpdateProhibited"`
	ServerDeleteProhibited   bool `json:"serverDeleteProhibited"`
	ServerRenewProhibited    bool `json:"serverPenewProhibited"`
	ServerHold               bool `json:"serverHold"`
	PendingCreate            bool `json:"pendingCreate"`
	PendingRenew             bool `json:"pendingRenew"`
	PendingTransfer          bool `json:"pendingTransfer"`
	PendingUpdate            bool `json:"pendingUpdate"`
	PendingRestore           bool `json:"pendingRestore"`
	PendingDelete            bool `json:"pendingDelete"`
}

// Valid checks if the DomainStatus object is valid
func (ds *DomainStatus) Validate() error {
	// TODO: Implement validation
	return nil
}

// DomainsRGPStatus value object
type DomainsRGPStatus struct {
	AddPeriodEnd           time.Time `json:"addPeriodEnd"`
	RenewPeriodEnd         time.Time `json:"renewPeriodEnd"`
	AutoRenewPeriodEnd     time.Time `json:"autoRenewPeriodEnd"`
	TransferPeriodEnd      time.Time `json:"transferPeriodEnd"`
	RedemptionPeriodEnd    time.Time `json:"redemptionPeriodEnd"`
	PendingDeletePeriodEnd time.Time `json:"pendingDeletePeriodEnd"`
}

// NewDomain creates a new Domain object. It should only be used directly in internal code (e.g. importing data). When Registering and Renewing domains, use the appropriate methods.
func NewDomain(name, authInfo string) (*Domain, error) {
	var err error

	d := &Domain{
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
	if err := d.Name.Validate(); err != nil {
		return err
	}
	if err := d.TLDName.Validate(); err != nil {
		return err
	}
	if err := d.AuthInfo.Validate(); err != nil {
		return err
	}
	if err := d.DomainStatus.Validate(); err != nil {
		return err
	}
	return nil
}
