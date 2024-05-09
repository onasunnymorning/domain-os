package postgres

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"gorm.io/gorm"
)

// RegistryOperatorRepository implements the RegistryOperatorRepository interface
type RegistryOperatorRepository struct {
	db *gorm.DB
}

// NewGORMRegistryOperatorRepository creates a new RegistryOperatorRepository
func NewGORMRegistryOperatorRepository(db *gorm.DB) *RegistryOperatorRepository {
	return &RegistryOperatorRepository{db: db}
}

// Create creates a new RegistryOperator in the database
func (r *RegistryOperatorRepository) Create(ctx context.Context, ro *entities.RegistryOperator) (*entities.RegistryOperator, error) {
	dbRO := &RegistryOperator{}
	dbRO.FromEntity(ro)
	if err := r.db.WithContext(ctx).Create(dbRO).Error; err != nil {
		return nil, err
	}
	return dbRO.ToEntity(), nil
}

// GetByRyID retrieves a RegistryOperator by its RyID
func (r *RegistryOperatorRepository) GetByRyID(ctx context.Context, ryID string) (*entities.RegistryOperator, error) {
	dbRO := &RegistryOperator{}
	err := r.db.WithContext(ctx).Where("ry_id = ?", ryID).First(dbRO).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, entities.ErrRegistryOperatorNotFound
		}
		return nil, err
	}

	return dbRO.ToEntity(), nil
}

// Update updates a RegistryOperator in the database
func (r *RegistryOperatorRepository) Update(ctx context.Context, ro *entities.RegistryOperator) (*entities.RegistryOperator, error) {
	dbRO := &RegistryOperator{}
	dbRO.FromEntity(ro)
	if err := r.db.WithContext(ctx).Save(dbRO).Error; err != nil {
		return nil, err
	}
	return dbRO.ToEntity(), nil
}

// DeleteByRyID deletes a RegistryOperator by its RyID
func (r *RegistryOperatorRepository) DeleteByRyID(ctx context.Context, ryID string) error {
	return r.db.WithContext(ctx).Where("ry_id = ?", ryID).Delete(&RegistryOperator{}).Error
}

// List retrieves RegistryOperators from the database
func (r *RegistryOperatorRepository) List(ctx context.Context, pagesize int, cursor string) ([]*entities.RegistryOperator, error) {
	dbRos := []*RegistryOperator{}

	err := r.db.WithContext(ctx).Order("ry_id ASC").Limit(pagesize).Find(&dbRos, "ry_id > ?", cursor).Error
	if err != nil {
		return nil, err
	}

	ros := make([]*entities.RegistryOperator, len(dbRos))
	for i, dbRo := range dbRos {
		ros[i] = dbRo.ToEntity()
	}

	return ros, nil
}
