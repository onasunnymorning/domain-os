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
