package postgres

import (
	"context"
	"errors"
	"strings"

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
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return nil, errors.Join(entities.ErrContactAlreadyExists, err)
		}
		return nil, err
	}

	return FromDBContact(dbContact), nil
}

// GetContactByID retrieves a contact from the database by its ID
func (r *ContactRepository) GetContactByID(ctx context.Context, id string) (*entities.Contact, error) {
	dbContact := &Contact{}
	err := r.db.WithContext(ctx).Where("id = ?", id).First(dbContact).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.Join(entities.ErrContactNotFound, err)
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
