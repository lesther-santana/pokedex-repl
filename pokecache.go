package main

import (
	"sync"
	"time"
)

type Cache struct {
	cache map[string]cacheEntry
	mu    sync.RWMutex
}

type cacheEntry struct {
	createdAt time.Time
	val       []byte
}

func (c *Cache) Add(k string, v []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[k] = cacheEntry{createdAt: time.Now(), val: v}
}

func (c *Cache) Get(k string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.cache[k]
	if !ok {
		return nil, false
	}
	return v.val, ok
}

func (c *Cache) reapLoop(interval time.Duration) {
	q := time.Tick(interval)
	for range q {
		c.mu.Lock()
		for k, entry := range c.cache {
			if time.Since(entry.createdAt) >= interval {
				delete(c.cache, k)
			}
		}
		c.mu.Unlock()
	}
}

func NewCache(interval time.Duration) *Cache {
	new := Cache{cache: make(map[string]cacheEntry)}
	go new.reapLoop(interval)
	return &new
}
