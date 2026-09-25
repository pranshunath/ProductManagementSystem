package cache

import (
	"context"
	"time"
)

// CacheService defines the contract for caching, rate limiting, and idempotency
type CacheService interface {
	// Basic key-value operations
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Delete(ctx context.Context, keys ...string) error
	DeletePrefix(ctx context.Context, prefix string) error

	// Atomic counters for rate limiting
	Increment(ctx context.Context, key string, expiration time.Duration) (int64, error)

	// Distributed locking / idempotency primitives
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error)

	// Connectivity and diagnostics
	Ping(ctx context.Context) error
	IsAvailable() bool
	Close() error
}
