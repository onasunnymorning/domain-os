package postgres

import (
	"context"
	"strconv"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"gorm.io/gorm"
)

// IANARegistrar is a struct representing an IANA Registrar in the database
type IANARegistrar struct {
	GurID     int `gorm:"primary_key;auto_increment:false"`
	Name      string
	Status    string
	RdapURL   string
	CreatedAt time.Time
}

func (IANARegistrar) TableName() string {
	return "iana_registrars"
}

// IANARegistrarRepository implements the IANARegistrarRepository interface
type IANARegistrarRepository struct {
	db *gorm.DB
}

// NewIANARegistrarRepository returns a new IANARegistrarRepository
func NewIANARegistrarRepository(db *gorm.DB) *IANARegistrarRepository {
	return &IANARegistrarRepository{
		db: db,
	}
}

// UpdateAll updates all IANARegistrars in the database
func (r *IANARegistrarRepository) UpdateAll(ctx context.Context, registrars []*entities.IANARegistrar) error {
	// Drop all records from the iana_registrars table
	err := r.db.WithContext(ctx).Exec("DELETE FROM iana_registrars").Error
	if err != nil {
		return err
	}

	// Insert all records into the iana_registrars table
	return r.db.WithContext(ctx).Create(&registrars).Error
}

// ListAll returns all IANARegistrars in the database
func (r *IANARegistrarRepository) List(ctx context.Context, pageSize int, pageCursor, nameSearchString, status string) ([]*entities.IANARegistrar, error) {
	var dbRegistrars []*IANARegistrar
	// Convert pageCursor to int since we are dealing with an int column
	var pageCursorInt int
	var err error
	if pageCursor == "" {
		pageCursorInt = 0
	} else {
		pageCursorInt, err = strconv.Atoi(pageCursor)
		if err != nil {
			return nil, err
		}
	}
	// Get the next page of results
	if nameSearchString == "" {
		// If no nameSearchString, then just get the next page of results
		err = r.db.WithContext(ctx).Order("gur_id ASC").Limit(pageSize).Where(&IANARegistrar{Status: status}).Find(&dbRegistrars, "gur_id > ?", pageCursorInt).Error
		if err != nil {
			return nil, err
		}
	} else {
		// If there is a nameSearchString, then get the next page of results that match the nameSearchString using ILIKE (case insensitive)
		err = r.db.WithContext(ctx).Order("gur_id ASC").Limit(pageSize).Where(&IANARegistrar{Status: status}).Where("name ILIKE ?", "%"+nameSearchString+"%").Find(&dbRegistrars, "gur_id > ?", pageCursorInt).Error
		if err != nil {
			return nil, err
		}
	}

	// Convert to entities
	registrars := make([]*entities.IANARegistrar, len(dbRegistrars))
	for i, dbrar := range dbRegistrars {
		registrars[i] = ToIanaRegistrar(dbrar)
	}

	return registrars, nil
}

// GetByGurID Retrieves gets a IANARegistrar by GurID
func (r *IANARegistrarRepository) GetByGurID(ctx context.Context, gurID int) (*entities.IANARegistrar, error) {
	var dbRegistrar IANARegistrar
	err := r.db.WithContext(ctx).First(&dbRegistrar, "gur_id = ?", gurID).Error
	if err != nil {
		return nil, err
	}
	return ToIanaRegistrar(&dbRegistrar), nil
}

// Count returns the number of IANARegistrars in the database
func (r *IANARegistrarRepository) Count(ctx context.Context) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&IANARegistrar{}).Count(&count).Error
	if err != nil {
		return 0, err
	}
	return int(count), nil
}
