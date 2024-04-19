package entities

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNTrue(t *testing.T) {
	t.Run("No true values", func(t *testing.T) {
		result := trueCount(false, false, false)
		require.Equal(t, 0, result, "Expected 0 true values")
	})

	t.Run("One true value", func(t *testing.T) {
		result := trueCount(false, true, false)
		require.Equal(t, 1, result, "Expected 1 true value")
	})

	t.Run("Multiple true values", func(t *testing.T) {
		result := trueCount(true, true, true)
		require.Equal(t, 3, result, "Expected 3 true values")
	})
}
