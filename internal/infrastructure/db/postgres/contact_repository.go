package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"gorm.io/gorm"
)

// ContactRepository implements the ContactRepository interface
type ContactRepository struct {
	db *gorm.DB
}

// NewContactRepository creates a new ContactRepository
func NewContactRepository(db *gorm.DB) *ContactRepository {
	return &ContactRepository{db}
}

// CreateContact creates a new contact
func (r *ContactRepository) CreateContact(ctx context.Context, c *entities.Contact) (*entities.Contact, error) {
	dbContact := ToDBContact(c)

	err := r.db.WithContext(ctx).Create(dbContact).Error
	if err != nil {
		var perr *pgconn.PgError
		if errors.As(err, &perr) && perr.Code == "23505" {
			return nil, errors.Join(entities.ErrContactAlreadyExists, err)
		}
		return nil, err
	}

	return FromDBContact(dbContact), nil
}

// BulkCreate creates multiple contacts at once
func (r *ContactRepository) BulkCreate(ctx context.Context, contacts []*entities.Contact) error {
	// convert entities to db entities
	dbContacts := make([]*Contact, len(contacts))
	for i, c := range contacts {
		dbContacts[i] = ToDBContact(c)
	}

	return r.db.WithContext(ctx).Create(dbContacts).Error
}

// GetContactByID retrieves a contact from the database by its ID
func (r *ContactRepository) GetContactByID(ctx context.Context, id string) (*entities.Contact, error) {
	dbContact := &Contact{}
	err := r.db.WithContext(ctx).Where("id = ?", id).First(dbContact).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entities.ErrContactNotFound
		}
		return nil, err
	}

	return FromDBContact(dbContact), nil
}

// UpdateContact updates a contact in the database
func (r *ContactRepository) UpdateContact(ctx context.Context, c *entities.Contact) (*entities.Contact, error) {
	dbContact := ToDBContact(c)

	err := r.db.WithContext(ctx).Save(dbContact).Error
	if err != nil {
		return nil, err
	}

	return FromDBContact(dbContact), nil
}

// DeleteContactByID deletes a contact from the database
func (r *ContactRepository) DeleteContactByID(ctx context.Context, id string) error {
	err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&Contact{}).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.Join(entities.ErrContactNotFound, err)
		}
		return err
	}
	return nil
}

// ListContacts returns a list of contacts
func (r *ContactRepository) ListContacts(ctx context.Context, params queries.ListItemsQuery) ([]*entities.Contact, string, error) {
	// Create a query object
	dbQuery := r.db.WithContext(ctx).Order("ro_id ASC")

	// Add cursor pagination if a cursor is provided
	var roidInt int64
	var err error
	if params.PageCursor != "" {
		roidInt, err = getInt64RoidFromContactRoidString(params.PageCursor)
		if err != nil {
			return nil, "", fmt.Errorf("invalid cursor: %w", err)
		}

		dbQuery = dbQuery.Where("ro_id > ?", roidInt)
	}

	// Add filters
	if params.Filter != nil {
		if f, ok := params.Filter.(queries.ListContactsFilter); !ok {
			return nil, "", ErrInvalidFilterType
		} else {
			if f.RoidGreaterThan != "" {
				roidInt, err := getInt64RoidFromContactRoidString(f.RoidGreaterThan)
				if err != nil {
					return nil, "", err
				}
				dbQuery = dbQuery.Where("ro_id > ?", roidInt)
			}
			if f.RoidLessThan != "" {
				roidInt, err := getInt64RoidFromContactRoidString(f.RoidGreaterThan)
				if err != nil {
					return nil, "", err
				}
				dbQuery = dbQuery.Where("ro_id < ?", roidInt)
			}
			if f.IdLike != "" {
				dbQuery = dbQuery.Where("id ILIKE ?", "%"+f.IdLike+"%")
			}
			if f.EmailLike != "" {
				dbQuery = dbQuery.Where("email ILIKE ?", "%"+f.EmailLike+"%")
			}
			if f.ClidEquals != "" {
				dbQuery = dbQuery.Where("cl_id = ?", f.ClidEquals)
			}
		}
	}

	// Limit the number of results
	dbQuery = dbQuery.Limit(params.PageSize + 1) // We fetch one more than the page size to determine if there are more results

	// Execute the query
	dbContacts := []*Contact{}
	err = dbQuery.Find(&dbContacts).Error
	if err != nil {
		return nil, "", err
	}

	// Check if there is a next page
	hasMore := len(dbContacts) == params.PageSize+1
	if hasMore {
		// Return only up to PageSize
		dbContacts = dbContacts[:params.PageSize]
	}

	// Map the results to entities
	contacts := make([]*entities.Contact, len(dbContacts))
	for i, c := range dbContacts {
		contacts[i] = FromDBContact(c)
	}

	// Set the next page cursor
	var nextPageCursor string
	if hasMore {
		nextPageCursor = contacts[len(contacts)-1].RoID.String()
	}

	return contacts, nextPageCursor, nil
}

func getInt64RoidFromContactRoidString(roidString string) (int64, error) {
	// If the cursor is empty, we don't need to paginate, this is not an error
	if roidString == "" {
		return 0, nil
	}
	roid := entities.RoidType(roidString)
	if validationErr := roid.Validate(); validationErr != nil {
		return 0, validationErr
	}
	if roid.ObjectIdentifier() != entities.CONTACT_ROID_ID {
		return 0, entities.ErrInvalidRoid
	}
	return roid.Int64()
}
