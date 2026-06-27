package askg

import (
	"context"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// InProcessToolExecutor tests — using the same mock patterns as MCP server
// ---------------------------------------------------------------------------

func TestInProcessToolExecutor_GetDomain_Found(t *testing.T) {
	now := time.Now().UTC()
	created := now.Add(-365 * 24 * time.Hour)
	expiry := now.Add(365 * 24 * time.Hour)

	testDomain := &entities.Domain{
		Name:       entities.DomainName("example.best"),
		ClID:       entities.ClIDType("registrar1"),
		CreatedAt:  created,
		ExpiryDate: expiry,
		Status:     entities.DomainStatus{OK: true},
		RGPStatus:  entities.DomainRGPStatus{},
		Hosts: []*entities.Host{
			{Name: entities.DomainName("ns1.example.best")},
			{Name: entities.DomainName("ns2.example.best")},
		},
	}

	exec := NewInProcessToolExecutor(
		&mockDomainSvc{getByName: func(_ context.Context, name string, _ bool) (*entities.Domain, error) {
			assert.Equal(t, "example.best", name)
			return testDomain, nil
		}},
		&mockTLDSvc{getByName: func(_ context.Context, _ string, _ bool) (*entities.TLD, error) {
			return nil, entities.ErrTLDNotFound
		}},
		newDiscardLogger(),
	)

	result := exec.Execute(context.Background(), ToolCall{
		ID:    "test-1",
		Name:  "get_domain",
		Input: map[string]any{"name": "Example.BEST"},
	}, CallerScope{UserID: "test-staff"})

	assert.False(t, result.IsError)
	assert.Equal(t, "test-1", result.CallID)
	require.NotNil(t, result.Result)
}

func TestInProcessToolExecutor_GetDomain_NotFound(t *testing.T) {
	exec := NewInProcessToolExecutor(
		&mockDomainSvc{getByName: func(_ context.Context, _ string, _ bool) (*entities.Domain, error) {
			return nil, entities.ErrDomainNotFound
		}},
		nil,
		newDiscardLogger(),
	)

	result := exec.Execute(context.Background(), ToolCall{
		ID:    "test-2",
		Name:  "get_domain",
		Input: map[string]any{"name": "missing.best"},
	}, CallerScope{UserID: "test-staff"})

	assert.True(t, result.IsError)
}

func TestInProcessToolExecutor_GetDomain_InvalidInput(t *testing.T) {
	exec := NewInProcessToolExecutor(nil, nil, newDiscardLogger())

	tests := []struct {
		name  string
		input any
	}{
		{"empty name", map[string]any{"name": ""}},
		{"no dot", map[string]any{"name": "nodot"}},
		{"missing field", map[string]any{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exec.Execute(context.Background(), ToolCall{
				ID: "test", Name: "get_domain", Input: tt.input,
			}, CallerScope{UserID: "test"})
			assert.True(t, result.IsError)
		})
	}
}

func TestInProcessToolExecutor_GetTLD_Found(t *testing.T) {
	now := time.Now().UTC()
	phaseEnd := now.Add(365 * 24 * time.Hour)
	allowAutoRenew := true

	testTLD := &entities.TLD{
		Name:      entities.DomainName("best"),
		UName:     entities.DomainName(""),
		Type:      entities.TLDTypeGTLD,
		RyID:      entities.ClIDType("apex"),
		EnableDNS: true,
		CreatedAt: now.Add(-730 * 24 * time.Hour),
		UpdatedAt: now.Add(-1 * time.Hour),
		Phases: []entities.Phase{
			{
				Name:   entities.ClIDType("ga-1"),
				Type:   entities.PhaseTypeGA,
				Starts: now.Add(-365 * 24 * time.Hour),
				Ends:   &phaseEnd,
				Prices: []entities.Price{
					{Currency: "USD", RegistrationAmount: 1000, RenewalAmount: 1000, TransferAmount: 1000, RestoreAmount: 5000},
				},
				Policy: entities.PhasePolicy{
					MinLabelLength: 3,
					MaxLabelLength: 63,
					RedemptionGP:   30,
					PendingDeleteGP: 5,
					AllowAutoRenew: &allowAutoRenew,
					BaseCurrency:   "USD",
				},
			},
		},
	}

	exec := NewInProcessToolExecutor(
		nil,
		&mockTLDSvc{getByName: func(_ context.Context, name string, _ bool) (*entities.TLD, error) {
			assert.Equal(t, "best", name)
			return testTLD, nil
		}},
		newDiscardLogger(),
	)

	result := exec.Execute(context.Background(), ToolCall{
		ID:    "test-3",
		Name:  "get_tld",
		Input: map[string]any{"name": ".BEST"},
	}, CallerScope{UserID: "test-staff"})

	assert.False(t, result.IsError)
	assert.Equal(t, "test-3", result.CallID)
}

func TestInProcessToolExecutor_GetTLD_NotFound(t *testing.T) {
	exec := NewInProcessToolExecutor(
		nil,
		&mockTLDSvc{getByName: func(_ context.Context, _ string, _ bool) (*entities.TLD, error) {
			return nil, entities.ErrTLDNotFound
		}},
		newDiscardLogger(),
	)

	result := exec.Execute(context.Background(), ToolCall{
		ID:    "test-4",
		Name:  "get_tld",
		Input: map[string]any{"name": "nonexistent"},
	}, CallerScope{UserID: "test-staff"})

	assert.True(t, result.IsError)
}

func TestInProcessToolExecutor_UnknownTool(t *testing.T) {
	exec := NewInProcessToolExecutor(nil, nil, newDiscardLogger())

	result := exec.Execute(context.Background(), ToolCall{
		ID:    "test-5",
		Name:  "unknown_tool",
		Input: map[string]any{},
	}, CallerScope{UserID: "test-staff"})

	assert.True(t, result.IsError)
}

func TestInProcessToolExecutor_Tools_ReturnsDefinitions(t *testing.T) {
	exec := NewInProcessToolExecutor(nil, nil, newDiscardLogger())
	tools := exec.Tools()

	require.Len(t, tools, 2)
	assert.Equal(t, "get_domain", tools[0].Name)
	assert.Equal(t, "get_tld", tools[1].Name)
	assert.NotEmpty(t, tools[0].Description)
	assert.NotNil(t, tools[0].InputSchema)
}

// ---------------------------------------------------------------------------
// Mock services — minimal implementations for tool executor tests
// ---------------------------------------------------------------------------

// mockDomainSvc is a minimal mock for the DomainService interface.
// Only GetDomainByName is used by the tool executor.
type mockDomainSvc struct {
	getByName func(ctx context.Context, name string, preloadHosts bool) (*entities.Domain, error)
}

func (m *mockDomainSvc) GetDomainByName(ctx context.Context, name string, preloadHosts bool) (*entities.Domain, error) {
	return m.getByName(ctx, name, preloadHosts)
}

// Unused interface methods — the tool executor only calls GetDomainByName.
// These are required to satisfy the DomainService interface.
func (m *mockDomainSvc) Create(context.Context, *commands.CreateDomainCommand) (*entities.Domain, error) {
	panic("not used")
}
func (m *mockDomainSvc) DeleteDomainByName(context.Context, string) error { panic("not used") }
func (m *mockDomainSvc) ListDomains(context.Context, queries.ListItemsQuery) ([]*entities.Domain, string, error) {
	panic("not used")
}
func (m *mockDomainSvc) UpdateDomain(context.Context, string, *commands.UpdateDomainCommand) (*entities.Domain, error) {
	panic("not used")
}
func (m *mockDomainSvc) AddHostToDomain(context.Context, string, string, bool) error {
	panic("not used")
}
func (m *mockDomainSvc) AddHostToDomainByHostName(context.Context, string, string, bool) error {
	panic("not used")
}
func (m *mockDomainSvc) RemoveAllDomainHosts(context.Context, string) error { panic("not used") }
func (m *mockDomainSvc) RemoveHostFromDomain(context.Context, string, string) error {
	panic("not used")
}
func (m *mockDomainSvc) RemoveHostFromDomainByHostName(context.Context, string, string) error {
	panic("not used")
}
func (m *mockDomainSvc) DropCatchDomain(context.Context, string, bool) error { panic("not used") }
func (m *mockDomainSvc) Count(context.Context, queries.ListDomainsFilter) (int64, error) {
	panic("not used")
}
func (m *mockDomainSvc) ListExpiringDomains(context.Context, *queries.ExpiringDomainsQuery, int, string) ([]*entities.Domain, error) {
	panic("not used")
}
func (m *mockDomainSvc) CountExpiringDomains(context.Context, *queries.ExpiringDomainsQuery) (int64, error) {
	panic("not used")
}
func (m *mockDomainSvc) ListEventsByDomain(ctx context.Context, domainName string) ([]entities.DomainEvent, error) {
	panic("not used")
}
func (m *mockDomainSvc) ListRecentEvents(ctx context.Context, limit int) ([]entities.DomainEvent, error) {
	panic("not used")
}
func (m *mockDomainSvc) ListPurgeableDomains(context.Context, *queries.PurgeableDomainsQuery, int, string) ([]*entities.Domain, error) {
	panic("not used")
}
func (m *mockDomainSvc) CountPurgeableDomains(context.Context, *queries.PurgeableDomainsQuery) (int64, error) {
	panic("not used")
}
func (m *mockDomainSvc) ListRestoredDomains(context.Context, *queries.RestoredDomainsQuery, int, string) ([]*entities.Domain, error) {
	panic("not used")
}
func (m *mockDomainSvc) CountRestoredDomains(context.Context, *queries.RestoredDomainsQuery) (int64, error) {
	panic("not used")
}
func (m *mockDomainSvc) BulkCreate(context.Context, []*commands.CreateDomainCommand) error {
	panic("not used")
}
func (m *mockDomainSvc) CheckDomainAvailability(context.Context, string, string) (*queries.DomainCheckResult, error) {
	panic("not used")
}
func (m *mockDomainSvc) GetQuote(context.Context, *queries.QuoteRequest) (*entities.Quote, error) {
	panic("not used")
}
func (m *mockDomainSvc) RegisterDomain(context.Context, *commands.RegisterDomainCommand) (*entities.Domain, error) {
	panic("not used")
}
func (m *mockDomainSvc) RenewDomain(context.Context, *commands.RenewDomainCommand, bool) (*entities.Domain, error) {
	panic("not used")
}
func (m *mockDomainSvc) CanAutoRenew(context.Context, string) (bool, error) { panic("not used") }
func (m *mockDomainSvc) AutoRenewDomain(context.Context, string, int) (*entities.Domain, error) {
	panic("not used")
}
func (m *mockDomainSvc) MarkDomainForDeletion(context.Context, string) (*entities.Domain, error) {
	panic("not used")
}
func (m *mockDomainSvc) ExpireDomain(context.Context, string) (*entities.Domain, error) {
	panic("not used")
}
func (m *mockDomainSvc) RestoreDomain(context.Context, string) (*entities.Domain, error) {
	panic("not used")
}
func (m *mockDomainSvc) PurgeDomain(context.Context, string) error         { panic("not used") }
func (m *mockDomainSvc) GetNSRecordsPerTLD(context.Context, queries.ActiveDomainsWithHostsQuery) ([]dns.RR, error) {
	panic("not used")
}
func (m *mockDomainSvc) GetGlueRecordsPerTLD(context.Context, string) ([]dns.RR, error) {
	panic("not used")
}
func (m *mockDomainSvc) SetStatus(context.Context, string, string) (*entities.Domain, error) {
	panic("not used")
}
func (m *mockDomainSvc) UnSetStatus(context.Context, string, string) (*entities.Domain, error) {
	panic("not used")
}

// mockTLDSvc is a minimal mock for the TLDService interface.
type mockTLDSvc struct {
	getByName func(ctx context.Context, name string, preloadAll bool) (*entities.TLD, error)
}

func (m *mockTLDSvc) GetTLDByName(ctx context.Context, name string, preloadAll bool) (*entities.TLD, error) {
	return m.getByName(ctx, name, preloadAll)
}

func (m *mockTLDSvc) CreateTLD(context.Context, *commands.CreateTLDCommand) (*entities.TLD, error) {
	panic("not used")
}
func (m *mockTLDSvc) ListTLDs(context.Context, queries.ListItemsQuery) ([]*entities.TLD, string, error) {
	panic("not used")
}
func (m *mockTLDSvc) DeleteTLDByName(context.Context, string) error { panic("not used") }
func (m *mockTLDSvc) GetTLDHeader(context.Context, string) (*entities.TLDHeader, error) {
	panic("not used")
}
func (m *mockTLDSvc) CountTLDs(context.Context, queries.ListTldsFilter) (int64, error) {
	panic("not used")
}
func (m *mockTLDSvc) SetAllowEscrowImport(context.Context, string, bool) (*entities.TLD, error) {
	panic("not used")
}
