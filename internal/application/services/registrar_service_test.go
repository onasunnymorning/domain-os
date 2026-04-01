package services

import (
	"context"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

type MockRegistrarRepository struct {
	Registrars   map[string]*entities.Registrar
	UpdateCalled bool
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

func (m *MockRegistrarRepository) BulkCreate(ctx context.Context, rars []*entities.Registrar) error {
	for _, rar := range rars {
		m.Registrars[rar.ClID.String()] = rar
	}
	return nil
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

	service := NewRegistrarService(repo)

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
