package postgres

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"gorm.io/gorm"
)

// RegistryOperatorRepository implements the RegistryOperatorRepository interface
type RegistryOperatorRepository struct {
	db *gorm.DB
}

// NewGORMRegistryOperatorRepository creates a new RegistryOperatorRepository
func NewGORMRegistryOperatorRepository(db *gorm.DB) *RegistryOperatorRepository {
	return &RegistryOperatorRepository{db: db}
}

// Create creates a new RegistryOperator in the database
func (r *RegistryOperatorRepository) Create(ctx context.Context, ro *entities.RegistryOperator) (*entities.RegistryOperator, error) {
	dbRO := &RegistryOperator{}
	dbRO.FromEntity(ro)
	if err := r.db.WithContext(ctx).Create(dbRO).Error; err != nil {
		return nil, err
	}
	return dbRO.ToEntity(), nil
}

// GetByRyID retrieves a RegistryOperator by its RyID
func (r *RegistryOperatorRepository) GetByRyID(ctx context.Context, ryID string) (*entities.RegistryOperator, error) {
	dbRO := &RegistryOperator{}
	err := r.db.WithContext(ctx).Where("ry_id = ?", ryID).First(dbRO).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, entities.ErrRegistryOperatorNotFound
		}
		return nil, err
	}

	return dbRO.ToEntity(), nil
}

// Update updates a RegistryOperator in the database
func (r *RegistryOperatorRepository) Update(ctx context.Context, ro *entities.RegistryOperator) (*entities.RegistryOperator, error) {
	dbRO := &RegistryOperator{}
	dbRO.FromEntity(ro)
	if err := r.db.WithContext(ctx).Save(dbRO).Error; err != nil {
		return nil, err
	}
	return dbRO.ToEntity(), nil
}

// DeleteByRyID deletes a RegistryOperator by its RyID
func (r *RegistryOperatorRepository) DeleteByRyID(ctx context.Context, ryID string) error {
	return r.db.WithContext(ctx).Where("ry_id = ?", ryID).Delete(&RegistryOperator{}).Error
}

// List retrieves RegistryOperators from the database
func (r *RegistryOperatorRepository) List(ctx context.Context, params queries.ListItemsQuery) ([]*entities.RegistryOperator, string, error) {
	// Get a query object ordering by PK
	dbQuery := r.db.WithContext(ctx).Order("ry_id ASC")

	// Add cursor pagination if a cursor is provided
	if params.PageCursor != "" {
		dbQuery = dbQuery.Where("ry_id > ?", params.PageCursor)
	}

	// Add filters if provided
	if params.Filter != nil {
		filter, ok := params.Filter.(queries.ListRegistryOperatorsFilter)
		if !ok {
			return nil, "", ErrInvalidFilterType
		}
		if filter.NameLike != "" {
			dbQuery = dbQuery.Where("name LIKE ?", "%"+filter.NameLike+"%")
		}
		if filter.RyidLike != "" {
			dbQuery = dbQuery.Where("ry_id LIKE ?", "%"+filter.RyidLike+"%")
		}
		if filter.EmailLike != "" {
			dbQuery = dbQuery.Where("email LIKE ?", "%"+filter.EmailLike+"%")
		}
	}

	// Limit the number of results
	dbQuery = dbQuery.Limit(params.PageSize + 1) // Fetch one more than the page size to determine if there is a next page

	// Execute the query
	dbRos := []*RegistryOperator{}
	err := dbQuery.Find(&dbRos).Error
	if err != nil {
		return nil, "", err
	}

	// Check if there is a next page
	hasMore := len(dbRos) == params.PageSize+1
	if hasMore {
		// Return only up to Pagesize
		dbRos = dbRos[:params.PageSize]
	}

	// Map the DBROs to ROs
	ros := make([]*entities.RegistryOperator, len(dbRos))
	for i, dbRo := range dbRos {
		ros[i] = dbRo.ToEntity()
	}

	// Set the cursor to the last ry_id in the list
	var lastID string
	if hasMore {
		lastID = ros[len(ros)-1].RyID.String()
	}

	return ros, lastID, nil
}
