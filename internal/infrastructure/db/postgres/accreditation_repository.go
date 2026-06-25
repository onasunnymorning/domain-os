package postgres

import (
	"context"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
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

	err := r.db.WithContext(ctx).Order("cl_id ASC").Limit(pageSize).Model(&TLD{Name: tldName}).Association("Registrars").Find(&dbRars, "cl_id > ?", cursor)
	if err != nil {
		return nil, err
	}

	rars := make([]*entities.Registrar, len(dbRars))
	for i, dbRar := range dbRars {
		rars[i] = FromDBRegistrar(dbRar)
	}

	return rars, nil
}

type dbAccreditedTLDRow struct {
	TLD
	DomainCount int `gorm:"column:domain_count"`
}

// ListRegistrarTLDs lists TLDs for a registrar
func (r *AccreditationRepository) ListRegistrarTLDs(ctx context.Context, pageSize int, cursor string, rarClID string) ([]*entities.TLD, error) {
	var rows []*dbAccreditedTLDRow

	dbQuery := r.db.WithContext(ctx).Table("tlds").
		Select("tlds.*, COALESCE(dc.domain_count, 0) as domain_count").
		Joins("INNER JOIN accreditations a ON a.tld_name = tlds.name AND a.registrar_cl_id = ?", rarClID).
		Joins("LEFT JOIN (SELECT tld_name AS dc_tld_name, COUNT(*) as domain_count FROM domains WHERE cl_id = ? GROUP BY tld_name) dc ON dc.dc_tld_name = tlds.name", rarClID).
		Order("tlds.name ASC")

	if cursor != "" {
		dbQuery = dbQuery.Where("tlds.name > ?", cursor)
	}

	err := dbQuery.Limit(pageSize).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	tlds := make([]*entities.TLD, len(rows))
	for i, row := range rows {
		tlds[i] = FromDBTLD(&row.TLD)
		tlds[i].DomainCount = row.DomainCount
	}

	return tlds, nil
}
