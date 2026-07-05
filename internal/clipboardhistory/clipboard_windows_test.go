//go:build windows

package clipboardhistory

import (
	"errors"
	"testing"
	"time"
)

func TestWriteClipboardWithRetryRecoversAfterTransientFailure(t *testing.T) {
	previousSleep := clipboardWriteSleep
	clipboardWriteSleep = func(time.Duration) {}
	t.Cleanup(func() {
		clipboardWriteSleep = previousSleep
	})

	attempts := 0
	err := writeClipboardWithRetry(func() error {
		attempts++
		if attempts < 3 {
			return errors.New("clipboard busy")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("expected retry to recover: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestWriteClipboardWithRetryReturnsLastError(t *testing.T) {
	previousSleep := clipboardWriteSleep
	clipboardWriteSleep = func(time.Duration) {}
	t.Cleanup(func() {
		clipboardWriteSleep = previousSleep
	})

	err := writeClipboardWithRetry(func() error {
		return errors.New("still busy")
	})

	if err == nil || err.Error() != "still busy" {
		t.Fatalf("expected last clipboard error, got %v", err)
	}
}
