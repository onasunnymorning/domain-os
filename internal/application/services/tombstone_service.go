package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// TombstoneService provides application-level operations for domain tombstones.
type TombstoneService struct {
	repo repositories.TombstoneRepository
}

// NewTombstoneService returns a new TombstoneService instance.
func NewTombstoneService(repo repositories.TombstoneRepository) *TombstoneService {
	return &TombstoneService{repo: repo}
}

// GetTombstoneByRoID retrieves a single tombstone by its ROID.
func (s *TombstoneService) GetTombstoneByRoID(ctx context.Context, roid string) (*entities.DomainTombstone, error) {
	roid = strings.TrimSpace(roid)
	if roid == "" {
		return nil, fmt.Errorf("GetTombstoneByRoID: %w: roid is empty", entities.ErrTombstoneNotFound)
	}
	return s.repo.GetTombstoneByRoID(ctx, roid)
}

// GetTombstonesByName retrieves all incarnations of a domain name, ordered by PurgedAt DESC.
func (s *TombstoneService) GetTombstonesByName(ctx context.Context, name string) ([]*entities.DomainTombstone, error) {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return nil, fmt.Errorf("GetTombstonesByName: name is empty")
	}
	return s.repo.GetTombstonesByName(ctx, name)
}

// ListTombstones returns a paginated list of tombstones with optional filters.
func (s *TombstoneService) ListTombstones(ctx context.Context, params queries.ListItemsQuery) ([]*entities.DomainTombstone, string, error) {
	if params.PageSize <= 0 {
		params.PageSize = 25
	}
	if params.PageSize > 200 {
		params.PageSize = 200
	}
	return s.repo.ListTombstones(ctx, params)
}

// CountTombstones returns the number of tombstones matching the given filter.
func (s *TombstoneService) CountTombstones(ctx context.Context, filter queries.ListTombstonesFilter) (int64, error) {
	return s.repo.CountTombstones(ctx, filter)
}
