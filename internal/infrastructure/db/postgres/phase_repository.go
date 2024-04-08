package postgres

import (
	"context"
	"errors"
	"strconv"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"gorm.io/gorm"
)

// PhaseRepository is the GORM postgres implementation of the PhaseRepository interface
type PhaseRepository struct {
	db *gorm.DB
}

// NewGormPhaseRepository creates a new instance of PhaseRepository
func NewGormPhaseRepository(db *gorm.DB) *PhaseRepository {
	return &PhaseRepository{db: db}
}

// CreatePhase creates a new phase
func (r *PhaseRepository) CreatePhase(ctx context.Context, phase *entities.Phase) (*entities.Phase, error) {
	gormPhase := &Phase{}
	gormPhase.FromEntity(phase)
	err := r.db.WithContext(ctx).Create(gormPhase).Error
	if err != nil {
		return nil, err
	}
	return gormPhase.ToEntity(), nil
}

// GetPhaseByTLDAndName gets a phase by its name
func (r *PhaseRepository) GetPhaseByTLDAndName(ctx context.Context, tld, name string) (*entities.Phase, error) {
	var phase Phase
	err := r.db.WithContext(ctx).Preload("Fees").Where("name = ? AND tld_name = ?", name, tld).First(&phase).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entities.ErrPhaseNotFound
		}
		return nil, err
	}
	return phase.ToEntity(), nil
}

// DeletePhaseByName deletes a phase by its name
func (r *PhaseRepository) DeletePhaseByTLDAndName(ctx context.Context, tld, name string) error {
	return r.db.WithContext(ctx).Where("name = ? AND tld_name = ?", name, tld).Delete(&Phase{}).Error
}

// UpdatePhase updates a phase
func (r *PhaseRepository) UpdatePhase(ctx context.Context, phase *entities.Phase) (*entities.Phase, error) {
	gormPhase := &Phase{}
	gormPhase.FromEntity(phase)
	err := r.db.WithContext(ctx).Save(gormPhase).Error
	if err != nil {
		return nil, err
	}
	return gormPhase.ToEntity(), nil
}

// ListPhasesByTLD lists all phases for a TLD
func (r *PhaseRepository) ListPhasesByTLD(ctx context.Context, tld string, pageSize int, pageCursor string) ([]*entities.Phase, error) {
	var gormPhases []*Phase
	var pageCursorInt64 int64
	var err error
	// pageCursor for phases is of type int64, so convert it to int64
	// TODO: improve error handling
	if pageCursor != "" {
		pageCursorInt64, err = strconv.ParseInt(pageCursor, 10, 64)
		if err != nil {
			return nil, err
		}
	}
	err = r.db.WithContext(ctx).Where("tld_name = ?", tld).Order("id ASC").Limit(pageSize).Find(&gormPhases, "id > ?", pageCursorInt64).Error
	if err != nil {
		return nil, err // this is hard to test
	}
	phases := make([]*entities.Phase, len(gormPhases))
	for i, phase := range gormPhases {
		phases[i] = phase.ToEntity()
	}
	return phases, nil
}
