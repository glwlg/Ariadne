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

func enableOrdinaryWindowTaskbarToggle(window application.Window) {
	if window == nil {
		return
	}
	native := window.NativeWindow()
	if native == nil {
		return
	}
	hwnd := w32.HWND(native)
	flags := uint(w32.SWP_NOMOVE | w32.SWP_NOSIZE | w32.SWP_NOOWNERZORDER)

	style := w32.GetWindowLongPtr(hwnd, w32.GWL_STYLE)
	targetStyle := ordinaryWindowTaskbarStyle(style)
	if targetStyle != style {
		w32.SetWindowLongPtr(hwnd, w32.GWL_STYLE, targetStyle)
		flags |= w32.SWP_FRAMECHANGED
	}

	exStyle := w32.GetWindowLongPtr(hwnd, w32.GWL_EXSTYLE)
	targetExStyle := ordinaryWindowTaskbarExStyle(exStyle)
	if targetExStyle != exStyle {
		w32.SetWindowLongPtr(hwnd, w32.GWL_EXSTYLE, targetExStyle)
		flags |= w32.SWP_FRAMECHANGED
	}

	w32.SetWindowLongPtr(hwnd, w32.GWLP_HWNDPARENT, 0)
	w32.SetWindowPos(hwnd, w32.HWND_NOTOPMOST, 0, 0, 0, 0, flags)
}

func setOrdinaryWindowIcon(window application.Window, icon []byte) {
	if window == nil || len(icon) == 0 {
		return
	}
	native := window.NativeWindow()
	if native == nil {
		return
	}
	hwnd := w32.HWND(native)
	if smallIcon, err := w32.CreateSmallHIconFromImage(icon); err == nil && smallIcon != 0 {
		w32.SendMessage(hwnd, w32.WM_SETICON, w32.ICON_SMALL, uintptr(smallIcon))
		w32.SendMessage(hwnd, w32.WM_SETICON, w32.ICON_SMALL2, uintptr(smallIcon))
	}
	if largeIcon, err := w32.CreateLargeHIconFromImage(icon); err == nil && largeIcon != 0 {
		w32.SendMessage(hwnd, w32.WM_SETICON, w32.ICON_BIG, uintptr(largeIcon))
	}
}

func ordinaryWindowTaskbarStyle(style uintptr) uintptr {
	return (style &^ uintptr(w32.WS_POPUP)) | uintptr(w32.WS_OVERLAPPEDWINDOW|w32.WS_VISIBLE)
}

func ordinaryWindowTaskbarExStyle(exStyle uintptr) uintptr {
	exStyle |= uintptr(w32.WS_EX_APPWINDOW)
	exStyle &^= uintptr(w32.WS_EX_TOOLWINDOW | w32.WS_EX_NOACTIVATE | w32.WS_EX_TOPMOST)
	return exStyle
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
