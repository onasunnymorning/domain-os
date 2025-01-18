package interfaces

import (
	"context"

	"github.com/miekg/dns"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

type DomainService interface {
	// These are ADMIN services
	GetDomainByName(ctx context.Context, name string, preloadHosts bool) (*entities.Domain, error)
	CreateDomain(ctx context.Context, cmd *commands.CreateDomainCommand) (*entities.Domain, error)
	DeleteDomainByName(ctx context.Context, name string) error
	ListDomains(ctx context.Context, pageSize int, cursor string) ([]*entities.Domain, error)
	UpdateDomain(ctx context.Context, name string, cmd *commands.UpdateDomainCommand) (*entities.Domain, error)
	AddHostToDomain(ctx context.Context, name string, hostRoID string, force bool) error
	AddHostToDomainByHostName(ctx context.Context, domainName, hostName string, force bool) error
	RemoveAllDomainHosts(ctx context.Context, name string) error
	RemoveHostFromDomain(ctx context.Context, name string, hostRoID string) error
	RemoveHostFromDomainByHostName(ctx context.Context, domainName, hostName string) error
	DropCatchDomain(ctx context.Context, name string, dropcatch bool) error
	Count(ctx context.Context) (int64, error)
	ListExpiringDomains(ctx context.Context, q *queries.ExpiringDomainsQuery, pageSize int, cursor string) ([]*entities.Domain, error)
	CountExpiringDomains(ctx context.Context, q *queries.ExpiringDomainsQuery) (int64, error)
	ListPurgeableDomains(ctx context.Context, q *queries.PurgeableDomainsQuery, pageSize int, cursor string) ([]*entities.Domain, error)
	CountPurgeableDomains(ctx context.Context, q *queries.PurgeableDomainsQuery) (int64, error)

	// These are Registrar services
	// CheckDomain checks if a domain is available
	CheckDomainAvailability(ctx context.Context, domainname, phaseName string) (*queries.DomainCheckResult, error)
	// GetQuote returns a quote for a domain transaction
	GetQuote(ctx context.Context, q *queries.QuoteRequest) (*entities.Quote, error)
	// RegisterDomain registers a domain as a registrar and supports the fee extension
	RegisterDomain(ctx context.Context, cmd *commands.RegisterDomainCommand) (*entities.Domain, error)
	// RenewDomain renews a domain as a registrar and supports the fee extension
	RenewDomain(ctx context.Context, cmd *commands.RenewDomainCommand) (*entities.Domain, error)
	// CanAutoRenewDomain checks if a domain can be auto-renewed
	CanAutoRenew(ctx context.Context, domainName string) (bool, error)
	// AutoRenewDomain renews the domain for the current registrar
	AutoRenewDomain(ctx context.Context, domainName string, years int) (*entities.Domain, error)
	// MarkDomainForDelete marks a domain for deletion as a registrar
	MarkDomainForDeletion(ctx context.Context, domainName string) (*entities.Domain, error)
	// ExpireDomain expires a domain
	ExpireDomain(ctx context.Context, domainName string) (*entities.Domain, error)
	// RestoreDomain restores a domain as a registrar
	RestoreDomain(ctx context.Context, domainName string) (*entities.Domain, error)
	// PurgeDomain purges a domain after it has reached it's purge date
	PurgeDomain(ctx context.Context, domainName string) error

	// These are DNS services
	GetNSRecordsPerTLD(ctx context.Context, tld string) ([]dns.RR, error)
	GetGlueRecordsPerTLD(ctx context.Context, tld string) ([]dns.RR, error)
}
