package postgres

import (
	"context"

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
		return nil, err
	}

	return FromDBContact(dbContact), nil
}

// GetContactByID retrieves a contact from the database by its ID
func (r *ContactRepository) GetContactByID(ctx context.Context, id string) (*entities.Contact, error) {
	dbContact := &Contact{}
	err := r.db.WithContext(ctx).Where("id = ?", id).First(dbContact).Error
	if err != nil {
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

// DeleteContact deletes a contact from the database
func (r *ContactRepository) DeleteContact(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&Contact{}).Error
}
