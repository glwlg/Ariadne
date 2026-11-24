package networkmonitor

import (
	"fmt"
	"testing"
)

func TestProcessIdentityCacheCachesAndEvictsExitedPIDs(t *testing.T) {
	cache := newProcessIdentityCache()
	lookups := 0
	lookup := func(pid uint32) [2]string {
		lookups++
		return [2]string{fmt.Sprintf("proc-%d", pid), fmt.Sprintf("C:\\bin\\proc-%d.exe", pid)}
	}

	// Snapshot 1: PIDs 10 and 20 active.
	active1 := map[uint32]struct{}{}
	got10 := cache.resolve(10, active1, lookup)
	got20 := cache.resolve(20, active1, lookup)
	cache.evictExcept(active1)
	if lookups != 2 || got10[0] != "proc-10" || got20[0] != "proc-20" {
		t.Fatalf("unexpected first snapshot results: lookups=%d got10=%v got20=%v", lookups, got10, got20)
	}

	// Snapshot 2: PID 10 still active (cache hit), PID 20 exited.
	active2 := map[uint32]struct{}{}
	cache.resolve(10, active2, lookup)
	cache.evictExcept(active2)
	if lookups != 2 {
		t.Fatalf("expected cache hit for surviving PID 10, got lookups=%d", lookups)
	}

	// Snapshot 3: PID 20 reused after eviction -> fresh lookup required.
	active3 := map[uint32]struct{}{}
	cache.resolve(10, active3, lookup)
	cache.resolve(20, active3, lookup)
	cache.evictExcept(active3)
	if lookups != 3 {
		t.Fatalf("expected evicted PID 20 to re-resolve when reused, got lookups=%d", lookups)
	}
}
