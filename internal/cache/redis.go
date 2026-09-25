package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"producthub/internal/config"

	"github.com/redis/go-redis/v9"
)

var (
	// ErrCacheMiss is returned when a key is not found in cache
	ErrCacheMiss = errors.New("cache: key not found")
)

// ============================================================================
// Redis Implementation
// ============================================================================

// RedisCacheService implements CacheService using a live Redis instance
type RedisCacheService struct {
	client *redis.Client
}

// NewRedisCache attempts to connect to Redis. If Redis is unreachable,
// it logs a warning and gracefully falls back to an in-memory thread-safe cache.
func NewRedisCache(cfg *config.RedisConfig) CacheService {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr(),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: 20,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("[REDIS WARNING] Could not connect to Redis at %s (%v). Gracefully falling back to in-memory cache.", cfg.Addr(), err)
		return NewMemoryCache()
	}

	log.Printf("[REDIS] Connected to Redis instance at %s (DB: %d)", cfg.Addr(), cfg.DB)
	return &RedisCacheService{client: client}
}

// Get retrieves a value by key
func (r *RedisCacheService) Get(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrCacheMiss
	}
	return val, err
}

// Set stores a value with TTL expiration
func (r *RedisCacheService) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	var valStr string
	switch v := value.(type) {
	case string:
		valStr = v
	case []byte:
		valStr = string(v)
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("failed to marshal cache value: %w", err)
		}
		valStr = string(data)
	}
	return r.client.Set(ctx, key, valStr, expiration).Err()
}

// Delete removes one or more keys
func (r *RedisCacheService) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return r.client.Del(ctx, keys...).Err()
}

// DeletePrefix deletes all keys matching a prefix
func (r *RedisCacheService) DeletePrefix(ctx context.Context, prefix string) error {
	iter := r.client.Scan(ctx, 0, prefix+"*", 100).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
		if len(keys) >= 100 {
			if err := r.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
			keys = keys[:0]
		}
	}
	if err := iter.Err(); err != nil {
		return err
	}
	if len(keys) > 0 {
		return r.client.Del(ctx, keys...).Err()
	}
	return nil
}

// Increment atomically increments a key's integer counter and sets TTL if first touch
func (r *RedisCacheService) Increment(ctx context.Context, key string, expiration time.Duration) (int64, error) {
	pipe := r.client.TxPipeline()
	incrCmd := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, expiration)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}
	return incrCmd.Val(), nil
}

// SetNX sets a key if it does not already exist (distributed lock / idempotency token)
func (r *RedisCacheService) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	var valStr string
	switch v := value.(type) {
	case string:
		valStr = v
	case []byte:
		valStr = string(v)
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return false, fmt.Errorf("failed to marshal cache value: %w", err)
		}
		valStr = string(data)
	}
	return r.client.SetNX(ctx, key, valStr, expiration).Result()
}

// Ping checks Redis connectivity
func (r *RedisCacheService) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// IsAvailable indicates Redis connection is active
func (r *RedisCacheService) IsAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	return r.client.Ping(ctx).Err() == nil
}

// Close closes the Redis connection pool
func (r *RedisCacheService) Close() error {
	return r.client.Close()
}

// ============================================================================
// Thread-Safe In-Memory Fallback Implementation
// ============================================================================

type memItem struct {
	val       string
	expiresAt time.Time
}

// MemoryCacheService provides a thread-safe in-memory cache implementation
// suitable for testing or as a fallback when Redis is unavailable
type MemoryCacheService struct {
	mu    sync.RWMutex
	items map[string]memItem
}

// NewMemoryCache creates a new in-memory CacheService
func NewMemoryCache() *MemoryCacheService {
	m := &MemoryCacheService{
		items: make(map[string]memItem),
	}
	// Background garbage collection routine
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		for range ticker.C {
			m.cleanup()
		}
	}()
	return m
}

func (m *MemoryCacheService) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for k, item := range m.items {
		if !item.expiresAt.IsZero() && now.After(item.expiresAt) {
			delete(m.items, k)
		}
	}
}

// Get retrieves a key from memory
func (m *MemoryCacheService) Get(ctx context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, exists := m.items[key]
	if !exists {
		return "", ErrCacheMiss
	}
	if !item.expiresAt.IsZero() && time.Now().After(item.expiresAt) {
		return "", ErrCacheMiss
	}
	return item.val, nil
}

// Set stores a key in memory with TTL
func (m *MemoryCacheService) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	var valStr string
	switch v := value.(type) {
	case string:
		valStr = v
	case []byte:
		valStr = string(v)
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("failed to marshal cache value: %w", err)
		}
		valStr = string(data)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var exp time.Time
	if expiration > 0 {
		exp = time.Now().Add(expiration)
	}
	m.items[key] = memItem{
		val:       valStr,
		expiresAt: exp,
	}
	return nil
}

// Delete removes one or more keys
func (m *MemoryCacheService) Delete(ctx context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range keys {
		delete(m.items, k)
	}
	return nil
}

// DeletePrefix removes all keys matching prefix
func (m *MemoryCacheService) DeletePrefix(ctx context.Context, prefix string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k := range m.items {
		if strings.HasPrefix(k, prefix) {
			delete(m.items, k)
		}
	}
	return nil
}

// Increment atomically increments a counter
func (m *MemoryCacheService) Increment(ctx context.Context, key string, expiration time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, exists := m.items[key]
	var current int64 = 0
	now := time.Now()

	if exists && (item.expiresAt.IsZero() || now.Before(item.expiresAt)) {
		var val int64
		if _, err := fmt.Sscanf(item.val, "%d", &val); err == nil {
			current = val
		}
	}

	current++
	var exp time.Time
	if exists && !item.expiresAt.IsZero() && now.Before(item.expiresAt) {
		exp = item.expiresAt
	} else if expiration > 0 {
		exp = now.Add(expiration)
	}

	m.items[key] = memItem{
		val:       fmt.Sprintf("%d", current),
		expiresAt: exp,
	}
	return current, nil
}

// SetNX sets value only if key does not exist or has expired
func (m *MemoryCacheService) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	var valStr string
	switch v := value.(type) {
	case string:
		valStr = v
	case []byte:
		valStr = string(v)
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return false, fmt.Errorf("failed to marshal cache value: %w", err)
		}
		valStr = string(data)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	item, exists := m.items[key]
	if exists && (item.expiresAt.IsZero() || now.Before(item.expiresAt)) {
		return false, nil // Key already exists and is active
	}

	var exp time.Time
	if expiration > 0 {
		exp = now.Add(expiration)
	}
	m.items[key] = memItem{
		val:       valStr,
		expiresAt: exp,
	}
	return true, nil
}

// Ping checks memory cache availability (always nil)
func (m *MemoryCacheService) Ping(ctx context.Context) error {
	return nil
}

// IsAvailable memory cache is always available
func (m *MemoryCacheService) IsAvailable() bool {
	return true
}

// Close clears memory cache
func (m *MemoryCacheService) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = make(map[string]memItem)
	return nil
}
