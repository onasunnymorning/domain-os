package services

import (
	"context"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

type MockRegistrarRepository struct {
	Registrars   map[string]*entities.Registrar
	UpdateCalled bool
	// ConflictClIDs simulates rows the database skips on a unique constraint.
	ConflictClIDs map[string]bool
}

func (m *MockRegistrarRepository) GetByClID(ctx context.Context, clid string, preloadTLDs bool) (*entities.Registrar, error) {
	if rar, ok := m.Registrars[clid]; ok {
		return rar, nil
	}
	return nil, entities.ErrRegistrarNotFound
}

func (m *MockRegistrarRepository) GetByGurID(ctx context.Context, gurID int) (*entities.Registrar, error) {
	for _, rar := range m.Registrars {
		if rar.GurID == gurID {
			return rar, nil
		}
	}
	return nil, entities.ErrRegistrarNotFound
}

func (m *MockRegistrarRepository) List(ctx context.Context, params queries.ListItemsQuery) ([]*entities.RegistrarListItem, string, error) {
	return nil, "", nil
}

func (m *MockRegistrarRepository) Count(ctx context.Context) (int64, error) {
	return int64(len(m.Registrars)), nil
}

func (m *MockRegistrarRepository) Create(ctx context.Context, rar *entities.Registrar) (*entities.Registrar, error) {
	m.Registrars[rar.ClID.String()] = rar
	return rar, nil
}

func (m *MockRegistrarRepository) BulkCreate(ctx context.Context, rars []*entities.Registrar) ([]string, error) {
	inserted := []string{}
	for _, rar := range rars {
		if m.ConflictClIDs[rar.ClID.String()] {
			continue
		}
		m.Registrars[rar.ClID.String()] = rar
		inserted = append(inserted, rar.ClID.String())
	}
	return inserted, nil
}

func (m *MockRegistrarRepository) Update(ctx context.Context, rar *entities.Registrar) (*entities.Registrar, error) {
	m.UpdateCalled = true
	m.Registrars[rar.ClID.String()] = rar
	return rar, nil
}

func (m *MockRegistrarRepository) Delete(ctx context.Context, clid string) error {
	delete(m.Registrars, clid)
	return nil
}

func (m *MockRegistrarRepository) IsRegistrarAccreditedForTLD(ctx context.Context, tldName string, rarClID string) (bool, error) {
	return false, nil
}

// TestRegistrarService_Update_PreservesCreatedAt verifies that we don't accidentally
// null out the CreatedAt date when the frontend sends an update without it.
func TestRegistrarService_Update_PreservesCreatedAt(t *testing.T) {
	ctx := context.Background()

	// 1. Create a dummy registrar with a specific created at date in the mock repository
	originalCreatedAt := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	clid := "test-registrar"
	rar := &entities.Registrar{
		ClID:      entities.ClIDType(clid),
		Name:      "Test Registrar",
		CreatedAt: originalCreatedAt,
	}

	repo := &MockRegistrarRepository{
		Registrars: map[string]*entities.Registrar{
			clid: rar,
		},
	}

	service := NewRegistrarService(repo, &noopPublisher{})

	// 2. Simulate an incoming update payload where CreatedAt is zero
	incomingUpdate := &entities.Registrar{
		ClID:      entities.ClIDType(clid),
		Name:      "Updated Registrar Name",
		CreatedAt: time.Time{}, // zero value, representing missing or nulled field from JSON
	}

	// 3. Call Update
	updatedRar, err := service.Update(ctx, incomingUpdate)
	if err != nil {
		t.Fatalf("unexpected error during update: %v", err)
	}

	// 4. Verify that the previous CreatedAt was preserved, instead of being overwritten with zero
	if updatedRar.CreatedAt.IsZero() {
		t.Errorf("expected CreatedAt to be preserved, but it was set to zero value")
	}

	if !updatedRar.CreatedAt.Equal(originalCreatedAt) {
		t.Errorf("expected CreatedAt to equal %v, got %v", originalCreatedAt, updatedRar.CreatedAt)
	}

	if !repo.UpdateCalled {
		t.Errorf("expected Update to be called on repository")
	}
}

// countingPublisher records how many events were published so tests can assert
// that no-op operations stay silent.
type countingPublisher struct {
	published int
}

func (c *countingPublisher) Publish(ctx context.Context, events ...entities.DomainEvent) error {
	c.published += len(events)
	return nil
}

// capturingPublisher stores published events so tests can inspect their payload.
type capturingPublisher struct {
	events []entities.DomainEvent
}

func (c *capturingPublisher) Publish(ctx context.Context, events ...entities.DomainEvent) error {
	c.events = append(c.events, events...)
	return nil
}

// TestRegistrarService_BulkCreate_RecordsClientIDs verifies that the
// registrar.bulk_created event payload lists every created registrar's ClID,
// so operators can see which registrars a batch touched from the event alone.
func TestRegistrarService_BulkCreate_RecordsClientIDs(t *testing.T) {
	ctx := context.Background()

	pi, err := entities.NewRegistrarPostalInfo(entities.PostalInfoEnumTypeINT, mustAddress(t))
	if err != nil {
		t.Fatalf("unexpected error building postal info: %v", err)
	}

	cmds := []*commands.CreateRegistrarCommand{
		{ClID: "9995-pdt-1", Name: "PDT 1", Email: "a@b.co", GurID: 9995, PostalInfo: [2]*entities.RegistrarPostalInfo{pi}},
		{ClID: "9996-pdt-2", Name: "PDT 2", Email: "a@b.co", GurID: 9996, PostalInfo: [2]*entities.RegistrarPostalInfo{pi}},
		{ClID: "9997-sla-monitor", Name: "SLA", Email: "a@b.co", GurID: 9997, PostalInfo: [2]*entities.RegistrarPostalInfo{pi}},
	}

	repo := &MockRegistrarRepository{Registrars: map[string]*entities.Registrar{}}
	pub := &capturingPublisher{}
	service := NewRegistrarService(repo, pub)

	created, err := service.BulkCreate(ctx, cmds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(created) != len(cmds) {
		t.Fatalf("expected %d created ClIDs, got %d (%v)", len(cmds), len(created), created)
	}

	if len(pub.events) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(pub.events))
	}

	payload, ok := pub.events[0].Data.(*entities.RegistrarLifecycleEvent)
	if !ok {
		t.Fatalf("expected payload of type *RegistrarLifecycleEvent, got %T", pub.events[0].Data)
	}

	want := []string{"9995-pdt-1", "9996-pdt-2", "9997-sla-monitor"}
	if len(payload.ClientIDs) != len(want) {
		t.Fatalf("expected %d ClientIDs, got %d (%v)", len(want), len(payload.ClientIDs), payload.ClientIDs)
	}
	for i, clid := range want {
		if payload.ClientIDs[i] != clid {
			t.Errorf("ClientIDs[%d]: got %q, want %q", i, payload.ClientIDs[i], clid)
		}
	}
}

// bulkCreateFixture builds three valid registrar create commands.
func bulkCreateFixture(t *testing.T) []*commands.CreateRegistrarCommand {
	t.Helper()
	pi, err := entities.NewRegistrarPostalInfo(entities.PostalInfoEnumTypeINT, mustAddress(t))
	if err != nil {
		t.Fatalf("unexpected error building postal info: %v", err)
	}
	return []*commands.CreateRegistrarCommand{
		{ClID: "9995-pdt-1", Name: "PDT 1", Email: "a@b.co", GurID: 9995, PostalInfo: [2]*entities.RegistrarPostalInfo{pi}},
		{ClID: "9996-pdt-2", Name: "PDT 2", Email: "a@b.co", GurID: 9996, PostalInfo: [2]*entities.RegistrarPostalInfo{pi}},
		{ClID: "9997-sla-monitor", Name: "SLA", Email: "a@b.co", GurID: 9997, PostalInfo: [2]*entities.RegistrarPostalInfo{pi}},
	}
}

// TestRegistrarService_BulkCreate_NothingInsertedEmitsNoEvent guards the daily
// sync noise: when every row is skipped on a unique constraint, nothing was
// created, so no registrar.bulk_created event may be published.
func TestRegistrarService_BulkCreate_NothingInsertedEmitsNoEvent(t *testing.T) {
	cmds := bulkCreateFixture(t)
	conflicts := map[string]bool{}
	for _, c := range cmds {
		conflicts[c.ClID] = true
	}
	repo := &MockRegistrarRepository{Registrars: map[string]*entities.Registrar{}, ConflictClIDs: conflicts}
	pub := &capturingPublisher{}
	service := NewRegistrarService(repo, pub)

	created, err := service.BulkCreate(context.Background(), cmds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(created) != 0 {
		t.Fatalf("expected no created ClIDs, got %v", created)
	}
	if len(pub.events) != 0 {
		t.Fatalf("expected no events when nothing was inserted, got %d", len(pub.events))
	}
}

// TestRegistrarService_BulkCreate_PartialInsertReportsOnlyInserted verifies the
// event and the returned ClIDs describe the rows that landed, not the request.
func TestRegistrarService_BulkCreate_PartialInsertReportsOnlyInserted(t *testing.T) {
	cmds := bulkCreateFixture(t)
	repo := &MockRegistrarRepository{
		Registrars:    map[string]*entities.Registrar{},
		ConflictClIDs: map[string]bool{"9996-pdt-2": true},
	}
	pub := &capturingPublisher{}
	service := NewRegistrarService(repo, pub)

	created, err := service.BulkCreate(context.Background(), cmds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"9995-pdt-1", "9997-sla-monitor"}
	if len(created) != len(want) || created[0] != want[0] || created[1] != want[1] {
		t.Fatalf("created: got %v, want %v", created, want)
	}

	if len(pub.events) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(pub.events))
	}
	if got := pub.events[0].Description; got != "bulk created 2 registrars" {
		t.Errorf("description: got %q, want %q", got, "bulk created 2 registrars")
	}
	payload, ok := pub.events[0].Data.(*entities.RegistrarLifecycleEvent)
	if !ok {
		t.Fatalf("expected payload of type *RegistrarLifecycleEvent, got %T", pub.events[0].Data)
	}
	if len(payload.ClientIDs) != 2 || payload.ClientIDs[0] != want[0] || payload.ClientIDs[1] != want[1] {
		t.Errorf("ClientIDs: got %v, want %v", payload.ClientIDs, want)
	}
}

// mustAddress builds a minimal valid address for test fixtures.
func mustAddress(t *testing.T) *entities.Address {
	t.Helper()
	a, err := entities.NewAddress("Test City", "US")
	if err != nil {
		t.Fatalf("unexpected error building address: %v", err)
	}
	return a
}

// TestRegistrarService_SetStatus_NoOpIsSilent verifies that setting a registrar's
// status to the value it already holds does not touch the repository or emit a
// lifecycle event. This is what keeps idempotent sync runs (e.g. the special
// reserved registrars re-forced to "ok") from spamming the event feed.
func TestRegistrarService_SetStatus_NoOpIsSilent(t *testing.T) {
	ctx := context.Background()
	clid := "9995-pdt-1"

	rar := &entities.Registrar{
		ClID:   entities.ClIDType(clid),
		Name:   "Pre-Delegation Testing",
		Status: entities.RegistrarStatusOK,
	}

	repo := &MockRegistrarRepository{
		Registrars: map[string]*entities.Registrar{clid: rar},
	}
	pub := &countingPublisher{}
	service := NewRegistrarService(repo, pub)

	// Set the status to the value it already has (case-insensitively).
	if err := service.SetStatus(ctx, clid, "OK"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.UpdateCalled {
		t.Errorf("expected no repository Update for a no-op status change")
	}
	if pub.published != 0 {
		t.Errorf("expected no events published for a no-op status change, got %d", pub.published)
	}
}

// TestRegistrarService_SetStatus_RealChangeIsApplied verifies that a genuine
// status change still writes and publishes exactly one event.
func TestRegistrarService_SetStatus_RealChangeIsApplied(t *testing.T) {
	ctx := context.Background()
	clid := "test-registrar"

	rar := &entities.Registrar{
		ClID:   entities.ClIDType(clid),
		Name:   "Test Registrar",
		Status: entities.RegistrarStatusOK,
	}

	repo := &MockRegistrarRepository{
		Registrars: map[string]*entities.Registrar{clid: rar},
	}
	pub := &countingPublisher{}
	service := NewRegistrarService(repo, pub)

	if err := service.SetStatus(ctx, clid, entities.RegistrarStatusReadonly); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !repo.UpdateCalled {
		t.Errorf("expected repository Update to be called for a real status change")
	}
	if pub.published != 1 {
		t.Errorf("expected exactly 1 event published for a real status change, got %d", pub.published)
	}
	if repo.Registrars[clid].Status != entities.RegistrarStatusReadonly {
		t.Errorf("expected status readonly, got %q", repo.Registrars[clid].Status)
	}
}
