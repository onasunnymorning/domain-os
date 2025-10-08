package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrTooManyConnections          = fmt.Errorf("too many connections from this IP")
	ErrTooManyRegistrarConnections = fmt.Errorf("too many connections for this registrar")
	ErrRateLimitExceeded           = fmt.Errorf("rate limit exceeded")
	ErrAccountLocked               = fmt.Errorf("account locked due to failed login attempts")
)

// RateLimitConfig defines the rate limiting configuration
type RateLimitConfig struct {
	// Connection limits
	MaxConnPerIP        int           // e.g., 10 connections per IP
	MaxConnPerRegistrar int           // e.g., 100 connections per registrar
	ConnTTL             time.Duration // How long to track connections

	// Request rate limits
	RequestsPerSecond int           // e.g., 100 req/s per registrar
	BurstSize         int           // e.g., 200 burst
	RequestWindow     time.Duration // e.g., 1 second

	// Failed login limits
	MaxFailedLogins int           // e.g., 5 failed attempts
	LockoutDuration time.Duration // e.g., 15 minutes
}

// DefaultRateLimitConfig returns a sensible default configuration
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		MaxConnPerIP:        10,
		MaxConnPerRegistrar: 100,
		ConnTTL:             5 * time.Minute,
		RequestsPerSecond:   100,
		BurstSize:           200,
		RequestWindow:       time.Second,
		MaxFailedLogins:     5,
		LockoutDuration:     15 * time.Minute,
	}
}

// RateLimiter handles connection and request rate limiting
type RateLimiter struct {
	redis  *redis.Client
	config *RateLimitConfig
	logger *slog.Logger
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(redisClient *redis.Client, config *RateLimitConfig, logger *slog.Logger) *RateLimiter {
	if config == nil {
		config = DefaultRateLimitConfig()
	}

	return &RateLimiter{
		redis:  redisClient,
		config: config,
		logger: logger,
	}
}

// CheckConnectionLimit verifies if a new connection is allowed
func (rl *RateLimiter) CheckConnectionLimit(ctx context.Context, clientIP string, registrarID string) error {
	// Check IP-based limit
	ipKey := fmt.Sprintf("conn:ip:%s", clientIP)
	ipConns, err := rl.redis.Get(ctx, ipKey).Int()
	if err != nil && err != redis.Nil {
		rl.logger.Error("Failed to get IP connection count", "error", err, "ip", clientIP)
		return err
	}

	if ipConns >= rl.config.MaxConnPerIP {
		rl.logger.Warn("IP connection limit exceeded",
			"ip", clientIP,
			"current", ipConns,
			"limit", rl.config.MaxConnPerIP)
		return ErrTooManyConnections
	}

	// Check registrar-based limit if registrar ID is provided
	if registrarID != "" {
		regKey := fmt.Sprintf("conn:reg:%s", registrarID)
		regConns, err := rl.redis.Get(ctx, regKey).Int()
		if err != nil && err != redis.Nil {
			rl.logger.Error("Failed to get registrar connection count",
				"error", err,
				"registrar", registrarID)
			return err
		}

		if regConns >= rl.config.MaxConnPerRegistrar {
			rl.logger.Warn("Registrar connection limit exceeded",
				"registrar", registrarID,
				"current", regConns,
				"limit", rl.config.MaxConnPerRegistrar)
			return ErrTooManyRegistrarConnections
		}
	}

	return nil
}

// IncrementConnection increments the connection count for an IP and optionally a registrar
func (rl *RateLimiter) IncrementConnection(ctx context.Context, clientIP string, registrarID string) error {
	pipe := rl.redis.Pipeline()

	// Increment IP counter
	ipKey := fmt.Sprintf("conn:ip:%s", clientIP)
	pipe.Incr(ctx, ipKey)
	pipe.Expire(ctx, ipKey, rl.config.ConnTTL)

	// Increment registrar counter if provided
	if registrarID != "" {
		regKey := fmt.Sprintf("conn:reg:%s", registrarID)
		pipe.Incr(ctx, regKey)
		pipe.Expire(ctx, regKey, rl.config.ConnTTL)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		rl.logger.Error("Failed to increment connection counters",
			"error", err,
			"ip", clientIP,
			"registrar", registrarID)
		return err
	}

	rl.logger.Debug("Connection counter incremented",
		"ip", clientIP,
		"registrar", registrarID)

	return nil
}

// DecrementConnection decrements the connection count for an IP and optionally a registrar
func (rl *RateLimiter) DecrementConnection(ctx context.Context, clientIP string, registrarID string) error {
	pipe := rl.redis.Pipeline()

	// Decrement IP counter
	ipKey := fmt.Sprintf("conn:ip:%s", clientIP)
	pipe.Decr(ctx, ipKey)

	// Decrement registrar counter if provided
	if registrarID != "" {
		regKey := fmt.Sprintf("conn:reg:%s", registrarID)
		pipe.Decr(ctx, regKey)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		rl.logger.Error("Failed to decrement connection counters",
			"error", err,
			"ip", clientIP,
			"registrar", registrarID)
		return err
	}

	rl.logger.Debug("Connection counter decremented",
		"ip", clientIP,
		"registrar", registrarID)

	return nil
}

// GetConnectionCount returns the current connection count for an IP
func (rl *RateLimiter) GetConnectionCount(ctx context.Context, clientIP string) (int, error) {
	ipKey := fmt.Sprintf("conn:ip:%s", clientIP)
	count, err := rl.redis.Get(ctx, ipKey).Int()
	if err == redis.Nil {
		return 0, nil
	}
	return count, err
}

// GetRegistrarConnectionCount returns the current connection count for a registrar
func (rl *RateLimiter) GetRegistrarConnectionCount(ctx context.Context, registrarID string) (int, error) {
	regKey := fmt.Sprintf("conn:reg:%s", registrarID)
	count, err := rl.redis.Get(ctx, regKey).Int()
	if err == redis.Nil {
		return 0, nil
	}
	return count, err
}

// CheckRequestRate verifies if a request is allowed based on rate limits
func (rl *RateLimiter) CheckRequestRate(ctx context.Context, registrarID string) error {
	key := fmt.Sprintf("rate:req:%s", registrarID)

	// Use Redis to implement token bucket algorithm
	// Get current count
	count, err := rl.redis.Get(ctx, key).Int()
	if err == redis.Nil {
		// First request, initialize
		err = rl.redis.Set(ctx, key, 1, rl.config.RequestWindow).Err()
		return err
	}
	if err != nil {
		return err
	}

	// Check if within burst limit
	if count >= rl.config.BurstSize {
		rl.logger.Warn("Request rate limit exceeded",
			"registrar", registrarID,
			"current", count,
			"limit", rl.config.BurstSize)
		return ErrRateLimitExceeded
	}

	// Increment counter
	pipe := rl.redis.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, rl.config.RequestWindow)
	_, err = pipe.Exec(ctx)

	return err
}

// RecordFailedLogin tracks failed login attempts
func (rl *RateLimiter) RecordFailedLogin(ctx context.Context, username string, ip string) error {
	key := fmt.Sprintf("failed:login:%s:%s", username, ip)

	// Increment failed login counter
	count, err := rl.redis.Incr(ctx, key).Result()
	if err != nil {
		return err
	}

	// Set TTL on first failed attempt
	if count == 1 {
		rl.redis.Expire(ctx, key, rl.config.LockoutDuration)
	}

	rl.logger.Warn("Failed login attempt recorded",
		"username", username,
		"ip", ip,
		"attempt", count,
		"max", rl.config.MaxFailedLogins)

	// Check if account should be locked
	if count >= int64(rl.config.MaxFailedLogins) {
		// Lock the account
		lockKey := fmt.Sprintf("locked:%s", username)
		err := rl.redis.Set(ctx, lockKey, "1", rl.config.LockoutDuration).Err()
		if err != nil {
			return err
		}

		rl.logger.Error("Account locked due to failed login attempts",
			"username", username,
			"ip", ip,
			"attempts", count)

		return ErrAccountLocked
	}

	return nil
}

// IsAccountLocked checks if an account is locked
func (rl *RateLimiter) IsAccountLocked(ctx context.Context, username string) (bool, error) {
	lockKey := fmt.Sprintf("locked:%s", username)
	exists, err := rl.redis.Exists(ctx, lockKey).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// ClearFailedLogins clears failed login attempts for a user (after successful login)
func (rl *RateLimiter) ClearFailedLogins(ctx context.Context, username string, ip string) error {
	key := fmt.Sprintf("failed:login:%s:%s", username, ip)
	return rl.redis.Del(ctx, key).Err()
}

// GetStats returns current rate limiter statistics
func (rl *RateLimiter) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Get all connection keys using SCAN instead of KEYS for better performance
	var ipKeys []string
	var cursor uint64
	for {
		var keys []string
		var err error
		keys, cursor, err = rl.redis.Scan(ctx, cursor, "conn:ip:*", 1000).Result()
		if err != nil {
			return nil, err
		}
		ipKeys = append(ipKeys, keys...)
		if cursor == 0 {
			break
		}
	}

	var regKeys []string
	cursor = 0
	for {
		var keys []string
		var err error
		keys, cursor, err = rl.redis.Scan(ctx, cursor, "conn:reg:*", 1000).Result()
		if err != nil {
			return nil, err
		}
		regKeys = append(regKeys, keys...)
		if cursor == 0 {
			break
		}
	}

	stats["total_ip_connections"] = len(ipKeys)
	stats["total_registrar_connections"] = len(regKeys)
	stats["config"] = rl.config

	return stats, nil
}
