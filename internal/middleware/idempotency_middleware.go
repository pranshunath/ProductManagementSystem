package middleware

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"producthub/internal/cache"
	"producthub/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// IdempotencyConfig configures the idempotency protection middleware
type IdempotencyConfig struct {
	Scope    string        // Prefix for the idempotency key (e.g. "order")
	TTL      time.Duration // Time to retain completed responses (default 24h)
	LockTTL  time.Duration // Time to retain in-progress lock (default 2m)
}

// Idempotency creates a Fiber middleware that guarantees idempotency for mutations
// by caching successful responses and preventing duplicate concurrent transactions.
func Idempotency(cacheService cache.CacheService, cfg IdempotencyConfig) fiber.Handler {
	if cfg.Scope == "" {
		cfg.Scope = "mutation"
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 24 * time.Hour
	}
	if cfg.LockTTL <= 0 {
		cfg.LockTTL = 2 * time.Minute
	}

	return func(c *fiber.Ctx) error {
		// Only mutation methods support idempotency
		method := c.Method()
		if method != fiber.MethodPost && method != fiber.MethodPut && method != fiber.MethodPatch {
			return c.Next()
		}

		keyHeader := strings.TrimSpace(c.Get("Idempotency-Key"))
		if keyHeader == "" {
			// No idempotency key provided; proceed normally
			return c.Next()
		}

		if len(keyHeader) < 4 || len(keyHeader) > 128 {
			return response.Error(c, fiber.StatusBadRequest, "INVALID_IDEMPOTENCY_KEY", "Idempotency-Key header must be between 4 and 128 characters")
		}

		if cacheService == nil {
			return c.Next()
		}

		ctx := c.Context()
		cacheKey := fmt.Sprintf("idempotency:%s:%s", cfg.Scope, keyHeader)

		// 1. Check if this idempotency key was previously seen
		cachedVal, err := cacheService.Get(ctx, cacheKey)
		if err == nil {
			if cachedVal == "IN_PROGRESS" {
				return response.Error(c, fiber.StatusConflict, "IDEMPOTENCY_IN_PROGRESS", "A request with this Idempotency-Key is currently being processed. Please wait.")
			}
			if strings.HasPrefix(cachedVal, "COMPLETED:") {
				// Replay previous response
				body := strings.TrimPrefix(cachedVal, "COMPLETED:")
				c.Set("X-Idempotency", "HIT")
				c.Set("Content-Type", "application/json")
				return c.Status(fiber.StatusOK).SendString(body)
			}
		}

		// 2. Acquire in-progress lock using SetNX
		acquired, err := cacheService.SetNX(ctx, cacheKey, "IN_PROGRESS", cfg.LockTTL)
		if err != nil || !acquired {
			return response.Error(c, fiber.StatusConflict, "IDEMPOTENCY_IN_PROGRESS", "A request with this Idempotency-Key is currently being processed.")
		}

		// 3. Execute the handler pipeline
		execErr := c.Next()

		// 4. Inspect status code
		statusCode := c.Response().StatusCode()
		if execErr == nil && statusCode >= 200 && statusCode < 300 {
			// Cache successful response body
			responseBody := string(c.Response().Body())
			_ = cacheService.Set(ctx, cacheKey, "COMPLETED:"+responseBody, cfg.TTL)
			c.Set("X-Idempotency", "BYPASS")
		} else {
			// Mutation failed or had an error: release key so client can retry
			_ = cacheService.Delete(ctx, cacheKey)
		}

		return execErr
	}
}

// InvalidateIdempotencyKey clears a specific idempotency key
func InvalidateIdempotencyKey(cacheService cache.CacheService, scope, key string) error {
	if cacheService == nil || key == "" {
		return nil
	}
	cacheKey := fmt.Sprintf("idempotency:%s:%s", scope, key)
	return cacheService.Delete(context.Background(), cacheKey)
}

// IsErrCacheMiss checks if error is cache miss
func IsErrCacheMiss(err error) bool {
	return errors.Is(err, cache.ErrCacheMiss)
}
