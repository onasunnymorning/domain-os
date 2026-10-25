package postgres

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// migrateLegacyPendingRestores brings the restores requested before the RGP
// restore (EPP Phase 4i-2) into its model. The previous release stored a
// requested restore as pendingRestore with pendingDelete cleared and no
// deadline, and completed it in RestoreWorkflow without a report; that
// completion is gone. Such a domain becomes pendingDelete again, as RFC 3915
// has a domain pending restore (entities.Domain.RequestRestore), with its
// report due the default pending restore period after this migration: the
// registrar sends the report, or the restore lapses and the domain returns
// to redemption. Its redemption period end and purge date are kept.
//
// A row is one to migrate while its deadline is unset: NULL, as AutoMigrate
// adds the column, or the zero time. A rerun reads nothing.
func migrateLegacyPendingRestores(db *gorm.DB) error {
	if !db.Migrator().HasTable("domains") {
		return nil
	}
	// Plain SQL with the times as parameters, so the tests that migrate an
	// SQLite database run it too.
	due := time.Now().UTC().AddDate(0, 0, entities.PendingRestorePeriod)
	unset := time.Date(2, 1, 1, 0, 0, 0, 0, time.UTC)
	err := db.Exec(`UPDATE domains SET pending_delete = true, ok = false, pending_restore_end = ?
		WHERE pending_restore = true AND (pending_restore_end IS NULL OR pending_restore_end < ?)`,
		due, unset).Error
	if err != nil {
		return fmt.Errorf("pending restore migration: %w", err)
	}
	return nil
}
