package applog

import (
	"bytes"
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

func TestLogRotationBoundsLargeWritesAndKeepsNewestBackups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ariadne.log")
	service := NewServiceWithPath(path)
	if err := service.Start(); err != nil {
		t.Fatal(err)
	}
	defer service.Stop()
	for _, marker := range []byte{'A', 'B', 'C', 'D', 'E'} {
		data := bytes.Repeat([]byte{marker}, maxLogBytes)
		if n, err := service.Write(data); err != nil || n != len(data) {
			t.Fatalf("write: %d, %v", n, err)
		}
	}
	for name, marker := range map[string]byte{path: 'E', path + ".1": 'E', path + ".2": 'D', path + ".3": 'C'} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if len(data) == 0 || len(data) > maxLogBytes || data[len(data)-1] != marker {
			t.Fatalf("wrong rotated log %s: %d bytes", name, len(data))
		}
	}
	if _, err := os.Stat(path + ".4"); !os.IsNotExist(err) {
		t.Fatalf("unexpected extra backup: %v", err)
	}
}

func TestLogRotationHandlesExistingLogAndReportsFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ariadne.log")
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), maxLogBytes), 0600); err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithPath(path)
	if err := service.Start(); err != nil {
		t.Fatal(err)
	}
	defer service.Stop()
	if data, err := os.ReadFile(path + ".1"); err != nil || len(data) != maxLogBytes {
		t.Fatalf("existing log not archived: %v", err)
	}
	// A nonempty directory blocks removal of the oldest backup on every platform.
	if err := os.Mkdir(path+".3", 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path+".3", "blocked"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Write(bytes.Repeat([]byte("y"), maxLogBytes)); err == nil {
		t.Fatal("expected rotation failure")
	}
	if service.Status().LastError == "" {
		t.Fatal("rotation failure was hidden")
	}
	if _, err := service.Write([]byte("next")); err == nil {
		t.Fatal("failed writer silently discarded data")
	}
}
