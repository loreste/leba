// This implementation was done by  on July 2024 by Lance, still trying to figure out if this is the best way forward.
package loadbalancer

import (
	"time"

	"github.com/patrickmn/go-cache"
)

// Cache represents the cache for storing query results or other data
type Cache struct {
	cache *cache.Cache
}

// CacheConfig holds the configuration for the cache
type CacheConfig struct {
	Enabled    bool
	Expiration time.Duration
	Cleanup    time.Duration
}

// NewCache creates a new Cache instance based on the given configuration
func NewCache(config CacheConfig) *Cache {
	if !config.Enabled {
		return nil
	}
	return &Cache{
		cache: cache.New(config.Expiration, config.Cleanup),
	}
}

// Set sets a key-value pair in the cache
func (c *Cache) Set(key string, value interface{}) {
	if c != nil && c.cache != nil {
		c.cache.Set(key, value, cache.DefaultExpiration)
	}
}

// Get retrieves a value from the cache by key
func (c *Cache) Get(key string) (interface{}, bool) {
	if c != nil && c.cache != nil {
		return c.cache.Get(key)
	}
	return nil, false
}

// Delete removes a key-value pair from the cache
func (c *Cache) Delete(key string) {
	if c != nil && c.cache != nil {
		c.cache.Delete(key)
	}
}

// Flush removes all items from the cache
func (c *Cache) Flush() {
	if c != nil && c.cache != nil {
		c.cache.Flush()
	}
}
