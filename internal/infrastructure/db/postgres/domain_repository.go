package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/miekg/dns"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
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

// Create creates a new domain in the database
func (dr *DomainRepository) Create(ctx context.Context, d *entities.Domain) (*entities.Domain, error) {
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

// Bulk Create Creates multiple domains in the repository, useful when importing data. Does not persist Hosts if present
func (r *DomainRepository) BulkCreate(ctx context.Context, doms []*entities.Domain) error {
	dbdoms := make([]*Domain, len(doms))
	for i, dom := range doms {
		dbdoms[i] = ToDBDomain(dom)
	}
	return r.db.WithContext(ctx).Omit("Hosts").Create(dbdoms).Error // We omit Hosts as we manage these through the Host linking functions
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

// GetDomainByName retrieves a domain from the database by its name it returns ErrDomainNotFound if the domain does not exist
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

// ListDomains retrieves domains from the database applying optional filters and cursor-based pagination.
// It constructs a query that orders domain records by their primary key (ro_id) in ascending order.
// It supports filtering by various domain attributes such as client ID, TLD name, domain name (both exact and partial matches),
// ROID, and by creation or expiry dates (before/after).
//
// If a page cursor is provided, the query starts after the given ro_id. The query limits the results to
// (PageSize + 1) records to determine if there is an additional page. If more results exist than PageSize,
// a new cursor is set to the ro_id of the last returned domain, enabling further pagination.
func (dr *DomainRepository) ListDomains(ctx context.Context, params queries.ListItemsQuery) ([]*entities.Domain, string, error) {
	// Create a query and order by our pk
	dbQuery := dr.db.WithContext(ctx).Order("ro_id ASC")

	// Add cursor pagination if a cursor is provided
	if params.PageCursor != "" {
		cursor, err := getInt64RoidFromDomainRoidString(params.PageCursor)
		if err != nil {
			return nil, "", fmt.Errorf("invalid page cursor: %w", err)
		}
		dbQuery = dbQuery.Where("ro_id > ?", cursor)
	}

	// Add filters if provided
	if params.Filter != nil {
		// cast interface to ListDomainsQueryFilter
		if filter, ok := params.Filter.(queries.ListDomainsFilter); !ok {
			return nil, "", ErrInvalidFilterType
		} else {
			if filter.ClIDEquals != "" {
				dbQuery = dbQuery.Where("cl_id = ?", filter.ClIDEquals)
			}
			if filter.TldEquals != "" {
				dbQuery = dbQuery.Where("tld_name = ?", filter.TldEquals)
			}
			if filter.NameLike != "" {
				dbQuery = dbQuery.Where("name ILIKE ?", "%"+filter.NameLike+"%")
			}
			if filter.NameEquals != "" {
				dbQuery = dbQuery.Where("name = ?", filter.NameEquals)
			}
			if filter.RoidGreaterThan != "" {
				roidInt, err := getInt64RoidFromDomainRoidString(filter.RoidGreaterThan)
				if err != nil {
					return nil, "", fmt.Errorf("invalid RoId for greater than filter: %w", err)
				}
				dbQuery = dbQuery.Where("ro_id > ?", roidInt)
			}
			if filter.RoidLessThan != "" {
				roidInt, err := getInt64RoidFromDomainRoidString(filter.RoidLessThan)
				if err != nil {
					return nil, "", fmt.Errorf("invalid RoId for less than filter: %w", err)
				}
				dbQuery = dbQuery.Where("ro_id < ?", roidInt)
			}
			if !filter.ExpiresBefore.IsZero() {
				dbQuery = dbQuery.Where("expiry_date < ?", filter.ExpiresBefore)
			}
			if !filter.ExpiresAfter.IsZero() {
				dbQuery = dbQuery.Where("expiry_date > ?", filter.ExpiresAfter)
			}
			if !filter.CreatedBefore.IsZero() {
				dbQuery = dbQuery.Where("created_at < ?", filter.CreatedBefore)
			}
			if !filter.CreatedAfter.IsZero() {
				dbQuery = dbQuery.Where("created_at > ?", filter.CreatedAfter)
			}
		}
	}

	// Limit the number of results
	dbQuery = dbQuery.Limit(params.PageSize + 1) // Fetch one more than the page size to determine if there is a next page

	// Execute the query
	dbDomains := []*Domain{}
	err := dbQuery.Find(&dbDomains).Error
	if err != nil {
		return nil, "", err
	}

	// Check if there is a next page
	hasMore := len(dbDomains) == params.PageSize+1
	if hasMore {
		// Return up to PageSize
		dbDomains = dbDomains[:params.PageSize]
	}

	// Map the DBDomains to Domains
	domains := make([]*entities.Domain, len(dbDomains))
	for i, d := range dbDomains {
		domains[i] = ToDomain(d)
	}

	// Set the cursor to the last element if needed
	var newCursor string
	if hasMore {
		newCursor = domains[len(domains)-1].RoID.String()
	}

	return domains, newCursor, nil
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
func (dr *DomainRepository) GetActiveDomainsWithHosts(ctx context.Context, params queries.ActiveDomainsWithHostsQuery) ([]dns.RR, error) {
	var queryResults []ActiveDomainQueryResult
	err := dr.db.Raw(`
		SELECT dom.name AS domain, ho.name AS host
		FROM public.domains dom
		LEFT JOIN domain_hosts dh ON dh.domain_ro_id = dom.ro_id
		LEFT JOIN hosts ho ON dh.host_ro_id = ho.ro_id
		WHERE dom.tld_name = ?
		AND dom.inactive = false
		AND dom.pending_delete = false
	`, params.TldName).Scan(&queryResults).Error
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
func (dr *DomainRepository) ListExpiringDomains(ctx context.Context, before time.Time, pagesize int, clid, tld, cursor string) ([]*entities.Domain, error) {
	roidInt, err := getInt64RoidFromDomainRoidString(cursor)
	if err != nil {
		return nil, err
	}

	var dbDomains []*Domain
	err = dr.db.WithContext(ctx).Order("ro_id ASC").Select("ro_id", "name", "expiry_date").Where(&Domain{ClID: clid, TLDName: tld}).Where("expiry_date < ? AND pending_delete = ? AND pending_renew = ? AND pending_restore = ?", before, false, false, false).Limit(pagesize).Find(&dbDomains, "ro_id > ?", roidInt).Error
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
func (dr *DomainRepository) CountExpiringDomains(ctx context.Context, before time.Time, clid, tld string) (int64, error) {
	var count int64
	err := dr.db.WithContext(ctx).Model(&Domain{}).Where(&Domain{ClID: clid, TLDName: tld}).Where("expiry_date <= ? AND pending_delete = ? AND pending_renew = ? AND pending_restore = ?", before, false, false, false).Count(&count).Error
	return count, err
}

// ListPurgeableDomains returns a list of domains that are pending deletion and have passed the grace period
func (dr *DomainRepository) ListPurgeableDomains(ctx context.Context, after time.Time, pagesize int, clid, cursor, tld string) ([]*entities.Domain, error) {
	roidInt, err := getInt64RoidFromDomainRoidString(cursor)
	if err != nil {
		return nil, err
	}

	var dbDomains []*Domain
	err = dr.db.WithContext(ctx).Order("ro_id ASC").Select("ro_id", "name", "expiry_date", "purge_date").Where(&Domain{ClID: clid}).Where("purge_date <= ? AND purge_date > '0001-01-01' AND pending_delete = true", after).Limit(pagesize).Find(&dbDomains, "ro_id > ?", roidInt).Error
	if err != nil {
		return nil, err
	}

	domains := make([]*entities.Domain, len(dbDomains))
	for i, d := range dbDomains {
		domains[i] = ToDomain(d)
	}

	return domains, nil
}

// CountPurgeableDomains returns the number of domains that are pending deletion and have passed the grace period
func (dr *DomainRepository) CountPurgeableDomains(ctx context.Context, after time.Time, clid, tld string) (int64, error) {
	var count int64
	err := dr.db.WithContext(ctx).Model(&Domain{}).Where(&Domain{ClID: clid, TLDName: tld}).Where("purge_date <= ? AND purge_date > '0001-01-01' AND pending_delete = true", after).Count(&count).Error
	return count, err
}

// CountRestoredDomains returns the number of domains that are in pendingRestore state (have been restored using the Domain.Restore() function)
func (dr *DomainRepository) CountRestoredDomains(ctx context.Context, clid, tld string) (int64, error) {
	var count int64
	err := dr.db.WithContext(ctx).Model(&Domain{}).Where(&Domain{ClID: clid, TLDName: tld}).Where("pending_restore = true").Count(&count).Error
	return count, err
}

// ListRestoredDomains returns a list of domains that are in pendingRestore state (have been restored using the Domain.Restore() function)
func (dr *DomainRepository) ListRestoredDomains(ctx context.Context, pagesize int, clid, tld, cursor string) ([]*entities.Domain, error) {
	roidInt, err := getInt64RoidFromDomainRoidString(cursor)
	if err != nil {
		return nil, err
	}

	var dbDomains []*Domain
	err = dr.db.WithContext(ctx).Order("ro_id ASC").Select("ro_id", "name", "cl_id").Where(&Domain{ClID: clid, TLDName: tld}).Where("pending_restore = true").Limit(pagesize).Find(&dbDomains, "ro_id > ?", roidInt).Error
	if err != nil {
		return nil, err
	}

	domains := make([]*entities.Domain, len(dbDomains))
	for i, d := range dbDomains {
		domains[i] = ToDomain(d)
	}

	return domains, nil
}

// getInt64RoidFromDomainRoidString converts a ROID string (1234_DOM-APEX) to an int64 (1234) if it is a valid DOMAIN_ROID_ID.
// It returns an error if the ROID is invalid.
// If the ROID is empty, it returns 0 and no error (e.g. no pagination is neeced)
func getInt64RoidFromDomainRoidString(roidString string) (int64, error) {
	// If the cursor is empty, we don't need to paginate, this is not an error
	if roidString == "" {
		return 0, nil
	}
	roid := entities.RoidType(roidString)
	if validationErr := roid.Validate(); validationErr != nil {
		return 0, validationErr
	}
	if roid.ObjectIdentifier() != entities.DOMAIN_ROID_ID {
		return 0, entities.ErrInvalidRoid
	}
	return roid.Int64()
}
