//go:build windows

package toolwindows

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"
)

var (
	procIsChildToolWindows     = user32ToolWindows.NewProc("IsChild")
	procGetAncestorToolWindows = user32ToolWindows.NewProc("GetAncestor")
)

const gaRootOwner = 3

func windowHWND(window application.Window) uintptr {
	if window == nil {
		return 0
	}
	native := window.NativeWindow()
	if native == nil {
		return 0
	}
	return uintptr(w32.HWND(native))
}

func captureForegroundWindowHWND() uintptr {
	return uintptr(w32.GetForegroundWindow())
}

func isValidWindowHWND(hwnd uintptr) bool {
	return hwnd != 0 && w32.IsWindow(w32.HWND(hwnd))
}

func restoreForegroundWindowHWND(hwnd uintptr) {
	if isValidWindowHWND(hwnd) {
		w32.SetForegroundWindow(w32.HWND(hwnd))
	}
}

func isLauncherOrChildForeground(window application.Window) bool {
	hwnd := windowHWND(window)
	if hwnd == 0 {
		return false
	}
	fg := uintptr(w32.GetForegroundWindow())
	if fg == 0 {
		return false
	}
	if fg == hwnd {
		return true
	}
	if child, _, _ := procIsChildToolWindows.Call(hwnd, fg); child != 0 {
		return true
	}
	if rootOwner, _, _ := procGetAncestorToolWindows.Call(fg, gaRootOwner); rootOwner == hwnd {
		return true
	}
	return false
}
