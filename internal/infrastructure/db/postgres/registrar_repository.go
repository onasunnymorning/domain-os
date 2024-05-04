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
func (r *GormRegistrarRepository) GetByClID(ctx context.Context, clid string) (*entities.Registrar, error) {
	dbRar := &Registrar{}

	err := r.db.WithContext(ctx).Where("cl_id = ?", clid).First(dbRar).Error
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
	soredDbRar, err := r.GetByClID(ctx, rar.ClID.String())
	if err != nil {
		return nil, err
	}

	return soredDbRar, nil
}

// Update Updates a registrar in the repository
func (r *GormRegistrarRepository) Update(ctx context.Context, rar *entities.Registrar) (*entities.Registrar, error) {
	// map
	dbRar := ToDBRegistrar(rar)

	err := r.db.WithContext(ctx).Omit("Addresses").Save(dbRar).Error // We omit TLDs as we manage these through the Accreditation repository
	if err != nil {
		return nil, err
	}

	// Read the data from the repo to ensure we return the same data that was written
	storedDbRar, err := r.GetByClID(ctx, rar.ClID.String())
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
func (r *GormRegistrarRepository) List(ctx context.Context, pagesize int, cursor string) ([]*entities.Registrar, error) {
	dbRars := []*Registrar{}

	err := r.db.WithContext(ctx).Order("cl_id ASC").Limit(pagesize).Find(&dbRars, "cl_id > ?", cursor).Error
	if err != nil {
		return nil, err
	}

	rars := make([]*entities.Registrar, len(dbRars))
	for i, dbRar := range dbRars {
		rars[i] = FromDBRegistrar(dbRar)
	}

	return rars, nil
}
