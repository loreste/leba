package loadbalancer

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

// Cache represents the cache for storing query results or other data
type Cache struct {
	cache     *cache.Cache
	enabled   bool
	hitCount  int64
	missCount int64
	mu        sync.RWMutex
}

// NewCache creates a new Cache instance based on the given configuration
func NewCache(expiration, cleanup time.Duration, enabled bool) *Cache {
	if !enabled {
		log.Println("Cache is disabled")
		return &Cache{enabled: false}
	}
	
	log.Printf("Initializing cache with expiration: %v, cleanup: %v", expiration, cleanup)
	return &Cache{
		cache:   cache.New(expiration, cleanup),
		enabled: true,
	}
}

// Set stores a key-value pair in the cache
func (c *Cache) Set(key string, value interface{}) {
	if !c.enabled || c.cache == nil {
		return
	}
	c.cache.Set(key, value, cache.DefaultExpiration)
}

// Get retrieves a value from the cache by key
func (c *Cache) Get(key string) (interface{}, bool) {
	if !c.enabled || c.cache == nil {
		c.mu.Lock()
		c.missCount++
		c.mu.Unlock()
		return nil, false
	}
	
	value, found := c.cache.Get(key)
	c.mu.Lock()
	if found {
		c.hitCount++
	} else {
		c.missCount++
	}
	c.mu.Unlock()
	
	return value, found
}

// Delete removes a key-value pair from the cache
func (c *Cache) Delete(key string) {
	if !c.enabled || c.cache == nil {
		return
	}
	c.cache.Delete(key)
}

// Flush removes all items from the cache
func (c *Cache) Flush() {
	if !c.enabled || c.cache == nil {
		return
	}
	c.cache.Flush()
	log.Println("Cache flushed")
}

// Stats returns statistics about the cache usage
func (c *Cache) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	stats := map[string]interface{}{
		"enabled":    c.enabled,
		"hit_count":  c.hitCount,
		"miss_count": c.missCount,
	}
	
	if c.enabled && c.cache != nil {
		itemCount := c.cache.ItemCount()
		stats["item_count"] = itemCount
		
		hitRatio := 0.0
		totalRequests := c.hitCount + c.missCount
		if totalRequests > 0 {
			hitRatio = float64(c.hitCount) / float64(totalRequests) * 100.0
		}
		stats["hit_ratio"] = hitRatio
	}
	
	return stats
}

// StatsJSON returns JSON-formatted statistics about the cache usage
func (c *Cache) StatsJSON() (string, error) {
	stats := c.Stats()
	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}