package activities

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBulkUpdateRegistrarStatuses(t *testing.T) {
	tests := []struct {
		name           string
		updates        []commands.UpdateRegistrarStatusCommand
		handler        http.HandlerFunc
		wantUpdated    int
		wantFailed     int
		wantUpdatedIDs []string
		wantErrCount   int
		wantErrOp      string // expected Operation in first error, if any
	}{
		{
			name:           "empty updates list",
			updates:        []commands.UpdateRegistrarStatusCommand{},
			handler:        func(w http.ResponseWriter, r *http.Request) {},
			wantUpdated:    0,
			wantFailed:     0,
			wantUpdatedIDs: []string{},
			wantErrCount:   0,
		},
		{
			name: "successful platform status update only",
			updates: []commands.UpdateRegistrarStatusCommand{
				{ClID: "rar1", NewStatus: "OK"},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				// Expect: /registrars/rar1/status/ok
				if strings.Contains(r.URL.Path, "/registrars/rar1/status/ok") {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			},
			wantUpdated:    1,
			wantFailed:     0,
			wantUpdatedIDs: []string{"rar1"},
			wantErrCount:   0,
		},
		{
			name: "successful IANA status update only",
			updates: []commands.UpdateRegistrarStatusCommand{
				{ClID: "rar2", NewIANAStatus: "accredited"},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				// Expect: /registrars/rar2/iana_status/accredited
				if strings.Contains(r.URL.Path, "/registrars/rar2/iana_status/accredited") {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			},
			wantUpdated:    1,
			wantFailed:     0,
			wantUpdatedIDs: []string{"rar2"},
			wantErrCount:   0,
		},
		{
			name: "both status updates succeed",
			updates: []commands.UpdateRegistrarStatusCommand{
				{ClID: "rar3", NewStatus: "OK", NewIANAStatus: "accredited"},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				if strings.Contains(r.URL.Path, "/registrars/rar3/status/ok") {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				if strings.Contains(r.URL.Path, "/registrars/rar3/iana_status/accredited") {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			},
			wantUpdated:    1,
			wantFailed:     0,
			wantUpdatedIDs: []string{"rar3"},
			wantErrCount:   0,
		},
		{
			name: "platform status update fails with 500",
			updates: []commands.UpdateRegistrarStatusCommand{
				{ClID: "rar4", NewStatus: "OK"},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "/registrars/rar4/status/ok") {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			},
			wantUpdated:    0,
			wantFailed:     1,
			wantUpdatedIDs: []string{},
			wantErrCount:   1,
			wantErrOp:      "update-status",
		},
		{
			name: "IANA status 404 treated as no-op success",
			updates: []commands.UpdateRegistrarStatusCommand{
				{ClID: "rar5", NewIANAStatus: "accredited"},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "/registrars/rar5/iana_status/accredited") {
					// Return 404 — updateIANAStatus treats this as success
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantUpdated:    1,
			wantFailed:     0,
			wantUpdatedIDs: []string{"rar5"},
			wantErrCount:   0,
		},
		{
			name: "multiple updates with mixed results",
			updates: []commands.UpdateRegistrarStatusCommand{
				{ClID: "ok-rar", NewStatus: "OK"},
				{ClID: "fail-rar", NewStatus: "TERMINATED"},
				{ClID: "both-rar", NewStatus: "OK", NewIANAStatus: "accredited"},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				path := r.URL.Path
				switch {
				case strings.Contains(path, "/registrars/ok-rar/status/ok"):
					w.WriteHeader(http.StatusNoContent)
				case strings.Contains(path, "/registrars/fail-rar/status/terminated"):
					w.WriteHeader(http.StatusInternalServerError)
				case strings.Contains(path, "/registrars/both-rar/status/ok"):
					w.WriteHeader(http.StatusNoContent)
				case strings.Contains(path, "/registrars/both-rar/iana_status/accredited"):
					w.WriteHeader(http.StatusNoContent)
				default:
					w.WriteHeader(http.StatusNotFound)
					fmt.Fprintf(w, "unexpected path: %s", path)
				}
			},
			wantUpdated:    2,
			wantFailed:     1,
			wantUpdatedIDs: []string{"ok-rar", "both-rar"},
			wantErrCount:   1,
			wantErrOp:      "update-status",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Create mock server
			mockServer := httptest.NewServer(tc.handler)
			defer mockServer.Close()

			// Override BASEURL to point to mock server
			originalBaseURL := BASEURL
			BASEURL = mockServer.URL
			defer func() { BASEURL = originalBaseURL }()

			// Override ADMIN_TOKEN env var (used by GetBearerToken fallback)
			originalAdminToken := os.Getenv("ADMIN_TOKEN")
			os.Setenv("ADMIN_TOKEN", "test-token")
			defer func() { os.Setenv("ADMIN_TOKEN", originalAdminToken) }()

			// Execute
			result, err := BulkUpdateRegistrarStatuses("test-correlation-id", tc.updates)

			// The function never returns an error at the top level
			require.NoError(t, err)

			// Verify counts
			assert.Equal(t, tc.wantUpdated, result.Updated, "Updated count mismatch")
			assert.Equal(t, tc.wantFailed, result.Failed, "Failed count mismatch")

			// Verify updated IDs
			assert.ElementsMatch(t, tc.wantUpdatedIDs, result.UpdatedIDs, "UpdatedIDs mismatch")

			// Verify error details
			assert.Len(t, result.Errors, tc.wantErrCount, "Error count mismatch")
			if tc.wantErrOp != "" && len(result.Errors) > 0 {
				assert.Equal(t, tc.wantErrOp, result.Errors[0].Operation, "Error operation mismatch")
				assert.NotEmpty(t, result.Errors[0].Error, "Error message should not be empty")
				assert.NotEmpty(t, result.Errors[0].ClID, "Error ClID should not be empty")
			}
		})
	}
}
