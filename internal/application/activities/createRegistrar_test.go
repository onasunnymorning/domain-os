package activities

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/assert"
)

func TestCreateRegistrar(t *testing.T) {
	BASEURL = "http://example.com"
	BEARER_TOKEN = "test-token"

	tests := []struct {
		name           string
		correlationID  string
		cmd            commands.CreateRegistrarCommand
		mockResponse   string
		mockStatusCode int
		expectedError  error
		expectedResult *entities.Registrar
	}{
		{
			name:          "successful creation",
			correlationID: "12345",
			cmd: commands.CreateRegistrarCommand{
				GurID: 123,
				Name:  "Test Registrar",
				Email: "me@email.com",
			},
			mockResponse:   `{"ClID": "1", "name": "Test Registrar", "email": "me@email.com"}`,
			mockStatusCode: http.StatusCreated,
			expectedError:  nil,
			expectedResult: &entities.Registrar{ClID: entities.ClIDType("1"), Name: "Test Registrar", Email: "me@email.com"},
		},
		{
			name:           "failed to add query params",
			correlationID:  "itwillbreak%$",
			cmd:            commands.CreateRegistrarCommand{},
			mockResponse:   "",
			mockStatusCode: http.StatusBadRequest,
			expectedError:  errors.New("(400) "),
			expectedResult: nil,
		},
		{
			name:          "failed to create registrar",
			correlationID: "12345",
			cmd: commands.CreateRegistrarCommand{
				GurID: 123,
				Name:  "Test Registrar",
			},
			mockResponse:   `{"error": "failed to create registrar"}`,
			mockStatusCode: http.StatusInternalServerError,
			expectedError:  errors.New("(500) {\"error\": \"failed to create registrar\"}"),
			expectedResult: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock the HTTP client
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.mockStatusCode)
				w.Write([]byte(tt.mockResponse))
			}))
			defer mockServer.Close()

			BASEURL = mockServer.URL

			result, err := CreateRegistrar(tt.correlationID, tt.cmd)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}
}
