package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// TestCheckRepositoryID pins ADR 0017's guard: the first start records the
// installation's repository ID, and a later start with another is refused,
// because only a ROID's number is stored.
func TestCheckRepositoryID(t *testing.T) {
	db := getTestDB()
	ctx := context.Background()
	clear := func() { db.Exec("DELETE FROM installation_settings WHERE key = ?", roidRepositoryIDKey) }
	clear()
	t.Cleanup(clear)

	require.NoError(t, CheckRepositoryID(ctx, db, "FIRST"), "the first start records its ID")
	require.NoError(t, CheckRepositoryID(ctx, db, "FIRST"), "and starts again with it")

	err := CheckRepositoryID(ctx, db, "OTHER")
	require.ErrorIs(t, err, entities.ErrRepositoryIDChanged)
	assert.Contains(t, err.Error(), `configured "OTHER", recorded "FIRST"`)

	require.ErrorIs(t, CheckRepositoryID(ctx, db, ""), entities.ErrInvalidRepositoryID, "an unset ID is never recorded")
	var n int64
	require.NoError(t, db.Model(&InstallationSetting{}).Where("key = ?", roidRepositoryIDKey).Count(&n).Error)
	assert.Equal(t, int64(1), n)
}
