package postgres

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/registry-os/internal/domain/entities"
)

func TestToDBTld(t *testing.T) {
	tld, err := entities.NewTLD("com")
	if err != nil {
		t.Fatal(err)
	}

	dbtld := ToDBTLD(tld)

	require.Equal(t, tld.Name.String(), dbtld.Name, "TLD Name mismatch")
	require.Equal(t, tld.Type.String(), dbtld.Type, "TLD Type mismatch")
	require.Equal(t, tld.UName, dbtld.UName, "TLD UName mismatch")
	require.Equal(t, tld.CreatedAt, dbtld.CreatedAt, "TLD CreatedAt mismatch")
	require.Equal(t, tld.UpdatedAt, dbtld.UpdatedAt, "TLD UpdatedAt mismatch")
}

func TestFromDBTld(t *testing.T) {
	dbtld := &TLD{
		Name:      "com",
		Type:      "generic",
		UName:     "com",
		CreatedAt: entities.RoundTime(time.Now().UTC()),
		UpdatedAt: entities.RoundTime(time.Now().UTC()),
	}

	tld := FromDBTLD(dbtld)

	require.Equal(t, dbtld.Name, tld.Name.String(), "TLD Name mismatch")
	require.Equal(t, dbtld.Type, tld.Type.String(), "TLD Type mismatch")
	require.Equal(t, dbtld.UName, tld.UName, "TLD UName mismatch")
	require.Equal(t, dbtld.CreatedAt, tld.CreatedAt, "TLD CreatedAt mismatch")
	require.Equal(t, dbtld.UpdatedAt, tld.UpdatedAt, "TLD UpdatedAt mismatch")
}
