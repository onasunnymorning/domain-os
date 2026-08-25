package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// ---------------------------------------------------------------------------
// Mock repository
// ---------------------------------------------------------------------------

type MockTombstoneRepository struct {
	Tombstones map[string]*entities.DomainTombstone // keyed by RoID
}

func NewMockTombstoneRepository() *MockTombstoneRepository {
	return &MockTombstoneRepository{
		Tombstones: make(map[string]*entities.DomainTombstone),
	}
}

func (m *MockTombstoneRepository) CreateTombstone(_ context.Context, t *entities.DomainTombstone) (*entities.DomainTombstone, error) {
	m.Tombstones[string(t.RoID)] = t
	return t, nil
}

func (m *MockTombstoneRepository) GetTombstoneByRoID(_ context.Context, roid string) (*entities.DomainTombstone, error) {
	if t, ok := m.Tombstones[roid]; ok {
		return t, nil
	}
	return nil, entities.ErrTombstoneNotFound
}

func (m *MockTombstoneRepository) GetTombstonesByName(_ context.Context, name string) ([]*entities.DomainTombstone, error) {
	var results []*entities.DomainTombstone
	for _, t := range m.Tombstones {
		if strings.EqualFold(t.Name.String(), name) {
			results = append(results, t)
		}
	}
	return results, nil
}

func (m *MockTombstoneRepository) ListTombstones(_ context.Context, _ queries.ListItemsQuery) ([]*entities.DomainTombstone, string, error) {
	var results []*entities.DomainTombstone
	for _, t := range m.Tombstones {
		results = append(results, t)
	}
	return results, "", nil
}

func (m *MockTombstoneRepository) CountTombstones(_ context.Context, _ queries.ListTombstonesFilter) (int64, error) {
	return int64(len(m.Tombstones)), nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestTombstoneService_GetByRoID(t *testing.T) {
	repo := NewMockTombstoneRepository()
	svc := NewTombstoneService(repo)
	ctx := context.Background()

	ts := &entities.DomainTombstone{
		RoID:         "ROID-12345",
		Name:         "example.com",
		TLDName:      "com",
		RegisteredAt: time.Now().Add(-365 * 24 * time.Hour),
		PurgedAt:     time.Now(),
		PurgeReason:  "expired",
	}
	repo.Tombstones["ROID-12345"] = ts

	t.Run("found", func(t *testing.T) {
		result, err := svc.GetTombstoneByRoID(ctx, "ROID-12345")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.RoID) != "ROID-12345" {
			t.Errorf("expected ROID-12345, got %s", result.RoID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := svc.GetTombstoneByRoID(ctx, "NONEXISTENT")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("empty roid", func(t *testing.T) {
		_, err := svc.GetTombstoneByRoID(ctx, "")
		if err == nil {
			t.Fatal("expected error for empty roid")
		}
	})

	t.Run("whitespace roid", func(t *testing.T) {
		_, err := svc.GetTombstoneByRoID(ctx, "   ")
		if err == nil {
			t.Fatal("expected error for whitespace roid")
		}
	})
}

func TestTombstoneService_GetByName(t *testing.T) {
	repo := NewMockTombstoneRepository()
	svc := NewTombstoneService(repo)
	ctx := context.Background()

	ts1 := &entities.DomainTombstone{
		RoID:        "ROID-1",
		Name:        "example.com",
		PurgedAt:    time.Now(),
		PurgeReason: "expired",
	}
	ts2 := &entities.DomainTombstone{
		RoID:        "ROID-2",
		Name:        "example.com",
		PurgedAt:    time.Now().Add(-24 * time.Hour),
		PurgeReason: "admin",
	}
	repo.Tombstones["ROID-1"] = ts1
	repo.Tombstones["ROID-2"] = ts2

	t.Run("finds all incarnations", func(t *testing.T) {
		results, err := svc.GetTombstonesByName(ctx, "example.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("expected 2 tombstones, got %d", len(results))
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		results, err := svc.GetTombstonesByName(ctx, "EXAMPLE.COM")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("expected 2 tombstones for uppercase input, got %d", len(results))
		}
	})

	t.Run("empty name", func(t *testing.T) {
		_, err := svc.GetTombstonesByName(ctx, "")
		if err == nil {
			t.Fatal("expected error for empty name")
		}
	})
}

func TestTombstoneService_ListAndCount(t *testing.T) {
	repo := NewMockTombstoneRepository()
	svc := NewTombstoneService(repo)
	ctx := context.Background()

	repo.Tombstones["ROID-A"] = &entities.DomainTombstone{
		RoID: "ROID-A", Name: "a.com", PurgedAt: time.Now(),
	}
	repo.Tombstones["ROID-B"] = &entities.DomainTombstone{
		RoID: "ROID-B", Name: "b.com", PurgedAt: time.Now(),
	}

	t.Run("list returns all", func(t *testing.T) {
		results, _, err := svc.ListTombstones(ctx, queries.ListItemsQuery{PageSize: 10})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("expected 2 tombstones, got %d", len(results))
		}
	})

	t.Run("list clamps page size", func(t *testing.T) {
		// Negative page size should default to 25
		_, _, err := svc.ListTombstones(ctx, queries.ListItemsQuery{PageSize: -1})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Large page size should be clamped to 200
		_, _, err = svc.ListTombstones(ctx, queries.ListItemsQuery{PageSize: 500})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("count", func(t *testing.T) {
		count, err := svc.CountTombstones(ctx, queries.ListTombstonesFilter{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 2 {
			t.Errorf("expected count 2, got %d", count)
		}
	})
}
