//go:build windows

package clipboardhistory

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

type fakeClipboardProc func(...uintptr) (uintptr, uintptr, error)

func (f fakeClipboardProc) Call(args ...uintptr) (uintptr, uintptr, error) {
	return f(args...)
}

func TestClipboardReadersCloseOnOpeningThread(t *testing.T) {
	for _, reader := range []struct {
		name string
		call func() error
	}{
		{"history", func() error { _, err := readSystemClipboardEntryOnce(t.TempDir(), "test"); return err }},
		{"text", func() error { _, err := readSystemClipboardTextOnce(); return err }},
	} {
		t.Run(reader.name, func(t *testing.T) {
			oldOpen, oldClose, oldFormat := procOpenClipboard, procCloseClipboard, procIsFormatAvailable
			oldProcs := runtime.GOMAXPROCS(1)
			releaseWorker := make(chan struct{})
			workerDone := make(chan struct{})
			t.Cleanup(func() {
				close(releaseWorker)
				<-workerDone
				procOpenClipboard, procCloseClipboard, procIsFormatAvailable = oldOpen, oldClose, oldFormat
				runtime.GOMAXPROCS(oldProcs)
			})
			var openThread, closeThread uint32
			procOpenClipboard = fakeClipboardProc(func(...uintptr) (uintptr, uintptr, error) {
				openThread = windows.GetCurrentThreadId()
				started := make(chan struct{})
				go func() {
					runtime.LockOSThread()
					defer runtime.UnlockOSThread()
					defer close(workerDone)
					close(started)
					<-releaseWorker
				}()
				<-started
				return 1, 0, nil
			})
			procCloseClipboard = fakeClipboardProc(func(...uintptr) (uintptr, uintptr, error) {
				closeThread = windows.GetCurrentThreadId()
				return 1, 0, nil
			})
			procIsFormatAvailable = fakeClipboardProc(func(...uintptr) (uintptr, uintptr, error) {
				return 0, 0, nil
			})

			if err := reader.call(); err != nil {
				t.Fatal(err)
			}
			if openThread == 0 || closeThread != openThread {
				t.Fatalf("clipboard lock leaked across OS threads: opened=%d closed=%d", openThread, closeThread)
			}
		})
	}
}

func TestClipboardAccessSerializesAndReleasesOnFailure(t *testing.T) {
	oldOpen, oldClose := procOpenClipboard, procCloseClipboard
	t.Cleanup(func() {
		procOpenClipboard, procCloseClipboard = oldOpen, oldClose
	})
	var active, overlaps, closes atomic.Int32
	procOpenClipboard = fakeClipboardProc(func(...uintptr) (uintptr, uintptr, error) {
		if active.Add(1) != 1 {
			overlaps.Add(1)
		}
		return 1, 0, nil
	})
	procCloseClipboard = fakeClipboardProc(func(...uintptr) (uintptr, uintptr, error) {
		active.Add(-1)
		closes.Add(1)
		return 1, 0, nil
	})
	failure := errors.New("clipboard operation failed")
	var workers sync.WaitGroup
	for i := range 64 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			var want error
			if i%2 == 0 {
				want = failure
			}
			err := withOpenClipboard(func() error {
				runtime.Gosched()
				return want
			})
			if !errors.Is(err, want) {
				t.Errorf("operation error changed: got %v, want %v", err, want)
			}
		}()
	}
	workers.Wait()
	if overlaps.Load() != 0 || active.Load() != 0 || closes.Load() != 64 {
		t.Fatalf("clipboard access overlapped or leaked: overlaps=%d active=%d closes=%d", overlaps.Load(), active.Load(), closes.Load())
	}
}

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
