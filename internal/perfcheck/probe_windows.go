//go:build windows

package perfcheck

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	psapi                   = windows.NewLazySystemDLL("psapi.dll")
	procEnumWindows         = user32.NewProc("EnumWindows")
	procGetWindowThreadPID  = user32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible     = user32.NewProc("IsWindowVisible")
	procGetWindowTextLength = user32.NewProc("GetWindowTextLengthW")
	procGetWindowText       = user32.NewProc("GetWindowTextW")
	procGetWindowRect       = user32.NewProc("GetWindowRect")
	procGetWindowLongPtr    = user32.NewProc("GetWindowLongPtrW")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSendInput           = user32.NewProc("SendInput")
	procRegisterHotKey      = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey    = user32.NewProc("UnregisterHotKey")
	procGetProcessMemory    = psapi.NewProc("GetProcessMemoryInfo")
)

const (
	gwlStyleIndex   = ^uintptr(15)
	gwlExStyleIndex = ^uintptr(19)

	wsCaption    = 0x00C00000
	wsThickFrame = 0x00040000
	wsExTopmost  = 0x00000008

	swHide = 0

	vkMenu = 0x12
	vkQ    = 0x51

	inputKeyboard = 1
	keyEventUp    = 0x0002

	modAlt      = 0x0001
	modNoRepeat = 0x4000

	processQueryInformation        = 0x0400
	processQueryLimitedInformation = 0x1000
	processVMRead                  = 0x0010
)

type winRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type processMemoryCounters struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

type keyboardInput struct {
	VirtualKey uint16
	ScanCode   uint16
	Flags      uint32
	Time       uint32
	ExtraInfo  uintptr
}

type inputEvent struct {
	Type     uint32
	_        uint32
	Keyboard keyboardInput
	_        [8]byte
}

func probeStartup(options Options, iteration int) StartupSample {
	sample := StartupSample{Iteration: iteration}
	exePath, err := filepath.Abs(options.ExePath)
	if err != nil {
		sample.Error = err.Error()
		return sample
	}
	if fileBytes(exePath) == 0 {
		sample.Error = "exe not found: " + exePath
		return sample
	}

	var tempRoot string
	if options.UseTempAppData {
		tempRoot, err = os.MkdirTemp("", "ariadne-perf-")
		if err != nil {
			sample.Error = "create temp appdata: " + err.Error()
			return sample
		}
		defer os.RemoveAll(tempRoot)
	}

	command := exec.Command(exePath)
	if options.UseTempAppData {
		roaming := filepath.Join(tempRoot, "Roaming")
		local := filepath.Join(tempRoot, "Local")
		_ = os.MkdirAll(roaming, 0o755)
		_ = os.MkdirAll(local, 0o755)
		command.Env = append(os.Environ(), "APPDATA="+roaming, "LOCALAPPDATA="+local)
	}

	startedAt := time.Now()
	if err := command.Start(); err != nil {
		sample.Error = "start exe: " + err.Error()
		return sample
	}
	sample.ProcessID = command.Process.Pid
	exited := make(chan error, 1)
	go func() {
		exited <- command.Wait()
	}()
	defer func() {
		_ = command.Process.Kill()
		select {
		case <-exited:
		case <-time.After(2 * time.Second):
		}
	}()

	deadline := startedAt.Add(time.Duration(options.TimeoutMs) * time.Millisecond)
	for time.Now().Before(deadline) {
		if window, ok := findProcessWindow(uint32(command.Process.Pid)); ok {
			sample.StartupMs = time.Since(startedAt).Milliseconds()
			sample.Window = window
			sample.WorkingSetBytes = int64(processWorkingSet(uint32(command.Process.Pid)))
			return sample
		}
		select {
		case err := <-exited:
			if err != nil {
				sample.Error = "process exited before window appeared: " + err.Error()
			} else {
				sample.Error = "process exited before window appeared"
			}
			return sample
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}
	sample.Error = fmt.Sprintf("Ariadne window not found within %dms", options.TimeoutMs)
	return sample
}

func probeHotkey(options Options) []HotkeySample {
	samples := make([]HotkeySample, 0, options.HotkeyIterations)
	exePath, err := filepath.Abs(options.ExePath)
	if err != nil {
		return hotkeyErrorSamples(options, err.Error())
	}
	if fileBytes(exePath) == 0 {
		return hotkeyErrorSamples(options, "exe not found: "+exePath)
	}

	var tempRoot string
	if options.UseTempAppData {
		tempRoot, err = os.MkdirTemp("", "ariadne-perf-hotkey-")
		if err != nil {
			return hotkeyErrorSamples(options, "create temp appdata: "+err.Error())
		}
		defer os.RemoveAll(tempRoot)
	}

	command := exec.Command(exePath)
	if options.UseTempAppData {
		roaming := filepath.Join(tempRoot, "Roaming")
		local := filepath.Join(tempRoot, "Local")
		_ = os.MkdirAll(roaming, 0o755)
		_ = os.MkdirAll(local, 0o755)
		command.Env = append(os.Environ(), "APPDATA="+roaming, "LOCALAPPDATA="+local)
	}

	if err := command.Start(); err != nil {
		return hotkeyErrorSamples(options, "start exe: "+err.Error())
	}
	pid := uint32(command.Process.Pid)
	exited := make(chan error, 1)
	go func() {
		exited <- command.Wait()
	}()
	defer func() {
		_ = command.Process.Kill()
		select {
		case <-exited:
		case <-time.After(2 * time.Second):
		}
	}()

	window, ok, err := waitForProcessWindow(pid, time.Duration(options.TimeoutMs)*time.Millisecond, false, exited)
	if err != nil {
		return hotkeyErrorSamples(options, err.Error())
	}
	if !ok {
		return hotkeyErrorSamples(options, fmt.Sprintf("Ariadne window not found within %dms", options.TimeoutMs))
	}
	hwnd := uintptr(window.Handle)
	time.Sleep(250 * time.Millisecond)

	for iteration := 1; iteration <= options.HotkeyIterations; iteration++ {
		sample := HotkeySample{Iteration: iteration, ProcessID: command.Process.Pid}
		hideWindow(hwnd)
		if !waitForWindowHidden(hwnd, 2*time.Second) {
			sample.Error = "launcher did not hide before Alt+Q sample"
			samples = append(samples, sample)
			continue
		}

		startedAt := time.Now()
		sendAltQ()
		window, ok, err := waitForProcessWindow(pid, time.Duration(options.TimeoutMs)*time.Millisecond, true, exited)
		if err != nil {
			sample.Error = err.Error()
		} else if !ok {
			sample.Error = fmt.Sprintf("launcher was not visible and foreground within %dms after Alt+Q", options.TimeoutMs)
		} else {
			sample.HotkeyMs = time.Since(startedAt).Milliseconds()
			sample.Window = window
			hwnd = uintptr(window.Handle)
		}
		samples = append(samples, sample)
	}
	return samples
}

func probeHotkeyRegistration(options Options) HotkeyRegistrationProbe {
	result := HotkeyRegistrationProbe{}
	before := tryRegisterAltQ(61001)
	result.BeforeAvailable = before.OK
	result.BeforeErrorCode = before.ErrorCode
	result.BeforeError = before.Error
	if before.OK {
		unregisterHotkeyProbe(61001)
	}

	exePath, err := filepath.Abs(options.ExePath)
	if err != nil {
		result.Note = err.Error()
		return result
	}
	if fileBytes(exePath) == 0 {
		result.Note = "exe not found: " + exePath
		return result
	}

	var tempRoot string
	if options.UseTempAppData {
		tempRoot, err = os.MkdirTemp("", "ariadne-perf-hotkey-reg-")
		if err != nil {
			result.Note = "create temp appdata: " + err.Error()
			return result
		}
		defer os.RemoveAll(tempRoot)
	}

	command := exec.Command(exePath)
	if options.UseTempAppData {
		roaming := filepath.Join(tempRoot, "Roaming")
		local := filepath.Join(tempRoot, "Local")
		_ = os.MkdirAll(roaming, 0o755)
		_ = os.MkdirAll(local, 0o755)
		command.Env = append(os.Environ(), "APPDATA="+roaming, "LOCALAPPDATA="+local)
	}
	if err := command.Start(); err != nil {
		result.Note = "start exe: " + err.Error()
		return result
	}
	exited := make(chan error, 1)
	go func() {
		exited <- command.Wait()
	}()
	defer func() {
		_ = command.Process.Kill()
		select {
		case <-exited:
		case <-time.After(2 * time.Second):
		}
	}()

	if _, ok, err := waitForProcessWindow(uint32(command.Process.Pid), time.Duration(options.TimeoutMs)*time.Millisecond, false, exited); err != nil {
		result.Note = err.Error()
		return result
	} else if !ok {
		result.Note = fmt.Sprintf("Ariadne window not found within %dms", options.TimeoutMs)
		return result
	}

	during := tryRegisterAltQ(61002)
	result.DuringBlocked = !during.OK && during.ErrorCode == int(windows.ERROR_HOTKEY_ALREADY_REGISTERED)
	result.DuringErrorCode = during.ErrorCode
	result.DuringError = during.Error
	if during.OK {
		unregisterHotkeyProbe(61002)
		result.Note = "Alt+Q was still available after Ariadne startup; Ariadne did not appear to own the hotkey"
	} else if result.DuringBlocked {
		result.Note = "Alt+Q registration is blocked while Ariadne is running, which is consistent with Ariadne owning the global hotkey"
	} else {
		result.Note = "Alt+Q registration failed for a non-ownership reason while Ariadne was running"
	}
	return result
}

func hotkeyErrorSamples(options Options, message string) []HotkeySample {
	samples := make([]HotkeySample, 0, options.HotkeyIterations)
	for iteration := 1; iteration <= options.HotkeyIterations; iteration++ {
		samples = append(samples, HotkeySample{Iteration: iteration, Error: message})
	}
	return samples
}

func waitForProcessWindow(pid uint32, timeout time.Duration, requireForeground bool, exited <-chan error) (WindowSample, bool, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if window, ok := findProcessWindow(pid); ok {
			if !requireForeground || window.IsForeground {
				return window, true, nil
			}
		}
		select {
		case err := <-exited:
			if err != nil {
				return WindowSample{}, false, fmt.Errorf("process exited before window appeared: %w", err)
			}
			return WindowSample{}, false, fmt.Errorf("process exited before window appeared")
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	return WindowSample{}, false, nil
}

func findProcessWindow(pid uint32) (WindowSample, bool) {
	state := &windowSearchState{pid: pid}
	callback := syscall.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
		search := (*windowSearchState)(unsafe.Pointer(lparam))
		var windowPID uint32
		procGetWindowThreadPID.Call(hwnd, uintptr(unsafe.Pointer(&windowPID)))
		if windowPID != search.pid {
			return 1
		}
		visible, _, _ := procIsWindowVisible.Call(hwnd)
		if visible == 0 {
			return 1
		}
		title := windowTitle(hwnd)
		if !strings.Contains(strings.ToLower(title), "ariadne") {
			return 1
		}
		search.window = readWindowSample(hwnd, title)
		search.found = true
		return 0
	})
	procEnumWindows.Call(callback, uintptr(unsafe.Pointer(state)))
	return state.window, state.found
}

type windowSearchState struct {
	pid    uint32
	found  bool
	window WindowSample
}

func windowTitle(hwnd uintptr) string {
	length, _, _ := procGetWindowTextLength.Call(hwnd)
	if length == 0 {
		return ""
	}
	buffer := make([]uint16, int(length)+1)
	procGetWindowText.Call(hwnd, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	return syscall.UTF16ToString(buffer)
}

func readWindowSample(hwnd uintptr, title string) WindowSample {
	rect := winRect{}
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	style, _, _ := procGetWindowLongPtr.Call(hwnd, gwlStyleIndex)
	exStyle, _, _ := procGetWindowLongPtr.Call(hwnd, gwlExStyleIndex)
	foreground, _, _ := procGetForegroundWindow.Call()
	return WindowSample{
		Handle:        uint64(hwnd),
		Title:         title,
		Visible:       true,
		X:             int(rect.Left),
		Y:             int(rect.Top),
		Width:         int(rect.Right - rect.Left),
		Height:        int(rect.Bottom - rect.Top),
		StyleHex:      fmt.Sprintf("0x%08X", uint64(style)&0xFFFFFFFF),
		ExStyleHex:    fmt.Sprintf("0x%08X", uint64(exStyle)&0xFFFFFFFF),
		HasCaption:    uint64(style)&wsCaption != 0,
		HasThickFrame: uint64(style)&wsThickFrame != 0,
		IsTopmost:     uint64(exStyle)&wsExTopmost != 0,
		IsForeground:  foreground == hwnd,
	}
}

func hideWindow(hwnd uintptr) {
	procShowWindow.Call(hwnd, swHide)
}

func waitForWindowHidden(hwnd uintptr, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !windowVisible(hwnd) {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func windowVisible(hwnd uintptr) bool {
	visible, _, _ := procIsWindowVisible.Call(hwnd)
	return visible != 0
}

func sendAltQ() {
	inputs := []inputEvent{
		keyDown(vkMenu),
		keyDown(vkQ),
		keyUp(vkQ),
		keyUp(vkMenu),
	}
	procSendInput.Call(
		uintptr(len(inputs)),
		uintptr(unsafe.Pointer(&inputs[0])),
		unsafe.Sizeof(inputs[0]),
	)
}

type hotkeyRegisterAttempt struct {
	OK        bool
	ErrorCode int
	Error     string
}

func tryRegisterAltQ(id int32) hotkeyRegisterAttempt {
	result, _, callErr := procRegisterHotKey.Call(0, uintptr(id), uintptr(modAlt|modNoRepeat), uintptr(vkQ))
	if result != 0 {
		return hotkeyRegisterAttempt{OK: true}
	}
	attempt := hotkeyRegisterAttempt{}
	if errno, ok := callErr.(windows.Errno); ok {
		attempt.ErrorCode = int(errno)
	}
	if callErr != windows.ERROR_SUCCESS {
		attempt.Error = callErr.Error()
	}
	return attempt
}

func unregisterHotkeyProbe(id int32) {
	procUnregisterHotKey.Call(0, uintptr(id))
}

func keyDown(virtualKey uint16) inputEvent {
	return inputEvent{
		Type: inputKeyboard,
		Keyboard: keyboardInput{
			VirtualKey: virtualKey,
		},
	}
}

func keyUp(virtualKey uint16) inputEvent {
	event := keyDown(virtualKey)
	event.Keyboard.Flags = keyEventUp
	return event
}

func processWorkingSet(pid uint32) uint64 {
	handle, err := windows.OpenProcess(processQueryInformation|processQueryLimitedInformation|processVMRead, false, pid)
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(handle)
	counters := processMemoryCounters{CB: uint32(unsafe.Sizeof(processMemoryCounters{}))}
	ret, _, _ := procGetProcessMemory.Call(uintptr(handle), uintptr(unsafe.Pointer(&counters)), uintptr(counters.CB))
	if ret == 0 {
		return 0
	}
	return uint64(counters.WorkingSetSize)
}
