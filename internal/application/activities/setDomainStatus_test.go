package activities

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/assert"
)

func TestSetDomainStatus(t *testing.T) {
	BASEURL = "http://example.com"
	BEARER_TOKEN = "test-token"

	tests := []struct {
		name           string
		cmd            SetStatusCommand
		mockStatusCode int
		mockResponse   string
		expectedError  string
		expectedDomain *entities.Domain
	}{
		{
			name: "successful request",
			cmd: SetStatusCommand{
				DomainName:    "example.com",
				Status:        "pendingCreate",
				CorrelationID: "12345",
				TraceID:       "trace-123",
			},
			mockStatusCode: http.StatusOK,
			mockResponse:   `{"name": "example.com", "status": {"pendingCreate": false}}`,
			expectedError:  "",
			expectedDomain: &entities.Domain{Name: "example.com", Status: entities.DomainStatus{
				PendingCreate: false,
			}},
		},
		{
			name: "failed request with unexpected status code",
			cmd: SetStatusCommand{
				DomainName:    "example.com",
				Status:        "inactive",
				CorrelationID: "12345",
				TraceID:       "trace-123",
			},
			mockStatusCode: http.StatusInternalServerError,
			mockResponse:   `{"error": "internal server error"}`,
			expectedError:  "unexpected status code: 500, response: {\"error\": \"internal server error\"}",
			expectedDomain: nil,
		},
		{
			name: "failed to unmarshal response",
			cmd: SetStatusCommand{
				DomainName:    "example.com",
				Status:        "inactive",
				CorrelationID: "12345",
				TraceID:       "trace-123",
			},
			mockStatusCode: http.StatusOK,
			mockResponse:   `invalid json`,
			expectedError:  "failed to unmarshal response: invalid character 'i' looking for beginning of value",
			expectedDomain: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock server
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, fmt.Sprintf("/domains/%s/status/%s", tt.cmd.DomainName, tt.cmd.Status), r.URL.Path)
				assert.Equal(t, BEARER_TOKEN, r.Header.Get("Authorization"))
				assert.Equal(t, tt.cmd.CorrelationID, r.URL.Query().Get("correlation_id"))

				w.WriteHeader(tt.mockStatusCode)
				w.Write([]byte(tt.mockResponse))
			}))
			defer mockServer.Close()

			BASEURL = mockServer.URL

			domain, err := SetDomainStatus(tt.cmd)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedDomain, domain)
			}
		})
	}
}
