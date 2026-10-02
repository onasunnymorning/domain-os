package main

import (
	"context"
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/internal/buildinfo"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"go.uber.org/zap"
)

// migrateLockTimeout bounds how long `migrate` waits for another migrator.
// Long enough for a concurrent run's index builds to finish; short enough that
// a stuck run surfaces as a failed deploy step rather than a hang.
const migrateLockTimeout = 10 * time.Minute

// runMigrate is `ryAdminAPI migrate`: the one deployed path that changes the
// schema. It applies AutoMigrate under an advisory lock and records this
// build's version in schema_version, which the services check before starting
// (see docs/adr/0010-database-migrations.md).
//
// It exits non-zero on any failure so a release script can stop before rolling
// a single service. It is idempotent: on an up-to-date schema it applies
// nothing.
func runMigrate(logger *zap.Logger) error {
	logger.Info("migrate: starting",
		zap.String("version", buildinfo.Version),
		zap.String("git_sha", buildinfo.GitSHA))

	gormDB, err := postgres.NewConnectionFromEnv()
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	logf := func(format string, args ...any) {
		logger.Info("migrate: " + fmt.Sprintf(format, args...))
	}
	err = postgres.Migrate(context.Background(), gormDB, postgres.MigrateOptions{
		Version:     buildinfo.Version,
		GitSHA:      buildinfo.GitSHA,
		LockTimeout: migrateLockTimeout,
		Logf:        logf,
	})
	if err != nil {
		return err
	}

	logger.Info("migrate: done", zap.String("version", buildinfo.Version))
	return nil
}
