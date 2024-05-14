package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
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
		var perr *pgconn.PgError
		if errors.As(err, &perr) && perr.Code == "23505" {
			return nil, entities.ErrDomainAlreadyExists
		}
		return nil, err
	}
	return ToDomain(dbDomain), nil
}

// GetDomainByID retrieves a domain from the database by its ID
func (dr *DomainRepository) GetDomainByID(ctx context.Context, id int64, preloadHosts bool) (*entities.Domain, error) {
	var err error
	d := &Domain{}
	if preloadHosts {
		err = dr.db.WithContext(ctx).Preload("Hosts").First(d, id).Error
	} else {
		err = dr.db.WithContext(ctx).First(d, id).Error
	}
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, entities.ErrDomainNotFound
		}
		return nil, err
	}
	return ToDomain(d), err
}

// GetDomainByName retrieves a domain from the database by its name
func (dr *DomainRepository) GetDomainByName(ctx context.Context, name string, preloadHosts bool) (*entities.Domain, error) {
	var err error
	d := &Domain{}
	if preloadHosts {
		err = dr.db.WithContext(ctx).Preload("Hosts").Where("name = ?", name).First(d).Error
	} else {
		err = dr.db.WithContext(ctx).Where("name = ?", name).First(d).Error
	}
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, entities.ErrDomainNotFound
		}
		return nil, err
	}
	return ToDomain(d), nil
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

// DeleteDomain deletes a domain from the database by its id
func (dr *DomainRepository) DeleteDomainByID(ctx context.Context, id int64) error {
	return dr.db.WithContext(ctx).Delete(&Domain{}, id).Error
}

// DeleteDomain deletes a domain from the database by its name
func (dr *DomainRepository) DeleteDomainByName(ctx context.Context, name string) error {
	return dr.db.WithContext(ctx).Where("name = ?", name).Delete(&Domain{}).Error
}

// ListDomains returns a list of Domains
func (dr *DomainRepository) ListDomains(ctx context.Context, pagesize int, cursor string) ([]*entities.Domain, error) {
	var roidInt int64
	var err error
	if cursor != "" {
		roid := entities.RoidType(cursor)
		if roid.ObjectIdentifier() != entities.DOMAIN_ROID_ID {
			return nil, entities.ErrInvalidRoid
		}
		roidInt, err = roid.Int64()
		if err != nil {
			return nil, err
		}
	}
	dbDomains := []*Domain{}
	err = dr.db.WithContext(ctx).Order("ro_id ASC").Limit(pagesize).Find(&dbDomains, "ro_id > ?", roidInt).Error
	if err != nil {
		return nil, err
	}

	domains := make([]*entities.Domain, len(dbDomains))
	for i, d := range dbDomains {
		domains[i] = ToDomain(d)
	}

	return domains, nil
}

// AddHostToDomain adds a domain_hosts association to the database
func (dr *DomainRepository) AddHostToDomain(ctx context.Context, domRoID int64, hostRoid int64) error {
	return dr.db.WithContext(ctx).Model(&Domain{RoID: domRoID}).Association("Hosts").Append(&Host{RoID: hostRoid})
}

// RemoveHostFromDomain removes a domain_hosts association from the database
func (dr *DomainRepository) RemoveHostFromDomain(ctx context.Context, domRoID int64, hostRoid int64) error {
	return dr.db.WithContext(ctx).Model(&Domain{RoID: domRoID}).Association("Hosts").Delete(&Host{RoID: hostRoid})
}
