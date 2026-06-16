package applog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServiceWritesLogFileAndReportsStatus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "ariadne.log")
	service := NewServiceWithPath(path)

	if err := service.Start(); err != nil {
		t.Fatalf("start log service: %v", err)
	}
	if _, err := service.Write([]byte("runtime event\n")); err != nil {
		t.Fatalf("write log: %v", err)
	}
	if err := service.Stop(); err != nil {
		t.Fatalf("stop log service: %v", err)
	}

	status := service.Status()
	if !status.DirectoryExists || !status.Exists || status.Bytes == 0 {
		t.Fatalf("expected log status to expose written file: %#v", status)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(raw), "ariadne log started") || !strings.Contains(string(raw), "runtime event") {
		t.Fatalf("log file missing expected entries: %q", string(raw))
	}
}
