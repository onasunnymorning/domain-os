package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/miekg/dns"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Domain service mock
// ---------------------------------------------------------------------------

type mockDomainService struct {
	getDomainByNameFn func(ctx context.Context, name string, preloadHosts bool) (*entities.Domain, error)
}

func (m *mockDomainService) GetDomainByName(ctx context.Context, name string, preloadHosts bool) (*entities.Domain, error) {
	return m.getDomainByNameFn(ctx, name, preloadHosts)
}

// Unused interface methods — panic if called.
func (m *mockDomainService) Create(context.Context, *commands.CreateDomainCommand) (*entities.Domain, error) {
	panic("not implemented")
}
func (m *mockDomainService) DeleteDomainByName(context.Context, string) error {
	panic("not implemented")
}
func (m *mockDomainService) ListDomains(context.Context, queries.ListItemsQuery) ([]*entities.Domain, string, error) {
	panic("not implemented")
}
func (m *mockDomainService) UpdateDomain(context.Context, string, *commands.UpdateDomainCommand) (*entities.Domain, error) {
	panic("not implemented")
}
func (m *mockDomainService) AddHostToDomain(context.Context, string, string, bool) error {
	panic("not implemented")
}
func (m *mockDomainService) AddHostToDomainByHostName(context.Context, string, string, bool) error {
	panic("not implemented")
}
func (m *mockDomainService) RemoveAllDomainHosts(context.Context, string) error {
	panic("not implemented")
}
func (m *mockDomainService) RemoveHostFromDomain(context.Context, string, string) error {
	panic("not implemented")
}
func (m *mockDomainService) RemoveHostFromDomainByHostName(context.Context, string, string) error {
	panic("not implemented")
}
func (m *mockDomainService) DropCatchDomain(context.Context, string, bool) error {
	panic("not implemented")
}
func (m *mockDomainService) Count(context.Context, queries.ListDomainsFilter) (int64, error) {
	panic("not implemented")
}
func (m *mockDomainService) ListExpiringDomains(context.Context, *queries.ExpiringDomainsQuery, int, string) ([]*entities.Domain, error) {
	panic("not implemented")
}
func (m *mockDomainService) CountExpiringDomains(context.Context, *queries.ExpiringDomainsQuery) (int64, error) {
	panic("not implemented")
}
func (m *mockDomainService) ListEventsByDomain(ctx context.Context, domainName string) ([]entities.DomainEvent, error) {
	panic("not implemented")
}
func (m *mockDomainService) ListRecentEvents(ctx context.Context, limit int) ([]entities.DomainEvent, error) {
	panic("not implemented")
}
func (m *mockDomainService) ListPurgeableDomains(context.Context, *queries.PurgeableDomainsQuery, int, string) ([]*entities.Domain, error) {
	panic("not implemented")
}
func (m *mockDomainService) CountPurgeableDomains(context.Context, *queries.PurgeableDomainsQuery) (int64, error) {
	panic("not implemented")
}
func (m *mockDomainService) ListRestoredDomains(context.Context, *queries.RestoredDomainsQuery, int, string) ([]*entities.Domain, error) {
	panic("not implemented")
}
func (m *mockDomainService) CountRestoredDomains(context.Context, *queries.RestoredDomainsQuery) (int64, error) {
	panic("not implemented")
}
func (m *mockDomainService) BulkCreate(context.Context, []*commands.CreateDomainCommand) error {
	panic("not implemented")
}
func (m *mockDomainService) CheckDomainAvailability(context.Context, string, string) (*queries.DomainCheckResult, error) {
	panic("not implemented")
}
func (m *mockDomainService) GetQuote(context.Context, *queries.QuoteRequest) (*entities.Quote, error) {
	panic("not implemented")
}
func (m *mockDomainService) RegisterDomain(context.Context, *commands.RegisterDomainCommand) (*entities.Domain, error) {
	panic("not implemented")
}
func (m *mockDomainService) RenewDomain(context.Context, *commands.RenewDomainCommand, bool) (*entities.Domain, error) {
	panic("not implemented")
}
func (m *mockDomainService) CanAutoRenew(context.Context, string) (bool, error) {
	panic("not implemented")
}
func (m *mockDomainService) AutoRenewDomain(context.Context, string, int) (*entities.Domain, error) {
	panic("not implemented")
}
func (m *mockDomainService) MarkDomainForDeletion(context.Context, string) (*entities.Domain, error) {
	panic("not implemented")
}
func (m *mockDomainService) ExpireDomain(context.Context, string) (*entities.Domain, error) {
	panic("not implemented")
}
func (m *mockDomainService) RestoreDomain(context.Context, string) (*entities.Domain, error) {
	panic("not implemented")
}
func (m *mockDomainService) PurgeDomain(context.Context, string) error {
	panic("not implemented")
}
func (m *mockDomainService) GetNSRecordsPerTLD(context.Context, queries.ActiveDomainsWithHostsQuery) ([]dns.RR, error) {
	panic("not implemented")
}
func (m *mockDomainService) GetGlueRecordsPerTLD(context.Context, string) ([]dns.RR, error) {
	panic("not implemented")
}
func (m *mockDomainService) SetStatus(context.Context, string, string) (*entities.Domain, error) {
	panic("not implemented")
}
func (m *mockDomainService) UnSetStatus(context.Context, string, string) (*entities.Domain, error) {
	panic("not implemented")
}

// ---------------------------------------------------------------------------
// TLD service mock
// ---------------------------------------------------------------------------

type mockTLDService struct {
	getTLDByNameFn func(ctx context.Context, name string, preloadAll bool) (*entities.TLD, error)
}

func (m *mockTLDService) GetTLDByName(ctx context.Context, name string, preloadAll bool) (*entities.TLD, error) {
	return m.getTLDByNameFn(ctx, name, preloadAll)
}
func (m *mockTLDService) CreateTLD(context.Context, *commands.CreateTLDCommand) (*entities.TLD, error) {
	panic("not implemented")
}
func (m *mockTLDService) ListTLDs(context.Context, queries.ListItemsQuery) ([]*entities.TLD, string, error) {
	panic("not implemented")
}
func (m *mockTLDService) DeleteTLDByName(context.Context, string) error {
	panic("not implemented")
}
func (m *mockTLDService) GetTLDHeader(context.Context, string) (*entities.TLDHeader, error) {
	panic("not implemented")
}
func (m *mockTLDService) CountTLDs(context.Context, queries.ListTldsFilter) (int64, error) {
	panic("not implemented")
}
func (m *mockTLDService) SetAllowEscrowImport(context.Context, string, bool) (*entities.TLD, error) {
	panic("not implemented")
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestServer(domainFn func(ctx context.Context, name string, preloadHosts bool) (*entities.Domain, error)) *Server {
	return &Server{
		domainService: &mockDomainService{getDomainByNameFn: domainFn},
		tldService: &mockTLDService{getTLDByNameFn: func(ctx context.Context, name string, preloadAll bool) (*entities.TLD, error) {
			return nil, entities.ErrTLDNotFound
		}},
	}
}

func newTestServerWithTLD(tldFn func(ctx context.Context, name string, preloadAll bool) (*entities.TLD, error)) *Server {
	return &Server{
		domainService: &mockDomainService{getDomainByNameFn: func(ctx context.Context, name string, preloadHosts bool) (*entities.Domain, error) {
			return nil, entities.ErrDomainNotFound
		}},
		tldService: &mockTLDService{getTLDByNameFn: tldFn},
	}
}

// ---------------------------------------------------------------------------
// get_domain tests
// ---------------------------------------------------------------------------

func TestGetDomain_Found(t *testing.T) {
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

	srv := newTestServer(func(ctx context.Context, name string, preloadHosts bool) (*entities.Domain, error) {
		assert.Equal(t, "example.best", name)
		assert.True(t, preloadHosts)
		return testDomain, nil
	})

	result, output, err := srv.GetDomain(context.Background(), &sdkmcp.CallToolRequest{}, GetDomainInput{Name: "Example.BEST"})

	require.NoError(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "example.best", output.Name)
	assert.Equal(t, []string{"ok"}, output.Statuses)
	assert.Equal(t, created.Format(time.RFC3339), output.CreatedDate)
	assert.Equal(t, expiry.Format(time.RFC3339), output.ExpiryDate)
	assert.Empty(t, output.RGPPhase)
	assert.Empty(t, output.RGPPhaseEndDate)
	assert.Equal(t, []string{"ns1.example.best", "ns2.example.best"}, output.Nameservers)
	assert.Equal(t, "registrar1", output.SponsoringRegistrar)
}

func TestGetDomain_NotFound(t *testing.T) {
	srv := newTestServer(func(ctx context.Context, name string, preloadHosts bool) (*entities.Domain, error) {
		return nil, entities.ErrDomainNotFound
	})

	result, output, err := srv.GetDomain(context.Background(), &sdkmcp.CallToolRequest{}, GetDomainInput{Name: "nonexistent.best"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Nil(t, result)
	assert.Equal(t, GetDomainOutput{}, output)
}

func TestGetDomain_InvalidName_Empty(t *testing.T) {
	srv := newTestServer(func(ctx context.Context, name string, preloadHosts bool) (*entities.Domain, error) {
		t.Fatal("should not be called for invalid input")
		return nil, nil
	})

	_, _, err := srv.GetDomain(context.Background(), &sdkmcp.CallToolRequest{}, GetDomainInput{Name: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid domain name")
}

func TestGetDomain_InvalidName_NoDot(t *testing.T) {
	srv := newTestServer(func(ctx context.Context, name string, preloadHosts bool) (*entities.Domain, error) {
		t.Fatal("should not be called for invalid input")
		return nil, nil
	})

	_, _, err := srv.GetDomain(context.Background(), &sdkmcp.CallToolRequest{}, GetDomainInput{Name: "nodot"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid domain name")
}

func TestGetDomain_RGPPhase_RedemptionPeriod(t *testing.T) {
	now := time.Now().UTC()
	redemptionEnd := now.Add(24 * time.Hour)

	testDomain := &entities.Domain{
		Name:       entities.DomainName("expired.best"),
		ClID:       entities.ClIDType("registrar1"),
		CreatedAt:  now.Add(-730 * 24 * time.Hour),
		ExpiryDate: now.Add(-30 * 24 * time.Hour),
		Status:     entities.DomainStatus{PendingDelete: true},
		RGPStatus: entities.DomainRGPStatus{
			RedemptionPeriodEnd: redemptionEnd,
			PurgeDate:           now.Add(5 * 24 * time.Hour),
		},
	}

	srv := newTestServer(func(ctx context.Context, name string, preloadHosts bool) (*entities.Domain, error) {
		return testDomain, nil
	})

	_, output, err := srv.GetDomain(context.Background(), &sdkmcp.CallToolRequest{}, GetDomainInput{Name: "expired.best"})

	require.NoError(t, err)
	assert.Equal(t, "redemptionPeriod", output.RGPPhase)
	assert.Equal(t, redemptionEnd.Format(time.RFC3339), output.RGPPhaseEndDate)
}

func TestGetDomain_RGPPhase_PendingRestore(t *testing.T) {
	now := time.Now().UTC()

	testDomain := &entities.Domain{
		Name:       entities.DomainName("restoring.best"),
		ClID:       entities.ClIDType("registrar1"),
		CreatedAt:  now.Add(-730 * 24 * time.Hour),
		ExpiryDate: now.Add(-30 * 24 * time.Hour),
		Status:     entities.DomainStatus{PendingRestore: true},
		RGPStatus:  entities.DomainRGPStatus{RedemptionPeriodEnd: now.Add(-1 * time.Hour)},
	}

	srv := newTestServer(func(ctx context.Context, name string, preloadHosts bool) (*entities.Domain, error) {
		return testDomain, nil
	})

	_, output, err := srv.GetDomain(context.Background(), &sdkmcp.CallToolRequest{}, GetDomainInput{Name: "restoring.best"})

	require.NoError(t, err)
	assert.Equal(t, "pendingRestore", output.RGPPhase)
	assert.Empty(t, output.RGPPhaseEndDate, "pendingRestore should have no end date")
}

func TestGetDomain_NoHosts_ReturnsEmptySlice(t *testing.T) {
	now := time.Now().UTC()

	testDomain := &entities.Domain{
		Name:       entities.DomainName("nohosts.best"),
		ClID:       entities.ClIDType("registrar1"),
		CreatedAt:  now.Add(-365 * 24 * time.Hour),
		ExpiryDate: now.Add(365 * 24 * time.Hour),
		Status:     entities.DomainStatus{OK: true, Inactive: true},
		Hosts:      nil,
	}

	srv := newTestServer(func(ctx context.Context, name string, preloadHosts bool) (*entities.Domain, error) {
		return testDomain, nil
	})

	_, output, err := srv.GetDomain(context.Background(), &sdkmcp.CallToolRequest{}, GetDomainInput{Name: "nohosts.best"})

	require.NoError(t, err)
	assert.NotNil(t, output.Nameservers, "nameservers should be empty slice, not nil")
	assert.Empty(t, output.Nameservers)
}

func TestServer_Run_RegistersToolsWithoutPanic(t *testing.T) {
	srv := newTestServer(func(ctx context.Context, name string, preloadHosts bool) (*entities.Domain, error) {
		return nil, entities.ErrDomainNotFound
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	assert.NotPanics(t, func() {
		_ = srv.Run(ctx)
	})
}

// ---------------------------------------------------------------------------
// get_tld tests
// ---------------------------------------------------------------------------

func TestGetTLD_Found(t *testing.T) {
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
			// Historical phase — should NOT appear in output
			{
				Name:   entities.ClIDType("launch-1"),
				Type:   entities.PhaseTypeLaunch,
				Starts: now.Add(-730 * 24 * time.Hour),
				Ends:   timePtr(now.Add(-365 * 24 * time.Hour)),
			},
		},
	}

	srv := newTestServerWithTLD(func(ctx context.Context, name string, preloadAll bool) (*entities.TLD, error) {
		assert.Equal(t, "best", name)
		assert.True(t, preloadAll)
		return testTLD, nil
	})

	result, output, err := srv.GetTLD(context.Background(), &sdkmcp.CallToolRequest{}, GetTLDInput{Name: "BEST"})

	require.NoError(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "best", output.Name)
	assert.Empty(t, output.UnicodeName)
	assert.Equal(t, "generic", output.Type)
	assert.Equal(t, "apex", output.RegistryOperatorID)
	assert.True(t, output.DNSEnabled)

	// Only the active GA phase should appear, not the historical launch phase
	require.Len(t, output.CurrentPhases, 1)
	phase := output.CurrentPhases[0]
	assert.Equal(t, "ga-1", phase.Name)
	assert.Equal(t, "GA", phase.Type)
	assert.Equal(t, phaseEnd.Format(time.RFC3339), phase.Ends)

	// Verify pricing
	require.Len(t, phase.Prices, 1)
	assert.Equal(t, "USD", phase.Prices[0].Currency)
	assert.Equal(t, uint64(1000), phase.Prices[0].RegistrationAmount)
	assert.Equal(t, uint64(5000), phase.Prices[0].RestoreAmount)

	// Verify policy
	assert.Equal(t, 3, phase.Policy.MinLabelLength)
	assert.Equal(t, 63, phase.Policy.MaxLabelLength)
	assert.Equal(t, 30, phase.Policy.RedemptionGP)
	assert.Equal(t, "USD", phase.Policy.BaseCurrency)
	require.NotNil(t, phase.Policy.AllowAutoRenew)
	assert.True(t, *phase.Policy.AllowAutoRenew)
}

func TestGetTLD_NotFound(t *testing.T) {
	srv := newTestServerWithTLD(func(ctx context.Context, name string, preloadAll bool) (*entities.TLD, error) {
		return nil, entities.ErrTLDNotFound
	})

	result, output, err := srv.GetTLD(context.Background(), &sdkmcp.CallToolRequest{}, GetTLDInput{Name: "nonexistent"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Nil(t, result)
	assert.Equal(t, GetTLDOutput{}, output)
}

func TestGetTLD_InvalidName_Empty(t *testing.T) {
	srv := newTestServerWithTLD(func(ctx context.Context, name string, preloadAll bool) (*entities.TLD, error) {
		t.Fatal("should not be called for invalid input")
		return nil, nil
	})

	_, _, err := srv.GetTLD(context.Background(), &sdkmcp.CallToolRequest{}, GetTLDInput{Name: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid TLD name")
}

func TestGetTLD_NoPhases(t *testing.T) {
	now := time.Now().UTC()

	testTLD := &entities.TLD{
		Name:      entities.DomainName("new"),
		Type:      entities.TLDTypeGTLD,
		RyID:      entities.ClIDType("apex"),
		CreatedAt: now,
		UpdatedAt: now,
		Phases:    nil,
	}

	srv := newTestServerWithTLD(func(ctx context.Context, name string, preloadAll bool) (*entities.TLD, error) {
		return testTLD, nil
	})

	_, output, err := srv.GetTLD(context.Background(), &sdkmcp.CallToolRequest{}, GetTLDInput{Name: "new"})

	require.NoError(t, err)
	assert.NotNil(t, output.CurrentPhases)
	assert.Empty(t, output.CurrentPhases)
}

func TestGetTLD_LeadingDotStripped(t *testing.T) {
	srv := newTestServerWithTLD(func(ctx context.Context, name string, preloadAll bool) (*entities.TLD, error) {
		assert.Equal(t, "best", name, "leading dot should be stripped")
		return nil, entities.ErrTLDNotFound
	})

	_, _, _ = srv.GetTLD(context.Background(), &sdkmcp.CallToolRequest{}, GetTLDInput{Name: ".best"})
}

func timePtr(t time.Time) *time.Time {
	return &t
}
