package postgres

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/miekg/dns"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
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

// GetDomainsByNames retrieves multiple domains from the database by their names.
// Returns the found domains; names that don't exist are silently skipped.
func (dr *DomainRepository) GetDomainsByNames(ctx context.Context, names []string, preloadHosts bool) ([]*entities.Domain, error) {
	if len(names) == 0 {
		return nil, nil
	}

	var dbDomains []Domain
	q := dr.db.WithContext(ctx)
	if preloadHosts {
		q = q.Preload("Hosts")
	}
	if err := q.Where("name IN ?", names).Find(&dbDomains).Error; err != nil {
		return nil, err
	}

	result := make([]*entities.Domain, len(dbDomains))
	for i := range dbDomains {
		result[i] = ToDomain(&dbDomains[i])
	}
	return result, nil
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
	var err error
	if params.Filter != nil {
		// cast interface to ListDomainsQueryFilter
		if filter, ok := params.Filter.(queries.ListDomainsFilter); !ok {
			return nil, "", ErrInvalidFilterType
		} else {
			if dbQuery, err = setDomainFilters(dbQuery, filter); err != nil {
				return nil, "", err
			}
		}
	}

	// Limit the number of results
	dbQuery = dbQuery.Limit(params.PageSize + 1) // Fetch one more than the page size to determine if there is a next page

	// Execute the query
	dbDomains := []*Domain{}
	err = dbQuery.Find(&dbDomains).Error
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
		AND COALESCE(dom.inactive, false) = false
		AND COALESCE(dom.pending_delete, false) = false
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
		AND COALESCE(dom.inactive, false) = false
		AND COALESCE(ho.in_bailiwick, false) = true
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
func (dr *DomainRepository) Count(ctx context.Context, filter queries.ListDomainsFilter) (int64, error) {
	var count int64

	// If no filters are provided, attempt to use pg_class estimates for performance (except in tests).
	if filter.IsEmpty() && flag.Lookup("test.v") == nil {
		err := dr.db.WithContext(ctx).Raw("SELECT COALESCE(reltuples::bigint, 0) FROM pg_class WHERE relname = 'domains'").Scan(&count).Error
		if err == nil && count > 0 {
			return count, nil
		}
	}

	// Create a query object
	dbQuery := dr.db.WithContext(ctx).Model(&Domain{})

	// Add filters
	var err error
	if dbQuery, err = setDomainFilters(dbQuery, filter); err != nil {
		return 0, err
	}

	// Execute the query
	err = dbQuery.Count(&count).Error

	// Return the count
	return count, err
}

// activeGAPhaseFilter is a SQL condition that restricts results to domains whose
// TLD has at least one currently active GA phase. A phase is active when its
// start date is in the past and it either has no end date or the end date is in
// the future. This prevents lifecycle workflows from processing domains in
// "sleeping" TLDs that have no active phase.
const activeGAPhaseFilter = `EXISTS (
	SELECT 1 FROM phases
	WHERE phases.tld_name = domains.tld_name
	  AND phases.type = 'GA'
	  AND phases.starts <= NOW()
	  AND (phases.ends IS NULL OR phases.ends > NOW())
)`

// ListExpiringDomains returns a list of domains that are expiring before the given time. These domain objects have minimal properties filled: RoID, Name and ExpiryDate
func (dr *DomainRepository) ListExpiringDomains(ctx context.Context, before time.Time, pagesize int, clid, tld, cursor string) ([]*entities.Domain, error) {
	roidInt, err := getInt64RoidFromDomainRoidString(cursor)
	if err != nil {
		return nil, err
	}

	var dbDomains []*Domain
	err = dr.db.WithContext(ctx).Order("ro_id ASC").Select("ro_id", "name", "expiry_date").Where(&Domain{ClID: clid, TLDName: tld}).Where("expiry_date <= ? AND COALESCE(pending_delete, false) = false AND COALESCE(pending_renew, false) = false AND COALESCE(pending_restore, false) = false", before).Where(activeGAPhaseFilter).Limit(pagesize).Find(&dbDomains, "ro_id > ?", roidInt).Error
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
	err := dr.db.WithContext(ctx).Model(&Domain{}).Where(&Domain{ClID: clid, TLDName: tld}).Where("expiry_date <= ? AND COALESCE(pending_delete, false) = false AND COALESCE(pending_renew, false) = false AND COALESCE(pending_restore, false) = false", before).Where(activeGAPhaseFilter).Count(&count).Error
	return count, err
}

// ListPurgeableDomains returns a list of domains that are pending deletion and
// whose purge date falls on or before the given cutoff time.
//
// Note: the parameter order (clid, tld, cursor) matches the
// repositories.DomainRepository interface. A previous version of this method
// declared (clid, cursor, tld), silently swapping the two string arguments at
// the call site and dropping the TLD filter entirely.
func (dr *DomainRepository) ListPurgeableDomains(ctx context.Context, before time.Time, pagesize int, clid, tld, cursor string) ([]*entities.Domain, error) {
	roidInt, err := getInt64RoidFromDomainRoidString(cursor)
	if err != nil {
		return nil, err
	}

	var dbDomains []*Domain
	err = dr.db.WithContext(ctx).Order("ro_id ASC").Select("ro_id", "name", "expiry_date", "purge_date").Where(&Domain{ClID: clid, TLDName: tld}).Where("purge_date <= ? AND purge_date > '0001-01-01' AND COALESCE(pending_delete, false) = true", before).Where(activeGAPhaseFilter).Limit(pagesize).Find(&dbDomains, "ro_id > ?", roidInt).Error
	if err != nil {
		return nil, err
	}

	domains := make([]*entities.Domain, len(dbDomains))
	for i, d := range dbDomains {
		domains[i] = ToDomain(d)
	}

	return domains, nil
}

// CountPurgeableDomains returns the number of domains that are pending deletion
// and whose purge date falls on or before the given cutoff time.
func (dr *DomainRepository) CountPurgeableDomains(ctx context.Context, before time.Time, clid, tld string) (int64, error) {
	var count int64
	err := dr.db.WithContext(ctx).Model(&Domain{}).Where(&Domain{ClID: clid, TLDName: tld}).Where("purge_date <= ? AND purge_date > '0001-01-01' AND COALESCE(pending_delete, false) = true", before).Where(activeGAPhaseFilter).Count(&count).Error
	return count, err
}

// CountRestoredDomains returns the number of domains that are in pendingRestore state (have been restored using the Domain.Restore() function)
func (dr *DomainRepository) CountRestoredDomains(ctx context.Context, clid, tld string) (int64, error) {
	var count int64
	err := dr.db.WithContext(ctx).Model(&Domain{}).Where(&Domain{ClID: clid, TLDName: tld}).Where("COALESCE(pending_restore, false) = true").Where(activeGAPhaseFilter).Count(&count).Error
	return count, err
}

// ListRestoredDomains returns a list of domains that are in pendingRestore state (have been restored using the Domain.Restore() function)
func (dr *DomainRepository) ListRestoredDomains(ctx context.Context, pagesize int, clid, tld, cursor string) ([]*entities.Domain, error) {
	roidInt, err := getInt64RoidFromDomainRoidString(cursor)
	if err != nil {
		return nil, err
	}

	var dbDomains []*Domain
	err = dr.db.WithContext(ctx).Order("ro_id ASC").Select("ro_id", "name", "cl_id").Where(&Domain{ClID: clid, TLDName: tld}).Where("COALESCE(pending_restore, false) = true").Where(activeGAPhaseFilter).Limit(pagesize).Find(&dbDomains, "ro_id > ?", roidInt).Error
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

func setDomainFilters(dbQuery *gorm.DB, filter queries.ListDomainsFilter) (*gorm.DB, error) {

	if filter.ClidEquals != "" {
		dbQuery = dbQuery.Where("cl_id = ?", filter.ClidEquals)
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
			return nil, fmt.Errorf("invalid RoId for greater than filter: %w", err)
		}
		dbQuery = dbQuery.Where("ro_id > ?", roidInt)
	}
	if filter.RoidLessThan != "" {
		roidInt, err := getInt64RoidFromDomainRoidString(filter.RoidLessThan)
		if err != nil {
			return nil, fmt.Errorf("invalid RoId for less than filter: %w", err)
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

	return dbQuery, nil
}

// ListEventsByDomain lists events matching the subject (domain name) in reverse chronological order.
// Results are capped at 1000 to prevent unbounded queries at scale.
func (dr *DomainRepository) ListEventsByDomain(ctx context.Context, domainName string) ([]entities.DomainEvent, error) {
	var dbEvents []DomainEventRecord
	err := dr.db.WithContext(ctx).
		Where("subject = ?", domainName).
		Order("occurred_at DESC").
		Limit(1000).
		Find(&dbEvents).Error
	if err != nil {
		return nil, fmt.Errorf("ListEventsByDomain(domain=%s) database query failed: %w. Check that the database is accessible and the domain_events table exists", domainName, err)
	}

	events := make([]entities.DomainEvent, len(dbEvents))
	for i, dbEvent := range dbEvents {
		e, err := dbEvent.ToDomainEvent()
		if err != nil {
			return nil, fmt.Errorf("ListEventsByDomain(domain=%s) deserialization error for event ID %s: %w", domainName, dbEvent.ID, err)
		}
		events[i] = e
	}
	return events, nil
}

// ListRecentEvents lists the most recent events across all subjects in reverse chronological order
func (dr *DomainRepository) ListRecentEvents(ctx context.Context, limit int) ([]entities.DomainEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var dbEvents []DomainEventRecord
	err := dr.db.WithContext(ctx).
		Order("occurred_at DESC").
		Limit(limit).
		Find(&dbEvents).Error
	if err != nil {
		return nil, fmt.Errorf("ListRecentEvents(limit=%d) database query failed: %w. Check that the database is accessible and the domain_events table exists", limit, err)
	}

	events := make([]entities.DomainEvent, len(dbEvents))
	for i, dbEvent := range dbEvents {
		e, err := dbEvent.ToDomainEvent()
		if err != nil {
			return nil, fmt.Errorf("ListRecentEvents(limit=%d) deserialization error for event ID %s: %w", limit, dbEvent.ID, err)
		}
		events[i] = e
	}
	return events, nil
}
