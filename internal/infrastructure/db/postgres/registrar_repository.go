package postgres

import (
	"context"
	"errors"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"gorm.io/gorm"
)

// GormRegistrarRepository implements the RegistrarRepository interface
type GormRegistrarRepository struct {
	db *gorm.DB
}

// NewGormRegistrarRepository returns a new GormRegistrarRepository
func NewGormRegistrarRepository(db *gorm.DB) *GormRegistrarRepository {
	return &GormRegistrarRepository{
		db: db,
	}
}

// GetByClID looks up a Regsitrar by ite ClID and returns it
func (r *GormRegistrarRepository) GetByClID(ctx context.Context, clid string, preloadTLDs bool) (*entities.Registrar, error) {
	dbRar := &Registrar{}
	var err error

	if preloadTLDs {
		err = r.db.WithContext(ctx).Preload("TLDs").Where("cl_id = ?", clid).First(dbRar).Error
	} else {
		err = r.db.WithContext(ctx).Where("cl_id = ?", clid).First(dbRar).Error
	}

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entities.ErrRegistrarNotFound
		}
		return nil, err
	}

	rar := FromDBRegistrar(dbRar)

	return rar, nil
}

// GetByGurID looks up a Registrar by its GurID and returns it
// TODO: FIXME: This may retrun multiple results (e.g. 9999), so we need to handle this like a list endpoint
func (r *GormRegistrarRepository) GetByGurID(ctx context.Context, gurID int) (*entities.Registrar, error) {
	dbRar := &Registrar{}

	err := r.db.WithContext(ctx).Where("gur_id = ?", gurID).First(dbRar).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entities.ErrRegistrarNotFound
		}
		return nil, err
	}

	rar := FromDBRegistrar(dbRar)

	return rar, nil
}

// Create Creates a new Registrar in the repository
func (r *GormRegistrarRepository) Create(ctx context.Context, rar *entities.Registrar) (*entities.Registrar, error) {
	// Map
	dbRar := ToDBRegistrar(rar)

	err := r.db.WithContext(ctx).Omit("TLDs").Create(dbRar).Error // We omit TLDs as we manage these through the Accreditation repository
	if err != nil {
		return nil, err
	}
	// Read the data from the repo to ensure we return the same data that was written
	soredDbRar, err := r.GetByClID(ctx, rar.ClID.String(), false)
	if err != nil {
		return nil, err
	}

	return soredDbRar, nil
}

// Bulk Create Creates multiple registrars in the repository
func (r *GormRegistrarRepository) BulkCreate(ctx context.Context, rars []*entities.Registrar) error {
	dbRars := make([]*Registrar, len(rars))
	for i, rar := range rars {
		dbRars[i] = ToDBRegistrar(rar)
	}
	return r.db.WithContext(ctx).Omit("TLDs").Create(dbRars).Error // We omit TLDs as we manage these through the Accreditation repository
}

// Update Updates a registrar in the repository
func (r *GormRegistrarRepository) Update(ctx context.Context, rar *entities.Registrar) (*entities.Registrar, error) {
	// map
	dbRar := ToDBRegistrar(rar)

	err := r.db.WithContext(ctx).Omit("TLDs").Save(dbRar).Error // We omit TLDs as we manage these through the Accreditation repository
	if err != nil {
		return nil, err
	}

	// Read the data from the repo to ensure we return the same data that was written
	storedDbRar, err := r.GetByClID(ctx, rar.ClID.String(), false)
	if err != nil {
		return nil, err
	}

	return storedDbRar, nil
}

// Delete Deletes a registrar from the repository
func (r *GormRegistrarRepository) Delete(ctx context.Context, clid string) error {
	return r.db.WithContext(ctx).Where("cl_id = ?", clid).Delete(&Registrar{}).Error
}

// List returns a list of registrars
func (r *GormRegistrarRepository) List(ctx context.Context, pagesize int, cursor string) ([]*entities.RegistrarListItem, error) {
	dbRars := []*Registrar{}

	err := r.db.WithContext(ctx).Order("cl_id ASC").Limit(pagesize).Find(&dbRars, "cl_id > ?", cursor).Error
	if err != nil {
		return nil, err
	}

	rarList := make([]*entities.RegistrarListItem, len(dbRars))
	for i, dbRar := range dbRars {
		rar := FromDBRegistrar(dbRar)
		rarList[i] = rar.GetListRegistrarItem()
	}

	return rarList, nil
}

// Count returns the total number of registrars in the repository
func (r *GormRegistrarRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&Registrar{}).Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

// IsRegistrarAccreditedForTLD checks whether the specified registrar is accredited
// for a particular top-level domain (TLD). It queries the underlying database to
// match the provided registrar ID and TLD name, returning true if accreditation
// is confirmed, and false otherwise. Any query error is also returned.
func (r *GormRegistrarRepository) IsRegistrarAccreditedForTLD(ctx context.Context, tldName, rarClID string) (bool, error) {
	var rar string
	err := r.db.WithContext(ctx).Raw("SELECT registrar_cl_id FROM accreditations WHERE registrar_cl_id = ? AND tld_name = ?", rarClID, tldName).Scan(&rar).Error
	if err != nil {
		return false, err
	}

	return rar == rarClID, nil
}
