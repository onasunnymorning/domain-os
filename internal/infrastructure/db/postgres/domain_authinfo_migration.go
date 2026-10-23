package postgres

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// legacyAuthInfoBatch is how many domains one pass of hashLegacyAuthInfo
// reads.
const legacyAuthInfoBatch = 1000

// migrateDomainAuthInfo brings the domains' transfer codes to RFC 9154
// storage, once and idempotently, in the migrate job (ADR 0010):
//   - auth_info becomes nullable: no code is NULL (RFC 9154 §4.3), and an
//     empty string is turned into NULL;
//   - every code still stored in plain text is replaced by its salted hash.
//
// A code already in the hash's form is left alone, so a rerun changes
// nothing. The domains' versions (updated_at) are not advanced: the codes
// match as they did.
//
// Deploying it is a cutover, not a rolling deploy (ADR 0010, "Destructive
// changes"): releases before it read and write plain-text codes, and the
// codes cannot be recovered from their hashes. Stop every process that
// writes domains (admin API, EPP, worker, MCP) before the migrate job runs,
// start only this release or later after it, and do not roll back past it.
// A plain-text code written after the job ran would make that domain
// unreadable until the job is run again, which hashes it.
//
// Its statements bind codes as parameters, so they run with a logger that
// never prints parameter values: GORM's default logger prints them in
// slow-query and error traces.
func migrateDomainAuthInfo(db *gorm.DB) error {
	if !db.Migrator().HasTable("domains") {
		return nil
	}
	db = db.Session(&gorm.Session{Logger: logger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), logger.Config{
		SlowThreshold:        200 * time.Millisecond,
		LogLevel:             logger.Warn,
		ParameterizedQueries: true,
	})})
	if db.Dialector.Name() == "postgres" {
		if err := db.Exec("ALTER TABLE domains ALTER COLUMN auth_info DROP NOT NULL").Error; err != nil {
			return fmt.Errorf("domain authInfo migration: %w", err)
		}
	}
	if err := db.Exec("UPDATE domains SET auth_info = NULL WHERE auth_info = ''").Error; err != nil {
		return fmt.Errorf("domain authInfo migration: %w", err)
	}
	return hashLegacyAuthInfo(db)
}

// hashLegacyAuthInfo replaces every plain-text code with its hash, a batch
// at a time, in ROID order. The query narrows the rows to values not shaped
// like a hash (its prefix and length), so a rerun reads nothing; each row is
// then checked as a hash in full, since a code stored before RFC 9154 may
// itself begin with the hash's prefix.
func hashLegacyAuthInfo(db *gorm.DB) error {
	var after int64
	for {
		var rows []struct {
			RoID     int64
			AuthInfo string
		}
		if err := db.Raw(`SELECT ro_id, auth_info FROM domains
			WHERE auth_info IS NOT NULL AND (auth_info NOT LIKE '$sha256$%' OR length(auth_info) <> ?) AND ro_id > ?
			ORDER BY ro_id LIMIT ?`, entities.AuthInfoHashLength, after, legacyAuthInfoBatch).Scan(&rows).Error; err != nil {
			return fmt.Errorf("domain authInfo migration: %w", err)
		}
		if len(rows) == 0 {
			return nil
		}
		err := db.Transaction(func(tx *gorm.DB) error {
			for _, r := range rows {
				if entities.IsAuthInfoHash(r.AuthInfo) {
					continue
				}
				h, err := entities.HashAuthInfo(r.AuthInfo)
				if err != nil {
					return err
				}
				// Only a value still in plain text is replaced: a code a
				// registrar set since the read is already a hash.
				if err := tx.Exec("UPDATE domains SET auth_info = ? WHERE ro_id = ? AND auth_info = ?", string(h), r.RoID, r.AuthInfo).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("domain authInfo migration: %w", err)
		}
		after = rows[len(rows)-1].RoID
	}
}
