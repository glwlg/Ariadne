//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"ariadne/internal/capturehistory"

	"golang.org/x/sys/windows"
)

const (
	gwlStyleIndex   = ^uintptr(15)
	gwlExStyleIndex = ^uintptr(19)

	wsCaption    = 0x00C00000
	wsThickFrame = 0x00040000
	wsExTopmost  = 0x00000008

	swHide = 0

	vkA    = 0x41
	vkMenu = 0x12
	vkP    = 0x50
	vkQ    = 0x51

	inputKeyboard = 1
	keyEventUp    = 0x0002

	modAlt      = 0x0001
	modNoRepeat = 0x4000

	mouseEventMove     = 0x0001
	mouseEventLeftDown = 0x0002
	mouseEventLeftUp   = 0x0004

	wmHotkey = 0x0312
)

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	procEnumWindows         = user32.NewProc("EnumWindows")
	procGetWindowThreadPID  = user32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible     = user32.NewProc("IsWindowVisible")
	procGetWindowTextLength = user32.NewProc("GetWindowTextLengthW")
	procGetWindowText       = user32.NewProc("GetWindowTextW")
	procGetWindowRect       = user32.NewProc("GetWindowRect")
	procGetWindowLongPtr    = user32.NewProc("GetWindowLongPtrW")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procShowWindow          = user32.NewProc("ShowWindow")
	procRegisterHotKey      = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey    = user32.NewProc("UnregisterHotKey")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procSendInput           = user32.NewProc("SendInput")
	procSetCursorPos        = user32.NewProc("SetCursorPos")
	procMouseEvent          = user32.NewProc("mouse_event")
	procPostThreadMessage   = user32.NewProc("PostThreadMessageW")
)

type options struct {
	ExePath         string
	OutputPath      string
	Timeout         time.Duration
	KeepTemp        bool
	SelectionWidth  int
	SelectionHeight int
}

type report struct {
	ProductName       string                        `json:"productName"`
	CreatedAt         int64                         `json:"createdAt"`
	ExePath           string                        `json:"exePath"`
	TempRoot          string                        `json:"tempRoot,omitempty"`
	VirtualScreen     capturehistory.ScreenBounds   `json:"virtualScreen"`
	Monitors          []capturehistory.ScreenBounds `json:"monitors"`
	Selection         capturehistory.ScreenBounds   `json:"selection"`
	HotkeyBefore      map[string]hotkeyAttempt      `json:"hotkeyBefore,omitempty"`
	HotkeyDuring      map[string]hotkeyAttempt      `json:"hotkeyDuring,omitempty"`
	StartedProcessID  int                           `json:"startedProcessId,omitempty"`
	MainWindow        windowSample                  `json:"mainWindow,omitempty"`
	OverlayWindow     windowSample                  `json:"overlayWindow,omitempty"`
	PinnedWindow      windowSample                  `json:"pinnedWindow,omitempty"`
	PinnedAfterDrag   windowSample                  `json:"pinnedAfterDrag,omitempty"`
	CapturedImagePath string                        `json:"capturedImagePath,omitempty"`
	CapturedWidth     int                           `json:"capturedWidth,omitempty"`
	CapturedHeight    int                           `json:"capturedHeight,omitempty"`
	PixelMatchPercent float64                       `json:"pixelMatchPercent,omitempty"`
	MeanAbsDiff       float64                       `json:"meanAbsDiff,omitempty"`
	PositionDeltaX    int                           `json:"positionDeltaX,omitempty"`
	PositionDeltaY    int                           `json:"positionDeltaY,omitempty"`
	DragDeltaX        int                           `json:"dragDeltaX,omitempty"`
	DragDeltaY        int                           `json:"dragDeltaY,omitempty"`
	AppLogTail        []string                      `json:"appLogTail,omitempty"`
	Steps             []stepResult                  `json:"steps"`
	Pass              bool                          `json:"pass"`
	Error             string                        `json:"error,omitempty"`
}

type stepResult struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Skipped bool   `json:"skipped,omitempty"`
	Detail  string `json:"detail,omitempty"`
	Elapsed int64  `json:"elapsedMs,omitempty"`
}

type hotkeyAttempt struct {
	Available bool   `json:"available"`
	ErrorCode int    `json:"errorCode,omitempty"`
	Error     string `json:"error,omitempty"`
}

type windowSample struct {
	Handle        uint64 `json:"handle,omitempty"`
	Title         string `json:"title,omitempty"`
	Visible       bool   `json:"visible"`
	X             int    `json:"x"`
	Y             int    `json:"y"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	StyleHex      string `json:"styleHex,omitempty"`
	ExStyleHex    string `json:"exStyleHex,omitempty"`
	HasCaption    bool   `json:"hasCaption"`
	HasThickFrame bool   `json:"hasThickFrame"`
	IsTopmost     bool   `json:"isTopmost"`
	IsForeground  bool   `json:"isForeground"`
}

type winRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
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

func main() {
	opts := parseOptions()
	result := run(opts)
	raw, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if opts.OutputPath != "" {
		if err := os.MkdirAll(filepath.Dir(opts.OutputPath), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := os.WriteFile(opts.OutputPath, raw, 0o600); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	fmt.Println(string(raw))
	if !result.Pass {
		os.Exit(2)
	}
}

func parseOptions() options {
	opts := options{}
	var timeoutMs int64
	flag.StringVar(&opts.ExePath, "exe", filepath.Join("bin", "ariadne.exe"), "Path to ariadne.exe")
	flag.StringVar(&opts.OutputPath, "output", "", "Optional JSON report path")
	flag.Int64Var(&timeoutMs, "timeout-ms", 12000, "Timeout per UI wait in milliseconds")
	flag.BoolVar(&opts.KeepTemp, "keep-temp", false, "Keep temporary APPDATA/LOCALAPPDATA after the smoke run")
	flag.IntVar(&opts.SelectionWidth, "selection-width", 260, "Native selection width to drag")
	flag.IntVar(&opts.SelectionHeight, "selection-height", 180, "Native selection height to drag")
	flag.Parse()
	opts.Timeout = time.Duration(timeoutMs) * time.Millisecond
	if opts.Timeout <= 0 {
		opts.Timeout = 12 * time.Second
	}
	return opts
}

func run(opts options) report {
	startedAt := time.Now()
	result := report{
		ProductName: "Ariadne",
		CreatedAt:   startedAt.Unix(),
		ExePath:     opts.ExePath,
	}
	exePath, err := filepath.Abs(opts.ExePath)
	if err != nil {
		return result.fail("resolve exe", err)
	}
	result.ExePath = exePath
	if info, err := os.Stat(exePath); err != nil || info.IsDir() {
		return result.fail("check exe", fmt.Errorf("exe not found: %s", exePath))
	}

	result.VirtualScreen = capturehistory.VirtualScreenBounds()
	result.Monitors = sortedMonitors(capturehistory.MonitorBounds())
	monitor, ok := primaryMonitor(result.Monitors)
	if !ok {
		return result.fail("read monitors", fmt.Errorf("no monitor bounds available"))
	}
	selection := chooseSelection(monitor, opts.SelectionWidth, opts.SelectionHeight)
	result.Selection = selection
	result.HotkeyBefore = map[string]hotkeyAttempt{
		"alt+a": tryRegisterHotkey(65001, vkA),
		"alt+q": tryRegisterHotkey(65002, vkQ),
	}

	tempRoot, err := os.MkdirTemp("", "ariadne-capture-smoke-")
	if err != nil {
		return result.fail("create temp appdata", err)
	}
	result.TempRoot = tempRoot
	if !opts.KeepTemp {
		defer os.RemoveAll(tempRoot)
	}
	roaming := filepath.Join(tempRoot, "Roaming")
	local := filepath.Join(tempRoot, "Local")
	if err := os.MkdirAll(roaming, 0o755); err != nil {
		return result.fail("create temp roaming", err)
	}
	if err := os.MkdirAll(local, 0o755); err != nil {
		return result.fail("create temp local", err)
	}

	command := exec.Command(exePath)
	command.Env = append(os.Environ(), "APPDATA="+roaming, "LOCALAPPDATA="+local)
	if err := command.Start(); err != nil {
		return result.fail("start app", err)
	}
	result.StartedProcessID = command.Process.Pid
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

	mainWindow, ok, err := waitForWindow(uint32(command.Process.Pid), opts.Timeout, func(window windowSample) bool {
		return strings.EqualFold(window.Title, "Ariadne")
	}, exited)
	if err != nil {
		return result.fail("wait main window", err)
	}
	if !ok {
		return result.fail("wait main window", fmt.Errorf("main Ariadne window not found within %s", opts.Timeout))
	}
	result.MainWindow = mainWindow
	result.HotkeyDuring = map[string]hotkeyAttempt{
		"alt+a": tryRegisterHotkey(65003, vkA),
		"alt+q": tryRegisterHotkey(65004, vkQ),
	}
	result.addStep("hotkey_registration", hotkeyBlocked(result.HotkeyDuring["alt+a"]) && hotkeyBlocked(result.HotkeyDuring["alt+q"]), fmt.Sprintf("during alt+a=%s alt+q=%s", hotkeyAttemptText(result.HotkeyDuring["alt+a"]), hotkeyAttemptText(result.HotkeyDuring["alt+q"])), 0)
	result.addStep("start_app", true, fmt.Sprintf("pid=%d title=%q", command.Process.Pid, mainWindow.Title), time.Since(startedAt))

	hideWindow(uintptr(mainWindow.Handle))
	if !waitForHidden(uintptr(mainWindow.Handle), 2*time.Second) {
		result.addStep("hide_main", false, "main window stayed visible before capture", 2*time.Second)
	} else {
		result.addStep("hide_main", true, "main window hidden before reference and overlay capture", 0)
	}

	moveCursor(selection.X-10, selection.Y-10)
	var referenceImage image.Image
	referencePNG, _, _, err := capturehistory.CaptureRegionPNG(selection.X, selection.Y, selection.Width, selection.Height)
	if err != nil {
		result.addSkippedStep("reference_capture", fmt.Sprintf("reference BitBlt unavailable in this desktop session: %v", err), 0)
	} else {
		referenceImage, err = decodePNG(referencePNG)
		if err != nil {
			result.addSkippedStep("reference_capture", fmt.Sprintf("reference PNG decode failed: %v", err), 0)
		} else {
			result.addStep("reference_capture", true, fmt.Sprintf("%dx%d at %d,%d", selection.Width, selection.Height, selection.X, selection.Y), 0)
		}
	}

	overlayStart := time.Now()
	sendAltKey(vkA)
	pid := uint32(command.Process.Pid)
	overlays, ok, err := waitForWindows(pid, minDuration(opts.Timeout, 1800*time.Millisecond), func(window windowSample) bool {
		return strings.Contains(window.Title, "截图覆盖层")
	}, exited)
	if err != nil {
		return result.fail("wait capture overlay", err)
	}
	if !ok {
		posted, postErr := postHotkeyToProcessThreads(pid, 2)
		if postErr != nil {
			result.addStep("fallback_post_screenshot_hotkey", false, postErr.Error(), 0)
		} else {
			result.addStep("fallback_post_screenshot_hotkey", posted > 0, fmt.Sprintf("posted WM_HOTKEY id=2 to %d process thread(s)", posted), 0)
		}
		overlays, ok, err = waitForWindows(pid, opts.Timeout, func(window windowSample) bool {
			return strings.Contains(window.Title, "截图覆盖层")
		}, exited)
		if err != nil {
			return result.fail("wait capture overlay", err)
		}
	}
	if !ok {
		result.AppLogTail = readLogTail(filepath.Join(roaming, "Ariadne", "logs", "ariadne.log"), 40)
		return result.fail("wait capture overlay", fmt.Errorf("capture overlay not found within %s after Alt+A", opts.Timeout))
	}
	overlay := chooseOverlay(overlays, selection)
	result.OverlayWindow = overlay
	result.addStep("open_overlay_alt_a", true, fmt.Sprintf("overlays=%d chosen=%q", len(overlays), overlay.Title), time.Since(overlayStart))

	procSetForegroundWindow.Call(uintptr(overlay.Handle))
	time.Sleep(120 * time.Millisecond)
	dragMouse(selection.X, selection.Y, selection.X+selection.Width, selection.Y+selection.Height)
	time.Sleep(350 * time.Millisecond)
	result.addStep("drag_selection", true, fmt.Sprintf("%dx%d", selection.Width, selection.Height), 0)

	sendKey(vkP)
	pinStart := time.Now()
	pinnedWindow, ok, err := waitForWindow(uint32(command.Process.Pid), opts.Timeout, func(window windowSample) bool {
		return strings.Contains(window.Title, "截图贴图")
	}, exited)
	if err != nil {
		return result.fail("wait pinned window", err)
	}
	if !ok {
		return result.fail("wait pinned window", fmt.Errorf("pinned image window not found within %s after P", opts.Timeout))
	}
	result.PinnedWindow = pinnedWindow
	result.addStep("pin_selection", true, fmt.Sprintf("title=%q", pinnedWindow.Title), time.Since(pinStart))

	latestPNG, err := latestPNG(filepath.Join(roaming, "Ariadne", "capture_images"))
	if err != nil {
		return result.fail("find captured png", err)
	}
	result.CapturedImagePath = latestPNG
	capturedFile, err := os.ReadFile(latestPNG)
	if err != nil {
		return result.fail("read captured png", err)
	}
	capturedImage, err := decodePNG(capturedFile)
	if err != nil {
		return result.fail("decode captured png", err)
	}
	result.CapturedWidth = capturedImage.Bounds().Dx()
	result.CapturedHeight = capturedImage.Bounds().Dy()
	dimensionsOK := result.CapturedWidth == selection.Width && result.CapturedHeight == selection.Height
	result.addStep(
		"check_capture_dimensions",
		dimensionsOK,
		fmt.Sprintf("saved=%dx%d expected=%dx%d", result.CapturedWidth, result.CapturedHeight, selection.Width, selection.Height),
		0,
	)
	if referenceImage == nil {
		result.addSkippedStep("compare_capture_content", "reference image unavailable; content equality was not checked", 0)
	} else {
		matchPercent, meanDiff := compareImages(referenceImage, capturedImage)
		result.PixelMatchPercent = matchPercent
		result.MeanAbsDiff = meanDiff
		contentOK := matchPercent >= 98 && meanDiff <= 2
		result.addStep(
			"compare_capture_content",
			contentOK,
			fmt.Sprintf("match=%.2f%% mean_abs_diff=%.3f", matchPercent, meanDiff),
			0,
		)
	}

	expectedX := selection.X - 15
	expectedY := selection.Y - 15
	result.PositionDeltaX = pinnedWindow.X - expectedX
	result.PositionDeltaY = pinnedWindow.Y - expectedY
	positionOK := abs(result.PositionDeltaX) <= 80 && abs(result.PositionDeltaY) <= 80
	result.addStep(
		"check_pin_position",
		positionOK,
		fmt.Sprintf("window=%d,%d expected_near=%d,%d delta=%d,%d", pinnedWindow.X, pinnedWindow.Y, expectedX, expectedY, result.PositionDeltaX, result.PositionDeltaY),
		0,
	)

	dragStartX := pinnedWindow.X + max(30, pinnedWindow.Width/2)
	dragStartY := pinnedWindow.Y + max(44, pinnedWindow.Height/2)
	if dragStartX >= pinnedWindow.X+pinnedWindow.Width-8 {
		dragStartX = pinnedWindow.X + pinnedWindow.Width/2
	}
	if dragStartY >= pinnedWindow.Y+pinnedWindow.Height-8 {
		dragStartY = pinnedWindow.Y + pinnedWindow.Height/2
	}
	dragMouse(dragStartX, dragStartY, dragStartX+90, dragStartY+55)
	time.Sleep(650 * time.Millisecond)
	afterDrag := readWindowSample(uintptr(pinnedWindow.Handle), pinnedWindow.Title)
	result.PinnedAfterDrag = afterDrag
	result.DragDeltaX = afterDrag.X - pinnedWindow.X
	result.DragDeltaY = afterDrag.Y - pinnedWindow.Y
	dragOK := abs(result.DragDeltaX) >= 20 || abs(result.DragDeltaY) >= 20
	result.addStep(
		"drag_pinned_window",
		dragOK,
		fmt.Sprintf("delta=%d,%d", result.DragDeltaX, result.DragDeltaY),
		0,
	)

	result.Pass = true
	result.AppLogTail = readLogTail(filepath.Join(roaming, "Ariadne", "logs", "ariadne.log"), 40)
	for _, step := range result.Steps {
		if !step.OK && !step.Skipped {
			result.Pass = false
			break
		}
	}
	return result
}

func (r report) fail(name string, err error) report {
	r.addStep(name, false, err.Error(), 0)
	r.Error = err.Error()
	r.Pass = false
	return r
}

func (r *report) addStep(name string, ok bool, detail string, elapsed time.Duration) {
	step := stepResult{Name: name, OK: ok, Detail: detail}
	if elapsed > 0 {
		step.Elapsed = elapsed.Milliseconds()
	}
	r.Steps = append(r.Steps, step)
}

func (r *report) addSkippedStep(name string, detail string, elapsed time.Duration) {
	step := stepResult{Name: name, OK: false, Skipped: true, Detail: detail}
	if elapsed > 0 {
		step.Elapsed = elapsed.Milliseconds()
	}
	r.Steps = append(r.Steps, step)
}

func sortedMonitors(monitors []capturehistory.ScreenBounds) []capturehistory.ScreenBounds {
	next := append([]capturehistory.ScreenBounds(nil), monitors...)
	sort.SliceStable(next, func(i int, j int) bool {
		if next[i].Y == next[j].Y {
			return next[i].X < next[j].X
		}
		return next[i].Y < next[j].Y
	})
	return next
}

func primaryMonitor(monitors []capturehistory.ScreenBounds) (capturehistory.ScreenBounds, bool) {
	for _, monitor := range monitors {
		if monitor.X <= 0 && monitor.Y <= 0 && monitor.X+monitor.Width > 0 && monitor.Y+monitor.Height > 0 {
			return monitor, true
		}
	}
	if len(monitors) > 0 {
		return monitors[0], true
	}
	return capturehistory.ScreenBounds{}, false
}

func chooseSelection(monitor capturehistory.ScreenBounds, requestedWidth int, requestedHeight int) capturehistory.ScreenBounds {
	width := clamp(requestedWidth, 80, max(80, monitor.Width-180))
	height := clamp(requestedHeight, 60, max(60, monitor.Height-180))
	xOffset := clamp(160, 30, max(30, monitor.Width-width-30))
	yOffset := clamp(140, 30, max(30, monitor.Height-height-30))
	return capturehistory.ScreenBounds{
		X:      monitor.X + xOffset,
		Y:      monitor.Y + yOffset,
		Width:  width,
		Height: height,
	}
}

func waitForWindow(pid uint32, timeout time.Duration, predicate func(windowSample) bool, exited <-chan error) (windowSample, bool, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, window := range processWindows(pid) {
			if predicate(window) {
				return window, true, nil
			}
		}
		select {
		case err := <-exited:
			if err != nil {
				return windowSample{}, false, fmt.Errorf("process exited before window appeared: %w", err)
			}
			return windowSample{}, false, fmt.Errorf("process exited before window appeared")
		default:
		}
		time.Sleep(30 * time.Millisecond)
	}
	return windowSample{}, false, nil
}

func waitForWindows(pid uint32, timeout time.Duration, predicate func(windowSample) bool, exited <-chan error) ([]windowSample, bool, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		matches := []windowSample{}
		for _, window := range processWindows(pid) {
			if predicate(window) {
				matches = append(matches, window)
			}
		}
		if len(matches) > 0 {
			return matches, true, nil
		}
		select {
		case err := <-exited:
			if err != nil {
				return nil, false, fmt.Errorf("process exited before window appeared: %w", err)
			}
			return nil, false, fmt.Errorf("process exited before window appeared")
		default:
		}
		time.Sleep(30 * time.Millisecond)
	}
	return nil, false, nil
}

func processWindows(pid uint32) []windowSample {
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
		if strings.TrimSpace(title) == "" {
			return 1
		}
		search.windows = append(search.windows, readWindowSample(hwnd, title))
		return 1
	})
	procEnumWindows.Call(callback, uintptr(unsafe.Pointer(state)))
	return state.windows
}

type windowSearchState struct {
	pid     uint32
	windows []windowSample
}

func chooseOverlay(overlays []windowSample, selection capturehistory.ScreenBounds) windowSample {
	cx := selection.X + selection.Width/2
	cy := selection.Y + selection.Height/2
	for _, overlay := range overlays {
		if cx >= overlay.X && cy >= overlay.Y && cx < overlay.X+overlay.Width && cy < overlay.Y+overlay.Height {
			return overlay
		}
	}
	sort.SliceStable(overlays, func(i int, j int) bool {
		if overlays[i].Y == overlays[j].Y {
			return overlays[i].X < overlays[j].X
		}
		return overlays[i].Y < overlays[j].Y
	})
	return overlays[0]
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

func readWindowSample(hwnd uintptr, title string) windowSample {
	rect := winRect{}
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	style, _, _ := procGetWindowLongPtr.Call(hwnd, gwlStyleIndex)
	exStyle, _, _ := procGetWindowLongPtr.Call(hwnd, gwlExStyleIndex)
	foreground, _, _ := procGetForegroundWindow.Call()
	return windowSample{
		Handle:        uint64(hwnd),
		Title:         title,
		Visible:       windowVisible(hwnd),
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

func windowVisible(hwnd uintptr) bool {
	visible, _, _ := procIsWindowVisible.Call(hwnd)
	return visible != 0
}

func hideWindow(hwnd uintptr) {
	procShowWindow.Call(hwnd, swHide)
}

func waitForHidden(hwnd uintptr, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !windowVisible(hwnd) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

func moveCursor(x int, y int) {
	procSetCursorPos.Call(uintptr(x), uintptr(y))
}

func dragMouse(x1 int, y1 int, x2 int, y2 int) {
	moveCursor(x1, y1)
	time.Sleep(70 * time.Millisecond)
	procMouseEvent.Call(mouseEventLeftDown, 0, 0, 0, 0)
	steps := 14
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		x := int(math.Round(float64(x1) + (float64(x2)-float64(x1))*t))
		y := int(math.Round(float64(y1) + (float64(y2)-float64(y1))*t))
		moveCursor(x, y)
		procMouseEvent.Call(mouseEventMove, 0, 0, 0, 0)
		time.Sleep(18 * time.Millisecond)
	}
	procMouseEvent.Call(mouseEventLeftUp, 0, 0, 0, 0)
}

func sendAltKey(virtualKey uint16) {
	inputs := []inputEvent{
		keyDown(vkMenu),
		keyDown(virtualKey),
		keyUp(virtualKey),
		keyUp(vkMenu),
	}
	sendInputs(inputs)
}

func sendKey(virtualKey uint16) {
	sendInputs([]inputEvent{keyDown(virtualKey), keyUp(virtualKey)})
}

func sendInputs(inputs []inputEvent) {
	if len(inputs) == 0 {
		return
	}
	procSendInput.Call(
		uintptr(len(inputs)),
		uintptr(unsafe.Pointer(&inputs[0])),
		unsafe.Sizeof(inputs[0]),
	)
	time.Sleep(120 * time.Millisecond)
}

func postHotkeyToProcessThreads(pid uint32, hotkeyID int32) (int, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(snapshot)

	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	if err := windows.Thread32First(snapshot, &entry); err != nil {
		return 0, err
	}
	posted := 0
	for {
		if entry.OwnerProcessID == pid {
			result, _, _ := procPostThreadMessage.Call(uintptr(entry.ThreadID), wmHotkey, uintptr(hotkeyID), 0)
			if result != 0 {
				posted++
			}
		}
		err = windows.Thread32Next(snapshot, &entry)
		if err != nil {
			if err == windows.ERROR_NO_MORE_FILES {
				break
			}
			return posted, err
		}
	}
	return posted, nil
}

func tryRegisterHotkey(id int32, virtualKey uint16) hotkeyAttempt {
	result, _, callErr := procRegisterHotKey.Call(0, uintptr(id), uintptr(modAlt|modNoRepeat), uintptr(virtualKey))
	if result != 0 {
		procUnregisterHotKey.Call(0, uintptr(id))
		return hotkeyAttempt{Available: true}
	}
	attempt := hotkeyAttempt{}
	if errno, ok := callErr.(windows.Errno); ok {
		attempt.ErrorCode = int(errno)
	}
	if callErr != windows.ERROR_SUCCESS {
		attempt.Error = callErr.Error()
	}
	return attempt
}

func hotkeyBlocked(attempt hotkeyAttempt) bool {
	return !attempt.Available && attempt.ErrorCode == int(windows.ERROR_HOTKEY_ALREADY_REGISTERED)
}

func hotkeyAttemptText(attempt hotkeyAttempt) string {
	if attempt.Available {
		return "available"
	}
	if attempt.ErrorCode != 0 {
		return fmt.Sprintf("blocked_or_failed:%d:%s", attempt.ErrorCode, attempt.Error)
	}
	return "blocked_or_failed"
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

func latestPNG(root string) (string, error) {
	var latest string
	var latestTime time.Time
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".png") {
			return nil
		}
		info, statErr := entry.Info()
		if statErr != nil {
			return nil
		}
		if latest == "" || info.ModTime().After(latestTime) {
			latest = path
			latestTime = info.ModTime()
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if latest == "" {
		return "", fmt.Errorf("no png found under %s", root)
	}
	return latest, nil
}

func readLogTail(path string, limit int) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	if limit > 0 && len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	for index := range lines {
		lines[index] = strings.TrimRight(lines[index], "\r")
	}
	return lines
}

func decodePNG(data []byte) (image.Image, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return img, nil
}

func compareImages(reference image.Image, captured image.Image) (float64, float64) {
	refBounds := reference.Bounds()
	capBounds := captured.Bounds()
	if refBounds.Dx() != capBounds.Dx() || refBounds.Dy() != capBounds.Dy() {
		return 0, math.MaxFloat64
	}
	var exact int64
	var total int64
	var diffTotal float64
	for y := 0; y < refBounds.Dy(); y++ {
		for x := 0; x < refBounds.Dx(); x++ {
			ref := rgba8(reference.At(refBounds.Min.X+x, refBounds.Min.Y+y))
			got := rgba8(captured.At(capBounds.Min.X+x, capBounds.Min.Y+y))
			if ref == got {
				exact++
			}
			diffTotal += math.Abs(float64(int(ref.R) - int(got.R)))
			diffTotal += math.Abs(float64(int(ref.G) - int(got.G)))
			diffTotal += math.Abs(float64(int(ref.B) - int(got.B)))
			total++
		}
	}
	if total == 0 {
		return 0, math.MaxFloat64
	}
	return float64(exact) * 100 / float64(total), diffTotal / float64(total*3)
}

func rgba8(value color.Color) color.RGBA {
	r, g, b, a := value.RGBA()
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
}

func clamp(value int, minValue int, maxValue int) int {
	if maxValue < minValue {
		return minValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func minDuration(a time.Duration, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
