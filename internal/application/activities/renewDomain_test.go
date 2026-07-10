package activities

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/stretchr/testify/assert"
)

func TestRenewDomain(t *testing.T) {
	// Create a mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate request method
		assert.Equal(t, "POST", r.Method)

		// Validate authorization header
		assert.Equal(t, "Bearer "+os.Getenv("ADMIN_TOKEN"), r.Header.Get("Authorization"))

		// Validate query params
		assert.Equal(t, "test-correlation-id", r.URL.Query().Get("correlation_id"))

		// Validate request body
		var cmd commands.RenewDomainCommand
		body, _ := io.ReadAll(r.Body)
		err := json.Unmarshal(body, &cmd)
		assert.NoError(t, err)
		assert.Equal(t, "example.com", cmd.Name)

		// Send mock response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer mockServer.Close()

	// Override BASEURL for testing
	BASEURL = mockServer.URL

	// Create a test command
	testCommand := commands.RenewDomainCommand{
		Name: "example.com",
	}

	// Set ADMIN_TOKEN for test
	os.Setenv("ADMIN_TOKEN", "test-token")

	// Call the function under test
	err := RenewDomain(context.Background(), "test-correlation-id", testCommand, false)

	// Assertions
	assert.NoError(t, err)
}

func TestRenewDomain_Force(t *testing.T) {
	// Create a mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate request method
		assert.Equal(t, "POST", r.Method)

		// Validate authorization header
		assert.Equal(t, "Bearer "+os.Getenv("ADMIN_TOKEN"), r.Header.Get("Authorization"))

		// Validate query params
		assert.Equal(t, "test-correlation-id", r.URL.Query().Get("correlation_id"))

		// Validate request body
		var cmd commands.RenewDomainCommand
		body, _ := io.ReadAll(r.Body)
		err := json.Unmarshal(body, &cmd)
		assert.NoError(t, err)
		assert.Equal(t, "example.com", cmd.Name)

		// Send mock response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer mockServer.Close()

	// Override BASEURL for testing
	BASEURL = mockServer.URL

	// Create a test command
	testCommand := commands.RenewDomainCommand{
		Name: "example.com",
	}

	// Set ADMIN_TOKEN for test
	os.Setenv("ADMIN_TOKEN", "test-token")

	// Call the function under test
	err := RenewDomain(context.Background(), "test-correlation-id", testCommand, true)

	// Assertions
	assert.NoError(t, err)
}
