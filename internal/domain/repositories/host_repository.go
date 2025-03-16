package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
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
}

// MochHostRepository is the mock implementation of the HostRepository
type MockHostRepository struct {
	CreateHostFunc              func(ctx context.Context, h *entities.Host) (*entities.Host, error)
	GetHostByRoidFunc           func(ctx context.Context, roid int64) (*entities.Host, error)
	GetHostByNameAndClIDFunc    func(ctx context.Context, name string, clid string) (*entities.Host, error)
	GetHostAssociationCountFunc func(ctx context.Context, roid int64) (int64, error)
	UpdateHostFunc              func(ctx context.Context, h *entities.Host) (*entities.Host, error)
	DeleteHostByRoidFunc        func(ctx context.Context, roid int64) error
	ListHostsFunc               func(ctx context.Context, paramas queries.ListItemsQuery) ([]*entities.Host, string, error)
	BulkCreateFunc              func(ctx context.Context, hosts []*entities.Host) error
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
	}
}
