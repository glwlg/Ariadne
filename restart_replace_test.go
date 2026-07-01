package main

import (
	"testing"
	"time"
)

func TestRestartReplacementArgsWaitForOldPID(t *testing.T) {
	var gotPID int
	var gotTimeout time.Duration

	handled, err := handleRestartReplacementArgs([]string{"--ariadne-replace-pid", "1234"}, 55, func(pid int, timeout time.Duration) error {
		gotPID = pid
		gotTimeout = timeout
		return nil
	})

	if err != nil || !handled || gotPID != 1234 || gotTimeout <= 0 {
		t.Fatalf("replacement args should wait for old pid, handled=%v pid=%d timeout=%s err=%v", handled, gotPID, gotTimeout, err)
	}
}

func TestRestartReplacementArgsIgnoreUnrelatedArgs(t *testing.T) {
	handled, err := handleRestartReplacementArgs([]string{"--hidden"}, 55, func(pid int, timeout time.Duration) error {
		t.Fatalf("unexpected wait for pid %d", pid)
		return nil
	})

	if err != nil || handled {
		t.Fatalf("unrelated args should be ignored, handled=%v err=%v", handled, err)
	}
}

func TestRestartReplacementArgsRejectInvalidPID(t *testing.T) {
	handled, err := handleRestartReplacementArgs([]string{"--ariadne-replace-pid", "bad"}, 55, nil)

	if !handled || err == nil {
		t.Fatalf("invalid replacement pid should be reported, handled=%v err=%v", handled, err)
	}
}
