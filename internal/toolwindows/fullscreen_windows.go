//go:build windows

package toolwindows

import "github.com/wailsapp/wails/v3/pkg/w32"

func defaultFullscreenDetector() (bool, error) {
	hwnd := w32.GetForegroundWindow()
	if hwnd == 0 || hwnd == w32.GetDesktopWindow() {
		return false, nil
	}
	return w32.IsWindowFullScreen(uintptr(hwnd)), nil
}
