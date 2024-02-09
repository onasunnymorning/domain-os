package entities

import (
	"fmt"
	"net/netip"
	"time"
)

const (
	MAX_ADDRESSES_PER_HOST           = 10
	HostStatusPendingCreate          = "pendingCreate"
	HostStatusPendingDelete          = "pendingDelete"
	HostStatusPendingUpdate          = "pendingUpdate"
	HostStatusPendingTransfer        = "pendingTransfer"
	HostStatusClientDeleteProhibited = "clientDeleteProhibited"
	HostStatusClientUpdateProhibited = "clientUpdateProhibited"
	HostStatusServerDeleteProhibited = "serverDeleteProhibited"
	HostStatusServerUpdateProhibited = "serverUpdateProhibited"
	HostStatusOK                     = "ok"
	HostStatusLinked                 = "linked"
)

var (
	ErrDuplicateHostAddress        = fmt.Errorf("duplicate host address")
	ErrHostAddressNotFound         = fmt.Errorf("host address not found")
	ErrMaxAddressesPerHostExceeded = fmt.Errorf("maximum number of %d addresses per host exceeded", MAX_ADDRESSES_PER_HOST)
	ErrHostStatusIncompatible      = fmt.Errorf("host status is incompatible")
	ErrOKStatusMustBeSet           = fmt.Errorf("ok status must be set when no prohibitions are set")
	ErrUnknownHostStatus           = fmt.Errorf("unknown host status")
	ErrHostUpdateProhibited        = fmt.Errorf("host update is prohibited")
)

// Host struct represents a host object based on https://datatracker.ietf.org/doc/html/rfc5732
type Host struct {
	RoID      RoidType     `json:"roId" example:"1729468286778740736_HOST-APEX" extensions:"x-order=0"`
	Name      DomainName   `json:"name" exmaple:"ns1.apex.domains" extensions:"x-order=1"`
	Addresses []netip.Addr `json:"addresses"`
	ClID      ClIDType     `json:"clID" example:"sh8013"`
	CrRr      ClIDType     `json:"crRR" example:"sh8013"`
	UpRr      ClIDType     `json:"upRR" example:"sh8013"`
	CreatedAt time.Time    `json:"crDate" example:"2023-04-03T22:00:00.0Z"`
	UpdatedAt time.Time    `json:"upDate" example:"2023-04-03T22:00:00.0Z"`
	// True if the host is used on a domain that is the parent of the host's FQDN. https://datatracker.ietf.org/doc/html/rfc5732#section-1.1
	// This is set/unset by the Domain.AddHost() and Domain.RemoveHost() when a host is added/removed from a domain.
	InBailiwick bool `json:"inBailiwick"  example:"true"`
	HostStatus
}

// HostStatus is an implementation of https://datatracker.ietf.org/doc/html/rfc5732#section-2.3
type HostStatus struct {
	OK                     bool `json:"ok" example:"true"`
	Linked                 bool `json:"linked" example:"true"`
	PendingCreate          bool `json:"pendingCreate" example:"false"`
	PendingDelete          bool `json:"pendingDelete" example:"false"`
	PendingUpdate          bool `json:"pendingUpdate" example:"false"`
	PendingTransfer        bool `json:"pendingTransfer" example:"false"`
	ClientDeleteProhibited bool `json:"clientDeleteProhibited" example:"false"`
	ClientUpdateProhibited bool `json:"clientUpdateProhibited" example:"false"`
	ServerDeleteProhibited bool `json:"serverDeleteProhibited" example:"false"`
	ServerUpdateProhibited bool `json:"serverUpdateProhibited" example:"false"`
}

// NewHost creates a new Host with required fields. It will normalize strings
func NewHost(name, roid, clid string) (*Host, error) {
	domainName, err := NewDomainName(name)
	if err != nil {
		return nil, err
	}
	h := &Host{
		Name: *domainName,
		RoID: RoidType(NormalizeString(roid)),
		ClID: ClIDType(NormalizeString(clid)),
		CrRr: ClIDType(NormalizeString(clid)),
		HostStatus: HostStatus{
			OK: true,
		},
	}
	return h, nil
}

// AddAddress adds a new address to the host. It will return an error if the address already exists or if the maximum number of addresses per host is exceeded. Or if the address is invalid
func (h *Host) AddAddress(addr string) error {
	if len(h.Addresses) >= MAX_ADDRESSES_PER_HOST {
		return ErrMaxAddressesPerHostExceeded
	}
	// Check if its valid
	a, err := netip.ParseAddr(addr)
	if err != nil {
		return ErrInvalidIP
	}
	// Check if it already exists
	for _, address := range h.Addresses {
		if address.String() == a.String() {
			return ErrDuplicateHostAddress
		}
	}
	h.Addresses = append(h.Addresses, a)
	return nil
}

// RemoveAddress removes an address from the host. If the address is not found or invalid it will return an error
func (h *Host) RemoveAddress(addr string) error {
	if len(h.Addresses) == 0 {
		return ErrHostAddressNotFound
	}
	// Check if its valid
	a, err := netip.ParseAddr(addr)
	if err != nil {
		return ErrInvalidIP
	}
	// Remove it
	for i, address := range h.Addresses {
		if address.String() == a.String() {
			h.Addresses = append(h.Addresses[:i], h.Addresses[i+1:]...)
			return nil
		}
	}
	// If we didn't return yet, the address was Not found
	return ErrHostAddressNotFound
}

// CanBeDeleted returns true if the host can be deleted and returns false if a status is set that prevents deletion (ServerDeleteProhibited or ClientDeleteProhibited)
func (h *Host) CanBeDeleted() bool {
	if h.ServerDeleteProhibited || h.ClientDeleteProhibited {
		return false
	}
	return true
}

// CanBeUpdated returns true if the host can be updated and returns false if a status is set that prevents update (ServerUpdateProhibited or ClientUpdateProhibited)
func (h *Host) CanBeUpdated() bool {
	if h.ServerUpdateProhibited || h.ClientUpdateProhibited {
		return false
	}
	return true
}

// ValidateStatus validates the status of the host and returns an error if the statuses are incompatible according to https://datatracker.ietf.org/doc/html/rfc5732#section-2.3
func (h *Host) ValidateStatus() error {
	// The pendingCreate, pendingDelete, pendingTransfer, and pendingUpdate status values MUST NOT be combined with each other.
	if h.PendingDelete && (h.PendingTransfer || h.PendingUpdate || h.PendingCreate) {
		return ErrHostStatusIncompatible
	}
	if h.PendingCreate && (h.PendingTransfer || h.PendingUpdate || h.PendingDelete) {
		return ErrHostStatusIncompatible
	}
	if h.PendingUpdate && (h.PendingTransfer || h.PendingDelete || h.PendingCreate) {
		return ErrHostStatusIncompatible
	}
	// "pendingUpdate" status MUST NOT be combined with either "clientUpdateProhibited" or "serverUpdateProhibited" status.
	if h.PendingUpdate && (h.ClientUpdateProhibited || h.ServerUpdateProhibited) {
		return ErrHostStatusIncompatible
	}
	// "pendingDelete" status MUST NOT be combined with either "clientDeleteProhibited" or "serverDeleteProhibited" status.
	if h.PendingDelete && (h.ClientDeleteProhibited || h.ServerDeleteProhibited) {
		return ErrHostStatusIncompatible
	}
	// "ok" status MAY only be combined with "linked" status.
	if h.OK && (h.PendingCreate || h.PendingDelete || h.PendingTransfer || h.PendingUpdate || h.ClientDeleteProhibited || h.ClientUpdateProhibited || h.ServerDeleteProhibited || h.ServerUpdateProhibited) {
		return ErrHostStatusIncompatible
	}
	// "ok" status MUST be set when no other status is set.
	if !h.OK && !h.PendingCreate && !h.PendingDelete && !h.PendingTransfer && !h.PendingUpdate && !h.ClientDeleteProhibited && !h.ClientUpdateProhibited && !h.ServerDeleteProhibited && !h.ServerUpdateProhibited {
		return ErrOKStatusMustBeSet
	}
	// Other status combinations not expressly prohibited MAY be used.
	return nil
}

func (h *Host) SetOKIfNeeded() {
	if !h.PendingCreate && !h.PendingDelete && !h.PendingTransfer && !h.PendingUpdate && !h.ClientDeleteProhibited && !h.ClientUpdateProhibited && !h.ServerDeleteProhibited && !h.ServerUpdateProhibited {
		h.OK = true
	}

}

// UnsetOKIfNeeded unsets the OK status if any other status prohibition is set
func (h *Host) UnsetOKIfNeeded() {
	if h.PendingCreate || h.PendingDelete || h.PendingTransfer || h.PendingUpdate || h.ClientDeleteProhibited || h.ClientUpdateProhibited || h.ServerDeleteProhibited || h.ServerUpdateProhibited {
		h.OK = false
	}
}

// SetStatus sets the status of the host and validates it. It will return an error if the status is incompatible
func (h *Host) SetStatus(s string) error {
	// If Update is prohibited, the only update allowed is to remove the prohibition (or in this case set it when it is already set)
	if !h.CanBeUpdated() && (s != HostStatusClientUpdateProhibited && s != HostStatusServerUpdateProhibited) {
		return ErrHostUpdateProhibited
	}
	switch s {
	case HostStatusOK:
		h.OK = true
	case HostStatusLinked:
		h.Linked = true
	case HostStatusPendingCreate:
		h.PendingCreate = true
	case HostStatusPendingDelete:
		h.PendingDelete = true
	case HostStatusPendingUpdate:
		h.PendingUpdate = true
	case HostStatusPendingTransfer:
		h.PendingTransfer = true
	case HostStatusClientDeleteProhibited:
		h.ClientDeleteProhibited = true
	case HostStatusClientUpdateProhibited:
		h.ClientUpdateProhibited = true
	case HostStatusServerDeleteProhibited:
		h.ServerDeleteProhibited = true
	case HostStatusServerUpdateProhibited:
		h.ServerUpdateProhibited = true
	default:
		return ErrUnknownHostStatus
	}
	h.UnsetOKIfNeeded()
	return h.ValidateStatus()
}

// UnsetHostStatus unsets the status of the host and validates it. It will return an error if the status is incompatible and set OK if appropriate
func (h *Host) UnsetStatus(s string) error {
	// If Update is prohibited, the only update allowed is to remove the prohibition
	if !h.CanBeUpdated() && (s != HostStatusClientUpdateProhibited && s != HostStatusServerUpdateProhibited) {
		return ErrHostUpdateProhibited
	}
	switch s {
	case HostStatusOK:
		h.OK = false
	case HostStatusLinked:
		h.Linked = false
	case HostStatusPendingCreate:
		h.PendingCreate = false
	case HostStatusPendingDelete:
		h.PendingDelete = false
	case HostStatusPendingUpdate:
		h.PendingUpdate = false
	case HostStatusPendingTransfer:
		h.PendingTransfer = false
	case HostStatusClientDeleteProhibited:
		h.ClientDeleteProhibited = false
	case HostStatusClientUpdateProhibited:
		h.ClientUpdateProhibited = false
	case HostStatusServerDeleteProhibited:
		h.ServerDeleteProhibited = false
	case HostStatusServerUpdateProhibited:
		h.ServerUpdateProhibited = false
	default:
		return ErrUnknownHostStatus
	}
	h.SetOKIfNeeded()
	return h.ValidateStatus()
}
