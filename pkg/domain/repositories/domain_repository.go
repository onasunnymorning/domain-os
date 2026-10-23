package repositories

import (
	"context"
	"time"

	"github.com/miekg/dns"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	"github.com/stretchr/testify/mock"
)

// DomainRepository is the interface for the DomainRepository
type DomainRepository interface {
	Create(ctx context.Context, d *entities.Domain) (*entities.Domain, error)
	GetDomainByName(ctx context.Context, name string, preloadHosts bool) (*entities.Domain, error)
	UpdateDomain(ctx context.Context, d *entities.Domain) (*entities.Domain, error)
	// UpdateDomainIfUnchanged saves the domain, its nameservers and its DS
	// records in one transaction, if the stored domain is still as it was
	// read (d.UpdatedAt) and sponsored by d.ClID: otherwise it is
	// entities.ErrDomainChanged, and if the domain is gone,
	// entities.ErrDomainNotFound.
	UpdateDomainIfUnchanged(ctx context.Context, d *entities.Domain) (*entities.Domain, error)
	// UpdateDomainColumnsIfUnchanged saves the domain's own columns, not
	// its nameservers or DS records, on the same condition as
	// UpdateDomainIfUnchanged, and queues msgs, the poll messages the
	// change causes, in the same transaction.
	UpdateDomainColumnsIfUnchanged(ctx context.Context, d *entities.Domain, msgs ...*entities.PollMessage) (*entities.Domain, error)
	// DeleteDomainByName deletes within scope; out of scope reads as not found.
	DeleteDomainByName(ctx context.Context, scope entities.RegistryScope, name string) error
	ListDomains(ctx context.Context, params queries.ListItemsQuery) ([]*entities.Domain, string, error)
	AddHostToDomain(ctx context.Context, domRoid int64, hostRoid int64) error
	RemoveHostFromDomain(ctx context.Context, domRoid int64, hostRoid int64) error
	GetActiveDomainsWithHosts(ctx context.Context, params queries.ActiveDomainsWithHostsQuery) ([]dns.RR, error)
	GetActiveDomainGlue(ctx context.Context, tld string) ([]dns.RR, error)
	// GetActiveDomainDS returns the DS records of the TLD's delegated
	// domains: active, not pending delete, with at least one nameserver.
	GetActiveDomainDS(ctx context.Context, tld string) ([]dns.RR, error)
	Count(ctx context.Context, filter queries.ListDomainsFilter) (int64, error)
	ListExpiringDomains(ctx context.Context, before time.Time, pageSize int, clid, tld, cursor string) ([]*entities.Domain, error)
	CountExpiringDomains(ctx context.Context, before time.Time, clid, tld string) (int64, error)
	ListPurgeableDomains(ctx context.Context, before time.Time, pageSize int, clid, tld, cursor string) ([]*entities.Domain, error)
	CountPurgeableDomains(ctx context.Context, before time.Time, clid, tld string) (int64, error)
	ListRestoredDomains(ctx context.Context, pageSize int, clid, tld, cursor string) ([]*entities.Domain, error)
	CountRestoredDomains(ctx context.Context, clid, tld string) (int64, error)
	// ListLapsedRestores returns up to limit domains whose restore lapsed by
	// dueBy: pendingRestore, with a PendingRestoreEnd at or before dueBy, in
	// (PendingRestoreEnd, ROID) order after the cursor (afterEnd, afterRoID;
	// zero for the start). Only their name, ROID and PendingRestoreEnd are
	// read. A sweep pages with the cursor, so a domain that cannot be
	// reverted is passed over until the next sweep.
	ListLapsedRestores(ctx context.Context, dueBy, afterEnd time.Time, afterRoID int64, limit int) ([]*entities.Domain, error)
	BulkCreate(ctx context.Context, domains []*entities.Domain) error
	GetDomainsByNames(ctx context.Context, names []string, preloadHosts bool) ([]*entities.Domain, error)
	ListEventsByDomain(ctx context.Context, domainName string) ([]entities.DomainEvent, error)
	ListRecentEvents(ctx context.Context, limit int) ([]entities.DomainEvent, error)
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

// UpdateDomainIfUnchanged saves a domain as read
func (m *MockDomainRepository) UpdateDomainIfUnchanged(ctx context.Context, d *entities.Domain) (*entities.Domain, error) {
	args := m.Called(ctx, d)
	return args.Get(0).(*entities.Domain), args.Error(1)
}

// UpdateDomainColumnsIfUnchanged saves a domain's columns as read
func (m *MockDomainRepository) UpdateDomainColumnsIfUnchanged(ctx context.Context, d *entities.Domain, _ ...*entities.PollMessage) (*entities.Domain, error) {
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

// GetDomainsByNames retrieves multiple domains by their names
func (m *MockDomainRepository) GetDomainsByNames(ctx context.Context, names []string, preloadHosts bool) ([]*entities.Domain, error) {
	args := m.Called(ctx, names, preloadHosts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entities.Domain), args.Error(1)
}

// UpdateDomain updates a domain
func (m *MockDomainRepository) UpdateDomain(ctx context.Context, d *entities.Domain) (*entities.Domain, error) {
	args := m.Called(ctx, d)
	return args.Get(0).(*entities.Domain), args.Error(1)
}

// DeleteDomainByName deletes a domain by its name
func (m *MockDomainRepository) DeleteDomainByName(ctx context.Context, _ entities.RegistryScope, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

// ListDomains lists all domains
func (m *MockDomainRepository) ListDomains(ctx context.Context, params queries.ListItemsQuery) ([]*entities.Domain, string, error) {
	args := m.Called(ctx, params)
	return args.Get(0).([]*entities.Domain), "", args.Error(1)
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

// GetActiveDomainDS retrieves the DS records of active domains
func (m *MockDomainRepository) GetActiveDomainDS(ctx context.Context, tld string) ([]dns.RR, error) {
	args := m.Called(ctx, tld)
	return args.Get(0).([]dns.RR), args.Error(1)
}

// Count counts the number of domains
func (m *MockDomainRepository) Count(ctx context.Context, filter queries.ListDomainsFilter) (int64, error) {
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

// ListLapsedRestores lists domains whose restore lapsed
func (m *MockDomainRepository) ListLapsedRestores(ctx context.Context, dueBy, afterEnd time.Time, afterRoID int64, limit int) ([]*entities.Domain, error) {
	args := m.Called(ctx, dueBy, afterEnd, afterRoID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entities.Domain), args.Error(1)
}

// ListEventsByDomain lists events by domain name
func (m *MockDomainRepository) ListEventsByDomain(ctx context.Context, domainName string) ([]entities.DomainEvent, error) {
	args := m.Called(ctx, domainName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]entities.DomainEvent), args.Error(1)
}

// ListRecentEvents lists the most recent events across all subjects
func (m *MockDomainRepository) ListRecentEvents(ctx context.Context, limit int) ([]entities.DomainEvent, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]entities.DomainEvent), args.Error(1)
}
