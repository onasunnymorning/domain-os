package postgres

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"gorm.io/gorm"
)

// DomainRepository is the postgres implementation of the DomainRepository Interface
type DomainRepository struct {
	db *gorm.DB
}

// NewDomainRepository creates a new DomainRepository
func NewDomainRepository(db *gorm.DB) *DomainRepository {
	return &DomainRepository{db}
}

// CreateDomain creates a new domain in the database
func (dr *DomainRepository) CreateDomain(ctx context.Context, d *entities.Domain) (*entities.Domain, error) {
	dbDomain := ToDBDomain(d)
	err := dr.db.WithContext(ctx).Create(dbDomain).Error
	if err != nil {
		return nil, err
	}
	return ToDomain(dbDomain), nil
}

// GetDomainByID retrieves a domain from the database by its ID
func (dr *DomainRepository) GetDomainByID(ctx context.Context, id int64) (*entities.Domain, error) {
	d := &Domain{}
	err := dr.db.First(d, id).Error
	return ToDomain(d), err
}

// GetDomainByName retrieves a domain from the database by its name
func (dr *DomainRepository) GetDomainByName(ctx context.Context, name string) (*entities.Domain, error) {
	d := &Domain{}
	err := dr.db.Where("name = ?", name).First(d).Error
	return ToDomain(d), err
}

// UpdateDomain updates a domain in the database
func (dr *DomainRepository) UpdateDomain(ctx context.Context, d *entities.Domain) (*entities.Domain, error) {
	dbDomain := ToDBDomain(d)
	err := dr.db.WithContext(ctx).Save(dbDomain).Error
	if err != nil {
		return nil, err
	}
	return ToDomain(dbDomain), nil
}

// DeleteDomain deletes a domain from the database
func (dr *DomainRepository) DeleteDomain(ctx context.Context, id int64) error {
	return dr.db.WithContext(ctx).Delete(&Domain{}, id).Error
}
