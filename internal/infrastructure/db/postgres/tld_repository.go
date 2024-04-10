package postgres

import (
	"context"
	"errors"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"gorm.io/gorm"
)

// GormTLDRepository implements the TLDRepo interface
type GormTLDRepository struct {
	db *gorm.DB
}

// NewGormTLDRepo returns a new GormTLDRepo
func NewGormTLDRepo(db *gorm.DB) *GormTLDRepository {
	return &GormTLDRepository{
		db: db,
	}
}

// GetByName returns a TLD by name
func (repo *GormTLDRepository) GetByName(ctx context.Context, name string) (*entities.TLD, error) {
	dbtld := &TLD{}

	err := repo.db.WithContext(ctx).Preload("Phases").Where("name = ?", name).First(dbtld).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entities.ErrTLDNotFound
		}
		return nil, err
	}

	tld := FromDBTLD(dbtld)

	return tld, nil
}

// Create creates a new TLD in the database
func (repo *GormTLDRepository) Create(ctx context.Context, tld *entities.TLD) error {
	// Map the TLD to a DBTLD
	dbtld := ToDBTLD(tld)

	err := repo.db.WithContext(ctx).Create(dbtld).Error
	if err != nil {
		return err
	}

	// Read the data from the repo to ensure we return the same data that was written
	storedDBTLD, err := repo.GetByName(ctx, tld.Name.String())
	if err != nil {
		return err
	}

	// Map the DBTLD back to a TLD
	*tld = *storedDBTLD

	return nil
}

// List returns a list of all TLDs. TLDs are ordered alphabetically by name and user pagination is supported by pagesize and cursor(name)
func (repo *GormTLDRepository) List(ctx context.Context, pageSize int, pageCursor string) ([]*entities.TLD, error) {
	dbtlds := []*TLD{}

	err := repo.db.WithContext(ctx).Order("name ASC").Limit(pageSize).Find(&dbtlds, "name > ?", pageCursor).Error
	if err != nil {
		return nil, err
	}

	tlds := make([]*entities.TLD, len(dbtlds))
	for i, dbtld := range dbtlds {
		tlds[i] = FromDBTLD(dbtld)
	}

	return tlds, nil
}

// Delete deletes a TLD from the database
func (repo *GormTLDRepository) DeleteByName(ctx context.Context, name string) error {
	return repo.db.WithContext(ctx).Where("name = ?", name).Delete(&TLD{}).Error
}

// Update updates a TLD in the database
func (repo *GormTLDRepository) Update(ctx context.Context, tld *entities.TLD) error {
	// Map the TLD to a DBTLD
	dbtld := ToDBTLD(tld)

	err := repo.db.WithContext(ctx).Save(dbtld).Error
	if err != nil {
		return err
	}

	// Read the data from the repo to ensure we return the same data that was written
	storedDBTLD, err := repo.GetByName(ctx, tld.Name.String())
	if err != nil {
		return err
	}

	// Map the DBTLD back to a TLD
	*tld = *storedDBTLD

	return nil
}
