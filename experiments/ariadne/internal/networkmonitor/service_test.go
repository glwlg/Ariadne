package networkmonitor

import (
	"errors"
	"testing"
	"time"
)

func TestSnapshotComputesTrafficRates(t *testing.T) {
	now := time.Unix(100, 0)
	calls := 0
	service := NewServiceWithReader(func() ([]interfaceCounter, error) {
		calls++
		if calls == 1 {
			return []interfaceCounter{counterFixture(1000, 5000)}, nil
		}
		return []interfaceCounter{counterFixture(3048, 7048)}, nil
	})
	service.now = func() time.Time { return now }

	first := service.Snapshot()
	if first.UploadBytesPerSecond != 0 || first.DownloadBytesPerSecond != 0 {
		t.Fatalf("first snapshot should not invent rates: %#v", first)
	}

	now = now.Add(2 * time.Second)
	second := service.Snapshot()

	if second.BytesSent != 3048 || second.BytesReceived != 7048 {
		t.Fatalf("unexpected totals: %#v", second)
	}
	if second.UploadBytesPerSecond != 1024 || second.DownloadBytesPerSecond != 1024 {
		t.Fatalf("expected 1024 B/s rates, got %#v", second)
	}
	if second.ActiveAdapterCount != 1 || len(second.Adapters) != 1 {
		t.Fatalf("expected active adapter summary: %#v", second)
	}
}

func TestSnapshotKeepsCounterErrorsVisible(t *testing.T) {
	service := NewServiceWithReader(func() ([]interfaceCounter, error) {
		return nil, errors.New("counter unavailable")
	})

	snapshot := service.Snapshot()

	if snapshot.LastError != "counter unavailable" {
		t.Fatalf("expected counter error in snapshot, got %#v", snapshot)
	}
}

func TestBytesPerSecondIgnoresCounterReset(t *testing.T) {
	if got := bytesPerSecond(10, 20, 1); got != 0 {
		t.Fatalf("counter reset should produce zero rate, got %f", got)
	}
}

func counterFixture(sent uint64, received uint64) interfaceCounter {
	return interfaceCounter{
		name:                   "Ethernet",
		alias:                  "Ethernet",
		description:            "Test adapter",
		interfaceIndex:         7,
		operational:            true,
		transmitLinkBitsPerSec: 1_000_000_000,
		receiveLinkBitsPerSec:  1_000_000_000,
		bytesSent:              sent,
		bytesReceived:          received,
	}
}
