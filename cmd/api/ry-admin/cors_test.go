package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// The browser sends a preflight naming the headers a request will carry;
// the API must allow each one or the request never reaches a handler.
func TestAdminCORSPreflightAllowsConditionalHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(cors.New(adminCORSConfig("http://localhost:3000")))
	r.PUT("/epp/policies/:name", func(c *gin.Context) { c.Status(http.StatusOK) })

	for _, h := range []string{"if-none-match", "if-match", "authorization,content-type,if-none-match"} {
		req := httptest.NewRequest(http.MethodOptions, "/epp/policies/gold", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		req.Header.Set("Access-Control-Request-Method", http.MethodPut)
		req.Header.Set("Access-Control-Request-Headers", h)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusNoContent, w.Code, h)
		require.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "If-None-Match", h)
		require.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "If-Match", h)
		require.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
	}

	req := httptest.NewRequest(http.MethodOptions, "/epp/policies/gold", nil)
	req.Header.Set("Origin", "http://evil.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodPut)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Empty(t, w.Header().Get("Access-Control-Allow-Origin"), "another origin gets nothing")
}
