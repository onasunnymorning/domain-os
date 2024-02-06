package postgres

import (
	"context"
	"time"

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
func (r *Spec5Repository) UpdateAll(labels []*entities.Spec5Label) error {
	// Drop all records from the spec5_labels table
	err := r.db.Exec("DELETE FROM spec5_labels").Error
	if err != nil {
		return err
	}

	// Convert to our DB model
	dbLabels := make([]*Spec5Label, len(labels))
	for i, label := range labels {
		dbLabels[i] = ToDBSpec5Label(label)
	}

	// Insert all records into the spec5_labels table
	return r.db.Create(&dbLabels).Error
}

// ListAll returns all Spec5Labels in the database
func (r *Spec5Repository) List(ctx context.Context, pageSize int, pageCursor string) ([]*entities.Spec5Label, error) {
	var dbLabels []*Spec5Label
	err := r.db.Order("label ASC").Limit(pageSize).Find(&dbLabels, "label > ?", pageCursor).Error
	if err != nil {
		return nil, err
	}

	labels := make([]*entities.Spec5Label, len(dbLabels))
	for i, dbLabel := range dbLabels {
		labels[i] = ToSpec5Label(dbLabel)
	}

	return labels, nil
}
