package services

import (
	"context"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

type MockRegistryOperatorRepository struct {
	RegistryOperators map[string]*entities.RegistryOperator
	UpdateCalled      bool
}

func (m *MockRegistryOperatorRepository) Create(ctx context.Context, r *entities.RegistryOperator) (*entities.RegistryOperator, error) {
	m.RegistryOperators[r.RyID.String()] = r
	return r, nil
}

func (m *MockRegistryOperatorRepository) GetByRyID(ctx context.Context, ryid string) (*entities.RegistryOperator, error) {
	if ry, ok := m.RegistryOperators[ryid]; ok {
		return ry, nil
	}
	return nil, entities.ErrRegistryOperatorNotFound
}

func (m *MockRegistryOperatorRepository) Update(ctx context.Context, r *entities.RegistryOperator) (*entities.RegistryOperator, error) {
	m.UpdateCalled = true
	m.RegistryOperators[r.RyID.String()] = r
	return r, nil
}

func (m *MockRegistryOperatorRepository) DeleteByRyID(ctx context.Context, ryid string) error {
	delete(m.RegistryOperators, ryid)
	return nil
}

func (m *MockRegistryOperatorRepository) List(ctx context.Context, params queries.ListItemsQuery) ([]*entities.RegistryOperator, string, error) {
	var list []*entities.RegistryOperator
	for _, ry := range m.RegistryOperators {
		list = append(list, ry)
	}
	return list, "", nil
}

func (m *MockRegistryOperatorRepository) Count(ctx context.Context, filter queries.ListRegistryOperatorsFilter) (int64, error) {
	return int64(len(m.RegistryOperators)), nil
}

// TestRegistryOperatorService_Update_PreservesCreatedAt verifies that we don't accidentally
// null out the CreatedAt date when the frontend sends an update without it.
func TestRegistryOperatorService_Update_PreservesCreatedAt(t *testing.T) {
	ctx := context.Background()

	// 1. Create a dummy registry operator with a specific created at date in the mock repository
	originalCreatedAt := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	ryid := "test-ry"
	ry := &entities.RegistryOperator{
		RyID:      entities.ClIDType(ryid),
		Name:      "Test Registry Operator",
		CreatedAt: originalCreatedAt,
	}

	repo := &MockRegistryOperatorRepository{
		RegistryOperators: map[string]*entities.RegistryOperator{
			ryid: ry,
		},
	}

	service := NewRegistryOperatorService(repo)

	// 2. Simulate an incoming update payload where CreatedAt is zero
	incomingUpdate := &entities.RegistryOperator{
		RyID:      entities.ClIDType(ryid),
		Name:      "Updated Registry Operator Name",
		CreatedAt: time.Time{}, // zero value, representing missing or nulled field from JSON
	}

	// 3. Call Update
	updatedRy, err := service.Update(ctx, incomingUpdate)
	if err != nil {
		t.Fatalf("unexpected error during update: %v", err)
	}

	// 4. Verify that the previous CreatedAt was preserved, instead of being overwritten with zero
	if updatedRy.CreatedAt.IsZero() {
		t.Errorf("expected CreatedAt to be preserved, but it was set to zero value")
	}

	if !updatedRy.CreatedAt.Equal(originalCreatedAt) {
		t.Errorf("expected CreatedAt to equal %v, got %v", originalCreatedAt, updatedRy.CreatedAt)
	}

	if !repo.UpdateCalled {
		t.Errorf("expected Update to be called on repository")
	}
}
