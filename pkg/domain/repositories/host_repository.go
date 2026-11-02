package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// HostRepository is the interface for the HostRepository
type HostRepository interface {
	// CreateHost creates a new host and does NOT create the addresses
	CreateHost(ctx context.Context, h *entities.Host) (*entities.Host, error)
	GetHostByRoid(ctx context.Context, roid int64) (*entities.Host, error)
	// GetHostByNameAndClID gets a host by its name and clid
	GetHostByNameAndClID(ctx context.Context, name string, clid string) (*entities.Host, error)
	UpdateHost(ctx context.Context, h *entities.Host) (*entities.Host, error)
	DeleteHostByRoid(ctx context.Context, roid int64) error
	ListHosts(ctx context.Context, params queries.ListItemsQuery) ([]*entities.Host, string, error)
	GetHostAssociationCount(ctx context.Context, roid int64) (int64, error)
	// BulkCreate creates multiple hosts in a single transaction. If addresses are provided, they will be created as well
	// Should one of the hosts fail to be created, the operation fails and no hosts are created, the error will be returned
	BulkCreate(ctx context.Context, hosts []*entities.Host) error
	Count(ctx context.Context, filter queries.ListHostsFilter) (int64, error)
	// CreateHostUnder creates the host and its addresses in one
	// transaction. A host placed under a domain (parent, empty if none)
	// locks that domain's row first, and is refused unless the domain is
	// registered (entities.ErrEPPSuperordinateDomainNotFound), sponsored by
	// the host's registrar (entities.ErrEPPNotSponsor) and not
	// pendingDelete (entities.ErrEPPSuperordinatePendingDelete).
	CreateHostUnder(ctx context.Context, h *entities.Host, parent string) (*entities.Host, error)
	// UpdateHostAndAddresses saves the host and replaces its addresses with
	// h.Addresses, in one transaction, if the stored host is still as it was
	// read (h.UpdatedAt): otherwise it is entities.ErrHostChanged, and if
	// the host is gone, entities.ErrHostNotFound. A host renamed under a
	// domain (parent, empty if none) is checked as CreateHostUnder checks
	// it.
	UpdateHostAndAddresses(ctx context.Context, h *entities.Host, parent string) (*entities.Host, error)
	// DeleteHostIfUnchanged deletes the host if the stored host is still the
	// one read: the same sponsor and version (h.UpdatedAt). Otherwise it is
	// entities.ErrHostChanged, and if the host is gone,
	// entities.ErrHostNotFound.
	DeleteHostIfUnchanged(ctx context.Context, h *entities.Host) error
	// CountHostsByName counts the host objects named name, whichever
	// registrar sponsors them.
	CountHostsByName(ctx context.Context, name string) (int64, error)
	// ListHostsByName returns every host object named name, whichever
	// registrar sponsors it, with its addresses, oldest first.
	ListHostsByName(ctx context.Context, name string) ([]*entities.Host, error)
	// CountHostLinksFromOtherSponsors counts the domains that have the host
	// as a nameserver and that clid does not sponsor.
	CountHostLinksFromOtherSponsors(ctx context.Context, roid int64, clid string) (int64, error)
	// ListHostNamesUnder returns the names of clid's host objects below
	// domain (RFC 5732 §1.1: any name under it, not the domain itself),
	// in name order.
	ListHostNamesUnder(ctx context.Context, clid, domain string) ([]string, error)
}

// MochHostRepository is the mock implementation of the HostRepository
type MockHostRepository struct {
	CreateHostFunc               func(ctx context.Context, h *entities.Host) (*entities.Host, error)
	GetHostByRoidFunc            func(ctx context.Context, roid int64) (*entities.Host, error)
	GetHostByNameAndClIDFunc     func(ctx context.Context, name string, clid string) (*entities.Host, error)
	GetHostAssociationCountFunc  func(ctx context.Context, roid int64) (int64, error)
	UpdateHostFunc               func(ctx context.Context, h *entities.Host) (*entities.Host, error)
	DeleteHostByRoidFunc         func(ctx context.Context, roid int64) error
	ListHostsFunc                func(ctx context.Context, paramas queries.ListItemsQuery) ([]*entities.Host, string, error)
	BulkCreateFunc               func(ctx context.Context, hosts []*entities.Host) error
	CountFunc                    func(ctx context.Context, filter queries.ListHostsFilter) (int64, error)
	UpdateHostAndAddressesFunc   func(ctx context.Context, h *entities.Host, parent string) (*entities.Host, error)
	CreateHostUnderFunc          func(ctx context.Context, h *entities.Host, parent string) (*entities.Host, error)
	CountHostsByNameFunc         func(ctx context.Context, name string) (int64, error)
	ListHostsByNameFunc          func(ctx context.Context, name string) ([]*entities.Host, error)
	ListHostNamesUnderFunc       func(ctx context.Context, clid, domain string) ([]string, error)
	DeleteHostIfUnchangedFunc    func(ctx context.Context, h *entities.Host) error
	CountHostLinksFromOthersFunc func(ctx context.Context, roid int64, clid string) (int64, error)
}

// CreateHost creates a host
func (m *MockHostRepository) CreateHost(ctx context.Context, h *entities.Host) (*entities.Host, error) {
	return m.CreateHostFunc(ctx, h)
}

// GetHostByRoid gets a host by its roid
func (m *MockHostRepository) GetHostByRoid(ctx context.Context, roid int64) (*entities.Host, error) {
	return m.GetHostByRoidFunc(ctx, roid)
}

// GetHostByNameAndClID gets a host by its name and clid
func (m *MockHostRepository) GetHostByNameAndClID(ctx context.Context, name string, clid string) (*entities.Host, error) {
	return m.GetHostByNameAndClIDFunc(ctx, name, clid)
}

// UpdateHost updates a host
func (m *MockHostRepository) UpdateHost(ctx context.Context, h *entities.Host) (*entities.Host, error) {
	return m.UpdateHostFunc(ctx, h)
}

// DeleteHostByRoid deletes a host by its roid
func (m *MockHostRepository) DeleteHostByRoid(ctx context.Context, roid int64) error {
	return m.DeleteHostByRoidFunc(ctx, roid)
}

// GetHostAssociationCount gets the number of associations a host has
func (m *MockHostRepository) GetHostAssociationCount(ctx context.Context, roid int64) (int64, error) {
	return m.GetHostAssociationCountFunc(ctx, roid)
}

// ListHosts lists hosts
func (m *MockHostRepository) ListHosts(ctx context.Context, params queries.ListItemsQuery) ([]*entities.Host, string, error) {
	return m.ListHostsFunc(ctx, params)
}

// BulkCreate creates multiple hosts
func (m *MockHostRepository) BulkCreate(ctx context.Context, hosts []*entities.Host) error {
	return m.BulkCreateFunc(ctx, hosts)
}

// Count counts hosts
func (m *MockHostRepository) Count(ctx context.Context, filter queries.ListHostsFilter) (int64, error) {
	return m.CountFunc(ctx, filter)
}

// UpdateHostAndAddresses saves a host and its addresses
func (m *MockHostRepository) UpdateHostAndAddresses(ctx context.Context, h *entities.Host, parent string) (*entities.Host, error) {
	return m.UpdateHostAndAddressesFunc(ctx, h, parent)
}

// CreateHostUnder creates a host under a domain
func (m *MockHostRepository) CreateHostUnder(ctx context.Context, h *entities.Host, parent string) (*entities.Host, error) {
	return m.CreateHostUnderFunc(ctx, h, parent)
}

// DeleteHostIfUnchanged deletes a host as read
func (m *MockHostRepository) DeleteHostIfUnchanged(ctx context.Context, h *entities.Host) error {
	return m.DeleteHostIfUnchangedFunc(ctx, h)
}

// CountHostsByName counts hosts by name
func (m *MockHostRepository) CountHostsByName(ctx context.Context, name string) (int64, error) {
	return m.CountHostsByNameFunc(ctx, name)
}

// ListHostsByName lists hosts by name
func (m *MockHostRepository) ListHostsByName(ctx context.Context, name string) ([]*entities.Host, error) {
	return m.ListHostsByNameFunc(ctx, name)
}

// ListHostNamesUnder lists the names of clid's hosts below domain
func (m *MockHostRepository) ListHostNamesUnder(ctx context.Context, clid, domain string) ([]string, error) {
	return m.ListHostNamesUnderFunc(ctx, clid, domain)
}

// CountHostLinksFromOtherSponsors counts other sponsors' domains using a host
func (m *MockHostRepository) CountHostLinksFromOtherSponsors(ctx context.Context, roid int64, clid string) (int64, error) {
	return m.CountHostLinksFromOthersFunc(ctx, roid, clid)
}

// NewMockHostRepository creates a new MockHostRepository
func NewMockHostRepository() *MockHostRepository {
	return &MockHostRepository{
		CreateHostFunc:              func(ctx context.Context, h *entities.Host) (*entities.Host, error) { return nil, nil },
		GetHostByRoidFunc:           func(ctx context.Context, roid int64) (*entities.Host, error) { return nil, nil },
		GetHostByNameAndClIDFunc:    func(ctx context.Context, name string, clid string) (*entities.Host, error) { return nil, nil },
		GetHostAssociationCountFunc: func(ctx context.Context, roid int64) (int64, error) { return 0, nil },
		UpdateHostFunc:              func(ctx context.Context, h *entities.Host) (*entities.Host, error) { return nil, nil },
		DeleteHostByRoidFunc:        func(ctx context.Context, roid int64) error { return nil },
		ListHostsFunc: func(ctx context.Context, params queries.ListItemsQuery) ([]*entities.Host, string, error) {
			return nil, "", nil
		},
		BulkCreateFunc: func(ctx context.Context, hosts []*entities.Host) error { return nil },
		CountFunc:      func(ctx context.Context, filter queries.ListHostsFilter) (int64, error) { return 0, nil },
		UpdateHostAndAddressesFunc: func(ctx context.Context, h *entities.Host, parent string) (*entities.Host, error) {
			return nil, nil
		},
		CreateHostUnderFunc:       func(ctx context.Context, h *entities.Host, parent string) (*entities.Host, error) { return nil, nil },
		CountHostsByNameFunc:      func(ctx context.Context, name string) (int64, error) { return 0, nil },
		ListHostsByNameFunc:       func(ctx context.Context, name string) ([]*entities.Host, error) { return nil, nil },
		ListHostNamesUnderFunc:    func(ctx context.Context, clid, domain string) ([]string, error) { return nil, nil },
		DeleteHostIfUnchangedFunc: func(ctx context.Context, h *entities.Host) error { return nil },
		CountHostLinksFromOthersFunc: func(ctx context.Context, roid int64, clid string) (int64, error) {
			return 0, nil
		},
	}
}
