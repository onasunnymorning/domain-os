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
