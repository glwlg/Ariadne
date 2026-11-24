//go:build !windows

package toolwindows

import "github.com/wailsapp/wails/v3/pkg/application"

func windowHWND(window application.Window) uintptr { return 0 }

func captureForegroundWindowHWND() uintptr { return 0 }

func isValidWindowHWND(hwnd uintptr) bool { return false }

func restoreForegroundWindowHWND(hwnd uintptr) {}

func isLauncherOrChildForeground(window application.Window) bool { return false }
