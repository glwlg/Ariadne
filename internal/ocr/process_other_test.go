//go:build !windows

package ocr

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRunLocalOCRBridgeTimeoutKillsSubprocessTree(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	fakePy := filepath.Join(dir, "fake_python.sh")
	script := "#!/bin/sh\n" +
		"sleep 30 &\n" +
		"CHILD=$!\n" +
		"echo \"$CHILD\" > \"" + pidFile + "\"\n" +
		"wait \"$CHILD\"\n"
	if err := os.WriteFile(fakePy, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FANCUN_OCR_PYTHON", fakePy)

	imagePath := testImage(t)
	service := NewService(nil, nil)
	service.timeout = 200 * time.Millisecond

	start := time.Now()
	result := service.RecognizeImagePath(imagePath)
	elapsed := time.Since(start)

	if result.OK || result.Error != "OCR 执行超时" {
		t.Fatalf("expected timeout error, got %#v", result)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("OCR timeout took too long (%v), subprocess tree may have lingered", elapsed)
	}

	rawPID, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("child pid file not written: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(rawPID)))
	if err != nil || pid <= 0 {
		t.Fatalf("invalid child pid %q: %v", string(rawPID), err)
	}
	// Give kernel a brief moment to reap/mark the SIGKILLed child process.
	deadline := time.Now().Add(time.Second)
	for {
		if processDeadOrZombie(pid) {
			break
		}
		if time.Now().After(deadline) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			statRaw, _ := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
			t.Fatalf("grandchild process %d survived OCR timeout (stat=%q)", pid, string(statRaw))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func processDeadOrZombie(pid int) bool {
	if err := syscall.Kill(pid, 0); err == syscall.ESRCH {
		return true
	}
	statRaw, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return true
	}
	// Format: "<pid> (<comm>) <state> ..."
	if idx := strings.LastIndex(string(statRaw), ")"); idx >= 0 && idx+2 < len(statRaw) {
		state := statRaw[idx+2]
		return state == 'Z' || state == 'X'
	}
	return false
}
