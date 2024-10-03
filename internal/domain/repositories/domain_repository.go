package repositories

import (
	"context"
	"time"

	"github.com/miekg/dns"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/mock"
)

// DomainRepository is the interface for the DomainRepository
type DomainRepository interface {
	CreateDomain(ctx context.Context, d *entities.Domain) (*entities.Domain, error)
	GetDomainByName(ctx context.Context, name string, preloadHosts bool) (*entities.Domain, error)
	UpdateDomain(ctx context.Context, d *entities.Domain) (*entities.Domain, error)
	DeleteDomainByName(ctx context.Context, name string) error
	ListDomains(ctx context.Context, pageSize int, cursor string) ([]*entities.Domain, error)
	AddHostToDomain(ctx context.Context, domRoid int64, hostRoid int64) error
	RemoveHostFromDomain(ctx context.Context, domRoid int64, hostRoid int64) error
	GetActiveDomainsWithHosts(ctx context.Context, tld string) ([]dns.RR, error)
	GetActiveDomainGlue(ctx context.Context, tld string) ([]dns.RR, error)
	Count(ctx context.Context) (int64, error)
	ListExpiringDomains(ctx context.Context, before time.Time, pageSize int, clid, cursor string) ([]*entities.Domain, error)
	CountExpiringDomains(ctx context.Context, before time.Time, clid string) (int64, error)
}

// MockDomainRepository is the mock implementation of the DomainRepository
type MockDomainRepository struct {
	mock.Mock
}

// CreateDomain creates a new domain
func (m *MockDomainRepository) CreateDomain(ctx context.Context, d *entities.Domain) (*entities.Domain, error) {
	args := m.Called(ctx, d)
	return args.Get(0).(*entities.Domain), args.Error(1)
}

// GetDomainByName retrieves a domain by its name
func (m *MockDomainRepository) GetDomainByName(ctx context.Context, name string, preloadHosts bool) (*entities.Domain, error) {
	args := m.Called(ctx, name, preloadHosts)
	return args.Get(0).(*entities.Domain), args.Error(1)
}

// UpdateDomain updates a domain
func (m *MockDomainRepository) UpdateDomain(ctx context.Context, d *entities.Domain) (*entities.Domain, error) {
	args := m.Called(ctx, d)
	return args.Get(0).(*entities.Domain), args.Error(1)
}

// DeleteDomainByName deletes a domain by its name
func (m *MockDomainRepository) DeleteDomainByName(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

// ListDomains lists all domains
func (m *MockDomainRepository) ListDomains(ctx context.Context, pageSize int, cursor string) ([]*entities.Domain, error) {
	args := m.Called(ctx, pageSize, cursor)
	return args.Get(0).([]*entities.Domain), args.Error(1)
}

// AddHostToDomain adds a host to a domain
func (m *MockDomainRepository) AddHostToDomain(ctx context.Context, domRoid int64, hostRoid int64) error {
	args := m.Called(ctx, domRoid, hostRoid)
	return args.Error(0)
}

// RemoveHostFromDomain removes a host from a domain
func (m *MockDomainRepository) RemoveHostFromDomain(ctx context.Context, domRoid int64, hostRoid int64) error {
	args := m.Called(ctx, domRoid, hostRoid)
	return args.Error(0)
}

// GetActiveDomainsWithHosts retrieves active domains with hosts
func (m *MockDomainRepository) GetActiveDomainsWithHosts(ctx context.Context, tld string) ([]dns.RR, error) {
	args := m.Called(ctx, tld)
	return args.Get(0).([]dns.RR), args.Error(1)
}

// GetActiveDomainGlue retrieves active domain glue
func (m *MockDomainRepository) GetActiveDomainGlue(ctx context.Context, tld string) ([]dns.RR, error) {
	args := m.Called(ctx, tld)
	return args.Get(0).([]dns.RR), args.Error(1)
}

// Count counts the number of domains
func (m *MockDomainRepository) Count(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

// ListExpiringDomains lists expiring domains
func (m *MockDomainRepository) ListExpiringDomains(ctx context.Context, before time.Time, pageSize int, clid, cursor string) ([]*entities.Domain, error) {
	args := m.Called(ctx, before, pageSize, clid, cursor)
	return args.Get(0).([]*entities.Domain), args.Error(1)
}

// CountExpiringDomains counts the number of expiring domains
func (m *MockDomainRepository) CountExpiringDomains(ctx context.Context, before time.Time, clid string) (int64, error) {
	args := m.Called(ctx, before, clid)
	return args.Get(0).(int64), args.Error(1)
}
