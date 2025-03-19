package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"gorm.io/gorm"
)

// PremiumListRepository implements the PremiumListRepository interface
type PremiumLabelRepository struct {
	db *gorm.DB
}

// NewPremiumListRepository creates a new PremiumListRepository instance
func NewGORMPremiumLabelRepository(db *gorm.DB) *PremiumLabelRepository {
	return &PremiumLabelRepository{
		db: db,
	}
}

// Create creates a new premium list in the database
func (plr *PremiumLabelRepository) Create(ctx context.Context, premiumLabel *entities.PremiumLabel) (*entities.PremiumLabel, error) {
	pl := FromEntity(premiumLabel)
	err := plr.db.WithContext(ctx).Create(pl).Error
	if err != nil {
		return nil, err
	}
	return pl.ToEntity(), nil
}

// GetByLabelListAndCurrency retrieves a premium label by label, list, and currency
func (plr *PremiumLabelRepository) GetByLabelListAndCurrency(ctx context.Context, label, list, currency string) (*entities.PremiumLabel, error) {
	pl := &PremiumLabel{}
	if err := plr.db.WithContext(ctx).Where("label = ? AND premium_list_name = ? AND currency = ?", label, list, strings.ToUpper(currency)).First(pl).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entities.ErrPremiumLabelNotFound
		}
		return nil, err
	}
	return pl.ToEntity(), nil
}

// DeleteByLabelListAndCurrency deletes a premium label by label, list, and currency
func (plr *PremiumLabelRepository) DeleteByLabelListAndCurrency(ctx context.Context, label, list, currency string) error {
	return plr.db.WithContext(ctx).Where("label = ? AND premium_list_name = ? AND currency = ?", label, list, currency).Delete(&PremiumLabel{}).Error
}

// List retrieves a list of premium labels
func (plr *PremiumLabelRepository) List(ctx context.Context, params queries.ListItemsQuery) ([]*entities.PremiumLabel, string, error) {
	// Create a query object ordering by label (PK used for cursor pagination)
	dbQuery := plr.db.WithContext(ctx).Order("id ASC")

	// Add cursor pagination if a cursor is provided
	if params.PageCursor != "" {
		cursorInt64, err := strconv.ParseInt(params.PageCursor, 10, 64)
		if err != nil {
			return nil, "", err
		}
		dbQuery = dbQuery.Where("id > ?", cursorInt64)
	}

	// Add Filters if provided
	if params.Filter != nil {
		var err error
		if filter, ok := params.Filter.(queries.ListPremiumLabelsFilter); !ok {
			return nil, "", ErrInvalidFilterType
		} else {
			dbQuery, err = setPremiumLabelFilters(dbQuery, filter)
			if err != nil {
				return nil, "", err
			}
		}
	}

	// Limit the number of results
	dbQuery = dbQuery.Limit(params.PageSize + 1) // Fetch one more than the limit to determine if there are more results

	// Execute the query
	dbpls := []*PremiumLabel{}
	err := dbQuery.Find(&dbpls).Error
	if err != nil {
		return nil, "", err
	}

	// Check if there are more results
	hasMore := len(dbpls) == params.PageSize+1
	if hasMore {
		// Return up to the pagesize
		dbpls = dbpls[:params.PageSize]
	}

	// Convert the results to entities
	pls := make([]*entities.PremiumLabel, len(dbpls))
	for i, dbpl := range dbpls {
		pls[i] = dbpl.ToEntity()
	}

	// Set cursor to the last label in the list if there are more results
	var cursor string
	if hasMore {
		cursor = fmt.Sprintf("%d", dbpls[len(dbpls)-1].ID)
	}

	// Return the results
	return pls, cursor, nil
}

func setPremiumLabelFilters(dbQuery *gorm.DB, filter queries.ListPremiumLabelsFilter) (*gorm.DB, error) {
	if filter.LabelLike != "" {
		dbQuery = dbQuery.Where("label ILIKE ?", "%"+filter.LabelLike+"%")
	}
	if filter.PremiumListNameEquals != "" {
		dbQuery = dbQuery.Where("premium_list_name = ?", filter.PremiumListNameEquals)
	}
	if filter.CurrencyEquals != "" {
		dbQuery = dbQuery.Where("currency = ?", strings.ToUpper(filter.CurrencyEquals))
	}
	if filter.ClassEquals != "" {
		dbQuery = dbQuery.Where("class = ?", filter.ClassEquals)
	}
	if filter.RegistrationAmountEquals != "" {
		dbQuery = dbQuery.Where("registration_amount = ?", filter.RegistrationAmountEquals)
	}
	if filter.RenewalAmountEquals != "" {
		dbQuery = dbQuery.Where("renewal_amount = ?", filter.RenewalAmountEquals)
	}
	if filter.TransferAmountEquals != "" {
		dbQuery = dbQuery.Where("transfer_amount = ?", filter.TransferAmountEquals)
	}
	if filter.RestoreAmountEquals != "" {
		dbQuery = dbQuery.Where("restore_amount = ?", filter.RestoreAmountEquals)
	}

	return dbQuery, nil
}
