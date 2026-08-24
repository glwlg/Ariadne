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

func TestProcessSnapshotUsesAdapterTotalAndKeepsUnattributedTrafficVisible(t *testing.T) {
	now := time.Unix(100, 0)
	adapterCalls := 0
	service := NewServiceWithReader(func() ([]interfaceCounter, error) {
		adapterCalls++
		if adapterCalls == 1 {
			return []interfaceCounter{counterFixture(1000, 5000)}, nil
		}
		return []interfaceCounter{counterFixture(3048, 9096)}, nil
	})
	service.now = func() time.Time { return now }
	processCalls := 0
	service.processReader = func() ([]processCounter, error) {
		processCalls++
		sent := uint64(1000)
		received := uint64(2000)
		if processCalls > 1 {
			sent += 1024
			received += 2048
		}
		return []processCounter{
			{
				pid: 7, name: "browser.exe", path: `C:\browser.exe`,
				bytesSent: sent, bytesReceived: received,
				connections: []ProcessConnection{
					{LocalAddress: "127.0.0.1:1", RemoteAddress: "127.0.0.1:2"},
					{LocalAddress: "127.0.0.1:3", RemoteAddress: "127.0.0.1:4"},
				},
			},
		}, nil
	}
	service.SetProcessIconResolver(func(path string) string { return "/icons/browser.png" })

	first := service.ProcessSnapshot()
	if first.ProcessCount != 1 || first.ConnectionCount != 2 || first.DownloadBytesPerSecond != 0 {
		t.Fatalf("first snapshot should keep connections without inventing adapter rates: %#v", first)
	}

	now = now.Add(2 * time.Second)
	second := service.ProcessSnapshot()
	if second.DownloadBytesPerSecond != 2048 || second.UploadBytesPerSecond != 1024 {
		t.Fatalf("expected authoritative adapter rates, got %#v", second)
	}
	if len(second.Processes) != 2 || second.Processes[0].Name != "browser.exe" || len(second.Processes[0].Connections) != 2 {
		t.Fatalf("expected process plus unattributed residual, got %#v", second)
	}
	if second.Processes[0].IconURL != "/icons/browser.png" || second.Processes[1].Name != "系统/其他" {
		t.Fatalf("expected real icon and explicit residual, got %#v", second.Processes)
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
