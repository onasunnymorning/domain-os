package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/miekg/dns"
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

// GetHostsForDomain retrieves the hosts associated with an active domain
type ActiveDomainQueryResult struct {
	Domain string
	Host   string
}

// GetActiveDomainsWithHosts gets the domains that are flagged as active and their associated hosts
// This data is used to build the NS records for a given TLD
func (dr *DomainRepository) GetActiveDomainsWithHosts(ctx context.Context, tld string) ([]dns.RR, error) {
	var queryResults []ActiveDomainQueryResult
	err := dr.db.Raw(`
		SELECT dom.name AS domain, ho.name AS host
		FROM public.domains dom
		LEFT JOIN domain_hosts dh ON dh.domain_ro_id = dom.ro_id
		LEFT JOIN hosts ho ON dh.host_ro_id = ho.ro_id
		WHERE dom.tld_name = ?
		AND dom.inactive = false
		AND dom.pending_delete = false
	`, tld).Scan(&queryResults).Error
	if err != nil {
		return nil, err
	}

	// Convert to DNS NS
	response := make([]dns.RR, len(queryResults))
	for i, result := range queryResults {
		ns, err := dns.NewRR(fmt.Sprintf("%s. 3600 IN NS %s", result.Domain, result.Host))
		if err != nil {
			return nil, err
		}
		response[i] = ns
	}

	return response, nil
}

// GlueQueryResult is a struct to hold the results of a query for glue records
// This is used to build the A or AAAA records (GLUE) for a given TLD
// These records are needed for in-bailiwick NS records
type GlueQueryResult struct {
	Host    string
	Address string
	Version int
}

// GetActiveDomainGlue gets the glue records for a given TLD
func (dr *DomainRepository) GetActiveDomainGlue(ctx context.Context, tld string) ([]dns.RR, error) {
	var queryResults []GlueQueryResult
	err := dr.db.Raw(`
		SELECT ho.name AS host, address, version
		FROM public.domains dom
		LEFT JOIN domain_hosts dh ON dh.domain_ro_id = dom.ro_id
		LEFT JOIN hosts ho ON dh.host_ro_id = ho.ro_id
		LEFT JOIN host_addresses ha ON ho.ro_id = ha.host_ro_id 
		WHERE dom.tld_name = ?
		AND dom.inactive = false
		AND ho.in_bailiwick = true
	`, tld).Scan(&queryResults).Error
	if err != nil {
		return nil, err
	}

	// Convert to DNS A or AAAA
	response := make([]dns.RR, len(queryResults))
	for i, result := range queryResults {
		t := "A"
		if result.Version == 6 {
			t = "AAAA"
		}
		rr, err := dns.NewRR(fmt.Sprintf("%s. 3600 IN %s %s", result.Host, t, result.Address))
		if err != nil {
			return nil, err
		}
		response[i] = rr

	}

	return response, nil
}

// Count returns the number of domains in the database
func (dr *DomainRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := dr.db.WithContext(ctx).Model(&Domain{}).Count(&count).Error
	return count, err
}

// ListExpiringDomains returns a list of domains that are expiring before the given time. These domain objects have minimal properties filled: RoID, Name and ExpiryDate
func (dr *DomainRepository) ListExpiringDomains(ctx context.Context, before time.Time, pagesize int, clid, cursor string) ([]*entities.Domain, error) {

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

	var dbDomains []*Domain
	err = dr.db.WithContext(ctx).Order("ro_id ASC").Select("ro_id", "name", "expiry_date").Where(&Domain{ClID: clid}).Where("expiry_date < ? AND pending_delete = ? AND pending_renew = ? AND pending_restore = ?", before, false, false, false).Limit(pagesize).Find(&dbDomains, "ro_id > ?", roidInt).Error
	if err != nil {
		return nil, err
	}

	domains := make([]*entities.Domain, len(dbDomains))
	for i, d := range dbDomains {
		domains[i] = ToDomain(d)
	}

	return domains, nil
}

// CountExiringDomains returns the number of domains that are expiring within the given number of days
func (dr *DomainRepository) CountExpiringDomains(ctx context.Context, days int, clid string) (int64, error) {
	var count int64
	err := dr.db.WithContext(ctx).Model(&Domain{}).Where(&Domain{ClID: clid}).Where("expiry_date <= ? AND pending_delete = ? AND pending_renew = ? AND pending_restore = ?", time.Now().AddDate(0, 0, days), false, false, false).Count(&count).Error
	return count, err
}
