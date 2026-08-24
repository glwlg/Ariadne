package networkmonitor

import (
	"testing"
	"time"
)

func TestTrafficAccumulatorCoversTCPUDPIPv4IPv6(t *testing.T) {
	collector := newTrafficAccumulator()
	for _, eventID := range []uint16{10, 26, 42, 58} {
		collector.record(eventID, 7, 100)
	}
	for _, eventID := range []uint16{11, 27, 43, 59} {
		collector.record(eventID, 7, 200)
	}
	collector.record(12, 7, 500)

	snapshot := collector.snapshot(time.Now())
	if len(snapshot.Processes) != 1 {
		t.Fatalf("processes = %#v", snapshot.Processes)
	}
	process := snapshot.Processes[0]
	if process.BytesSent != 400 || process.BytesReceived != 800 {
		t.Fatalf("totals = %#v", process)
	}
}

func TestTrafficAccumulatorSnapshotIsNonDestructive(t *testing.T) {
	collector := newTrafficAccumulator()
	collector.record(10, 9, 1024)

	first := collector.snapshot(time.Now())
	second := collector.snapshot(time.Now())
	if len(first.Processes) != 1 || len(second.Processes) != 1 || second.Processes[0].BytesSent != 1024 {
		t.Fatalf("snapshot should not consume cumulative counters: first=%#v second=%#v", first, second)
	}
}
