package mosapi

import (
	"net/http/cookiejar"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewHTTPClient_Basic(t *testing.T) {
	client, err := NewHTTPClient(&MosapiConfig{
		Username: "test",
		Password: "test",
		AuthType: AuthTypeBasic,
	})
	require.NoError(t, err, "Failed to create HTTP client")

	// Verify that the client has a cookie jar
	_, ok := client.Jar.(*cookiejar.Jar)
	require.True(t, ok, "Client does not have a cookie jar")
}

func TestNewHTTPClient_Certificate(t *testing.T) {
	client, err := NewHTTPClient(&MosapiConfig{
		Certificate: "../../../../testdata/example56.cert.pem",
		Key:         "../../../../testdata/example56.key.pem",
		AuthType:    AuthTypeCertificate,
	})
	require.NoError(t, err, "Failed to create HTTP client")

	// Verify that the client has a transport
	require.NotNil(t, client.Transport, "Client does not have a transport")
}

func TestNewHTTPClient_Certificate_KeyMissing(t *testing.T) {
	client, err := NewHTTPClient(&MosapiConfig{
		Certificate: "../../../../testdata/example56.cert.pem",
		AuthType:    AuthTypeCertificate,
	})
	require.Error(t, err, "should error our")
	require.Nil(t, client, "client should be nil")
}

func TestNewHTTPClient_Certificate_CertMissing(t *testing.T) {
	client, err := NewHTTPClient(&MosapiConfig{
		Key:      "../../../../testdata/example56.key.pem",
		AuthType: AuthTypeCertificate,
	})
	require.Error(t, err, "should error our")
	require.Nil(t, client, "client should be nil")
}

func TestNewHTTPClient_InvalidAuthType(t *testing.T) {
	client, err := NewHTTPClient(&MosapiConfig{
		AuthType: "invalid",
	})
	require.Error(t, err, "should error our")
	require.Nil(t, client, "client should be nil")
}
