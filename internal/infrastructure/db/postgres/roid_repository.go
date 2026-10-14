package postgres

import (
	"fmt"

	"gorm.io/gorm"
)

// preConfigRepositoryID is the repository ID every ROID was minted with
// before it became configuration (ADR 0017): the code constant "APEX".
// Only rows that predate the ro_id_repository column take it.
const preConfigRepositoryID = "APEX"

// roidRepositoryTables are the tables of objects with a ROID. Each stores the
// ROID's number in ro_id and its repository ID in ro_id_repository, so an
// object keeps the ROID it was minted with when the configured ID changes.
var roidRepositoryTables = []string{"contacts", "hosts", "domains"}

// addROIDRepositoryColumns adds ro_id_repository to tables that predate it,
// before AutoMigrate, which could not: the column is NOT NULL, and the rows
// already there need a value. They get preConfigRepositoryID, and the default
// is dropped again in the same transaction, so no writer that leaves the
// column out ever gets "APEX" silently. On Postgres 11 and later, adding a
// column with a constant default does not rewrite the table.
//
// A fresh database has none of these tables yet, and AutoMigrate creates the
// column with them. The information_schema check keeps the ALTER, and its
// ACCESS EXCLUSIVE lock, off every later boot; the exception handler covers
// two migrators passing the check at once.
func addROIDRepositoryColumns(db *gorm.DB) error {
	if db.Dialector.Name() != "postgres" {
		return nil
	}
	for _, table := range roidRepositoryTables {
		// #nosec G201 -- the table names are a package-level literal list and
		// the default a constant, never input.
		stmt := fmt.Sprintf(`
DO $$
BEGIN
	IF to_regclass('public.%[1]s') IS NOT NULL AND NOT EXISTS (
		SELECT 1 FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = '%[1]s' AND column_name = 'ro_id_repository'
	) THEN
		ALTER TABLE %[1]s ADD COLUMN ro_id_repository text NOT NULL DEFAULT '%[2]s';
		ALTER TABLE %[1]s ALTER COLUMN ro_id_repository DROP DEFAULT;
	END IF;
EXCEPTION
	WHEN duplicate_column THEN NULL;
END $$;`, table, preConfigRepositoryID)
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("add ro_id_repository to %s: %w", table, err)
		}
	}
	return nil
}
