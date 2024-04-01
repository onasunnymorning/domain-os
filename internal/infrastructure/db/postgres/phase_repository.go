package postgres

import (
	"context"

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

// GetPhaseByName gets a phase by its name
func (r *PhaseRepository) GetPhaseByName(ctx context.Context, name string) (*entities.Phase, error) {
	var phase Phase
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&phase).Error
	if err != nil {
		return nil, err
	}
	return phase.ToEntity(), nil
}

// DeletePhaseByName deletes a phase by its name
func (r *PhaseRepository) DeletePhaseByName(ctx context.Context, name string) error {
	return r.db.WithContext(ctx).Where("name = ?", name).Delete(&Phase{}).Error
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
