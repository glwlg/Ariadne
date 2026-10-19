//go:build windows

package shell

import (
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	whKeyboardLL                 = 13
	wmKeydown                    = 0x0100
	wmKeyup                      = 0x0101
	wmSyskeydown                 = 0x0104
	wmSyskeyup                   = 0x0105
	screenshotKeyboardHookQuit   = 0x0012
	screenshotKeyboardHookThread = "ariadne-screenshot-hotkey"
)

var (
	user32ScreenshotHook                = windows.NewLazySystemDLL("user32.dll")
	kernel32ScreenshotHook              = windows.NewLazySystemDLL("kernel32.dll")
	procSetWindowsHookEx                = user32ScreenshotHook.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx             = user32ScreenshotHook.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx                  = user32ScreenshotHook.NewProc("CallNextHookEx")
	procGetMessageScreenshotHook        = user32ScreenshotHook.NewProc("GetMessageW")
	procTranslateMessageScreenshotHook  = user32ScreenshotHook.NewProc("TranslateMessage")
	procDispatchMessageScreenshotHook   = user32ScreenshotHook.NewProc("DispatchMessageW")
	procPostThreadMessageScreenshotHook = user32ScreenshotHook.NewProc("PostThreadMessageW")
	procGetCurrentThreadScreenshotHook  = kernel32ScreenshotHook.NewProc("GetCurrentThreadId")
	procGetAsyncKeyState                = user32ScreenshotHook.NewProc("GetAsyncKeyState")
	screenshotKeyboardHookCallback      = windows.NewCallback(screenshotKeyboardHookProc)
	screenshotKeyboardHooks             sync.Map
	screenshotKeyboardHookInstalled     atomic.Bool
)

type screenshotKeyboardHookState struct {
	spec   HotkeySpec
	freeze func()
	down   atomic.Bool
}

type screenshotKeyboardHookPoint struct {
	X int32
	Y int32
}

type screenshotKeyboardHookMessage struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      screenshotKeyboardHookPoint
}

type screenshotLowLevelKeyboard struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type windowsScreenshotHotkeyHook struct {
	spec     HotkeySpec
	freeze   func()
	ready    chan struct{}
	stopOnce sync.Once
	stopped  chan struct{}
	threadID uint32
	hook     uintptr
	startErr error
}

func startWindowsScreenshotHotkeyHook(spec HotkeySpec, freeze func()) (*windowsScreenshotHotkeyHook, error) {
	hook := &windowsScreenshotHotkeyHook{
		spec:    spec,
		freeze:  freeze,
		ready:   make(chan struct{}),
		stopped: make(chan struct{}),
	}
	go hook.loop()
	<-hook.ready
	if hook.startErr != nil {
		<-hook.stopped
		return nil, hook.startErr
	}
	return hook, nil
}

func (h *windowsScreenshotHotkeyHook) loop() {
	runtimeThread(screenshotKeyboardHookThread)
	defer close(h.stopped)

	threadID, _, _ := procGetCurrentThreadScreenshotHook.Call()
	h.threadID = uint32(threadID)
	handle, _, callErr := procSetWindowsHookEx.Call(
		whKeyboardLL,
		screenshotKeyboardHookCallback,
		0,
		0,
	)
	if handle == 0 {
		h.startErr = callErr
		close(h.ready)
		return
	}
	h.hook = handle
	screenshotKeyboardHooks.Store(handle, &screenshotKeyboardHookState{spec: h.spec, freeze: h.freeze})
	screenshotKeyboardHookInstalled.Store(true)
	close(h.ready)
	defer func() {
		screenshotKeyboardHooks.Delete(handle)
		screenshotKeyboardHookInstalled.Store(false)
		procUnhookWindowsHookEx.Call(handle)
	}()

	var message screenshotKeyboardHookMessage
	for {
		result, _, _ := procGetMessageScreenshotHook.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) <= 0 || message.Message == screenshotKeyboardHookQuit {
			return
		}
		procTranslateMessageScreenshotHook.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessageScreenshotHook.Call(uintptr(unsafe.Pointer(&message)))
	}
}

func (h *windowsScreenshotHotkeyHook) stop() {
	if h == nil {
		return
	}
	h.stopOnce.Do(func() {
		if h.threadID != 0 {
			procPostThreadMessageScreenshotHook.Call(uintptr(h.threadID), screenshotKeyboardHookQuit, 0, 0)
		}
		<-h.stopped
	})
}

func (h *windowsScreenshotHotkeyHook) active() bool {
	return h != nil && h.hook != 0
}

func startScreenshotHotkeyHook(spec HotkeySpec, freeze func()) (screenshotHotkeyHook, error) {
	return startWindowsScreenshotHotkeyHook(spec, freeze)
}

func screenshotKeyboardHookProc(code int32, wParam, lParam uintptr) uintptr {
	next, _, _ := procCallNextHookEx.Call(0, uintptr(code), wParam, lParam)
	if code < 0 || lParam == 0 {
		return next
	}
	info := (*screenshotLowLevelKeyboard)(unsafe.Pointer(lParam))
	keyDown := wParam == wmKeydown || wParam == wmSyskeydown
	keyUp := wParam == wmKeyup || wParam == wmSyskeyup
	if !keyDown && !keyUp {
		return next
	}
	var matched *screenshotKeyboardHookState
	screenshotKeyboardHooks.Range(func(_, value any) bool {
		state, ok := value.(*screenshotKeyboardHookState)
		if ok && state.spec.KeyCode == info.VkCode && screenshotModifiersDown(state.spec, info.VkCode) {
			matched = state
			return false
		}
		return true
	})
	if matched == nil {
		return next
	}
	repeat := false
	if keyDown {
		repeat = matched.down.Swap(true)
	} else {
		matched.down.Store(false)
	}
	decision := HandleScreenshotHotkey(ScreenshotHotkeyEvent{
		KeyDown: keyDown,
		Repeat:  repeat,
		Matches: true,
	}, nil)
	if decision.Freeze && matched.freeze != nil {
		go matched.freeze()
	}
	if decision.Consume {
		return 1
	}
	return next
}

func screenshotModifiersDown(spec HotkeySpec, virtualKey uint32) bool {
	checks := []struct {
		enabled bool
		key     uint32
	}{
		{spec.Modifiers&modAlt != 0, 0x12},
		{spec.Modifiers&modControl != 0, 0x11},
		{spec.Modifiers&modShift != 0, 0x10},
		{spec.Modifiers&modWin != 0, 0x5B},
	}
	for _, check := range checks {
		if !check.enabled || check.key == virtualKey {
			continue
		}
		state, _, _ := procGetAsyncKeyState.Call(uintptr(check.key))
		if state&0x8000 == 0 {
			right := check.key
			switch check.key {
			case 0x12:
				right = 0xA5
			case 0x11:
				right = 0xA3
			case 0x10:
				right = 0xA1
			case 0x5B:
				right = 0x5C
			}
			state, _, _ = procGetAsyncKeyState.Call(uintptr(right))
			if state&0x8000 == 0 {
				return false
			}
		}
	}
	return true
}

func runtimeThread(name string) {
	runtime.LockOSThread()
	_ = name
}
