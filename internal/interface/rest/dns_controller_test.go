package rest_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/interface/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDNSController_GetHealth tests the health endpoint
func TestDNSController_GetHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock DNS service with nil batch publisher (simulates disabled)
	dnsService := services.NewDNSService(nil)

	router := gin.Default()
	rest.NewDNSController(router, dnsService, func(c *gin.Context) {
		c.Next() // No auth for testing
	})

	// Test health endpoint
	req, _ := http.NewRequest("GET", "/api/v1/dns/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response services.DNSHealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "unavailable", response.Status)
	assert.False(t, response.Checks["publisher_configured"])
	assert.Contains(t, response.Issues, "DNS batch publisher not configured")
}

// TestDNSController_GetQueueStats tests the queue stats endpoint
func TestDNSController_GetQueueStats(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock DNS service with nil batch publisher
	dnsService := services.NewDNSService(nil)

	router := gin.Default()
	rest.NewDNSController(router, dnsService, func(c *gin.Context) {
		c.Next()
	})

	// Test queue stats endpoint
	req, _ := http.NewRequest("GET", "/api/v1/dns/queue/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return error when publisher not configured
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "DNS batch publisher not configured")
}

// TestDNSController_GetPendingChanges tests the pending changes endpoint
func TestDNSController_GetPendingChanges(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dnsService := services.NewDNSService(nil)

	router := gin.Default()
	rest.NewDNSController(router, dnsService, func(c *gin.Context) {
		c.Next()
	})

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{
			name:       "no filters",
			query:      "",
			wantStatus: http.StatusInternalServerError, // No publisher configured
		},
		{
			name:       "with zone filter",
			query:      "?zone=tld.",
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "with pagination",
			query:      "?limit=10&offset=0",
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "with all filters",
			query:      "?zone=tld.&domain=example.tld.&change_type=ADD&record_type=NS&limit=50&offset=10",
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/api/v1/dns/queue/pending"+tt.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

// TestDNSController_GetErroredChanges tests the errored changes endpoint
func TestDNSController_GetErroredChanges(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dnsService := services.NewDNSService(nil)

	router := gin.Default()
	rest.NewDNSController(router, dnsService, func(c *gin.Context) {
		c.Next()
	})

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{
			name:       "no filters",
			query:      "",
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "with zone filter",
			query:      "?zone=tld.",
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "with pagination",
			query:      "?limit=25&offset=5",
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/api/v1/dns/queue/errors"+tt.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

// TestDNSController_GetQueueStatsForZone tests the zone-specific stats endpoint
func TestDNSController_GetQueueStatsForZone(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dnsService := services.NewDNSService(nil)

	router := gin.Default()
	rest.NewDNSController(router, dnsService, func(c *gin.Context) {
		c.Next()
	})

	req, _ := http.NewRequest("GET", "/api/v1/dns/queue/stats/tld.", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "DNS batch publisher not configured")
}

// TestDNSController_Routes verifies all routes are registered
func TestDNSController_Routes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dnsService := services.NewDNSService(nil)
	router := gin.Default()
	rest.NewDNSController(router, dnsService, func(c *gin.Context) {
		c.Next()
	})

	routes := router.Routes()

	expectedPaths := []string{
		"/api/v1/dns/queue/stats",
		"/api/v1/dns/queue/stats/:zone",
		"/api/v1/dns/queue/pending",
		"/api/v1/dns/queue/errors",
		"/api/v1/dns/health",
	}

	for _, expected := range expectedPaths {
		found := false
		for _, route := range routes {
			if route.Path == expected {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected route %s not found", expected)
	}
}
