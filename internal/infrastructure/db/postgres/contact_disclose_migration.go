package postgres

import (
	"fmt"

	"gorm.io/gorm"
)

// migrateContactDisclose moves a contact's disclosure flags into columns of
// their own. Until it, Contact embedded entities.ContactDisclose without a
// prefix: the flags for name, org, voice, fax and email shared the names of
// the contact's own columns, so only the address flags were stored, in
// addr_int and addr_loc.
//
// It runs once, before AutoMigrate creates the other disclose_ columns: on
// a database that has addr_int but no disclose_addr_int, it adds the two
// address columns and copies the stored flags in one transaction. The old
// columns stay, so a process of the previous release still running during a
// deploy can write a contact; a later release drops them.
func migrateContactDisclose(db *gorm.DB) error {
	m := db.Migrator()
	if !m.HasTable("contacts") || !m.HasColumn("contacts", "addr_int") || m.HasColumn("contacts", "disclose_addr_int") {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for _, stmt := range []string{
			"ALTER TABLE contacts ADD COLUMN disclose_addr_int boolean",
			"ALTER TABLE contacts ADD COLUMN disclose_addr_loc boolean",
			"UPDATE contacts SET disclose_addr_int = addr_int, disclose_addr_loc = addr_loc",
		} {
			if err := tx.Exec(stmt).Error; err != nil {
				return fmt.Errorf("contact disclose migration: %w", err)
			}
		}
		return nil
	})
}

// fillContactDisclose sets the disclose_ columns AutoMigrate added to
// existing contacts to false, not disclosed, where they are NULL. It is
// idempotent: every contact written since stores all nine flags, through
// GORM and, with use_zero, through go-pg.
func fillContactDisclose(db *gorm.DB) error {
	for _, col := range []string{"name_int", "name_loc", "org_int", "org_loc", "addr_int", "addr_loc", "voice", "fax", "email"} {
		if err := db.Exec("UPDATE contacts SET disclose_" + col + " = false WHERE disclose_" + col + " IS NULL").Error; err != nil {
			return fmt.Errorf("contact disclose migration: %w", err)
		}
	}
	return nil
}
