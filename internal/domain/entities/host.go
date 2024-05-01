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
	ErrInvalidHost                 = fmt.Errorf("invalid host")
	ErrHostNotFound                = fmt.Errorf("host not found")
	ErrHostAlreadyExists           = fmt.Errorf("host already exists - hostnames must be unique for every registrar")
	ErrDuplicateHostAddress        = fmt.Errorf("duplicate host address")
	ErrHostAddressNotFound         = fmt.Errorf("host address not found")
	ErrMaxAddressesPerHostExceeded = fmt.Errorf("maximum number of %d addresses per host exceeded", MAX_ADDRESSES_PER_HOST)
	ErrHostStatusIncompatible      = fmt.Errorf("host status is incompatible")
	ErrOKStatusMustBeSet           = fmt.Errorf("ok status must be set when no prohibitions are set")
	ErrUnknownHostStatus           = fmt.Errorf("unknown host status")
	ErrHostUpdateProhibited        = fmt.Errorf("host update is prohibited")
	ErrInvalidHostRoID             = fmt.Errorf("invalid Host.RoID.ObjectIdentifier(), expecting '%s'", HOST_ROID_ID)
)

// Host struct represents a host object based on https://datatracker.ietf.org/doc/html/rfc5732
type Host struct {
	RoID      RoidType     `json:"RoID" example:"1729468286778740736_HOST-APEX" extensions:"x-order=0"`
	Name      DomainName   `json:"Name" exmaple:"ns1.apex.domains" extensions:"x-order=1"`
	Addresses []netip.Addr `json:"Addresses"`
	ClID      ClIDType     `json:"ClID" example:"sh8013"`
	CrRr      ClIDType     `json:"CrRr" example:"sh8013"`
	UpRr      ClIDType     `json:"UpRr" example:"sh8013"`
	CreatedAt time.Time    `json:"CrDate" example:"2023-04-03T22:00:00.0Z"`
	UpdatedAt time.Time    `json:"UpDate" example:"2023-04-03T22:00:00.0Z"`
	// True if the host is used on a domain that is the parent of the host's FQDN. https://datatracker.ietf.org/doc/html/rfc5732#section-1.1
	// This is set/unset by the Domain.AddHost() and Domain.RemoveHost() when a host is added/removed from a domain.
	InBailiwick bool       `json:"InBailiwick"  example:"true"` // Not implemented yet
	Status      HostStatus `json:"Status"`
}

// HostStatus is an implementation of https://datatracker.ietf.org/doc/html/rfc5732#section-2.3
type HostStatus struct {
	OK                     bool `json:"OK" example:"true"`
	Linked                 bool `json:"Linked" example:"true"`
	PendingCreate          bool `json:"PendingCreate" example:"false"`
	PendingDelete          bool `json:"PendingDelete" example:"false"`
	PendingUpdate          bool `json:"PendingUpdate" example:"false"`
	PendingTransfer        bool `json:"PendingTransfer" example:"false"`
	ClientDeleteProhibited bool `json:"ClientDeleteProhibited" example:"false"`
	ClientUpdateProhibited bool `json:"ClientUpdateProhibited" example:"false"`
	ServerDeleteProhibited bool `json:"ServerDeleteProhibited" example:"false"`
	ServerUpdateProhibited bool `json:"ServerUpdateProhibited" example:"false"`
}

// IsNil checks if the HostStatus is nil (all fields are false)
func (hs *HostStatus) IsNil() bool {
	return !hs.OK && !hs.Linked && !hs.PendingCreate && !hs.PendingDelete && !hs.PendingUpdate && !hs.PendingTransfer && !hs.ClientDeleteProhibited && !hs.ClientUpdateProhibited && !hs.ServerDeleteProhibited && !hs.ServerUpdateProhibited
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
		Status: HostStatus{
			OK: true,
		},
	}
	return h, nil
}

// AddAddress adds a new address to the host. It will return an error if the address already exists or if the maximum number of addresses per host is exceeded. Or if the address is invalid
func (h *Host) AddAddress(addr string) (*netip.Addr, error) {
	if !h.CanBeUpdated() {
		return nil, ErrHostUpdateProhibited
	}
	if len(h.Addresses) >= MAX_ADDRESSES_PER_HOST {
		return nil, ErrMaxAddressesPerHostExceeded
	}
	// Check if its valid
	a, err := netip.ParseAddr(addr)
	if err != nil {
		return nil, ErrInvalidIP
	}
	// Check if it already exists
	for _, address := range h.Addresses {
		if address.String() == a.String() {
			return nil, ErrDuplicateHostAddress
		}
	}
	h.Addresses = append(h.Addresses, a)
	return &a, nil
}

// RemoveAddress removes an address from the host. If the address is not found or invalid it will return an error
func (h *Host) RemoveAddress(addr string) (*netip.Addr, error) {
	if len(h.Addresses) == 0 {
		return nil, ErrHostAddressNotFound
	}
	// Check if its valid
	a, err := netip.ParseAddr(addr)
	if err != nil {
		return nil, ErrInvalidIP
	}
	// Remove it
	for i, address := range h.Addresses {
		if address.String() == a.String() {
			h.Addresses = append(h.Addresses[:i], h.Addresses[i+1:]...)
			return &address, nil
		}
	}
	// If we didn't return yet, the address was Not found
	return nil, ErrHostAddressNotFound
}

// CanBeDeleted returns true if the host can be deleted and returns false if a status is set that prevents deletion (ServerDeleteProhibited or ClientDeleteProhibited)
func (h *Host) CanBeDeleted() bool {
	if h.Status.ServerDeleteProhibited || h.Status.ClientDeleteProhibited {
		return false
	}
	return true
}

// CanBeUpdated returns true if the host can be updated and returns false if a status is set that prevents update (ServerUpdateProhibited or ClientUpdateProhibited)
func (h *Host) CanBeUpdated() bool {
	if h.Status.ServerUpdateProhibited || h.Status.ClientUpdateProhibited {
		return false
	}
	return true
}

// ValidateStatus validates the status of the host and returns an error if the statuses are incompatible according to https://datatracker.ietf.org/doc/html/rfc5732#section-2.3
func (h *Host) ValidateStatus() error {
	// The pendingCreate, pendingDelete, pendingTransfer, and pendingUpdate status values MUST NOT be combined with each other.
	if h.Status.PendingDelete && (h.Status.PendingTransfer || h.Status.PendingUpdate || h.Status.PendingCreate) {
		return ErrHostStatusIncompatible
	}
	if h.Status.PendingCreate && (h.Status.PendingTransfer || h.Status.PendingUpdate || h.Status.PendingDelete) {
		return ErrHostStatusIncompatible
	}
	if h.Status.PendingUpdate && (h.Status.PendingTransfer || h.Status.PendingDelete || h.Status.PendingCreate) {
		return ErrHostStatusIncompatible
	}
	// "pendingUpdate" status MUST NOT be combined with either "clientUpdateProhibited" or "serverUpdateProhibited" status.
	if h.Status.PendingUpdate && (h.Status.ClientUpdateProhibited || h.Status.ServerUpdateProhibited) {
		return ErrHostStatusIncompatible
	}
	// "pendingDelete" status MUST NOT be combined with either "clientDeleteProhibited" or "serverDeleteProhibited" status.
	if h.Status.PendingDelete && (h.Status.ClientDeleteProhibited || h.Status.ServerDeleteProhibited) {
		return ErrHostStatusIncompatible
	}
	// "ok" status MAY only be combined with "linked" status.
	if h.Status.OK && (h.Status.PendingCreate || h.Status.PendingDelete || h.Status.PendingTransfer || h.Status.PendingUpdate || h.Status.ClientDeleteProhibited || h.Status.ClientUpdateProhibited || h.Status.ServerDeleteProhibited || h.Status.ServerUpdateProhibited) {
		return ErrHostStatusIncompatible
	}
	// "ok" status MUST be set when no other status is set.
	if !h.Status.OK && !h.Status.PendingCreate && !h.Status.PendingDelete && !h.Status.PendingTransfer && !h.Status.PendingUpdate && !h.Status.ClientDeleteProhibited && !h.Status.ClientUpdateProhibited && !h.Status.ServerDeleteProhibited && !h.Status.ServerUpdateProhibited {
		return ErrOKStatusMustBeSet
	}
	// Other status combinations not expressly prohibited MAY be used.
	return nil
}

func (h *Host) SetOKIfNeeded() {
	if !h.Status.PendingCreate && !h.Status.PendingDelete && !h.Status.PendingTransfer && !h.Status.PendingUpdate && !h.Status.ClientDeleteProhibited && !h.Status.ClientUpdateProhibited && !h.Status.ServerDeleteProhibited && !h.Status.ServerUpdateProhibited {
		h.Status.OK = true
	}

}

// UnsetOKIfNeeded unsets the OK status if any other status prohibition is set
func (h *Host) UnsetOKIfNeeded() {
	if h.Status.PendingCreate || h.Status.PendingDelete || h.Status.PendingTransfer || h.Status.PendingUpdate || h.Status.ClientDeleteProhibited || h.Status.ClientUpdateProhibited || h.Status.ServerDeleteProhibited || h.Status.ServerUpdateProhibited {
		h.Status.OK = false
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
		h.Status.OK = true
	case HostStatusLinked:
		h.Status.Linked = true
	case HostStatusPendingCreate:
		h.Status.PendingCreate = true
	case HostStatusPendingDelete:
		h.Status.PendingDelete = true
	case HostStatusPendingUpdate:
		h.Status.PendingUpdate = true
	case HostStatusPendingTransfer:
		h.Status.PendingTransfer = true
	case HostStatusClientDeleteProhibited:
		h.Status.ClientDeleteProhibited = true
	case HostStatusClientUpdateProhibited:
		h.Status.ClientUpdateProhibited = true
	case HostStatusServerDeleteProhibited:
		h.Status.ServerDeleteProhibited = true
	case HostStatusServerUpdateProhibited:
		h.Status.ServerUpdateProhibited = true
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
		h.Status.OK = false
	case HostStatusLinked:
		h.Status.Linked = false
	case HostStatusPendingCreate:
		h.Status.PendingCreate = false
	case HostStatusPendingDelete:
		h.Status.PendingDelete = false
	case HostStatusPendingUpdate:
		h.Status.PendingUpdate = false
	case HostStatusPendingTransfer:
		h.Status.PendingTransfer = false
	case HostStatusClientDeleteProhibited:
		h.Status.ClientDeleteProhibited = false
	case HostStatusClientUpdateProhibited:
		h.Status.ClientUpdateProhibited = false
	case HostStatusServerDeleteProhibited:
		h.Status.ServerDeleteProhibited = false
	case HostStatusServerUpdateProhibited:
		h.Status.ServerUpdateProhibited = false
	default:
		return ErrUnknownHostStatus
	}
	h.SetOKIfNeeded()
	return h.ValidateStatus()
}

// IsValid checks if the host is valid including all its statuses and addresses
func (h *Host) Validate() error {
	if err := h.Name.Validate(); err != nil {
		return err
	}
	if err := h.RoID.Validate(); err != nil {
		return err
	}
	if h.RoID.ObjectIdentifier() != HOST_ROID_ID {
		return ErrInvalidHostRoID
	}
	if err := h.ClID.Validate(); err != nil {
		return err
	}
	if err := h.ValidateStatus(); err != nil {
		return err
	}
	for _, address := range h.Addresses {
		if !address.IsValid() {
			return ErrInvalidIP
		}
	}
	return nil
}
