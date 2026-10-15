package postgres

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// InstallationSetting is a setting the whole installation must agree on,
// across processes and over time. The first process to start records it.
type InstallationSetting struct {
	Key       string `gorm:"primaryKey"`
	Value     string `gorm:"not null"`
	CreatedAt time.Time
}

// TableName returns the table name for InstallationSetting
func (InstallationSetting) TableName() string {
	return "installation_settings"
}

const roidRepositoryIDKey = "roid_repository_id"

// CheckRepositoryID guards the installation's ROID repository ID (ADR 0017).
// Tables store only a ROID's number, and every read appends the configured
// ID, so a changed ID would silently rename every object. The first process
// to start records its ID; from then on a process configured with a
// different one gets entities.ErrRepositoryIDChanged and must not start.
// Concurrent first starts are safe: only one insert wins, and the others
// compare against it.
func CheckRepositoryID(ctx context.Context, db *gorm.DB, repository entities.RepositoryID) error {
	if err := repository.Validate(); err != nil {
		return err
	}
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&InstallationSetting{
		Key: roidRepositoryIDKey, Value: repository.String(),
	}).Error; err != nil {
		return fmt.Errorf("record the ROID repository ID: %w", err)
	}
	var recorded InstallationSetting
	if err := db.WithContext(ctx).Where("key = ?", roidRepositoryIDKey).First(&recorded).Error; err != nil {
		return fmt.Errorf("read the ROID repository ID: %w", err)
	}
	if recorded.Value != repository.String() {
		return fmt.Errorf("%w: configured %q, recorded %q on %s", entities.ErrRepositoryIDChanged,
			repository, recorded.Value, recorded.CreatedAt.UTC().Format(time.DateOnly))
	}
	return nil
}
