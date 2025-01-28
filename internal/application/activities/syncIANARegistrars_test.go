package activities

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestSyncIanaRegistrars tests the SyncIanaRegistrars function with a mock HTTP server.
func TestSyncIanaRegistrars(t *testing.T) {
	// We'll define a few test cases to verify different outcomes.
	tests := []struct {
		name               string
		correlationID      string
		serverStatusCode   int
		serverResponseBody string
		wantErr            bool
	}{
		{
			name:               "success response",
			correlationID:      "corr-id-123",
			serverStatusCode:   http.StatusOK,
			serverResponseBody: "OK",
			wantErr:            false,
		},
		{
			name:               "non-200 response",
			correlationID:      "corr-id-456",
			serverStatusCode:   http.StatusBadRequest,
			serverResponseBody: "Bad Request",
			wantErr:            true,
		},
	}

	for _, tc := range tests {
		tc := tc // capture tc for parallel test usage
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock server
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify the request method is PUT
				if r.Method != http.MethodPut {
					t.Errorf("expected method PUT, got %s", r.Method)
				}

				// Check correlationID query param
				q := r.URL.Query()
				if q.Get("correlationID") != tc.correlationID {
					t.Errorf("expected correlationID=%s, got %s", tc.correlationID, q.Get("correlationID"))
				}

				// Respond with the test's configured status code and body
				w.WriteHeader(tc.serverStatusCode)
				_, _ = w.Write([]byte(tc.serverResponseBody))
			}))
			defer mockServer.Close()

			// Override the global BASEURL with our mock server's URL
			originalBaseURL := BASEURL
			BASEURL = mockServer.URL
			defer func() {
				BASEURL = originalBaseURL
			}()

			// Override the BEARER_TOKEN if needed
			originalBearerToken := BEARER_TOKEN
			BEARER_TOKEN = "test-token" // a placeholder token
			defer func() {
				BEARER_TOKEN = originalBearerToken
			}()

			// Call the function under test
			err := SyncIanaRegistrars(tc.correlationID)
			if (err != nil) != tc.wantErr {
				t.Errorf("SyncIanaRegistrars() error = %v, wantErr %v", err, tc.wantErr)
			}

			// If we expect an error, we can also inspect the error message if needed
			if tc.wantErr && err == nil {
				t.Error("expected an error but got none")
			}
		})
	}
}
