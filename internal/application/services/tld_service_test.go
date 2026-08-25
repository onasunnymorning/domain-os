package services

import (
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"golang.org/x/net/context"
)

type MockDNSRecordRepository struct {
	Header *entities.TLDHeader
}

// GetByZone returns a list of DNSRecords by zone
func (repo *MockDNSRecordRepository) GetByZone(ctx context.Context, zone string) ([]*postgres.TLDDNSRecord, error) {
	return nil, nil
}

// Create creates a DNSRecord
func (repo *MockDNSRecordRepository) Create(ctx context.Context, record *postgres.TLDDNSRecord) (*postgres.TLDDNSRecord, error) {
	return nil, nil
}

// Delete deletes a DNSRecord
func (repo *MockDNSRecordRepository) Delete(ctx context.Context, id int) error {
	return nil
}

// MocktldRepository is a mock implementation of the tldRepository interface
type MocktldRepository struct {
	Tlds []*entities.TLD
}

// Update updates a TLD
func (repo *MocktldRepository) Update(ctx context.Context, tld *entities.TLD) error {
	return nil
}

// CreateTLD creates a TLD
func (repo *MocktldRepository) Create(ctx context.Context, tld *entities.TLD) error {
	repo.Tlds = append(repo.Tlds, tld)
	return nil
}

// GetByName returns a TLD by name
func (repo *MocktldRepository) GetByName(ctx context.Context, name string, preloadAll bool) (*entities.TLD, error) {
	for _, tld := range repo.Tlds {
		if tld.Name.String() == name {
			return tld, nil
		}
	}
	return nil, nil
}

// List returns a list of all TLDs
func (repo *MocktldRepository) List(ctx context.Context, params queries.ListItemsQuery) ([]*entities.TLD, string, error) {
	return repo.Tlds, "", nil
}

// DeleteByName deletes a TLD by name
func (repo *MocktldRepository) DeleteByName(ctx context.Context, name string) error {
	for i, tld := range repo.Tlds {
		if tld.Name.String() == name {
			repo.Tlds = append(repo.Tlds[:i], repo.Tlds[i+1:]...)
			return nil
		}
	}
	return nil
}

// Count returns the number of TLDs
func (repo *MocktldRepository) Count(ctx context.Context, filter queries.ListTldsFilter) (int64, error) {
	return int64(len(repo.Tlds)), nil
}

func TestTLDService_CreateTLD(t *testing.T) {
	tldRepo := MocktldRepository{}
	dnsRecRepo := MockDNSRecordRepository{}
	service := NewTLDService(&tldRepo, &dnsRecRepo)

	tld, err := entities.NewTLD("com", "apex")
	if err != nil {
		t.Error(err)
	}

	cmd := getCreateTLDCommand(tld)

	_, err = service.CreateTLD(context.Background(), cmd)
	if err != nil {
		t.Error(err)
	}

	if len(tldRepo.Tlds) != 1 {
		t.Errorf("Expected 1 tld, got %d", len(tldRepo.Tlds))
	}

}

func getCreateTLDCommand(tld *entities.TLD) *commands.CreateTLDCommand {
	return &commands.CreateTLDCommand{
		Name: tld.Name.String(),
		RyID: tld.RyID.String(),
	}
}

func TestTLDService_GetTLDByName(t *testing.T) {
	tldRepo := MocktldRepository{}
	dnsRecRepo := MockDNSRecordRepository{}
	service := NewTLDService(&tldRepo, &dnsRecRepo)

	// Create 2 TLDs
	tld, err := entities.NewTLD("apex", "apex")
	if err != nil {
		t.Error(err)
	}
	cmd := getCreateTLDCommand(tld)
	_, err = service.CreateTLD(context.Background(), cmd)
	if err != nil {
		t.Error(err)
	}
	tld, err = entities.NewTLD("com.apex", "apex")
	if err != nil {
		t.Error(err)
	}
	cmd = getCreateTLDCommand(tld)
	_, err = service.CreateTLD(context.Background(), cmd)
	if err != nil {
		t.Error(err)
	}

	// Get the first TLD
	tld, err = service.GetTLDByName(context.Background(), "apex", false)
	if err != nil {
		t.Error(err)
	}
	if tld.Name.String() != "apex" {
		t.Errorf("Expected apex, got %s", tld.Name.String())
	}

	// Get the second TLD
	tld, err = service.GetTLDByName(context.Background(), "com.apex", false)
	if err != nil {
		t.Error(err)
	}
	if tld.Name.String() != "com.apex" {
		t.Errorf("Expected com.apex, got %s", tld.Name.String())
	}
}

func TestTLDService_ListTLDs(t *testing.T) {
	tldRepo := MocktldRepository{}
	dnsRecRepo := MockDNSRecordRepository{}
	service := NewTLDService(&tldRepo, &dnsRecRepo)

	// Create 2 TLDs
	tld, err := entities.NewTLD("apex", "apex")
	if err != nil {
		t.Error(err)
	}
	cmd := getCreateTLDCommand(tld)
	_, err = service.CreateTLD(context.Background(), cmd)
	if err != nil {
		t.Error(err)
	}
	tld, err = entities.NewTLD("com.apex", "apex")
	if err != nil {
		t.Error(err)
	}
	cmd = getCreateTLDCommand(tld)
	_, err = service.CreateTLD(context.Background(), cmd)
	if err != nil {
		t.Error(err)
	}

	// List all TLDs
	tlds, _, err := service.ListTLDs(context.Background(), queries.ListItemsQuery{PageSize: 100})
	if err != nil {
		t.Error(err)
	}
	if len(tlds) != 2 {
		t.Errorf("Expected 2 tlds, got %d", len(tlds))
	}
}

func TestTLDService_DeleteTLDByName(t *testing.T) {
	tldRepo := MocktldRepository{}
	dnsRecRepo := MockDNSRecordRepository{}
	service := NewTLDService(&tldRepo, &dnsRecRepo)

	// Create 2 TLDs
	tld, err := entities.NewTLD("apex", "apex")
	if err != nil {
		t.Error(err)
	}
	cmd := getCreateTLDCommand(tld)
	_, err = service.CreateTLD(context.Background(), cmd)
	if err != nil {
		t.Error(err)
	}
	tld, err = entities.NewTLD("com.apex", "apex")
	if err != nil {
		t.Error(err)
	}
	cmd = getCreateTLDCommand(tld)
	_, err = service.CreateTLD(context.Background(), cmd)
	if err != nil {
		t.Error(err)
	}

	// Delete the first TLD
	err = service.DeleteTLDByName(context.Background(), "apex")
	if err != nil {
		t.Error(err)
	}

	// List all TLDs
	tlds, _, err := service.ListTLDs(context.Background(), queries.ListItemsQuery{PageSize: 100})
	if err != nil {
		t.Error(err)
	}
	if len(tlds) != 1 {
		t.Errorf("Expected 1 tld, got %d", len(tlds))
	}

	// Delete the second TLD
	err = service.DeleteTLDByName(context.Background(), "com.apex")
	if err != nil {
		t.Error(err)
	}

	// List all TLDs
	tlds, _, err = service.ListTLDs(context.Background(), queries.ListItemsQuery{PageSize: 100})
	if err != nil {
		t.Error(err)
	}
	if len(tlds) != 0 {
		t.Errorf("Expected 0 tlds, got %d", len(tlds))
	}
}

func TestTLDService_CountTLDs(t *testing.T) {
	tldRepo := MocktldRepository{}
	dnsRecRepo := MockDNSRecordRepository{}
	service := NewTLDService(&tldRepo, &dnsRecRepo)

	// Create 2 TLDs
	tld, err := entities.NewTLD("apex", "apex")
	if err != nil {
		t.Error(err)
	}
	cmd := getCreateTLDCommand(tld)
	_, err = service.CreateTLD(context.Background(), cmd)
	if err != nil {
		t.Error(err)
	}
	tld, err = entities.NewTLD("com.apex", "apex")
	if err != nil {
		t.Error(err)
	}
	cmd = getCreateTLDCommand(tld)
	_, err = service.CreateTLD(context.Background(), cmd)
	if err != nil {
		t.Error(err)
	}

	// Count all TLDs
	count, err := service.CountTLDs(context.Background(), queries.ListTldsFilter{})
	if err != nil {
		t.Error(err)
	}
	if count != 2 {
		t.Errorf("Expected 2 tlds, got %d", count)
	}
}
func TestTLDService_SetAllowEscrowImport(t *testing.T) {
	tldRepo := MocktldRepository{}
	dnsRecRepo := MockDNSRecordRepository{}
	service := NewTLDService(&tldRepo, &dnsRecRepo)

	// Create a TLD
	tld, err := entities.NewTLD("apex", "apex")
	if err != nil {
		t.Error(err)
	}
	cmd := getCreateTLDCommand(tld)
	_, err = service.CreateTLD(context.Background(), cmd)
	if err != nil {
		t.Error(err)
	}

	// Set AllowEscrowImport to true
	updatedTLD, err := service.SetAllowEscrowImport(context.Background(), "apex", true)
	if err != nil {
		t.Error(err)
	}
	if !updatedTLD.AllowEscrowImport {
		t.Errorf("Expected AllowEscrowImport to be true, got %v", updatedTLD.AllowEscrowImport)
	}

	// Set AllowEscrowImport to false
	updatedTLD, err = service.SetAllowEscrowImport(context.Background(), "apex", false)
	if err != nil {
		t.Error(err)
	}
	if updatedTLD.AllowEscrowImport {
		t.Errorf("Expected AllowEscrowImport to be false, got %v", updatedTLD.AllowEscrowImport)
	}
}

// capturingTLDRepository records the preloadAll argument for verification.
type capturingTLDRepository struct {
	MocktldRepository
	capturedPreloadAll bool
}

func (r *capturingTLDRepository) GetByName(ctx context.Context, name string, preloadAll bool) (*entities.TLD, error) {
	r.capturedPreloadAll = preloadAll
	return r.MocktldRepository.GetByName(ctx, name, preloadAll)
}

func TestTLDService_GetTLDByName_ForwardsPreloadAll(t *testing.T) {
	tests := []struct {
		name       string
		preloadAll bool
	}{
		{"preloadAll true is forwarded to repository", true},
		{"preloadAll false is forwarded to repository", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tld, _ := entities.NewTLD("best", "apex")
			repo := &capturingTLDRepository{
				MocktldRepository: MocktldRepository{Tlds: []*entities.TLD{tld}},
			}
			svc := NewTLDService(repo, &MockDNSRecordRepository{})

			_, err := svc.GetTLDByName(context.Background(), "BEST", tt.preloadAll)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if repo.capturedPreloadAll != tt.preloadAll {
				t.Errorf("expected preloadAll=%v to be forwarded, got %v", tt.preloadAll, repo.capturedPreloadAll)
			}
		})
	}
}

// --- Operator Registrar Auto-Provisioning Tests ---

// mockRegistrarRepo is a minimal in-memory mock for RegistrarRepository.
type mockRegistrarRepo struct {
	created []*entities.Registrar
}

func (m *mockRegistrarRepo) Create(ctx context.Context, rar *entities.Registrar) (*entities.Registrar, error) {
	m.created = append(m.created, rar)
	return rar, nil
}
func (m *mockRegistrarRepo) GetByClID(ctx context.Context, clid string, preloadTLDs bool) (*entities.Registrar, error) {
	return nil, nil
}
func (m *mockRegistrarRepo) GetByGurID(ctx context.Context, gurID int) (*entities.Registrar, error) {
	return nil, nil
}
func (m *mockRegistrarRepo) BulkCreate(ctx context.Context, rars []*entities.Registrar) error {
	return nil
}
func (m *mockRegistrarRepo) Update(ctx context.Context, rar *entities.Registrar) (*entities.Registrar, error) {
	return rar, nil
}
func (m *mockRegistrarRepo) Delete(ctx context.Context, clid string) error { return nil }
func (m *mockRegistrarRepo) List(ctx context.Context, params queries.ListItemsQuery) ([]*entities.RegistrarListItem, string, error) {
	return nil, "", nil
}
func (m *mockRegistrarRepo) Count(ctx context.Context) (int64, error) { return 0, nil }
func (m *mockRegistrarRepo) IsRegistrarAccreditedForTLD(ctx context.Context, tldName, rarClID string) (bool, error) {
	return false, nil
}

// mockAccreditationRepo is a minimal mock for AccreditationRepository.
type mockAccreditationRepo struct {
	accreditations []string // "tld:clid" pairs
}

func (m *mockAccreditationRepo) CreateAccreditation(ctx context.Context, tldName, rarClID string) error {
	m.accreditations = append(m.accreditations, tldName+":"+rarClID)
	return nil
}
func (m *mockAccreditationRepo) DeleteAccreditation(ctx context.Context, tldName, rarClID string) error {
	return nil
}
func (m *mockAccreditationRepo) ListTLDRegistrars(ctx context.Context, pageSize int, cursor string, tldName string) ([]*entities.Registrar, error) {
	return nil, nil
}
func (m *mockAccreditationRepo) ListRegistrarTLDs(ctx context.Context, pageSize int, cursor string, rarClID string) ([]*entities.TLD, error) {
	return nil, nil
}

// mockRyRepo is a minimal mock for RegistryOperatorRepository.
type mockRyRepo struct {
	ry *entities.RegistryOperator
}

func (m *mockRyRepo) GetByRyID(ctx context.Context, ryID string) (*entities.RegistryOperator, error) {
	if m.ry != nil {
		return m.ry, nil
	}
	return nil, entities.ErrRegistryOperatorNotFound
}
func (m *mockRyRepo) Create(ctx context.Context, ro *entities.RegistryOperator) (*entities.RegistryOperator, error) {
	return ro, nil
}
func (m *mockRyRepo) Update(ctx context.Context, ro *entities.RegistryOperator) (*entities.RegistryOperator, error) {
	return ro, nil
}
func (m *mockRyRepo) DeleteByRyID(ctx context.Context, ryID string) error { return nil }
func (m *mockRyRepo) List(ctx context.Context, params queries.ListItemsQuery) ([]*entities.RegistryOperator, string, error) {
	return nil, "", nil
}
func (m *mockRyRepo) Count(ctx context.Context, filter queries.ListRegistryOperatorsFilter) (int64, error) {
	return 0, nil
}

// mockEventPublisher is a no-op event publisher for testing.
type mockEventPublisher struct{}

func (m *mockEventPublisher) Publish(ctx context.Context, events ...entities.DomainEvent) error {
	return nil
}

func TestTLDService_CreateTLD_WithOperatorRegistrars(t *testing.T) {
	tldRepo := &MocktldRepository{}
	rarRepo := &mockRegistrarRepo{}
	accRepo := &mockAccreditationRepo{}
	ry, _ := entities.NewRegistryOperator("apex", "Apex Registry", "ops@apex.tld")
	ryRepo := &mockRyRepo{ry: ry}
	eventPub := &mockEventPublisher{}

	svc := NewTLDService(tldRepo, &MockDNSRecordRepository{},
		WithOperatorRegistrarDeps(rarRepo, accRepo, ryRepo, eventPub),
	)

	cmd := &commands.CreateTLDCommand{
		Name:                     "best",
		RyID:                     "apex",
		CreateOperatorRegistrars: true,
	}

	_, err := svc.CreateTLD(context.Background(), cmd)
	if err != nil {
		t.Fatalf("unexpected error creating TLD: %v", err)
	}

	// Should have created 2 registrars (9998-best, 9999-best)
	if len(rarRepo.created) != 2 {
		t.Fatalf("expected 2 operator registrars created, got %d", len(rarRepo.created))
	}

	// Verify ClIDs
	clids := map[string]bool{}
	for _, rar := range rarRepo.created {
		clids[rar.ClID.String()] = true
	}
	if !clids["9998-best"] {
		t.Error("expected registrar with ClID '9998-best' to be created")
	}
	if !clids["9999-best"] {
		t.Error("expected registrar with ClID '9999-best' to be created")
	}

	// Verify names include registry operator name and billing distinction
	for _, rar := range rarRepo.created {
		name := rar.Name
		if rar.GurID == 9998 {
			if name != "best - Reserved - Billable" {
				t.Errorf("expected 9998 name 'best - Reserved - Billable', got %q", name)
			}
		}
		if rar.GurID == 9999 {
			if name != "best - Reserved - Non-Billable" {
				t.Errorf("expected 9999 name 'best - Reserved - Non-Billable', got %q", name)
			}
		}
	}

	// Verify accreditations
	if len(accRepo.accreditations) != 2 {
		t.Fatalf("expected 2 accreditations, got %d", len(accRepo.accreditations))
	}
}

func TestTLDService_CreateTLD_WithoutOperatorRegistrars(t *testing.T) {
	tldRepo := &MocktldRepository{}
	rarRepo := &mockRegistrarRepo{}
	accRepo := &mockAccreditationRepo{}

	svc := NewTLDService(tldRepo, &MockDNSRecordRepository{},
		WithOperatorRegistrarDeps(rarRepo, accRepo, &mockRyRepo{}, &mockEventPublisher{}),
	)

	cmd := &commands.CreateTLDCommand{
		Name:                     "noops",
		RyID:                     "apex",
		CreateOperatorRegistrars: false,
	}

	_, err := svc.CreateTLD(context.Background(), cmd)
	if err != nil {
		t.Fatalf("unexpected error creating TLD: %v", err)
	}

	// Should NOT have created any registrars
	if len(rarRepo.created) != 0 {
		t.Fatalf("expected 0 operator registrars when opted out, got %d", len(rarRepo.created))
	}
	if len(accRepo.accreditations) != 0 {
		t.Fatalf("expected 0 accreditations when opted out, got %d", len(accRepo.accreditations))
	}
}

func TestTLDService_CreateTLD_NoDepsSkipsProvisioning(t *testing.T) {
	tldRepo := &MocktldRepository{}

	// No operator registrar deps provided
	svc := NewTLDService(tldRepo, &MockDNSRecordRepository{})

	cmd := &commands.CreateTLDCommand{
		Name:                     "nodeps",
		RyID:                     "apex",
		CreateOperatorRegistrars: true, // requested but deps missing
	}

	_, err := svc.CreateTLD(context.Background(), cmd)
	if err != nil {
		t.Fatalf("unexpected error creating TLD without deps: %v", err)
	}
	// No panic, no error — silently skips
}

