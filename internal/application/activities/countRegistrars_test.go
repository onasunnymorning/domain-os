package activities

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCountRegistrars(t *testing.T) {
	// Define test cases in a table-driven format
	tests := []struct {
		name               string
		correlationID      string
		serverStatusCode   int
		serverResponseBody string
		wantCount          int64
		wantErr            bool
	}{
		{
			name:               "success response",
			correlationID:      "corr-id-123",
			serverStatusCode:   http.StatusOK,
			serverResponseBody: `{"count": 42}`,
			wantCount:          42,
			wantErr:            false,
		},
		{
			name:               "non-200 response",
			correlationID:      "corr-id-456",
			serverStatusCode:   http.StatusBadRequest,
			serverResponseBody: "Bad Request",
			wantErr:            true,
		},
		{
			name:               "invalid JSON response",
			correlationID:      "corr-id-789",
			serverStatusCode:   http.StatusOK,
			serverResponseBody: `{"count": "not-an-integer"}`,
			wantErr:            true,
		},
	}

	for _, tc := range tests {
		tc := tc // capture range variable for parallel test usage if desired

		t.Run(tc.name, func(t *testing.T) {
			// Create a mock server
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Ensure the request method is GET
				if r.Method != http.MethodGet {
					t.Errorf("expected method GET, got %s", r.Method)
				}

				// Check correlationID query param
				q := r.URL.Query()
				if q.Get("correlationID") != tc.correlationID {
					t.Errorf("expected correlationID=%s, got %s", tc.correlationID, q.Get("correlationID"))
				}

				// Write the test's configured status and response
				w.WriteHeader(tc.serverStatusCode)
				_, _ = w.Write([]byte(tc.serverResponseBody))
			}))
			defer mockServer.Close()

			// Override BASEURL to point to our mock server
			originalBaseURL := BASEURL
			BASEURL = mockServer.URL
			defer func() {
				BASEURL = originalBaseURL
			}()

			// Override BEARER_TOKEN if needed
			originalBearerToken := BEARER_TOKEN
			BEARER_TOKEN = "test-token"
			defer func() {
				BEARER_TOKEN = originalBearerToken
			}()

			// Invoke the function under test
			countResult, err := CountRegistrars(tc.correlationID)

			if (err != nil) != tc.wantErr {
				t.Fatalf("CountRegistrars() error = %v, wantErr %v", err, tc.wantErr)
			}

			// If no error is expected, verify the returned count
			if !tc.wantErr {
				if countResult == nil {
					t.Fatal("expected a valid CountResult, got nil")
				}
				if countResult.Count != tc.wantCount {
					t.Errorf("expected count = %d, got %d", tc.wantCount, countResult.Count)
				}
			} else if err == nil {
				t.Error("expected an error but got none")
			}
		})
	}
}
