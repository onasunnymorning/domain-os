package postgres

import (
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

func TestPremiumList_ToEntity(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "Test ToEntity",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pl := &PremiumList{
				Name:      "example",
				CreatedAt: "2021-08-01T00:00:00Z",
				UpdatedAt: "2021-08-01T00:00:00Z",
			}
			entity := pl.ToEntity()
			if entity.Name != pl.Name {
				t.Errorf("Expected %s, got %s", pl.Name, entity.Name)
			}
			if entity.CreatedAt != pl.CreatedAt {
				t.Errorf("Expected %s, got %s", pl.CreatedAt, entity.CreatedAt)
			}
			if entity.UpdatedAt != pl.UpdatedAt {
				t.Errorf("Expected %s, got %s", pl.UpdatedAt, entity.UpdatedAt)
			}
		})
	}
}

func TestPremiumList_FromEntity(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "Test FromEntity",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pl := &PremiumList{}
			entity := &entities.PremiumList{
				Name:      "example",
				CreatedAt: "2021-08-01T00:00:00Z",
				UpdatedAt: "2021-08-01T00:00:00Z",
			}
			pl.FromEntity(entity)
			if pl.Name != entity.Name {
				t.Errorf("Expected %s, got %s", entity.Name, pl.Name)
			}
			if pl.CreatedAt != entity.CreatedAt {
				t.Errorf("Expected %s, got %s", entity.CreatedAt, pl.CreatedAt)
			}
			if pl.UpdatedAt != entity.UpdatedAt {
				t.Errorf("Expected %s, got %s", entity.UpdatedAt, pl.UpdatedAt)
			}
		})
	}
}
