//go:build windows

package shell

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmHotkey = 0x0312
	wmQuit   = 0x0012
)

var (
	user32                   = windows.NewLazySystemDLL("user32.dll")
	procRegisterHotKey       = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey     = user32.NewProc("UnregisterHotKey")
	procGetMessageW          = user32.NewProc("GetMessageW")
	procPostThreadMessageW   = user32.NewProc("PostThreadMessageW")
	procPeekMessageW         = user32.NewProc("PeekMessageW")
	globalHotkeyIDCounter    atomic.Int32
	errHotkeyStopTimeout     = fmt.Errorf("timed out stopping global hotkey")
	globalHotkeyStopWaitTime = 1200 * time.Millisecond
)

type point struct {
	X int32
	Y int32
}

type message struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Point   point
}

type HotkeyRegistration struct {
	id       int32
	threadID atomic.Uint32
	done     chan struct{}
	stopped  atomic.Bool
}

func RegisterGlobalHotkey(spec HotkeySpec, callback func()) (*HotkeyRegistration, error) {
	if callback == nil {
		return nil, fmt.Errorf("global hotkey callback is nil")
	}

	registration := &HotkeyRegistration{
		id:   globalHotkeyIDCounter.Add(1),
		done: make(chan struct{}),
	}
	ready := make(chan error, 1)

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(registration.done)

		registration.threadID.Store(windows.GetCurrentThreadId())
		createMessageQueue()

		if err := registerHotkey(registration.id, spec); err != nil {
			ready <- err
			return
		}
		defer unregisterHotkey(registration.id)
		ready <- nil

		for {
			var msg message
			result, err := getMessage(&msg)
			if result == -1 {
				return
			}
			if result == 0 {
				return
			}
			if msg.Message == wmHotkey && int32(msg.WParam) == registration.id {
				go callback()
			}
			_ = err
		}
	}()

	if err := <-ready; err != nil {
		<-registration.done
		return nil, err
	}
	return registration, nil
}

func (r *HotkeyRegistration) Stop() error {
	if r == nil {
		return nil
	}
	if !r.stopped.CompareAndSwap(false, true) {
		return nil
	}
	threadID := r.threadID.Load()
	if threadID != 0 {
		procPostThreadMessageW.Call(uintptr(threadID), wmQuit, 0, 0)
	}
	select {
	case <-r.done:
		return nil
	case <-time.After(globalHotkeyStopWaitTime):
		return errHotkeyStopTimeout
	}
}

func createMessageQueue() {
	var msg message
	procPeekMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 0)
}

func registerHotkey(id int32, spec HotkeySpec) error {
	result, _, callErr := procRegisterHotKey.Call(0, uintptr(id), uintptr(spec.Modifiers), uintptr(spec.KeyCode))
	if result == 0 {
		if callErr != windows.ERROR_SUCCESS {
			return fmt.Errorf("register global hotkey %q: %w", spec.Raw, callErr)
		}
		return fmt.Errorf("register global hotkey %q failed", spec.Raw)
	}
	return nil
}

func unregisterHotkey(id int32) {
	procUnregisterHotKey.Call(0, uintptr(id))
}

func getMessage(msg *message) (int32, error) {
	result, _, callErr := procGetMessageW.Call(uintptr(unsafe.Pointer(msg)), 0, 0, 0)
	if int32(result) == -1 {
		if callErr != windows.ERROR_SUCCESS {
			return -1, callErr
		}
		return -1, fmt.Errorf("GetMessageW failed")
	}
	return int32(result), nil
}
