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
func (dr *DomainRepository) CreateDomain(ctx context.Context, *entities.Domain) (*entities.Domain, error) {
	dbDomain := ToDBDomain(d)
	return dr.db.WithContext(ctx).(dbDomain).Error
}

// GetDomainByID retrieves a domain from the database by its ID
func (dr *DomainRepository) GetDomainByID(id int64) (*Domain, error) {
	d := &Domain{}
	err := dr.db.First(d, id).Error
	return d, err
}

// GetDomainByName retrieves a domain from the database by its name
func (dr *DomainRepository) GetDomainByName(name string) (*Domain, error) {
	d := &Domain{}
	err := dr.db.Where("name = ?", name).First(d).Error
	return d, err
}

// UpdateDomain updates a domain in the database
func (dr *DomainRepository) UpdateDomain(d *Domain) error {
	return dr.db.Save(d).Error
}
