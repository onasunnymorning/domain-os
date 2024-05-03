package postgres

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"gorm.io/gorm"
)

// AccreditationRepository implements the AccreditationRepository interface
type AccreditationRepository struct {
	db *gorm.DB
}

// NewAccreditationRepository creates a new AccreditationRepository
func NewAccreditationRepository(db *gorm.DB) *AccreditationRepository {
	return &AccreditationRepository{db}
}

// CreateAccreditation creates a new accreditation
func (r *AccreditationRepository) CreateAccreditation(ctx context.Context, tld *entities.TLD, rar *entities.Registrar) error {
	return r.db.WithContext(ctx).Model(&TLD{Name: tld.Name.String()}).Association("Registrars").Append(&Registrar{ClID: rar.ClID.String()})
}

// DeleteAccreditation deletes an accreditation
func (r *AccreditationRepository) DeleteAccreditation(ctx context.Context, tld *entities.TLD, rar *entities.Registrar) error {
	dbTLD := ToDBTLD(tld)
	dbRar := ToDBRegistrar(rar)
	return r.db.WithContext(ctx).Model(&dbTLD).Association("Registrars").Delete(&dbRar)
}

// ListTLDRegistrars lists registrars for a TLD
func (r *AccreditationRepository) ListTLDRegistrars(ctx context.Context, pageSize int, cursor string, tld *entities.TLD) ([]*entities.Registrar, error) {
	dbTLD := ToDBTLD(tld)
	dbRars := []*Registrar{}
	err := r.db.WithContext(ctx).Model(&dbTLD).Association("Registrars").Find(&dbRars)
	if err != nil {
		return nil, err
	}

	rars := make([]*entities.Registrar, len(dbRars))
	for i, dbRar := range dbRars {
		rars[i] = FromDBRegistrar(dbRar)
	}

	return rars, nil
}

// ListRegistrarTLDs lists TLDs for a registrar
func (r *AccreditationRepository) ListRegistrarTLDs(ctx context.Context, pageSize int, cursor string, rar *entities.Registrar) ([]*entities.TLD, error) {
	dbRar := ToDBRegistrar(rar)
	dbTLDs := []*TLD{}
	err := r.db.WithContext(ctx).Model(&dbRar).Association("TLDs").Find(&dbTLDs)
	if err != nil {
		return nil, err
	}

	tlds := make([]*entities.TLD, len(dbTLDs))
	for i, dbTLD := range dbTLDs {
		tlds[i] = FromDBTLD(dbTLD)
	}

	return tlds, nil
}
