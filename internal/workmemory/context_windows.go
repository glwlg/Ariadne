//go:build windows

package workmemory

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32WorkMemory                  = windows.NewLazySystemDLL("user32.dll")
	kernel32WorkMemory                = windows.NewLazySystemDLL("kernel32.dll")
	procGetForegroundWindowWorkMemory = user32WorkMemory.NewProc("GetForegroundWindow")
	procGetWindowTextLengthWorkMemory = user32WorkMemory.NewProc("GetWindowTextLengthW")
	procGetWindowTextWorkMemory       = user32WorkMemory.NewProc("GetWindowTextW")
	procGetWindowThreadPIDWorkMemory  = user32WorkMemory.NewProc("GetWindowThreadProcessId")
	procSetWinEventHookWorkMemory     = user32WorkMemory.NewProc("SetWinEventHook")
	procUnhookWinEventWorkMemory      = user32WorkMemory.NewProc("UnhookWinEvent")
	procGetMessageWorkMemory          = user32WorkMemory.NewProc("GetMessageW")
	procTranslateMessageWorkMemory    = user32WorkMemory.NewProc("TranslateMessage")
	procDispatchMessageWorkMemory     = user32WorkMemory.NewProc("DispatchMessageW")
	procPostThreadMessageWorkMemory   = user32WorkMemory.NewProc("PostThreadMessageW")
	procGetCurrentThreadIDWorkMemory  = kernel32WorkMemory.NewProc("GetCurrentThreadId")
)

const (
	eventSystemForeground  = 0x0003
	wineventOutOfContext   = 0x0000
	wineventSkipOwnProcess = 0x0002
	wmQuit                 = 0x0012
)

type foregroundWatchPoint struct {
	X int32
	Y int32
}

type foregroundWatchMessage struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      foregroundWatchPoint
}

var (
	foregroundWatchCallbacks sync.Map
	foregroundWatchCallback  = windows.NewCallback(foregroundWinEventProc)
)

func defaultWindowContextProvider() func() windowContext {
	return foregroundWindowContext
}

func watchForegroundWindow(stop <-chan struct{}, onChange func()) error {
	if onChange == nil {
		onChange = func() {}
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	threadID, _, _ := procGetCurrentThreadIDWorkMemory.Call()
	hook, _, callErr := procSetWinEventHookWorkMemory.Call(
		uintptr(eventSystemForeground),
		uintptr(eventSystemForeground),
		0,
		foregroundWatchCallback,
		0,
		0,
		uintptr(wineventOutOfContext|wineventSkipOwnProcess),
	)
	if hook == 0 {
		return fmt.Errorf("set foreground window event hook: %w", callErr)
	}
	foregroundWatchCallbacks.Store(hook, onChange)
	done := make(chan struct{})
	go func() {
		select {
		case <-stop:
			procPostThreadMessageWorkMemory.Call(threadID, uintptr(wmQuit), 0, 0)
		case <-done:
		}
	}()
	defer close(done)
	defer foregroundWatchCallbacks.Delete(hook)
	defer procUnhookWinEventWorkMemory.Call(hook)

	var msg foregroundWatchMessage
	for {
		ret, _, err := procGetMessageWorkMemory.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(ret) == -1 {
			return fmt.Errorf("read foreground window event message: %w", err)
		}
		if ret == 0 {
			return nil
		}
		procTranslateMessageWorkMemory.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageWorkMemory.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func foregroundWinEventProc(hook uintptr, event uint32, hwnd uintptr, idObject int32, idChild int32, eventThread uint32, eventTime uint32) uintptr {
	if event == eventSystemForeground && hwnd != 0 {
		if callback, ok := foregroundWatchCallbacks.Load(hook); ok {
			callback.(func())()
		}
	}
	return 0
}

func foregroundWindowContext() windowContext {
	hwnd, _, _ := procGetForegroundWindowWorkMemory.Call()
	if hwnd == 0 {
		return windowContext{title: "Ariadne", app: "Ariadne"}
	}
	title := foregroundWindowTitle(hwnd)
	app := foregroundWindowApp(hwnd)
	if title == "" && app == "" {
		return windowContext{title: "Ariadne", app: "Ariadne"}
	}
	return windowContext{title: title, app: app}
}

func foregroundWindowTitle(hwnd uintptr) string {
	length, _, _ := procGetWindowTextLengthWorkMemory.Call(hwnd)
	if length == 0 {
		return ""
	}
	buffer := make([]uint16, int(length)+1)
	procGetWindowTextWorkMemory.Call(hwnd, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	return strings.TrimSpace(windows.UTF16ToString(buffer))
}

func foregroundWindowApp(hwnd uintptr) string {
	var pid uint32
	procGetWindowThreadPIDWorkMemory.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return ""
	}
	path := processImagePathForWindow(pid)
	if path == "" {
		return ""
	}
	return filepath.Base(path)
}

func processImagePathForWindow(pid uint32) string {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(handle)

	buffer := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
		return ""
	}
	return windows.UTF16ToString(buffer[:size])
}
