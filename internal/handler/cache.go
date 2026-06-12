package handler

import (
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// responseCache is a small in-memory TTL + LRU cache for marshaled
// responses, keyed by endpoint + request body hash.
//
// Because the key covers the entire request body, any change to a
// transport file (legs or stops added/removed, another vehicle assigned)
// changes the waypoints or vehicle in the request and therefore misses
// the cache — stale routes cannot be served for modified files. The TTL
// bounds the remaining staleness sources (tariff updates, the weekly
// OSM tile rebuild); deploys replace the instance and start empty.
type responseCache struct {
	mu      sync.Mutex
	entries map[string]*list.Element
	order   *list.List // front = most recently used
	maxSize int
	ttl     time.Duration
}

type cacheEntry struct {
	key     string
	body    []byte
	expires time.Time
}

func newResponseCache(maxSize int, ttl time.Duration) *responseCache {
	return &responseCache{
		entries: make(map[string]*list.Element),
		order:   list.New(),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Key builds the cache key for an endpoint and raw request body.
func (c *responseCache) Key(endpoint string, body []byte) string {
	sum := sha256.Sum256(body)
	return endpoint + ":" + hex.EncodeToString(sum[:])
}

func (c *responseCache) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	element, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	entry := element.Value.(*cacheEntry)
	if time.Now().After(entry.expires) {
		c.order.Remove(element)
		delete(c.entries, key)
		return nil, false
	}
	c.order.MoveToFront(element)
	return entry.body, true
}

func (c *responseCache) Set(key string, body []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if element, ok := c.entries[key]; ok {
		entry := element.Value.(*cacheEntry)
		entry.body = body
		entry.expires = time.Now().Add(c.ttl)
		c.order.MoveToFront(element)
		return
	}

	for len(c.entries) >= c.maxSize {
		oldest := c.order.Back()
		if oldest == nil {
			break
		}
		c.order.Remove(oldest)
		delete(c.entries, oldest.Value.(*cacheEntry).key)
	}

	element := c.order.PushFront(&cacheEntry{
		key:     key,
		body:    body,
		expires: time.Now().Add(c.ttl),
	})
	c.entries[key] = element
}
