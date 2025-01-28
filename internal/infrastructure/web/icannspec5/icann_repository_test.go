package icannspec5

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewICANNSpec5Repo(t *testing.T) {
	repo := NewICANNRepo()

	// Add your assertions here
	require.NotNil(t, repo, "ICANNSpec5Repository is nil")
	require.Equal(t, ICANN_SPEC5_XML_URL, repo.XMLSpec5URL, "ICANNSpec5Repository XMLSpec5URL mismatch")
}
