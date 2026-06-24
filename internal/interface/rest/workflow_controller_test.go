package rest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestStartRegistrarSync_Returns500WhenTemporalConfigInvalid(t *testing.T) {
	// Ensure env vars are empty or invalid so Temporal client creation fails fast
	_ = os.Unsetenv("TEMPORAL_HOST_PORT")
	_ = os.Unsetenv("TEMPORAL_NAMESPACE")
	_ = os.Unsetenv("TEMPORAL_CLIENT_KEY")
	_ = os.Unsetenv("TEMPORAL_CLIENT_CERT")
	_ = os.Unsetenv("TEMPORAL_QUEUE")

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
