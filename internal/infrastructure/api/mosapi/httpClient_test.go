package mosapi

import (
	"net/http/cookiejar"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewHTTPClient(t *testing.T) {
	client, err := NewHTTPClient()
	require.NoError(t, err, "Failed to create HTTP client")

	// Verify that the client has a cookie jar
	_, ok := client.Jar.(*cookiejar.Jar)
	require.True(t, ok, "Client does not have a cookie jar")
}
