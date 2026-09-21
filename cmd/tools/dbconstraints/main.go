// Command dbconstraints reports, and where possible promotes, the CHECK
// constraints that tie a domain or NNDN name to its TLD.
//
// AutoMigrate adds those constraints NOT VALID, so they enforce every new write
// without scanning the table at boot. Promoting one to validated means Postgres
// has confirmed the rows already there obey it too — a full scan under a
// SHARE UPDATE EXCLUSIVE lock, which is an operator's decision to make, not
// something to run on every pod start. See ADR-0006 and issue #415.
//
// Exit codes: 0 all constraints present and validated, 1 something drifted or
// the run failed.
package main

import (
	"context"
	"fmt"
	"os"

	postgres "github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"gorm.io/gorm"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "dbconstraints: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	db, err := connect()
	if err != nil {
		return err
	}

	statuses, err := postgres.ValidateNameUnderTLDConstraints(context.Background(), db)
	if err != nil {
		return err
	}

	drifted := false
	for _, st := range statuses {
		switch {
		case !st.Present:
			drifted = true
			fmt.Printf("%-10s %-28s MISSING — run the API or a worker once to apply AutoMigrate\n", st.Table, st.Constraint)
		case st.Violations > 0:
			drifted = true
			fmt.Printf("%-10s %-28s %d row(s) violate the rule; the constraint stays NOT VALID and keeps enforcing new writes\n",
				st.Table, st.Constraint, st.Violations)
			for _, sample := range st.Samples {
				fmt.Printf("%-10s   %s\n", "", sample)
			}
		case st.Validated:
			fmt.Printf("%-10s %-28s validated\n", st.Table, st.Constraint)
		default:
			drifted = true
			fmt.Printf("%-10s %-28s present but not validated\n", st.Table, st.Constraint)
		}
	}
	if drifted {
		return fmt.Errorf("one or more constraints are missing or unvalidated")
	}
	return nil
}

// connect follows the DB idiom the workers use: DATABASE_URL when it is set,
// the DB_* variables otherwise. AutoMigrate is never run from here — this
// command reports on the schema, it does not change it.
func connect() (*gorm.DB, error) {
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		return postgres.NewConnectionFromURL(dbURL, false)
	}
	return postgres.NewConnection(postgres.Config{
		User:    os.Getenv("DB_USER"),
		Pass:    os.Getenv("DB_PASS"),
		Host:    os.Getenv("DB_HOST"),
		Port:    os.Getenv("DB_PORT"),
		DBName:  os.Getenv("DB_NAME"),
		SSLmode: os.Getenv("DB_SSLMODE"),
	})
}
