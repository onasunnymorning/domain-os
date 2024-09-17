package entities

import (
	"fmt"
	"slices"

	"errors"
)

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
	Inactive                 bool `json:"Inactive"`
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
		return errors.Join(ErrInvalidDomainStatus, fmt.Errorf("unknown Domain status: %s", s))
	}
	if s == DomainStatusOK {
		return errors.Join(ErrInvalidDomainStatus, fmt.Errorf("cannot set Domain status to OK, it will be set automatically if no prohibitions or pending actions are set"))
	}
	if s == DomainStatusInactive {
		return errors.Join(ErrInvalidDomainStatus, fmt.Errorf("cannot set Domain status to Inactive, it will be set automatically depending on the host associated with the Domain"))
	}

	if d.Status.UpdateProhibited() && !(s == DomainStatusClientUpdateProhibited || s == DomainStatusServerUpdateProhibited) {
		return ErrDomainUpdateNotAllowed
	}

	switch s {
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
	// SetUnset the Inactive status if needed
	d.SetUnsetInactiveStatus()
	// Check if as a result of the update we need to unset the OK status
	d.UnSetOKStatusIfNeeded()

	return d.Status.Validate()
}

// UnSetStatus unsets the status of the DomainStatus object
func (d *Domain) UnSetStatus(s string) error {
	// Unknown status value
	if !slices.Contains(ValidDomainStatuses, s) {
		return errors.Join(ErrInvalidDomainStatus, fmt.Errorf("unknown Domain status: %s", s))
	}
	if s == DomainStatusOK {
		return errors.Join(ErrInvalidDomainStatus, fmt.Errorf("cannot unset Domain status to OK, it will be set automatically if no prohibitions or pending actions are set"))
	}
	if s == DomainStatusInactive {
		return errors.Join(ErrInvalidDomainStatus, fmt.Errorf("cannot unset Domain status to Inactive, it will be set automatically depending on the host associated with the Domain"))
	}

	if d.Status.UpdateProhibited() && !(s == DomainStatusClientUpdateProhibited || s == DomainStatusServerUpdateProhibited) {
		return ErrDomainUpdateNotAllowed
	}

	switch s {
	case "clientTransferProhibited":
		d.Status.ClientTransferProhibited = false
	case "clientUpdateProhibited":
		d.Status.ClientUpdateProhibited = false
	case "clientDeleteProhibited":
		d.Status.ClientDeleteProhibited = false
	case "clientRenewProhibited":
		d.Status.ClientRenewProhibited = false
	case "clientHold":
		d.Status.ClientHold = false
	case "serverTransferProhibited":
		d.Status.ServerTransferProhibited = false
	case "serverUpdateProhibited":
		d.Status.ServerUpdateProhibited = false
	case "serverDeleteProhibited":
		d.Status.ServerDeleteProhibited = false
	case "serverRenewProhibited":
		d.Status.ServerRenewProhibited = false
	case "serverHold":
		d.Status.ServerHold = false
	case "pendingCreate":
		d.Status.PendingCreate = false
	case "pendingRenew":
		d.Status.PendingRenew = false
	case "pendingTransfer":
		d.Status.PendingTransfer = false
	case "pendingUpdate":
		d.Status.PendingUpdate = false
	case "pendingRestore":
		d.Status.PendingRestore = false
	case "pendingDelete":
		d.Status.PendingDelete = false
	}
	// SetUnset the Inactive status if needed
	d.SetUnsetInactiveStatus()
	// Check if as a result of the update we need to unset the OK status
	d.SetOKStatusIfNeeded()

	return d.Status.Validate()
}

// IsNil checks if the Domainstatus has all false values
func (ds *DomainStatus) IsNil() bool {
	return !ds.OK && !ds.Inactive && !ds.ClientTransferProhibited && !ds.ClientUpdateProhibited && !ds.ClientDeleteProhibited && !ds.ClientRenewProhibited && !ds.ClientHold && !ds.ServerTransferProhibited && !ds.ServerUpdateProhibited && !ds.ServerDeleteProhibited && !ds.ServerRenewProhibited && !ds.ServerHold && !ds.PendingCreate && !ds.PendingRenew && !ds.PendingTransfer && !ds.PendingUpdate && !ds.PendingRestore && !ds.PendingDelete
}

// HasProhibitions returns true if the DomainsStatus has any prohibitions set. This includes ClientHold and ServerHold
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

// UpdateProhibited returns true if the DomainStatus has any update prohibitions set (ClientUpdateProhibited or ServerUpdateProhibited)
func (ds *DomainStatus) UpdateProhibited() bool {
	return ds.ClientUpdateProhibited || ds.ServerUpdateProhibited
}

// StringSlice returns a slice of strings representing the DomainStatus. This is useful for building WHOIS responses
func (ds *DomainStatus) StringSlice() []string {
	var status []string
	if ds.OK {
		status = append(status, DomainStatusOK)
	}
	if ds.Inactive {
		status = append(status, DomainStatusInactive)
	}
	if ds.ClientTransferProhibited {
		status = append(status, DomainStatusClientTransferProhibited)
	}
	if ds.ClientUpdateProhibited {
		status = append(status, DomainStatusClientUpdateProhibited)
	}
	if ds.ClientDeleteProhibited {
		status = append(status, DomainStatusClientDeleteProhibited)
	}
	if ds.ClientRenewProhibited {
		status = append(status, DomainStatusClientRenewProhibited)
	}
	if ds.ClientHold {
		status = append(status, DomainStatusClientHold)
	}
	if ds.ServerTransferProhibited {
		status = append(status, DomainStatusServerTransferProhibited)
	}
	if ds.ServerUpdateProhibited {
		status = append(status, DomainStatusServerUpdateProhibited)
	}
	if ds.ServerDeleteProhibited {
		status = append(status, DomainStatusServerDeleteProhibited)
	}
	if ds.ServerRenewProhibited {
		status = append(status, DomainStatusServerRenewProhibited)
	}
	if ds.ServerHold {
		status = append(status, DomainStatusServerHold)
	}
	if ds.PendingCreate {
		status = append(status, DomainStatusPendingCreate)
	}
	if ds.PendingRenew {
		status = append(status, DomainStatusPendingRenew)
	}
	if ds.PendingTransfer {
		status = append(status, DomainStatusPendingTransfer)
	}
	if ds.PendingUpdate {
		status = append(status, DomainStatusPendingUpdate)
	}
	if ds.PendingRestore {
		status = append(status, DomainStatusPendingRestore)
	}
	if ds.PendingDelete {
		status = append(status, DomainStatusPendingDelete)
	}
	return status
}
