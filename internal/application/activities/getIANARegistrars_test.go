package activities

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/stretchr/testify/assert"
)

func TestGetIANARegistrars(t *testing.T) {
	// Mock server to simulate API responses
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, BEARER_TOKEN, r.Header.Get("Authorization"))

		// Mock response
		ianaRegistrars := []entities.IANARegistrar{
			{GurID: 1, Name: "Registrar 1"},
			{GurID: 2, Name: "Registrar 2"},
		}
		apiResponse := response.ListItemResult{
			Data: &ianaRegistrars,
			Meta: response.PaginationMetaData{
				NextLink: "",
			},
		}
		resp, _ := json.Marshal(apiResponse)
		w.WriteHeader(http.StatusOK)
		w.Write(resp)
	}))
	defer mockServer.Close()

	// Override the BASEURL with the mock server URL
	BASEURL = mockServer.URL

	// Test the function
	registrars, err := GetIANARegistrars("test-correlation-id")
	assert.NoError(t, err)
	assert.Len(t, registrars, 2)
	assert.Equal(t, 1, registrars[0].GurID)
	assert.Equal(t, "Registrar 1", registrars[0].Name)
	assert.Equal(t, 2, registrars[1].GurID)
	assert.Equal(t, "Registrar 2", registrars[1].Name)
}

func TestGetIANARegistrars_Error(t *testing.T) {
	// Mock server to simulate API error response
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer mockServer.Close()

	// Override the BASEURL with the mock server URL
	BASEURL = mockServer.URL

	// Test the function
	registrars, err := GetIANARegistrars("test-correlation-id")
	assert.Error(t, err)
	assert.Nil(t, registrars)
	assert.Contains(t, err.Error(), "500")
}
