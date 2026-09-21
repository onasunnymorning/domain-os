package postgres

import (
	"context"
	"fmt"
	"sort"

	"gorm.io/gorm"
)

// NameUnderTLDConstraints names the CHECK constraints that tie a name to its
// TLD, by table. Exported so the validation tool and its tests do not restate
// them.
//
// Why the database and not the application layer: nothing below that layer
// enforced the rule until #415, when an escrow deposit whose names ended in
// ".paco" was imported as TLD "gza". The upsert was keyed on the globally
// unique name, so 3,007 domains moved into another TLD, and every TLD-filtered
// query, the zone build and TLD cleanup then treated them as gza's. name and
// tld_name are redundant state, and this is the one place that stops the two
// drifting apart for every writer at once — including the go-pg bulk importer,
// which never builds an entity and so never passes through Domain.Validate.
//
// TLD is also the operator boundary (ADR-0006: RegistryOperator.RyID ->
// TLD.RyID -> Domain.TLDName), so a name that drifts across TLDs is a row that
// has drifted across tenants.
var NameUnderTLDConstraints = map[string]string{
	"domains": "ck_domains_name_under_tld",
	"nndns":   "ck_nndns_name_under_tld",
}

// nameUnderTLDCheck is the rule itself: a name is either the TLD (an apex
// record) or one label directly beneath it — which is what both NewDomain and
// NewNNDN derive with ParentDomain(), so the database and the entities agree.
//
// "One label beneath", not merely "ends with the TLD": where a registry runs
// both `uk` and `co.uk` as TLDs, x.co.uk filed under `uk` is cross-TLD drift
// between two operators, and the foreign key alone would allow it.
//
// right()/length() rather than LIKE '%.' || tld_name: in a LIKE pattern the
// TLD's own characters are interpreted, and '_' is a single-character wildcard.
// Measured cost of the whole expression is ~0.25µs per row, against 5-20µs for
// the insert it rides along with.
const nameUnderTLDCheck = `tld_name <> '' AND (
	name = tld_name
	OR (
		length(name) > length(tld_name) + 1
		AND right(name, length(tld_name) + 1) = '.' || tld_name
		AND strpos(left(name, length(name) - length(tld_name) - 1), '.') = 0
	)
)`

// nameUnderTLDConstraints builds the ADD CONSTRAINT statements.
//
// NOT VALID, deliberately: the constraint enforces every INSERT and UPDATE from
// the moment it exists, which is the whole point, but Postgres then skips the
// full-table scan over the rows already there. A deployment against a database
// that has already drifted starts and holds the line, rather than blocking on a
// table lock at boot. Validating the existing rows is a separate, explicit step
// — see `make db-validate-constraints`.
//
// The pg_constraint check keeps the statement off the table on the common path
// (ALTER TABLE takes an ACCESS EXCLUSIVE lock even when it is about to fail),
// and the exception handler covers the race: every API and worker pod runs
// AutoMigrate at boot, so two of them can pass the check at the same time.
//
// One consequence of checking by name: changing nameUnderTLDCheck does NOT
// update a constraint that already exists. Tightening or loosening the rule
// means adding a DROP CONSTRAINT to the legacy-cleanup list in connection.go,
// the same way a replaced index is dropped there.
func nameUnderTLDConstraints() []string {
	tables := make([]string, 0, len(NameUnderTLDConstraints))
	for table := range NameUnderTLDConstraints {
		tables = append(tables, table)
	}
	sort.Strings(tables) // deterministic order, so a failure is reproducible

	stmts := make([]string, 0, len(tables))
	for _, table := range tables {
		// #nosec G201 -- the table and constraint names are keys of a package-level
		// literal map, never input; the rule itself is a constant.
		stmts = append(stmts, fmt.Sprintf(`
DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1 FROM pg_constraint
		WHERE conrelid = '%[1]s'::regclass AND conname = '%[2]s'
	) THEN
		ALTER TABLE %[1]s ADD CONSTRAINT %[2]s CHECK (%[3]s) NOT VALID;
	END IF;
EXCEPTION
	WHEN duplicate_object THEN NULL;
END $$;`, table, NameUnderTLDConstraints[table], nameUnderTLDCheck))
	}
	return stmts
}

// ConstraintStatus is one table's standing against its name/tld_name CHECK.
type ConstraintStatus struct {
	Table      string
	Constraint string
	Present    bool
	Validated  bool
	Violations int64
	// Samples holds up to ten offending rows as "name -> tld_name", so an
	// operator can see what drifted without writing the query themselves.
	Samples []string
}

// ValidateNameUnderTLDConstraints reports each table's standing and, where the
// table is clean, promotes its constraint from NOT VALID to validated.
//
// This is deliberately not part of AutoMigrate. VALIDATE CONSTRAINT takes a
// SHARE UPDATE EXCLUSIVE lock, which collides with the DDL other pods run at
// boot, and on a cold cache at a hundred million rows it is I/O-bound. It is
// cheap when it is cheap — ~123ms over 623k rows on the dev database — but it
// belongs in an operator's hands, not on the startup path. Reads and writes are
// never blocked while it runs.
//
// A table with violations is left NOT VALID and reported: the constraint is
// still enforcing every new write, so the drift cannot spread while it is
// repaired.
func ValidateNameUnderTLDConstraints(ctx context.Context, db *gorm.DB) ([]ConstraintStatus, error) {
	tables := make([]string, 0, len(NameUnderTLDConstraints))
	for table := range NameUnderTLDConstraints {
		tables = append(tables, table)
	}
	sort.Strings(tables)

	out := make([]ConstraintStatus, 0, len(tables))
	for _, table := range tables {
		st := ConstraintStatus{Table: table, Constraint: NameUnderTLDConstraints[table]}

		var found struct {
			Convalidated bool
		}
		err := db.WithContext(ctx).Raw(
			`SELECT convalidated FROM pg_constraint WHERE conrelid = ?::regclass AND conname = ?`,
			table, st.Constraint,
		).Scan(&found).Error
		if err != nil {
			return out, fmt.Errorf("%s: reading constraint state: %w", table, err)
		}
		// Scan leaves the zero value when no row matched, so ask again rather
		// than reading "absent" as "present but not validated".
		var present int64
		if err := db.WithContext(ctx).Raw(
			`SELECT count(*) FROM pg_constraint WHERE conrelid = ?::regclass AND conname = ?`,
			table, st.Constraint,
		).Scan(&present).Error; err != nil {
			return out, fmt.Errorf("%s: reading constraint state: %w", table, err)
		}
		st.Present, st.Validated = present > 0, found.Convalidated

		if !st.Present {
			out = append(out, st)
			continue
		}
		if st.Validated {
			out = append(out, st)
			continue
		}

		// #nosec G202 -- nameUnderTLDCheck is a package constant and table is a
		// key of the package-level map, never input.
		countQ := `SELECT count(*) FROM ` + table + ` WHERE NOT (` + nameUnderTLDCheck + `)`
		if err := db.WithContext(ctx).Raw(countQ).Scan(&st.Violations).Error; err != nil {
			return out, fmt.Errorf("%s: counting violations: %w", table, err)
		}

		if st.Violations > 0 {
			// #nosec G202 -- as above.
			sampleQ := `SELECT name || ' -> ' || tld_name FROM ` + table +
				` WHERE NOT (` + nameUnderTLDCheck + `) ORDER BY name LIMIT 10`
			if err := db.WithContext(ctx).Raw(sampleQ).Scan(&st.Samples).Error; err != nil {
				return out, fmt.Errorf("%s: sampling violations: %w", table, err)
			}
			out = append(out, st)
			continue
		}

		// #nosec G201 -- table and constraint names come from the package map.
		if err := db.WithContext(ctx).Exec(
			fmt.Sprintf(`ALTER TABLE %s VALIDATE CONSTRAINT %s`, table, st.Constraint),
		).Error; err != nil {
			return out, fmt.Errorf("%s: validating constraint: %w", table, err)
		}
		st.Validated = true
		out = append(out, st)
	}
	return out, nil
}
