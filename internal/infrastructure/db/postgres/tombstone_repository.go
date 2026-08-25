package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// DomainTombstoneRecord is the GORM representation of a DomainTombstone for database interaction.
type DomainTombstoneRecord struct {
	RoID          string     `gorm:"primaryKey;column:ro_id;type:text"`
	Name          string     `gorm:"index:idx_tombstones_name;not null"`
	UName         string
	TLDName       string     `gorm:"index;not null"`
	RegistrarClID string     `gorm:"index"`
	RegisteredAt  time.Time  `gorm:"not null"`
	ExpiredAt     *time.Time
	PurgedAt      time.Time  `gorm:"not null;index"`
	PurgeReason   string
	DropCatch     bool       `gorm:"default:false"`
	LastSnapshot  []byte     `gorm:"type:jsonb"`
	CreatedAt     time.Time
}

// TableName specifies the table name for DomainTombstoneRecord.
func (DomainTombstoneRecord) TableName() string {
	return "domain_tombstones"
}

// toTombstone converts a DomainTombstoneRecord to a domain model *entities.DomainTombstone.
func (r *DomainTombstoneRecord) toTombstone() *entities.DomainTombstone {
	t := &entities.DomainTombstone{
		RoID:          entities.RoidType(r.RoID),
		Name:          entities.DomainName(r.Name),
		UName:         entities.DomainName(r.UName),
		TLDName:       entities.DomainName(r.TLDName),
		RegistrarClID: r.RegistrarClID,
		RegisteredAt:  r.RegisteredAt,
		ExpiredAt:     r.ExpiredAt,
		PurgedAt:      r.PurgedAt,
		PurgeReason:   r.PurgeReason,
		DropCatch:     r.DropCatch,
		CreatedAt:     r.CreatedAt,
	}

	if len(r.LastSnapshot) > 0 {
		var snapshot interface{}
		if err := json.Unmarshal(r.LastSnapshot, &snapshot); err == nil {
			t.LastSnapshot = snapshot
		}
	}

	return t
}

// fromTombstone converts a domain model DomainTombstone to a DomainTombstoneRecord.
func fromTombstone(t *entities.DomainTombstone) *DomainTombstoneRecord {
	rec := &DomainTombstoneRecord{
		RoID:          string(t.RoID),
		Name:          t.Name.String(),
		UName:         t.UName.String(),
		TLDName:       t.TLDName.String(),
		RegistrarClID: t.RegistrarClID,
		RegisteredAt:  t.RegisteredAt,
		ExpiredAt:     t.ExpiredAt,
		PurgedAt:      t.PurgedAt,
		PurgeReason:   t.PurgeReason,
		DropCatch:     t.DropCatch,
		CreatedAt:     t.CreatedAt,
	}

	if t.LastSnapshot != nil {
		data, err := json.Marshal(t.LastSnapshot)
		if err == nil {
			rec.LastSnapshot = data
		}
	}

	return rec
}

// GormTombstoneRepository implements the TombstoneRepository interface.
type GormTombstoneRepository struct {
	db *gorm.DB
}

// NewGormTombstoneRepository returns a new GormTombstoneRepository.
func NewGormTombstoneRepository(db *gorm.DB) *GormTombstoneRepository {
	return &GormTombstoneRepository{
		db: db,
	}
}

func (r *GormTombstoneRepository) CreateTombstone(ctx context.Context, tombstone *entities.DomainTombstone) (*entities.DomainTombstone, error) {
	rec := fromTombstone(tombstone)
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(rec)
	if err := result.Error; err != nil {
		return nil, fmt.Errorf("CreateTombstone(roid=%s): %w", tombstone.RoID, err)
	}
	return rec.toTombstone(), nil
}

func (r *GormTombstoneRepository) GetTombstoneByRoID(ctx context.Context, roid string) (*entities.DomainTombstone, error) {
	var rec DomainTombstoneRecord
	result := r.db.WithContext(ctx).Where("ro_id = ?", roid).First(&rec)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, entities.ErrTombstoneNotFound
		}
		return nil, result.Error
	}
	return rec.toTombstone(), nil
}

func (r *GormTombstoneRepository) GetTombstonesByName(ctx context.Context, name string) ([]*entities.DomainTombstone, error) {
	var records []*DomainTombstoneRecord
	result := r.db.WithContext(ctx).Where("name = ?", strings.ToLower(name)).Order("purged_at DESC").Find(&records)
	if result.Error != nil {
		return nil, result.Error
	}

	tombstones := make([]*entities.DomainTombstone, len(records))
	for i, rec := range records {
		tombstones[i] = rec.toTombstone()
	}
	return tombstones, nil
}

func (r *GormTombstoneRepository) ListTombstones(ctx context.Context, params queries.ListItemsQuery) ([]*entities.DomainTombstone, string, error) {
	// Get a query object ordering by name (used for cursor pagination)
	dbQuery := r.db.WithContext(ctx).Order("name ASC")

	// Add cursor pagination if a cursor is provided
	if params.PageCursor != "" {
		dbQuery = dbQuery.Where("name > ?", params.PageCursor)
	}

	// Add filters if provided
	if params.Filter != nil {
		if filter, ok := params.Filter.(queries.ListTombstonesFilter); !ok {
			return nil, "", ErrInvalidFilterType
		} else {
			dbQuery = setTombstoneFilters(dbQuery, filter)
		}
	}

	// Limit the number of results — fetch one more to determine if there are more results
	dbQuery = dbQuery.Limit(params.PageSize + 1)

	// Execute the query
	var records []*DomainTombstoneRecord
	if err := dbQuery.Find(&records).Error; err != nil {
		return nil, "", err
	}

	// Check if there are more results
	hasMore := len(records) == params.PageSize+1
	if hasMore {
		records = records[:params.PageSize]
	}

	// Map records to domain entities
	tombstones := make([]*entities.DomainTombstone, len(records))
	for i, rec := range records {
		tombstones[i] = rec.toTombstone()
	}

	// Set the cursor to the last name in the list
	var newCursor string
	if hasMore {
		newCursor = tombstones[len(tombstones)-1].Name.String()
	}

	return tombstones, newCursor, nil
}

func (r *GormTombstoneRepository) CountTombstones(ctx context.Context, filter queries.ListTombstonesFilter) (int64, error) {
	dbQuery := r.db.WithContext(ctx).Model(&DomainTombstoneRecord{})
	dbQuery = setTombstoneFilters(dbQuery, filter)
	var count int64
	err := dbQuery.Count(&count).Error
	return count, err
}

func setTombstoneFilters(dbQuery *gorm.DB, filter queries.ListTombstonesFilter) *gorm.DB {
	if filter.NameLike != "" {
		dbQuery = dbQuery.Where("name ILIKE ?", "%"+filter.NameLike+"%")
	}
	if filter.NameEquals != "" {
		dbQuery = dbQuery.Where("name = ?", strings.ToLower(filter.NameEquals))
	}
	if filter.TLDEquals != "" {
		dbQuery = dbQuery.Where("tld_name = ?", strings.ToLower(filter.TLDEquals))
	}
	if filter.RegistrarClID != "" {
		dbQuery = dbQuery.Where("registrar_cl_id = ?", filter.RegistrarClID)
	}
	if filter.PurgeReason != "" {
		dbQuery = dbQuery.Where("purge_reason = ?", filter.PurgeReason)
	}
	return dbQuery
}
