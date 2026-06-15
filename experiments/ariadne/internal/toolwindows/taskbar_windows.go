//go:build windows

package toolwindows

import (
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"
)

func applyNetworkMiniTaskbarOwner(window application.Window) {
	setNetworkMiniTaskbarLayer(window, true)
}

func refreshNetworkMiniTaskbarLayer(window application.Window) {
	setNetworkMiniTaskbarLayer(window, false)
}

func setNetworkMiniTaskbarLayer(window application.Window, show bool) {
	if window == nil {
		return
	}
	native := window.NativeWindow()
	if native == nil {
		return
	}
	hwnd := w32.HWND(native)
	flags := uint(w32.SWP_NOMOVE | w32.SWP_NOSIZE | w32.SWP_NOACTIVATE)
	if show {
		flags |= w32.SWP_SHOWWINDOW
	}
	exStyle := w32.GetWindowLongPtr(hwnd, w32.GWL_EXSTYLE)
	targetExStyle := exStyle
	targetExStyle |= uintptr(w32.WS_EX_TOOLWINDOW | w32.WS_EX_TOPMOST | w32.WS_EX_NOACTIVATE)
	targetExStyle &^= uintptr(w32.WS_EX_APPWINDOW)
	if targetExStyle != exStyle {
		w32.SetWindowLongPtr(hwnd, w32.GWL_EXSTYLE, targetExStyle)
		flags |= w32.SWP_FRAMECHANGED
	}
	taskbar := w32.FindWindowW(w32.MustStringToUTF16Ptr("Shell_TrayWnd"), nil)
	if taskbar != 0 {
		w32.SetWindowLongPtr(hwnd, w32.GWLP_HWNDPARENT, uintptr(unsafe.Pointer(taskbar)))
	}
	w32.SetWindowPos(
		hwnd,
		w32.HWND_TOPMOST,
		0,
		0,
		0,
		0,
		flags,
	)
}
