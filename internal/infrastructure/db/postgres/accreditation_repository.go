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
func (r *AccreditationRepository) CreateAccreditation(ctx context.Context, tldName, rarClID string) error {
	return r.db.WithContext(ctx).Model(&TLD{Name: tldName}).Association("Registrars").Append(&Registrar{ClID: rarClID})
}

// DeleteAccreditation deletes an accreditation
func (r *AccreditationRepository) DeleteAccreditation(ctx context.Context, tldName, rarClID string) error {
	return r.db.WithContext(ctx).Model(&TLD{Name: tldName}).Association("Registrars").Delete(&Registrar{ClID: rarClID})
}

// ListTLDRegistrars lists registrars for a TLD
func (r *AccreditationRepository) ListTLDRegistrars(ctx context.Context, pageSize int, cursor string, tldName string) ([]*entities.Registrar, error) {
	dbRars := []*Registrar{}
	err := r.db.WithContext(ctx).Model(&TLD{Name: tldName}).Association("Registrars").Find(&dbRars)
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
func (r *AccreditationRepository) ListRegistrarTLDs(ctx context.Context, pageSize int, cursor string, rarClID string) ([]*entities.TLD, error) {
	dbTLDs := []*TLD{}
	err := r.db.WithContext(ctx).Model(&Registrar{ClID: rarClID}).Association("TLDs").Find(&dbTLDs)
	if err != nil {
		return nil, err
	}

	tlds := make([]*entities.TLD, len(dbTLDs))
	for i, dbTLD := range dbTLDs {
		tlds[i] = FromDBTLD(dbTLD)
	}

	return tlds, nil
}
