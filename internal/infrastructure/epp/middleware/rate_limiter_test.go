//go:build integration
// +build integration

package middleware

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a test Redis client
func setupTestRedis(t *testing.T) *redis.Client {
	// Use miniredis for testing or connect to test Redis instance
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Use a separate DB for tests
	})

	// Ping to check connection
	ctx := context.Background()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		t.Skip("Redis not available for testing, skipping tests")
	}

	// Clear the test database
	client.FlushDB(ctx)

	return client
}

func TestNewRateLimiter(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	t.Run("with custom config", func(t *testing.T) {
		config := &RateLimitConfig{
			MaxConnPerIP:        5,
			MaxConnPerRegistrar: 50,
			ConnTTL:             1 * time.Minute,
		}

		rl := NewRateLimiter(client, config, logger)
		assert.NotNil(t, rl)
		assert.Equal(t, 5, rl.config.MaxConnPerIP)
		assert.Equal(t, 50, rl.config.MaxConnPerRegistrar)
	})

	t.Run("with default config", func(t *testing.T) {
		rl := NewRateLimiter(client, nil, logger)
		assert.NotNil(t, rl)
		assert.Equal(t, 10, rl.config.MaxConnPerIP)
		assert.Equal(t, 100, rl.config.MaxConnPerRegistrar)
	})
}

func TestCheckConnectionLimit(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	config := &RateLimitConfig{
		MaxConnPerIP:        3,
		MaxConnPerRegistrar: 5,
		ConnTTL:             1 * time.Minute,
	}

	rl := NewRateLimiter(client, config, logger)
	ctx := context.Background()

	t.Run("allows connection under IP limit", func(t *testing.T) {
		client.FlushDB(ctx)

		err := rl.CheckConnectionLimit(ctx, "192.168.1.1", "")
		assert.NoError(t, err)
	})

	t.Run("blocks connection over IP limit", func(t *testing.T) {
		client.FlushDB(ctx)

		// Set connection count to max
		client.Set(ctx, "conn:ip:192.168.1.2", 3, time.Minute)

		err := rl.CheckConnectionLimit(ctx, "192.168.1.2", "")
		assert.ErrorIs(t, err, ErrTooManyConnections)
	})

	t.Run("blocks connection over registrar limit", func(t *testing.T) {
		client.FlushDB(ctx)

		// Set registrar connection count to max
		client.Set(ctx, "conn:reg:REG123", 5, time.Minute)

		err := rl.CheckConnectionLimit(ctx, "192.168.1.3", "REG123")
		assert.ErrorIs(t, err, ErrTooManyRegistrarConnections)
	})
}

func TestIncrementConnection(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	rl := NewRateLimiter(client, nil, logger)
	ctx := context.Background()

	t.Run("increments IP counter", func(t *testing.T) {
		client.FlushDB(ctx)

		err := rl.IncrementConnection(ctx, "192.168.1.1", "")
		require.NoError(t, err)

		count, err := rl.GetConnectionCount(ctx, "192.168.1.1")
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("increments both IP and registrar counters", func(t *testing.T) {
		client.FlushDB(ctx)

		err := rl.IncrementConnection(ctx, "192.168.1.2", "REG123")
		require.NoError(t, err)

		ipCount, err := rl.GetConnectionCount(ctx, "192.168.1.2")
		require.NoError(t, err)
		assert.Equal(t, 1, ipCount)

		regCount, err := rl.GetRegistrarConnectionCount(ctx, "REG123")
		require.NoError(t, err)
		assert.Equal(t, 1, regCount)
	})

	t.Run("multiple increments", func(t *testing.T) {
		client.FlushDB(ctx)

		for i := 0; i < 3; i++ {
			err := rl.IncrementConnection(ctx, "192.168.1.3", "REG456")
			require.NoError(t, err)
		}

		ipCount, err := rl.GetConnectionCount(ctx, "192.168.1.3")
		require.NoError(t, err)
		assert.Equal(t, 3, ipCount)

		regCount, err := rl.GetRegistrarConnectionCount(ctx, "REG456")
		require.NoError(t, err)
		assert.Equal(t, 3, regCount)
	})
}

func TestDecrementConnection(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	rl := NewRateLimiter(client, nil, logger)
	ctx := context.Background()

	t.Run("decrements IP counter", func(t *testing.T) {
		client.FlushDB(ctx)

		// Set up initial count
		client.Set(ctx, "conn:ip:192.168.1.1", 3, time.Minute)

		err := rl.DecrementConnection(ctx, "192.168.1.1", "")
		require.NoError(t, err)

		count, err := rl.GetConnectionCount(ctx, "192.168.1.1")
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("decrements both IP and registrar counters", func(t *testing.T) {
		client.FlushDB(ctx)

		// Set up initial counts
		client.Set(ctx, "conn:ip:192.168.1.2", 5, time.Minute)
		client.Set(ctx, "conn:reg:REG123", 10, time.Minute)

		err := rl.DecrementConnection(ctx, "192.168.1.2", "REG123")
		require.NoError(t, err)

		ipCount, err := rl.GetConnectionCount(ctx, "192.168.1.2")
		require.NoError(t, err)
		assert.Equal(t, 4, ipCount)

		regCount, err := rl.GetRegistrarConnectionCount(ctx, "REG123")
		require.NoError(t, err)
		assert.Equal(t, 9, regCount)
	})
}

func TestCheckRequestRate(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	config := &RateLimitConfig{
		BurstSize:     5,
		RequestWindow: 1 * time.Second,
	}
	rl := NewRateLimiter(client, config, logger)
	ctx := context.Background()

	t.Run("allows requests under limit", func(t *testing.T) {
		client.FlushDB(ctx)

		for i := 0; i < 4; i++ {
			err := rl.CheckRequestRate(ctx, "REG123")
			assert.NoError(t, err)
		}
	})

	t.Run("blocks requests over burst limit", func(t *testing.T) {
		client.FlushDB(ctx)

		// Fill up to burst limit
		for i := 0; i < 5; i++ {
			rl.CheckRequestRate(ctx, "REG456")
		}

		// Next request should be blocked
		err := rl.CheckRequestRate(ctx, "REG456")
		assert.ErrorIs(t, err, ErrRateLimitExceeded)
	})
}

func TestFailedLoginTracking(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	config := &RateLimitConfig{
		MaxFailedLogins: 3,
		LockoutDuration: 5 * time.Minute,
	}
	rl := NewRateLimiter(client, config, logger)
	ctx := context.Background()

	t.Run("tracks failed login attempts", func(t *testing.T) {
		client.FlushDB(ctx)

		err := rl.RecordFailedLogin(ctx, "user1", "192.168.1.1")
		assert.NoError(t, err)

		err = rl.RecordFailedLogin(ctx, "user1", "192.168.1.1")
		assert.NoError(t, err)
	})

	t.Run("locks account after max failed attempts", func(t *testing.T) {
		client.FlushDB(ctx)

		// Record 3 failed attempts (max)
		for i := 0; i < 2; i++ {
			err := rl.RecordFailedLogin(ctx, "user2", "192.168.1.2")
			assert.NoError(t, err)
		}

		// Third attempt should lock the account
		err := rl.RecordFailedLogin(ctx, "user2", "192.168.1.2")
		assert.ErrorIs(t, err, ErrAccountLocked)

		// Check if account is locked
		locked, err := rl.IsAccountLocked(ctx, "user2")
		require.NoError(t, err)
		assert.True(t, locked)
	})

	t.Run("clears failed logins after successful login", func(t *testing.T) {
		client.FlushDB(ctx)

		// Record some failed attempts
		rl.RecordFailedLogin(ctx, "user3", "192.168.1.3")
		rl.RecordFailedLogin(ctx, "user3", "192.168.1.3")

		// Clear after successful login
		err := rl.ClearFailedLogins(ctx, "user3", "192.168.1.3")
		require.NoError(t, err)

		// Next login should start fresh
		err = rl.RecordFailedLogin(ctx, "user3", "192.168.1.3")
		assert.NoError(t, err)
	})

	t.Run("account not locked returns false", func(t *testing.T) {
		client.FlushDB(ctx)

		locked, err := rl.IsAccountLocked(ctx, "user4")
		require.NoError(t, err)
		assert.False(t, locked)
	})
}

func TestGetStats(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	rl := NewRateLimiter(client, nil, logger)
	ctx := context.Background()

	t.Run("returns statistics", func(t *testing.T) {
		client.FlushDB(ctx)

		// Create some connections
		rl.IncrementConnection(ctx, "192.168.1.1", "REG1")
		rl.IncrementConnection(ctx, "192.168.1.2", "REG2")
		rl.IncrementConnection(ctx, "192.168.1.3", "REG1")

		stats, err := rl.GetStats(ctx)
		require.NoError(t, err)

		assert.Equal(t, 3, stats["total_ip_connections"])
		assert.Equal(t, 2, stats["total_registrar_connections"])
		assert.NotNil(t, stats["config"])
	})
}
