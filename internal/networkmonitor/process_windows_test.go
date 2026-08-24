//go:build windows

package networkmonitor

import (
	"testing"
	"unsafe"
)

func TestTCPStatsDataLayoutMatchesWindowsABI(t *testing.T) {
	if size := unsafe.Sizeof(mibTCPRowOwnerPID{}); size != 24 {
		t.Fatalf("unexpected MIB_TCPROW_OWNER_PID size: %d", size)
	}
	if port := networkPort(0x5000); port != 80 {
		t.Fatalf("expected network-order port 80, got %d", port)
	}
}
