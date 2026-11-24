package networkmonitor

import "sync"

type processIdentityCache struct {
	mu      sync.Mutex
	entries map[uint32][2]string
}

func newProcessIdentityCache() *processIdentityCache {
	return &processIdentityCache{
		entries: make(map[uint32][2]string),
	}
}

func (c *processIdentityCache) resolve(pid uint32, active map[uint32]struct{}, lookup func(uint32) [2]string) [2]string {
	if active != nil {
		active[pid] = struct{}{}
	}
	c.mu.Lock()
	if cached, ok := c.entries[pid]; ok {
		c.mu.Unlock()
		return cached
	}
	c.mu.Unlock()

	resolved := lookup(pid)
	c.mu.Lock()
	c.entries[pid] = resolved
	c.mu.Unlock()
	return resolved
}

func (c *processIdentityCache) evictExcept(active map[uint32]struct{}) {
	c.mu.Lock()
	for pid := range c.entries {
		if _, ok := active[pid]; !ok {
			delete(c.entries, pid)
		}
	}
	c.mu.Unlock()
}
