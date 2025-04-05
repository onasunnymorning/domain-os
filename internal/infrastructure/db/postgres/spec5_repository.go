package postgres

import (
	"context"
	"strconv"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"gorm.io/gorm"
)

// Spec5Label is a struct representing an label blocked by RA Specification 5 in the database
type Spec5Label struct {
	Label     string `gorm:"primary_key"`
	Type      string
	CreatedAt time.Time
}

func (Spec5Label) TableName() string {
	return "spec5_labels"
}

// Spec5Repository implements the Spec5Repository interface
type Spec5Repository struct {
	db *gorm.DB
}

// NewSpec5Repository returns a new Spec5Repository
func NewSpec5Repository(db *gorm.DB) *Spec5Repository {
	return &Spec5Repository{
		db: db,
	}
}

// UpdateAll updates all Spec5Labels in the database
func (r *Spec5Repository) UpdateAll(ctx context.Context, labels []*entities.Spec5Label) error {
	// Drop all records from the spec5_labels table
	err := r.db.WithContext(ctx).Exec("DELETE FROM spec5_labels").Error
	if err != nil {
		return err
	}

	// Convert to our DB model
	dbLabels := make([]*Spec5Label, len(labels))
	for i, label := range labels {
		dbLabels[i] = ToDBSpec5Label(label)
	}

	// Insert all records into the spec5_labels table
	return r.db.WithContext(ctx).Create(&dbLabels).Error
}

// ListAll returns all Spec5Labels in the database
func (r *Spec5Repository) List(ctx context.Context, params queries.ListItemsQuery) ([]*entities.Spec5Label, string, error) {
	// Get a query object ordering by PK
	dbQuery := r.db.WithContext(ctx).Order("label ASC")

	// Add cursor pagination if a cursor is provided
	if params.PageCursor != "" {
		cursorInt64, err := strconv.ParseInt(params.PageCursor, 10, 64)
		if err != nil {
			return nil, "", err
		}

		dbQuery = dbQuery.Where("label > ?", cursorInt64)
	}
	// Add filters if provided
	if params.Filter != nil {
		var err error
		filter, ok := params.Filter.(queries.ListSpec5LabelsFilter)
		if !ok {
			return nil, "", ErrInvalidFilterType
		} else {
			dbQuery, err = setSpec5LabelFilters(dbQuery, filter)
			if err != nil {
				return nil, "", err
			}
		}
	}
	// Limit the number of results
	dbQuery = dbQuery.Limit(params.PageSize + 1) // Fetch one more than the page size to determine if there is a next page
	// Execute the query
	dbLabels := []*Spec5Label{}
	err := dbQuery.Find(&dbLabels).Error
	if err != nil {
		return nil, "", err
	}
	// Check if there is a next page
	hasMore := len(dbLabels) == params.PageSize+1
	if hasMore {
		// Return up to the page size
		dbLabels = dbLabels[:params.PageSize]
	}
	// Convert the results to entities
	labels := make([]*entities.Spec5Label, len(dbLabels))
	for i, dbLabel := range dbLabels {
		labels[i] = ToSpec5Label(dbLabel)
	}
	// Set the cursor to the last label if there are more results
	var cursor string
	if hasMore {
		cursor = dbLabels[len(dbLabels)-1].Label
	}
	return labels, cursor, nil
}

// setSpec5LabelFilters applies filters to the query
func setSpec5LabelFilters(dbQuery *gorm.DB, filter queries.ListSpec5LabelsFilter) (*gorm.DB, error) {
	if filter.LabelLike != "" {
		dbQuery = dbQuery.Where("label ILIKE ?", "%"+filter.LabelLike+"%")
	}
	if filter.TypeEquals != "" {
		dbQuery = dbQuery.Where("type = ?", filter.TypeEquals)
	}
	return dbQuery, nil
}
