//go:build windows

package clipboardhistory

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

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

func TestClipboardExclusionFormatsSkipCapture(t *testing.T) {
	const (
		excludeFmt = uintptr(0xC101)
		historyFmt = uintptr(0xC102)
	)
	oldOpen, oldClose, oldAvail, oldReg, oldGet, oldLock, oldUnlock, oldSize :=
		procOpenClipboard, procCloseClipboard, procIsFormatAvailable, procRegisterClipboardFormatW,
		procGetClipboardData, procGlobalLock, procGlobalUnlock, procGlobalSize
	t.Cleanup(func() {
		procOpenClipboard, procCloseClipboard, procIsFormatAvailable, procRegisterClipboardFormatW,
			procGetClipboardData, procGlobalLock, procGlobalUnlock, procGlobalSize =
			oldOpen, oldClose, oldAvail, oldReg, oldGet, oldLock, oldUnlock, oldSize
	})

	procOpenClipboard = fakeClipboardProc(func(...uintptr) (uintptr, uintptr, error) { return 1, 0, nil })
	procCloseClipboard = fakeClipboardProc(func(...uintptr) (uintptr, uintptr, error) { return 1, 0, nil })
	procRegisterClipboardFormatW = fakeClipboardProc(func(args ...uintptr) (uintptr, uintptr, error) {
		switch args[0] {
		case uintptr(unsafe.Pointer(excludeMonitorFormatName)):
			return excludeFmt, 0, nil
		case uintptr(unsafe.Pointer(canIncludeHistoryName)):
			return historyFmt, 0, nil
		default:
			return 0, 0, nil
		}
	})

	var dwordBuf [4]byte
	var excludeAvail, historyAvail uintptr
	procIsFormatAvailable = fakeClipboardProc(func(args ...uintptr) (uintptr, uintptr, error) {
		switch args[0] {
		case excludeFmt:
			return excludeAvail, 0, nil
		case historyFmt:
			return historyAvail, 0, nil
		default:
			return 0, 0, nil
		}
	})
	procGetClipboardData = fakeClipboardProc(func(args ...uintptr) (uintptr, uintptr, error) {
		if args[0] == historyFmt {
			return 1, 0, nil
		}
		return 0, 0, nil
	})
	procGlobalLock = fakeClipboardProc(func(...uintptr) (uintptr, uintptr, error) {
		return uintptr(unsafe.Pointer(&dwordBuf[0])), 0, nil
	})
	procGlobalUnlock = fakeClipboardProc(func(...uintptr) (uintptr, uintptr, error) { return 1, 0, nil })
	procGlobalSize = fakeClipboardProc(func(...uintptr) (uintptr, uintptr, error) { return 4, 0, nil })

	// 1. ExcludeClipboardContentFromMonitorProcessing present -> skip
	excludeAvail, historyAvail = 1, 0
	entry, err := readSystemClipboardEntryOnce(t.TempDir(), "watcher")
	if err != nil || entryIsValid(entry) {
		t.Fatalf("expected ExcludeClipboardContentFromMonitorProcessing to skip capture, got entry=%#v err=%v", entry, err)
	}

	// 2. CanIncludeInClipboardHistory == 0 -> skip
	excludeAvail, historyAvail = 0, 1
	binary.LittleEndian.PutUint32(dwordBuf[:], 0)
	entry, err = readSystemClipboardEntryOnce(t.TempDir(), "watcher")
	if err != nil || entryIsValid(entry) {
		t.Fatalf("expected CanIncludeInClipboardHistory=0 to skip capture, got entry=%#v err=%v", entry, err)
	}

	// 3. CanIncludeInClipboardHistory == 1 -> allow reading (no text/image -> empty without exclusion)
	binary.LittleEndian.PutUint32(dwordBuf[:], 1)
	if isClipboardCaptureExcluded() {
		t.Fatal("CanIncludeInClipboardHistory=1 should not be excluded")
	}
}

func TestImageToDIBAndDIBToImageRoundTrip(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 3))
	src.SetRGBA(0, 0, color.RGBA{R: 12, G: 34, B: 56, A: 255})
	src.SetRGBA(3, 2, color.RGBA{R: 200, G: 150, B: 100, A: 255})

	dib := imageToDIB(src)
	decoded, err := dibToImage(dib)
	if err != nil {
		t.Fatalf("dibToImage failed: %v", err)
	}
	if decoded.RGBAAt(0, 0) != (color.RGBA{R: 12, G: 34, B: 56, A: 255}) {
		t.Fatalf("unexpected top-left pixel: %#v", decoded.RGBAAt(0, 0))
	}
	if decoded.RGBAAt(3, 2) != (color.RGBA{R: 200, G: 150, B: 100, A: 255}) {
		t.Fatalf("unexpected bottom-right pixel: %#v", decoded.RGBAAt(3, 2))
	}
	pngBytes, err := dibToPNG(dib)
	if err != nil {
		t.Fatalf("dibToPNG failed: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(pngBytes)); err != nil {
		t.Fatalf("dibToPNG produced invalid PNG: %v", err)
	}
}
