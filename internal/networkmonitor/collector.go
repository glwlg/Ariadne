package networkmonitor

import (
	"sync"
	"time"
)

type privilegedProcessTraffic struct {
	PID           uint32 `json:"pid"`
	BytesSent     uint64 `json:"bytesSent"`
	BytesReceived uint64 `json:"bytesReceived"`
}

type privilegedTrafficSnapshot struct {
	Processes []privilegedProcessTraffic `json:"processes"`
	LastError string                     `json:"lastError,omitempty"`
}

type trafficBytes struct {
	sent     uint64
	received uint64
}

type trafficAccumulator struct {
	mu       sync.Mutex
	totals   map[uint32]trafficBytes
	lastSeen map[uint32]time.Time
}

func newTrafficAccumulator() *trafficAccumulator {
	return &trafficAccumulator{
		totals:   make(map[uint32]trafficBytes),
		lastSeen: make(map[uint32]time.Time),
	}
}

func (a *trafficAccumulator) record(eventID uint16, pid uint32, size uint64) {
	direction := networkEventDirection(eventID)
	if direction == 0 || size == 0 {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	total := a.totals[pid]
	if direction > 0 {
		total.sent += size
	} else {
		total.received += size
	}
	a.totals[pid] = total
	a.lastSeen[pid] = time.Now()
}

func (a *trafficAccumulator) snapshot(now time.Time) privilegedTrafficSnapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	processes := make([]privilegedProcessTraffic, 0, len(a.totals))
	for pid, total := range a.totals {
		if now.Sub(a.lastSeen[pid]) > time.Minute {
			delete(a.totals, pid)
			delete(a.lastSeen, pid)
			continue
		}
		processes = append(processes, privilegedProcessTraffic{
			PID:           pid,
			BytesSent:     total.sent,
			BytesReceived: total.received,
		})
	}
	return privilegedTrafficSnapshot{Processes: processes}
}

func networkEventDirection(eventID uint16) int {
	switch eventID {
	case 10, 26, 42, 58:
		return 1
	case 11, 27, 43, 59:
		return -1
	default:
		return 0
	}
}
