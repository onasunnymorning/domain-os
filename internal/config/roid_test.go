package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

func TestLoadRepositoryID(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		t.Setenv("ROID_REPOSITORY_ID", " VRSN ")
		id, err := LoadRepositoryID()
		require.NoError(t, err)
		assert.Equal(t, entities.RepositoryID("VRSN"), id)
	})
	t.Run("unset is an error, not a default", func(t *testing.T) {
		t.Setenv("ROID_REPOSITORY_ID", "")
		_, err := LoadRepositoryID()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ROID_REPOSITORY_ID is not set")
	})
	for _, bad := range []string{"TOOLONG99", "AB-C", "AB_C", "Ω"} {
		t.Run(bad+" is refused", func(t *testing.T) {
			t.Setenv("ROID_REPOSITORY_ID", bad)
			_, err := LoadRepositoryID()
			require.ErrorIs(t, err, entities.ErrInvalidRepositoryID)
		})
	}
}

func TestLoadSnowflakeNodeID(t *testing.T) {
	t.Run("unset is not set, not an error", func(t *testing.T) {
		t.Setenv("SNOWFLAKE_NODE_ID", " ")
		_, set, err := LoadSnowflakeNodeID()
		require.NoError(t, err)
		assert.False(t, set)
	})
	for _, v := range []string{"0", " 42 ", "1023"} {
		t.Run(v+" is set", func(t *testing.T) {
			t.Setenv("SNOWFLAKE_NODE_ID", v)
			id, set, err := LoadSnowflakeNodeID()
			require.NoError(t, err)
			assert.True(t, set)
			assert.GreaterOrEqual(t, id, int64(0))
			assert.LessOrEqual(t, id, int64(MaxSnowflakeNodeID))
		})
	}
	for _, bad := range []string{"-1", "1024", "seven", "1.5"} {
		t.Run(bad+" is refused", func(t *testing.T) {
			t.Setenv("SNOWFLAKE_NODE_ID", bad)
			_, _, err := LoadSnowflakeNodeID()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "SNOWFLAKE_NODE_ID")
		})
	}
}
