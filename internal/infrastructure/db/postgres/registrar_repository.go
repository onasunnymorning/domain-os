package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"gorm.io/gorm"
)

type dbRegistrarListItem struct {
	ClID       string `gorm:"column:cl_id"`
	Name       string `gorm:"column:name"`
	GurID      int    `gorm:"column:gur_id"`
	Status     string `gorm:"column:status"`
	IANAStatus string `gorm:"column:iana_status"`
	Autorenew  bool   `gorm:"column:autorenew"`
	DomainCount int    `gorm:"column:domain_count"`
	TLDList     string `gorm:"column:tld_list"`
}

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
func (r *GormRegistrarRepository) List(ctx context.Context, params queries.ListItemsQuery) ([]*entities.RegistrarListItem, string, error) {
	// Base query from table
	dbQuery := r.db.WithContext(ctx).Table("registrars")

	// Select optimized query fields joining aggregated count and TLD string
	selectFields := "registrars.cl_id, name, gur_id, status, autorenew, iana_status, " +
		"COALESCE(dc.domain_count, 0) as domain_count, " +
		"COALESCE(ac.tld_list, '') as tld_list"
	
	dbQuery = dbQuery.Select(selectFields).
		Joins("LEFT JOIN (SELECT cl_id AS dc_cl_id, COUNT(*) as domain_count FROM domains GROUP BY cl_id) dc ON dc.dc_cl_id = registrars.cl_id").
		Joins("LEFT JOIN (SELECT registrar_cl_id AS ac_cl_id, string_agg(tld_name, ',') as tld_list FROM accreditations GROUP BY registrar_cl_id) ac ON ac.ac_cl_id = registrars.cl_id")

	// Add filters if provided
	var filter queries.ListRegistrarsFilter
	if params.Filter != nil {
		var ok bool
		filter, ok = params.Filter.(queries.ListRegistrarsFilter)
		if !ok {
			return nil, "", ErrInvalidFilterType
		} else {
			var err error
			dbQuery, err = setRegistrarFilters(dbQuery, filter)
			if err != nil {
				return nil, "", err
			}
		}
	}

	// Filter by TLD accreditation if specified
	if filter.TLD != "" {
		dbQuery = dbQuery.Where("registrars.cl_id IN (SELECT registrar_cl_id FROM accreditations WHERE tld_name = ?)", filter.TLD)
	}

	// Apply cursor pagination and sorting
	if filter.SortBy == "domain_count" {
		if params.PageCursor != "" && strings.Contains(params.PageCursor, "|") {
			parts := strings.Split(params.PageCursor, "|")
			if len(parts) == 2 {
				cursorDomainCount, _ := strconv.Atoi(parts[0])
				cursorClID := parts[1]
				if filter.SortOrder == "asc" {
					dbQuery = dbQuery.Where(
						"COALESCE(dc.domain_count, 0) > ? OR (COALESCE(dc.domain_count, 0) = ? AND registrars.cl_id > ?)",
						cursorDomainCount, cursorDomainCount, cursorClID,
					)
				} else { // default desc
					dbQuery = dbQuery.Where(
						"COALESCE(dc.domain_count, 0) < ? OR (COALESCE(dc.domain_count, 0) = ? AND registrars.cl_id > ?)",
						cursorDomainCount, cursorDomainCount, cursorClID,
					)
				}
			}
		}

		if filter.SortOrder == "asc" {
			dbQuery = dbQuery.Order("domain_count ASC, registrars.cl_id ASC")
		} else {
			dbQuery = dbQuery.Order("domain_count DESC, registrars.cl_id ASC")
		}
	} else {
		// Default sorting by Client ID
		if params.PageCursor != "" {
			dbQuery = dbQuery.Where("registrars.cl_id > ?", params.PageCursor)
		}
		dbQuery = dbQuery.Order("registrars.cl_id ASC")
	}

	// Limit the number of results
	dbQuery = dbQuery.Limit(params.PageSize + 1) // Fetch one more than the page size to determine if there is a next page

	// Execute the query
	var dbRars []*dbRegistrarListItem
	err := dbQuery.Scan(&dbRars).Error
	if err != nil {
		return nil, "", err
	}

	// Check if there is a next page
	hasMore := len(dbRars) == params.PageSize+1
	if hasMore {
		// Return up to the page size
		dbRars = dbRars[:params.PageSize]
	}

	// Convert the results to entities
	rarList := make([]*entities.RegistrarListItem, len(dbRars))
	for i, dbRar := range dbRars {
		var tlds []string
		if dbRar.TLDList != "" {
			tlds = strings.Split(dbRar.TLDList, ",")
		} else {
			tlds = []string{}
		}

		rarList[i] = &entities.RegistrarListItem{
			ClID:        entities.ClIDType(dbRar.ClID),
			Name:        dbRar.Name,
			GurID:       dbRar.GurID,
			Status:      entities.RegistrarStatus(dbRar.Status),
			IANAStatus:  entities.IANARegistrarStatus(dbRar.IANAStatus),
			Autorenew:   dbRar.Autorenew,
			DomainCount: dbRar.DomainCount,
			TLDCount:    len(tlds),
			TLDList:     tlds,
		}
	}

	// Set the cursor to the last item in the list
	var lastID string
	if hasMore && len(rarList) > 0 {
		lastRar := rarList[len(rarList)-1]
		if filter.SortBy == "domain_count" {
			lastID = fmt.Sprintf("%d|%s", lastRar.DomainCount, lastRar.ClID.String())
		} else {
			lastID = lastRar.ClID.String()
		}
	}

	return rarList, lastID, nil
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

// setRegistrarFilters applies the provided filters to the query
func setRegistrarFilters(dbQuery *gorm.DB, filter queries.ListRegistrarsFilter) (*gorm.DB, error) {
	if filter.ClidLike != "" {
		dbQuery = dbQuery.Where("cl_id ILIKE ?", "%"+filter.ClidLike+"%")
	}
	if filter.NameLike != "" {
		dbQuery = dbQuery.Where("name ILIKE ?", "%"+filter.NameLike+"%")
	}
	if filter.NickNameLike != "" {
		dbQuery = dbQuery.Where("nick_name ILIKE ?", "%"+filter.NickNameLike+"%")
	}
	if filter.GuridEquals != 0 {
		dbQuery = dbQuery.Where("gur_id = ?", filter.GuridEquals)
	}
	if filter.EmailLike != "" {
		dbQuery = dbQuery.Where("email ILIKE ?", "%"+filter.EmailLike+"%")
	}
	if filter.StatusEquals != "" {
		dbQuery = dbQuery.Where("status = ?", filter.StatusEquals)
	}
	if filter.IANAStatusEquals != "" {
		dbQuery = dbQuery.Where("iana_status = ?", filter.IANAStatusEquals)
	}
	if filter.AutorenewEquals != "" {
		dbQuery = dbQuery.Where("autorenew = ?", filter.AutorenewEquals)
	}

	return dbQuery, nil
}
