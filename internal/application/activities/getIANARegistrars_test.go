package activities

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGetIANARegistrars verifies the activity handles Data:null, empty arrays, and valid arrays.
func TestGetIANARegistrars(t *testing.T) {
	tests := []struct {
		name         string
		responseBody string
		wantCount    int
		wantErr      bool
	}{
		{
			name:         "data null treated as empty",
			responseBody: `{"Meta":{},"Data":null}`,
			wantCount:    0,
			wantErr:      false,
		},
		{
			name:         "data empty array",
			responseBody: `{"Meta":{},"Data":[]}`,
			wantCount:    0,
			wantErr:      false,
		},
		{
			name:         "data with registrars",
			responseBody: `{"Meta":{},"Data":[{"GurID":1,"Name":"Example Registrar, Inc.","Status":"Accredited","RdapURL":"https://example.com/rdap","CreatedAt":"2021-01-01T00:00:00Z"}]}`,
			wantCount:    1,
			wantErr:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Mock server that returns the configured body and 200 OK
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Fatalf("expected GET, got %s", r.Method)
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tc.responseBody))
			}))
			defer srv.Close()

			// Call the activity
			got, err := GetIANARegistrars("corr-xyz", srv.URL, "Bearer test", 100)
			if (err != nil) != tc.wantErr {
				t.Fatalf("GetIANARegistrars() error = %v, wantErr=%v", err, tc.wantErr)
			}
			if err == nil && len(got) != tc.wantCount {
				t.Fatalf("GetIANARegistrars() count = %d, want %d", len(got), tc.wantCount)
			}
		})
	}
}
