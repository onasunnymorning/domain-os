package postgres

import (
	"fmt"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"gorm.io/gorm"
)

// migrateLaunchSettings gives the launch phases from before ADR 0024 the
// default launch settings (open, first come, signed marks accepted), the
// settings a launch phase created today starts with, so that every launch
// phase row validates. GA phases keep the zero value. Idempotent: a launch
// phase with a mode is left alone.
func migrateLaunchSettings(db *gorm.DB) error {
	if !db.Migrator().HasTable("phases") || !db.Migrator().HasColumn("phases", "launch_mode") {
		return nil
	}
	defaults := entities.NewLaunchSettings()
	err := db.Exec(
		"UPDATE phases SET launch_mode = ?, launch_model = ?, launch_accept_signed_marks = ?, launch_accept_code_marks = ?, launch_retain_marks = ? WHERE type = ? AND launch_mode = ''",
		defaults.Mode.String(), defaults.Model.String(), defaults.AcceptSignedMarks, defaults.AcceptCodeMarks, defaults.RetainMarks,
		string(entities.PhaseTypeLaunch),
	).Error
	if err != nil {
		return fmt.Errorf("fill the launch phases' settings: %w", err)
	}
	return nil
}
