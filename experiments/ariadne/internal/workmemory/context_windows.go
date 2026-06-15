//go:build windows

package workmemory

import (
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32WorkMemory                  = windows.NewLazySystemDLL("user32.dll")
	procGetForegroundWindowWorkMemory = user32WorkMemory.NewProc("GetForegroundWindow")
	procGetWindowTextLengthWorkMemory = user32WorkMemory.NewProc("GetWindowTextLengthW")
	procGetWindowTextWorkMemory       = user32WorkMemory.NewProc("GetWindowTextW")
	procGetWindowThreadPIDWorkMemory  = user32WorkMemory.NewProc("GetWindowThreadProcessId")
)

func defaultWindowContextProvider() func() windowContext {
	return foregroundWindowContext
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
