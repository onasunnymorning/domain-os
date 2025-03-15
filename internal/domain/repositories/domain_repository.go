package repositories

import (
	"context"
	"time"

	"github.com/miekg/dns"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/mock"
)

// DomainRepository is the interface for the DomainRepository
type DomainRepository interface {
	Create(ctx context.Context, d *entities.Domain) (*entities.Domain, error)
	GetDomainByName(ctx context.Context, name string, preloadHosts bool) (*entities.Domain, error)
	UpdateDomain(ctx context.Context, d *entities.Domain) (*entities.Domain, error)
	DeleteDomainByName(ctx context.Context, name string) error
	ListDomains(ctx context.Context, params queries.ListDomainsQuery) ([]*entities.Domain, error)
	AddHostToDomain(ctx context.Context, domRoid int64, hostRoid int64) error
	RemoveHostFromDomain(ctx context.Context, domRoid int64, hostRoid int64) error
	GetActiveDomainsWithHosts(ctx context.Context, params queries.ActiveDomainsWithHostsQuery) ([]dns.RR, error)
	GetActiveDomainGlue(ctx context.Context, tld string) ([]dns.RR, error)
	Count(ctx context.Context) (int64, error)
	ListExpiringDomains(ctx context.Context, before time.Time, pageSize int, clid, tld, cursor string) ([]*entities.Domain, error)
	CountExpiringDomains(ctx context.Context, before time.Time, clid, tld string) (int64, error)
	ListPurgeableDomains(ctx context.Context, after time.Time, pageSize int, clid, tld, cursor string) ([]*entities.Domain, error)
	CountPurgeableDomains(ctx context.Context, after time.Time, clid, tld string) (int64, error)
	ListRestoredDomains(ctx context.Context, pageSize int, clid, tld, cursor string) ([]*entities.Domain, error)
	CountRestoredDomains(ctx context.Context, clid, tld string) (int64, error)
	BulkCreate(ctx context.Context, domains []*entities.Domain) error
}

// MockDomainRepository is the mock implementation of the DomainRepository
type MockDomainRepository struct {
	mock.Mock
}

// CreateDomain creates a new domain
func (m *MockDomainRepository) Create(ctx context.Context, d *entities.Domain) (*entities.Domain, error) {
	args := m.Called(ctx, d)
	return args.Get(0).(*entities.Domain), args.Error(1)
}

// BulkCreate creates multiple domains
func (m *MockDomainRepository) BulkCreate(ctx context.Context, domains []*entities.Domain) error {
	args := m.Called(ctx, domains)
	return args.Error(0)
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
func (m *MockDomainRepository) ListDomains(ctx context.Context, params queries.ListDomainsQuery) ([]*entities.Domain, error) {
	args := m.Called(ctx, params)
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
func (m *MockDomainRepository) GetActiveDomainsWithHosts(ctx context.Context, params queries.ActiveDomainsWithHostsQuery) ([]dns.RR, error) {
	args := m.Called(ctx, params)
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
func (m *MockDomainRepository) ListExpiringDomains(ctx context.Context, before time.Time, pageSize int, clid, tld, cursor string) ([]*entities.Domain, error) {
	args := m.Called(ctx, before, pageSize, clid, cursor)
	return args.Get(0).([]*entities.Domain), args.Error(1)
}

// CountExpiringDomains counts the number of expiring domains
func (m *MockDomainRepository) CountExpiringDomains(ctx context.Context, before time.Time, clid, tld string) (int64, error) {
	args := m.Called(ctx, before, clid)
	return args.Get(0).(int64), args.Error(1)
}

// ListPurgeableDomains lists purgeable domains
func (m *MockDomainRepository) ListPurgeableDomains(ctx context.Context, before time.Time, pageSize int, clid, tld, cursor string) ([]*entities.Domain, error) {
	args := m.Called(ctx, before, pageSize, clid, cursor)
	return args.Get(0).([]*entities.Domain), args.Error(1)
}

// CountPurgeableDomains counts the number of purgeable domains
func (m *MockDomainRepository) CountPurgeableDomains(ctx context.Context, before time.Time, clid, tld string) (int64, error) {
	args := m.Called(ctx, before, clid)
	return args.Get(0).(int64), args.Error(1)
}

// ListRestoredDomains lists restored domains
func (m *MockDomainRepository) ListRestoredDomains(ctx context.Context, pageSize int, clid, tld, cursor string) ([]*entities.Domain, error) {
	args := m.Called(ctx, pageSize, clid, tld, cursor)
	return args.Get(0).([]*entities.Domain), args.Error(1)
}

// CountRestoredDomains counts the number of restored domains
func (m *MockDomainRepository) CountRestoredDomains(ctx context.Context, clid, tld string) (int64, error) {
	args := m.Called(ctx, clid, tld)
	return args.Get(0).(int64), args.Error(1)
}
