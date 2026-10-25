package postgres

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
)

// dbFrom is the handle a repository uses for ctx: the transaction a
// Transactor opened, if ctx carries one, or db. A repository that opens a
// transaction of its own on it gets a savepoint in the outer one.
func dbFrom(ctx context.Context, db *gorm.DB) *gorm.DB {
	if tx, ok := appcontext.Transaction(ctx).(*gorm.DB); ok {
		return tx.WithContext(ctx)
	}
	return db.WithContext(ctx)
}

// Transactor is the Postgres repositories.Transactor.
type Transactor struct{ db *gorm.DB }

// NewTransactor returns a transactor on db.
func NewTransactor(db *gorm.DB) *Transactor { return &Transactor{db: db} }

// Begin implements repositories.Transactor.
func (t *Transactor) Begin(ctx context.Context) (context.Context, func(error) error) {
	if _, ok := appcontext.Transaction(ctx).(*gorm.DB); ok {
		return ctx, func(err error) error { return err }
	}
	tx := t.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		// Carried anyway: every query on it fails with the same error, so
		// nothing is written outside a transaction.
		begun := tx.Error
		return appcontext.WithTransaction(ctx, tx), func(err error) error {
			if err != nil {
				return err
			}
			return fmt.Errorf("begin transaction: %w", begun)
		}
	}
	return appcontext.WithTransaction(ctx, tx), func(err error) error {
		if err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit().Error; err != nil {
			return fmt.Errorf("commit transaction: %w", err)
		}
		return nil
	}
}
