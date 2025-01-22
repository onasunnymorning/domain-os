package activities

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

func TestUnSetDomainStatus(t *testing.T) {
	tests := []struct {
		name           string
		cmd            UnsetStatusCommand
		mockStatusCode int
		mockResponse   string
		expectedError  bool
	}{
		{
			name: "successful request",
			cmd: UnsetStatusCommand{
				DomainName:    "example.com",
				Status:        entities.DomainStatusPendingCreate,
				CorrelationID: "12345",
				TraceID:       "trace-123",
			},
			mockStatusCode: http.StatusNoContent,
			mockResponse:   "",
			expectedError:  false,
		},
		{
			name: "unexpected status code",
			cmd: UnsetStatusCommand{
				DomainName:    "example.com",
				Status:        "inactive",
				CorrelationID: "12345",
				TraceID:       "trace-123",
			},
			mockStatusCode: http.StatusBadRequest,
			mockResponse:   "Bad Request",
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.mockStatusCode)
				w.Write([]byte(tt.mockResponse))
			}))
			defer server.Close()

			// Override the BASEURL with the mock server URL
			BASEURL = server.URL

			err := UnSetDomainStatus(tt.cmd)
			if (err != nil) != tt.expectedError {
				t.Errorf("UnSetDomainStatus() error = %v, expectedError %v", err, tt.expectedError)
			}
		})
	}
}
