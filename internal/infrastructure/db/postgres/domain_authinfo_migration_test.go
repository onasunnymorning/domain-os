package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// TestMigrateDomainAuthInfo runs the RFC 9154 migration on a domains table
// as the previous release left it: auth_info NOT NULL, in plain text. It
// works in a schema of its own, inside a transaction that is rolled back.
func TestMigrateDomainAuthInfo(t *testing.T) {
	tx := getTestDB().Begin()
	defer tx.Rollback()
	hashed, err := entities.HashAuthInfo("Tr4nsf3r-C0de-Xq9#Lm2!")
	require.NoError(t, err)
	for _, stmt := range []string{
		"CREATE SCHEMA authinfo_migration_test",
		"SET LOCAL search_path TO authinfo_migration_test",
		"CREATE TABLE domains (ro_id bigint PRIMARY KEY, auth_info text NOT NULL)",
		"INSERT INTO domains VALUES (1, 'Secret-Auth-1!'), (2, ''), (3, '" + string(hashed) + "')",
	} {
		require.NoError(t, tx.Exec(stmt).Error, stmt)
	}

	require.NoError(t, migrateDomainAuthInfo(tx))
	read := func() map[int64]*string {
		var rows []struct {
			RoID     int64
			AuthInfo *string
		}
		require.NoError(t, tx.Raw("SELECT ro_id, auth_info FROM domains ORDER BY ro_id").Scan(&rows).Error)
		out := map[int64]*string{}
		for _, r := range rows {
			out[r.RoID] = r.AuthInfo
		}
		return out
	}
	after := read()
	require.NotNil(t, after[1])
	assert.True(t, entities.AuthInfoHash(*after[1]).Matches("Secret-Auth-1!"), "a plain-text code is replaced by its hash, which matches it")
	assert.Nil(t, after[2], "an empty code becomes NULL: no code")
	require.NotNil(t, after[3])
	assert.Equal(t, string(hashed), *after[3], "a hash is left alone")

	t.Run("a rerun changes nothing", func(t *testing.T) {
		require.NoError(t, migrateDomainAuthInfo(tx))
		again := read()
		assert.Equal(t, *after[1], *again[1])
		assert.Nil(t, again[2])
		assert.Equal(t, *after[3], *again[3])
	})
	t.Run("a domain can be stored with no code", func(t *testing.T) {
		require.NoError(t, tx.Exec("INSERT INTO domains VALUES (4, NULL)").Error)
	})
}
