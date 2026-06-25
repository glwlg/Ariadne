//go:build windows

package toolwindows

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"
	"golang.org/x/sys/windows"
)

var (
	user32ToolWindows                  = windows.NewLazySystemDLL("user32.dll")
	kernel32ToolWindows                = windows.NewLazySystemDLL("kernel32.dll")
	procSetWinEventHookToolWindows     = user32ToolWindows.NewProc("SetWinEventHook")
	procUnhookWinEventToolWindows      = user32ToolWindows.NewProc("UnhookWinEvent")
	procGetMessageToolWindows          = user32ToolWindows.NewProc("GetMessageW")
	procTranslateMessageToolWindows    = user32ToolWindows.NewProc("TranslateMessage")
	procDispatchMessageToolWindows     = user32ToolWindows.NewProc("DispatchMessageW")
	procPostThreadMessageToolWindows   = user32ToolWindows.NewProc("PostThreadMessageW")
	procGetCurrentThreadIDToolWindows  = kernel32ToolWindows.NewProc("GetCurrentThreadId")
	networkMiniTaskbarForegroundHooks  sync.Map
	networkMiniTaskbarForegroundHookCB = windows.NewCallback(networkMiniTaskbarForegroundWinEventProc)
)

const (
	networkMiniEventSystemForeground = 0x0003
	networkMiniWinEventOutOfContext  = 0x0000
	networkMiniWinEventSkipOwnProc   = 0x0002
	networkMiniWMQuit                = 0x0012
)

type networkMiniTaskbarWatchPoint struct {
	X int32
	Y int32
}

type networkMiniTaskbarWatchMessage struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      networkMiniTaskbarWatchPoint
}

func applyNetworkMiniTaskbarOwner(window application.Window) {
	setNetworkMiniTaskbarLayer(window, true)
}

func refreshNetworkMiniTaskbarLayer(window application.Window) {
	setNetworkMiniTaskbarLayer(window, false)
}

func networkMiniTaskbarForegroundActive() bool {
	return isNetworkMiniTaskbarWindow(w32.GetForegroundWindow())
}

func watchNetworkMiniTaskbarForeground(stop <-chan struct{}, onTaskbarForeground func()) error {
	if onTaskbarForeground == nil {
		onTaskbarForeground = func() {}
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	threadID, _, _ := procGetCurrentThreadIDToolWindows.Call()
	hook, _, callErr := procSetWinEventHookToolWindows.Call(
		uintptr(networkMiniEventSystemForeground),
		uintptr(networkMiniEventSystemForeground),
		0,
		networkMiniTaskbarForegroundHookCB,
		0,
		0,
		uintptr(networkMiniWinEventOutOfContext|networkMiniWinEventSkipOwnProc),
	)
	if hook == 0 {
		return fmt.Errorf("set taskbar foreground event hook: %w", callErr)
	}
	networkMiniTaskbarForegroundHooks.Store(hook, onTaskbarForeground)
	done := make(chan struct{})
	go func() {
		select {
		case <-stop:
			procPostThreadMessageToolWindows.Call(threadID, uintptr(networkMiniWMQuit), 0, 0)
		case <-done:
		}
	}()
	defer close(done)
	defer networkMiniTaskbarForegroundHooks.Delete(hook)
	defer procUnhookWinEventToolWindows.Call(hook)

	var msg networkMiniTaskbarWatchMessage
	for {
		ret, _, err := procGetMessageToolWindows.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(ret) == -1 {
			return fmt.Errorf("read taskbar foreground event message: %w", err)
		}
		if ret == 0 {
			return nil
		}
		procTranslateMessageToolWindows.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageToolWindows.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func networkMiniTaskbarForegroundWinEventProc(hook uintptr, event uint32, hwnd uintptr, idObject int32, idChild int32, eventThread uint32, eventTime uint32) uintptr {
	if event == networkMiniEventSystemForeground && hwnd != 0 && isNetworkMiniTaskbarWindow(w32.HWND(hwnd)) {
		if callback, ok := networkMiniTaskbarForegroundHooks.Load(hook); ok {
			callback.(func())()
		}
	}
	return 0
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

func isNetworkMiniTaskbarWindow(hwnd w32.HWND) bool {
	if hwnd == 0 {
		return false
	}
	if isNetworkMiniTaskbarClassName(w32.GetClassName(hwnd)) {
		return true
	}
	taskbar := w32.FindWindowW(w32.MustStringToUTF16Ptr("Shell_TrayWnd"), nil)
	return taskbar != 0 && hwnd == taskbar
}
