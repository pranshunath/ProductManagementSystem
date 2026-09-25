package middleware

import (
	"fmt"
	"strconv"
	"time"

	"producthub/internal/cache"
	"producthub/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// RateLimitConfig holds configuration for the rate limiting middleware
type RateLimitConfig struct {
	Scope        string                    // Scope/prefix to partition rate limit keys (e.g., "global", "auth", "orders")
	Max          int                       // Maximum number of requests allowed in the window
	Window       time.Duration             // Duration of the rate limit window
	KeyGenerator func(c *fiber.Ctx) string // Custom function to extract rate limit key
	Skip         func(c *fiber.Ctx) bool   // Optional function to skip rate limiting
}

// RateLimiter creates a Fiber middleware that enforces rate limiting backed by Redis/CacheService
func RateLimiter(cacheService cache.CacheService, cfg RateLimitConfig) fiber.Handler {
	if cfg.Max <= 0 {
		cfg.Max = 60
	}
	if cfg.Window <= 0 {
		cfg.Window = 1 * time.Minute
	}
	if cfg.Scope == "" {
		cfg.Scope = "api"
	}

	return func(c *fiber.Ctx) error {
		// Check if request should skip rate limiting
		if cfg.Skip != nil && cfg.Skip(c) {
			return c.Next()
		}

		if cacheService == nil {
			return c.Next()
		}

		// Determine client identifier
		var identifier string
		if cfg.KeyGenerator != nil {
			identifier = cfg.KeyGenerator(c)
		} else {
			// If authenticated, rate limit per user; otherwise rate limit per IP
			if userID, err := GetAuthenticatedUserID(c); err == nil && userID > 0 {
				identifier = fmt.Sprintf("user:%d", userID)
			} else {
				ip := c.IP()
				if ip == "" {
					ip = "unknown"
				}
				identifier = "ip:" + ip
			}
		}

		key := fmt.Sprintf("ratelimit:%s:%s", cfg.Scope, identifier)

		// Atomically increment counter
		count, err := cacheService.Increment(c.Context(), key, cfg.Window)
		if err != nil {
			// Fail open on cache error so we do not block legitimate traffic
			return c.Next()
		}

		// Set rate limit headers
		c.Set("X-RateLimit-Limit", strconv.Itoa(cfg.Max))
		remaining := cfg.Max - int(count)
		if remaining < 0 {
			remaining = 0
		}
		c.Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

		// Check threshold
		if count > int64(cfg.Max) {
			c.Set("Retry-After", strconv.Itoa(int(cfg.Window.Seconds())))
			return response.Error(c, fiber.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Rate limit exceeded. Please try again later.")
		}

		return c.Next()
	}
}
