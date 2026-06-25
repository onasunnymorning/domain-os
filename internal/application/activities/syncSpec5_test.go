package activities

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// TestSyncSpec5 tests the SyncSpec5 function with a mock HTTP server.
func TestSyncSpec5(t *testing.T) {
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
			serverResponseBody: `{"Message":"Successfully synced spec5 labels"}`,
			wantErr:            false,
		},
		{
			name:               "non-200 response",
			correlationID:      "corr-id-456",
			serverStatusCode:   http.StatusInternalServerError,
			serverResponseBody: "Internal Server Error",
			wantErr:            true,
		},
	}

	for _, tc := range tests {
		tc := tc
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
			originalAdminToken := os.Getenv("ADMIN_TOKEN")
			os.Setenv("ADMIN_TOKEN", "test-token") // a placeholder token
			defer func() {
				os.Setenv("ADMIN_TOKEN", originalAdminToken)
			}()

			// Call the function under test
			err := SyncSpec5(tc.correlationID)
			if (err != nil) != tc.wantErr {
				t.Errorf("SyncSpec5() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
