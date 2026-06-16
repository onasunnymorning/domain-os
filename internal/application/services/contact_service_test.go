package services

import (
	"context"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

type MockContactRepository struct {
	Contacts     map[string]*entities.Contact
	UpdateCalled bool
}

func (m *MockContactRepository) CreateContact(ctx context.Context, c *entities.Contact) (*entities.Contact, error) {
	m.Contacts[c.ID.String()] = c
	return c, nil
}

func (m *MockContactRepository) GetContactByID(ctx context.Context, id string) (*entities.Contact, error) {
	if c, ok := m.Contacts[id]; ok {
		return c, nil
	}
	return nil, entities.ErrContactNotFound
}

func (m *MockContactRepository) UpdateContact(ctx context.Context, c *entities.Contact) (*entities.Contact, error) {
	m.UpdateCalled = true
	m.Contacts[c.ID.String()] = c
	return c, nil
}

func (m *MockContactRepository) DeleteContactByID(ctx context.Context, id string) error {
	delete(m.Contacts, id)
	return nil
}

func (m *MockContactRepository) ListContacts(ctx context.Context, params queries.ListItemsQuery) ([]*entities.Contact, string, error) {
	return nil, "", nil
}

func (m *MockContactRepository) Count(ctx context.Context, filter queries.ListContactsFilter) (int64, error) {
	return int64(len(m.Contacts)), nil
}

func (m *MockContactRepository) BulkCreate(ctx context.Context, contacts []*entities.Contact) error {
	return nil
}

// TestContactService_UpdateContact_PreservesCreatedAt verifies that we don't accidentally
// null out the CreatedAt date when the frontend sends an update without it.
func TestContactService_UpdateContact_PreservesCreatedAt(t *testing.T) {
	ctx := context.Background()

	// 1. Create a dummy contact with a specific created at date in the mock repository
	originalCreatedAt := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	cid := "test-contact"
	c := &entities.Contact{
		ID:        entities.ClIDType(cid),
		CreatedAt: originalCreatedAt,
	}

	repo := &MockContactRepository{
		Contacts: map[string]*entities.Contact{
			cid: c,
		},
	}

	service := NewContactService(repo, RoidService{})

	// 2. Simulate an incoming update payload where CreatedAt is zero
	incomingUpdate := &entities.Contact{
		ID:        entities.ClIDType(cid),
		CreatedAt: time.Time{}, // zero value, representing missing or nulled field from JSON
	}

	// 3. Call Update
	updatedC, err := service.UpdateContact(ctx, incomingUpdate)
	if err != nil {
		t.Fatalf("unexpected error during update: %v", err)
	}

	// 4. Verify that the previous CreatedAt was preserved, instead of being overwritten with zero
	if updatedC.CreatedAt.IsZero() {
		t.Errorf("expected CreatedAt to be preserved, but it was set to zero value")
	}

	if !updatedC.CreatedAt.Equal(originalCreatedAt) {
		t.Errorf("expected CreatedAt to equal %v, got %v", originalCreatedAt, updatedC.CreatedAt)
	}

	if !repo.UpdateCalled {
		t.Errorf("expected Update to be called on repository")
	}
}
