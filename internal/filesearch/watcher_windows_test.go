//go:build windows

package filesearch

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"golang.org/x/sys/windows"
)

func TestParseWindowsNotifyBufferReturnsCreatedPath(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "搜索测试.txt")
	if err := os.WriteFile(filePath, []byte("ok"), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	buffer := windowsNotifyBuffer(t, windows.FILE_ACTION_ADDED, "搜索测试.txt")

	results := parseWindowsNotifyBuffer(root, buffer)

	if len(results) != 1 {
		t.Fatalf("expected one changed path, got %#v", results)
	}
	if results[0].Name != "搜索测试.txt" || results[0].Path != filePath {
		t.Fatalf("expected changed file path, got %#v", results[0])
	}
}

func windowsNotifyBuffer(t *testing.T, action uint32, name string) []byte {
	t.Helper()
	units, err := syscall.UTF16FromString(name)
	if err != nil {
		t.Fatalf("encode notify name: %v", err)
	}
	units = units[:len(units)-1]
	buffer := make([]byte, windowsFileNotifyHeaderSize+len(units)*2)
	binary.LittleEndian.PutUint32(buffer[4:8], action)
	binary.LittleEndian.PutUint32(buffer[8:12], uint32(len(units)*2))
	for index, unit := range units {
		binary.LittleEndian.PutUint16(buffer[windowsFileNotifyHeaderSize+index*2:], unit)
	}
	return buffer
}
