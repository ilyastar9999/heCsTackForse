package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache is a thin wrapper around a Redis/Valkey client.
// When disabled (URL is empty) all operations become no-ops.
type Cache struct {
	client *redis.Client
	prefix string
	noop   bool
}

// New creates a Cache. If url is empty the cache is disabled (all ops no-op).
func New(url, prefix string) *Cache {
	if strings.TrimSpace(url) == "" {
		return &Cache{noop: true}
	}
	if prefix == "" {
		prefix = "hectackforse:"
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		log.Printf("warning: failed to parse cache URL %q: %v; cache disabled", url, err)
		return &Cache{noop: true}
	}
	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("warning: cache ping failed: %v; cache disabled", err)
		_ = client.Close()
		return &Cache{noop: true}
	}
	log.Printf("cache connected to %s", url)
	return &Cache{client: client, prefix: prefix}
}

// Close shuts down the cache client.
func (c *Cache) Close() {
	if c.noop || c.client == nil {
		return
	}
	_ = c.client.Close()
}

// IsEnabled returns whether the cache is actually connected.
func (c *Cache) IsEnabled() bool { return !c.noop }

func (c *Cache) key(k string) string { return c.prefix + k }

// Get retrieves a cached value by key and deserializes it into dest.
// Returns true if the value was found and deserialized successfully.
func (c *Cache) Get(ctx context.Context, key string, dest any) bool {
	if c.noop {
		return false
	}
	data, err := c.client.Get(ctx, c.key(key)).Bytes()
	if err != nil {
		return false
	}
	return json.Unmarshal(data, dest) == nil
}

// Set stores a value under key with the given TTL. value is JSON-serialized.
func (c *Cache) Set(ctx context.Context, key string, value any, ttl time.Duration) {
	if c.noop {
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	_ = c.client.Set(ctx, c.key(key), data, ttl).Err()
}

// Invalidate deletes a single key.
func (c *Cache) Invalidate(ctx context.Context, key string) {
	if c.noop {
		return
	}
	_ = c.client.Del(ctx, c.key(key)).Err()
}

// InvalidatePrefix deletes all keys matching prefix + pattern.
// pattern should be a glob like "scoreboard:*" or "ad:*".
func (c *Cache) InvalidatePrefix(ctx context.Context, pattern string) {
	if c.noop {
		return
	}
	fullPattern := c.prefix + pattern
	keys, err := c.client.Keys(ctx, fullPattern).Result()
	if err != nil || len(keys) == 0 {
		return
	}
	_ = c.client.Del(ctx, keys...).Err()
}

// ParseDuration parses a duration string, falling back to the given default.
func ParseDuration(s string, fallback time.Duration) time.Duration {
	if d, err := time.ParseDuration(s); err == nil && d > 0 {
		return d
	}
	return fallback
}

// Key builds a colon-joined cache key string.
func Key(parts ...string) string {
	return strings.Join(parts, ":")
}

// FormatInt formats an int64 as a string for cache key usage.
func FormatInt(n int64) string {
	return fmt.Sprintf("%d", n)
}
