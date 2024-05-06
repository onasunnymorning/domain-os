package postgres

import (
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
func (r *RegistryOperatorRepository) Create(ro *entities.RegistryOperator) (*entities.RegistryOperator, error) {
	dbRO := &RegistryOperator{}
	dbRO.FromEntity(ro)
	if err := r.db.Create(dbRO).Error; err != nil {
		return nil, err
	}
	return dbRO.ToEntity(), nil
}

// GetByRyID retrieves a RegistryOperator by its RyID
func (r *RegistryOperatorRepository) GetByRyID(ryID string) (*entities.RegistryOperator, error) {
	dbRO := &RegistryOperator{}
	err := r.db.Where("ry_id = ?", ryID).First(dbRO).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, entities.ErrRegistryOperatorNotFound
		}
		return nil, err
	}

	return dbRO.ToEntity(), nil
}

// Update updates a RegistryOperator in the database
func (r *RegistryOperatorRepository) Update(ro *entities.RegistryOperator) (*entities.RegistryOperator, error) {
	dbRO := &RegistryOperator{}
	dbRO.FromEntity(ro)
	if err := r.db.Save(dbRO).Error; err != nil {
		return nil, err
	}
	return dbRO.ToEntity(), nil
}

// DeleteByRyID deletes a RegistryOperator by its RyID
func (r *RegistryOperatorRepository) DeleteByRyID(ryID string) error {
	return r.db.Where("ry_id = ?", ryID).Delete(&RegistryOperator{}).Error
}
