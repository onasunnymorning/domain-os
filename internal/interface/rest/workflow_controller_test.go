package rest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestStartRegistrarSync_Returns500WhenTemporalConfigInvalid(t *testing.T) {
	// Point Temporal at an unreachable address to guarantee connection failure.
	// Note: simply unsetting env vars doesn't work — the SDK defaults empty
	// HostPort to localhost:7233 which succeeds if a local server is running.
	t.Setenv("TEMPORAL_HOST_PORT", "localhost:1") // port 1 is unreachable
	t.Setenv("TEMPORAL_NAMESPACE", "")
	t.Setenv("TEMPORAL_CLIENT_KEY", "")
	t.Setenv("TEMPORAL_CLIENT_CERT", "")
	t.Setenv("TEMPORAL_API_KEY", "")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	// No-op auth handler
	noop := func(c *gin.Context) { c.Next() }
	NewWorkflowController(r, noop)

	// Prepare request body
	body, _ := json.Marshal(map[string]any{"batchSize": 50})
	req := httptest.NewRequest(http.MethodPost, "/workflows/registrars/sync", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// Attach an Authorization header since real router uses auth; our noop ignores it, but include anyway
	req.Header.Set("Authorization", "Bearer test")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d, body: %s", w.Code, w.Body.String())
	}
	// Body should be JSON with an error field
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("expected JSON body, got error: %v", err)
	}
	if _, ok := resp["error"]; !ok {
		t.Fatalf("expected 'error' key in response, got: %#v", resp)
	}
}
