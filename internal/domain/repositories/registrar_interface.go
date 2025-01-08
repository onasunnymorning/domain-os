package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/mock"
)

// RegistrarRepository is the interface for the registrar repository
type RegistrarRepository interface {
	GetByClID(ctx context.Context, clid string, preloadTLDs bool) (*entities.Registrar, error)
	GetByGurID(ctx context.Context, gurID int) (*entities.Registrar, error)
	Create(ctx context.Context, rar *entities.Registrar) (*entities.Registrar, error)
	BulkCreate(ctx context.Context, cmds []*entities.Registrar) error
	Update(ctx context.Context, rar *entities.Registrar) (*entities.Registrar, error)
	Delete(ctx context.Context, clid string) error
	List(ctx context.Context, pagesize int, pagecursor string) ([]*entities.Registrar, error)
	Count(ctx context.Context) (int64, error)
	IsRegistrarAccreditedForTLD(ctx context.Context, tldName, rarClID string) (bool, error)
}

// MockRegistrarRepository is the mock implementation of the RegistrarRepository
type MockRegistrarRepository struct {
	mock.Mock
}

// GetByClID retrieves a registrar by its clid
func (m *MockRegistrarRepository) GetByClID(ctx context.Context, clid string, preloadTLDs bool) (*entities.Registrar, error) {
	args := m.Called(ctx, clid, preloadTLDs)
	return args.Get(0).(*entities.Registrar), args.Error(1)
}

// GetByGurID retrieves a registrar by its gurid
func (m *MockRegistrarRepository) GetByGurID(ctx context.Context, gurID int) (*entities.Registrar, error) {
	args := m.Called(ctx, gurID)
	return args.Get(0).(*entities.Registrar), args.Error(1)
}

// Create creates a new registrar
func (m *MockRegistrarRepository) Create(ctx context.Context, rar *entities.Registrar) (*entities.Registrar, error) {
	args := m.Called(ctx, rar)
	return args.Get(0).(*entities.Registrar), args.Error(1)
}

// BulkCreate creates multiple registrars
func (m *MockRegistrarRepository) BulkCreate(ctx context.Context, cmds []*entities.Registrar) error {
	args := m.Called(ctx, cmds)
	return args.Error(0)
}

// Update updates a registrar
func (m *MockRegistrarRepository) Update(ctx context.Context, rar *entities.Registrar) (*entities.Registrar, error) {
	args := m.Called(ctx, rar)
	return args.Get(0).(*entities.Registrar), args.Error(1)
}

// Delete deletes a registrar by its clid
func (m *MockRegistrarRepository) Delete(ctx context.Context, clid string) error {
	args := m.Called(ctx, clid)
	return args.Error(0)
}

// List lists all registrars
func (m *MockRegistrarRepository) List(ctx context.Context, pagesize int, pagecursor string) ([]*entities.Registrar, error) {
	args := m.Called(ctx, pagesize, pagecursor)
	return args.Get(0).([]*entities.Registrar), args.Error(1)
}

// Count counts the number of registrars
func (m *MockRegistrarRepository) Count(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

// IsRegistrarAccreditedForTLD checks if a registrar is accredited for a tld
func (m *MockRegistrarRepository) IsRegistrarAccreditedForTLD(ctx context.Context, clid, tld string) (bool, error) {
	args := m.Called(ctx, clid, tld)
	return args.Bool(0), args.Error(1)
}
