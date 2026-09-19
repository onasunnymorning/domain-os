package services

import (
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/require"
)

func TestNewImportedNNDN_TagsEscrowImportReason(t *testing.T) {
	createdAt := time.Date(2024, 1, 19, 15, 4, 5, 0, time.UTC)

	n := newImportedNNDN("XN--FSQ.Radio", "例子.radio", "Radio", "blocked", createdAt)

	require.Equal(t, "xn--fsq.radio", n.Name)
	require.Equal(t, "radio", n.TLDName)
	require.Equal(t, "blocked", n.NameState)
	require.Equal(t, createdAt, n.CreatedAt)
	require.Equal(t, "RDE-import", n.Reason)

	// The reason is stored as a ClIDType, so it must survive that validation
	// or the API would refuse to read the row back.
	_, err := entities.NewClIDType(n.Reason)
	require.NoError(t, err)
}
