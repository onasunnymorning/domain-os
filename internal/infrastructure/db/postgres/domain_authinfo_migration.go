package postgres

import (
	"fmt"

	"gorm.io/gorm"

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
func migrateDomainAuthInfo(db *gorm.DB) error {
	if !db.Migrator().HasTable("domains") {
		return nil
	}
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
// at a time, in ROID order.
func hashLegacyAuthInfo(db *gorm.DB) error {
	var after int64
	for {
		var rows []struct {
			RoID     int64
			AuthInfo string
		}
		if err := db.Raw(`SELECT ro_id, auth_info FROM domains
			WHERE auth_info IS NOT NULL AND auth_info NOT LIKE '$sha256$%' AND ro_id > ?
			ORDER BY ro_id LIMIT ?`, after, legacyAuthInfoBatch).Scan(&rows).Error; err != nil {
			return fmt.Errorf("domain authInfo migration: %w", err)
		}
		if len(rows) == 0 {
			return nil
		}
		err := db.Transaction(func(tx *gorm.DB) error {
			for _, r := range rows {
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
