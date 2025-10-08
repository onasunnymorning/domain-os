package middleware

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestRateLimiterConfig tests the configuration
func TestDefaultRateLimitConfig(t *testing.T) {
	config := DefaultRateLimitConfig()

	assert.Equal(t, 10, config.MaxConnPerIP)
	assert.Equal(t, 100, config.MaxConnPerRegistrar)
	assert.Equal(t, 5*time.Minute, config.ConnTTL)
	assert.Equal(t, 100, config.RequestsPerSecond)
	assert.Equal(t, 200, config.BurstSize)
	assert.Equal(t, time.Second, config.RequestWindow)
	assert.Equal(t, 5, config.MaxFailedLogins)
	assert.Equal(t, 15*time.Minute, config.LockoutDuration)
}

func TestCustomRateLimitConfig(t *testing.T) {
	config := &RateLimitConfig{
		MaxConnPerIP:        5,
		MaxConnPerRegistrar: 50,
		ConnTTL:             1 * time.Minute,
		RequestsPerSecond:   50,
		BurstSize:           100,
		RequestWindow:       2 * time.Second,
		MaxFailedLogins:     3,
		LockoutDuration:     10 * time.Minute,
	}

	assert.Equal(t, 5, config.MaxConnPerIP)
	assert.Equal(t, 50, config.MaxConnPerRegistrar)
	assert.Equal(t, 1*time.Minute, config.ConnTTL)
	assert.Equal(t, 50, config.RequestsPerSecond)
	assert.Equal(t, 100, config.BurstSize)
	assert.Equal(t, 2*time.Second, config.RequestWindow)
	assert.Equal(t, 3, config.MaxFailedLogins)
	assert.Equal(t, 10*time.Minute, config.LockoutDuration)
}

func TestErrorTypes(t *testing.T) {
	assert.NotNil(t, ErrTooManyConnections)
	assert.NotNil(t, ErrTooManyRegistrarConnections)
	assert.NotNil(t, ErrRateLimitExceeded)
	assert.NotNil(t, ErrAccountLocked)

	assert.Equal(t, "too many connections from this IP", ErrTooManyConnections.Error())
	assert.Equal(t, "too many connections for this registrar", ErrTooManyRegistrarConnections.Error())
	assert.Equal(t, "rate limit exceeded", ErrRateLimitExceeded.Error())
	assert.Equal(t, "account locked due to failed login attempts", ErrAccountLocked.Error())
}

// Note: Integration tests that require Redis are in rate_limiter_integration_test.go
// To run integration tests: go test -tags=integration
