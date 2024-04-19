package entities

import (
	"slices"

	"github.com/pkg/errors"
)

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

var (
	ErrInvalidDomainStatusCombination = errors.New("invalid Domain status combination")
	ErrDomainUpdateNotAllowed         = errors.New("domain update not allowed")
	ErrInvalidDomainStatus            = errors.New("invalid Domain status")

	ValidDomainStatuses = []string{
		DomainStatusOK,
		DomainStatusInactive,
		DomainStatusClientTransferProhibited,
		DomainStatusClientUpdateProhibited,
		DomainStatusClientDeleteProhibited,
		DomainStatusClientRenewProhibited,
		DomainStatusClientHold,
		DomainStatusServerTransferProhibited,
		DomainStatusServerUpdateProhibited,
		DomainStatusServerDeleteProhibited,
		DomainStatusServerRenewProhibited,
		DomainStatusServerHold,
		DomainStatusPendingCreate,
		DomainStatusPendingRenew,
		DomainStatusPendingTransfer,
		DomainStatusPendingUpdate,
		DomainStatusPendingRestore,
		DomainStatusPendingDelete,
	}
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
	if ds.Inactive && ds.OK {
		return ErrInvalidDomainStatusCombination
	}
	if ds.PendingDelete && (ds.ClientDeleteProhibited || ds.ServerDeleteProhibited) {
		return ErrInvalidDomainStatusCombination
	}
	if ds.PendingRenew && (ds.ClientRenewProhibited || ds.ServerRenewProhibited) {
		return ErrInvalidDomainStatusCombination
	}
	if ds.PendingTransfer && (ds.ClientTransferProhibited || ds.ServerTransferProhibited) {
		return ErrInvalidDomainStatusCombination
	}
	if ds.PendingUpdate && (ds.ClientUpdateProhibited || ds.ServerUpdateProhibited) {
		return ErrInvalidDomainStatusCombination
	}
	if trueCount(ds.PendingCreate, ds.PendingRenew, ds.PendingTransfer, ds.PendingUpdate, ds.PendingDelete, ds.PendingRestore) > 1 {
		return ErrInvalidDomainStatusCombination
	}

	return nil
}

// SetStatus sets the status of the DomainStatus object
func (d *Domain) SetStatus(s string) error {
	// Unknown status value
	if !slices.Contains(ValidDomainStatuses, s) {
		return ErrInvalidDomainStatus
	}

	//  Ensure idempotence when setting prohibitions that are already set
	if (s == DomainStatusClientUpdateProhibited && d.Status.ClientUpdateProhibited) || (s == DomainStatusServerUpdateProhibited && d.Status.ServerUpdateProhibited) {
		return nil
	}

	// If a prohibition is present, only allow setting inactive status
	if d.Status.HasProhibitions() && s != DomainStatusInactive {
		return ErrDomainUpdateNotAllowed
	}

	switch s {
	case "ok":
		d.Status.OK = true
	case "inactive":
		d.Status.Inactive = true
	case "clientTransferProhibited":
		d.Status.ClientTransferProhibited = true
	case "clientUpdateProhibited":
		d.Status.ClientUpdateProhibited = true
	case "clientDeleteProhibited":
		d.Status.ClientDeleteProhibited = true
	case "clientRenewProhibited":
		d.Status.ClientRenewProhibited = true
	case "clientHold":
		d.Status.ClientHold = true
	case "serverTransferProhibited":
		d.Status.ServerTransferProhibited = true
	case "serverUpdateProhibited":
		d.Status.ServerUpdateProhibited = true
	case "serverDeleteProhibited":
		d.Status.ServerDeleteProhibited = true
	case "serverRenewProhibited":
		d.Status.ServerRenewProhibited = true
	case "serverHold":
		d.Status.ServerHold = true
	case "pendingCreate":
		d.Status.PendingCreate = true
	case "pendingRenew":
		d.Status.PendingRenew = true
	case "pendingTransfer":
		d.Status.PendingTransfer = true
	case "pendingUpdate":
		d.Status.PendingUpdate = true
	case "pendingRestore":
		d.Status.PendingRestore = true
	case "pendingDelete":
		d.Status.PendingDelete = true
	}
	// Check if as a result of the update we need to unset the OK status
	d.UnSetOKStatusIfNeeded()

	return d.Status.Validate()
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

// HasHold returns true if the DomainStatus has any hold status set (ClientHold or ServerHold)
func (ds *DomainStatus) HasHold() bool {
	return ds.ClientHold || ds.ServerHold
}
