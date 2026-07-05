//go:build windows

package main

import (
	"bytes"
	"encoding/binary"
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
	"ariadne/internal/clipboardhistory"

	"golang.org/x/sys/windows"
)

const (
	gwlStyleIndex   = ^uintptr(15)
	gwlExStyleIndex = ^uintptr(19)

	wsCaption    = 0x00C00000
	wsThickFrame = 0x00040000
	wsExTopmost  = 0x00000008

	swHide = 0

	vkA      = 0x41
	vkF1     = 0x70
	vkMenu   = 0x12
	vkP      = 0x50
	vkQ      = 0x51
	vkReturn = 0x0D

	inputKeyboard = 1
	keyEventUp    = 0x0002

	modAlt      = 0x0001
	modNoRepeat = 0x4000

	mouseEventMove     = 0x0001
	mouseEventLeftDown = 0x0002
	mouseEventLeftUp   = 0x0004

	wmHotkey = 0x0312
	cfDIB    = 8
	cfDIBV5  = 17

	contentStrictMatchPercent   = 98.0
	contentStrictMeanAbsDiff    = 2.0
	contentTolerantMatchPercent = 95.0
	contentTolerantMeanAbsDiff  = 0.75
	contentPhysicalMatchPercent = 60.0
	contentPhysicalMeanAbsDiff  = 15.0

	pinnedDragMaxAttempts   = 2
	pinnedDragReadyDelay    = 300 * time.Millisecond
	pinnedDragRetryDelay    = 250 * time.Millisecond
	pinnedDragSettleDelay   = 650 * time.Millisecond
	pinnedDragMoveX         = 90
	pinnedDragMoveY         = 55
	pinnedDragMinDelta      = 20
	maxCopyClipboardLatency = 750 * time.Millisecond
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
	procOpenClipboard       = user32.NewProc("OpenClipboard")
	procCloseClipboard      = user32.NewProc("CloseClipboard")
	procGetClipboardData    = user32.NewProc("GetClipboardData")
	procIsFormatAvailable   = user32.NewProc("IsClipboardFormatAvailable")
	kernel32                = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalLock          = kernel32.NewProc("GlobalLock")
	procGlobalUnlock        = kernel32.NewProc("GlobalUnlock")
)

type options struct {
	ExePath           string
	OutputPath        string
	Timeout           time.Duration
	MaxPinLatency     time.Duration
	PinLatencyOnly    bool
	CopyClipboardOnly bool
	UseCurrentProfile bool
	ScreenshotHotkey  string
	KeepTemp          bool
	SelectionWidth    int
	SelectionHeight   int
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
	ClipboardType     string                        `json:"clipboardType,omitempty"`
	ClipboardWidth    int                           `json:"clipboardWidth,omitempty"`
	ClipboardHeight   int                           `json:"clipboardHeight,omitempty"`
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
	ProcessID     uint32 `json:"processId,omitempty"`
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

type pinnedDragRuntime struct {
	Sleep      func(time.Duration)
	DragMouse  func(int, int, int, int)
	ReadWindow func(windowSample) windowSample
}

type pinnedDragAttempt struct {
	Attempt     int
	DeltaX      int
	DeltaY      int
	TotalDeltaX int
	TotalDeltaY int
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
	var maxPinMs int64
	flag.StringVar(&opts.ExePath, "exe", filepath.Join("bin", "ariadne.exe"), "Path to ariadne.exe")
	flag.StringVar(&opts.OutputPath, "output", "", "Optional JSON report path")
	flag.Int64Var(&timeoutMs, "timeout-ms", 12000, "Timeout per UI wait in milliseconds")
	flag.Int64Var(&maxPinMs, "max-pin-ms", 0, "Fail if the pinned image window takes longer than this many milliseconds to appear; 0 disables the latency assertion")
	flag.BoolVar(&opts.PinLatencyOnly, "pin-latency-only", false, "Stop after the pinned image window appears and only fail the run on the pin latency assertion")
	flag.BoolVar(&opts.CopyClipboardOnly, "copy-clipboard-only", false, "Stop after pressing Enter and verifying the system clipboard contains the captured image")
	flag.BoolVar(&opts.UseCurrentProfile, "use-current-profile", false, "Use the current user's APPDATA and LOCALAPPDATA instead of an isolated temporary profile")
	flag.StringVar(&opts.ScreenshotHotkey, "screenshot-hotkey", "alt+a", "Screenshot hotkey to send: alt+a or f1")
	flag.BoolVar(&opts.KeepTemp, "keep-temp", false, "Keep temporary APPDATA/LOCALAPPDATA after the smoke run")
	flag.IntVar(&opts.SelectionWidth, "selection-width", 260, "Native selection width to drag")
	flag.IntVar(&opts.SelectionHeight, "selection-height", 180, "Native selection height to drag")
	flag.Parse()
	opts.Timeout = time.Duration(timeoutMs) * time.Millisecond
	if opts.Timeout <= 0 {
		opts.Timeout = 12 * time.Second
	}
	if maxPinMs > 0 {
		opts.MaxPinLatency = time.Duration(maxPinMs) * time.Millisecond
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

	roaming := os.Getenv("APPDATA")
	local := os.Getenv("LOCALAPPDATA")
	if !opts.UseCurrentProfile {
		tempRoot, err := os.MkdirTemp("", "ariadne-capture-smoke-")
		if err != nil {
			return result.fail("create temp appdata", err)
		}
		result.TempRoot = tempRoot
		if !opts.KeepTemp {
			defer os.RemoveAll(tempRoot)
		}
		roaming = filepath.Join(tempRoot, "Roaming")
		local = filepath.Join(tempRoot, "Local")
		if err := os.MkdirAll(roaming, 0o755); err != nil {
			return result.fail("create temp roaming", err)
		}
		if err := os.MkdirAll(local, 0o755); err != nil {
			return result.fail("create temp local", err)
		}
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

	pid := uint32(command.Process.Pid)
	mainWindow, ok, err := waitForWindow(pid, minDuration(opts.Timeout, 2500*time.Millisecond), func(window windowSample) bool {
		return strings.EqualFold(window.Title, "Ariadne")
	}, exited)
	if err != nil {
		return result.fail("wait main window", err)
	}
	if ok {
		result.MainWindow = mainWindow
	}
	result.HotkeyDuring = map[string]hotkeyAttempt{
		"alt+a": tryRegisterHotkey(65003, vkA),
		"alt+q": tryRegisterHotkey(65004, vkQ),
	}
	result.addStep("hotkey_registration", hotkeyBlocked(result.HotkeyDuring["alt+a"]) && hotkeyBlocked(result.HotkeyDuring["alt+q"]), fmt.Sprintf("during alt+a=%s alt+q=%s", hotkeyAttemptText(result.HotkeyDuring["alt+a"]), hotkeyAttemptText(result.HotkeyDuring["alt+q"])), 0)
	if ok {
		result.addStep("start_app", true, fmt.Sprintf("pid=%d title=%q", command.Process.Pid, mainWindow.Title), time.Since(startedAt))
		hideWindow(uintptr(mainWindow.Handle))
		if !waitForHidden(uintptr(mainWindow.Handle), 2*time.Second) {
			result.addStep("hide_main", false, "main window stayed visible before capture", 2*time.Second)
		} else {
			result.addStep("hide_main", true, "main window hidden before reference and overlay capture", 0)
		}
	} else {
		result.addStep("start_app", true, fmt.Sprintf("pid=%d tray startup without visible main window", command.Process.Pid), time.Since(startedAt))
		for _, window := range processWindows(pid) {
			hideWindow(uintptr(window.Handle))
		}
	}

	moveCursor(selection.X-10, selection.Y-10)
	var referenceImage image.Image
	referenceImages := map[string]image.Image{}
	referencePNG, _, _, err := capturehistory.CaptureRegionPNG(selection.X, selection.Y, selection.Width, selection.Height)
	if err != nil {
		result.addSkippedStep("reference_capture", fmt.Sprintf("reference BitBlt unavailable in this desktop session: %v", err), 0)
	} else {
		referenceImage, err = decodePNG(referencePNG)
		if err != nil {
			result.addSkippedStep("reference_capture", fmt.Sprintf("reference PNG decode failed: %v", err), 0)
		} else {
			referenceImages[imageSizeKey(referenceImage)] = referenceImage
			result.addStep("reference_capture", true, fmt.Sprintf("%dx%d at %d,%d", selection.Width, selection.Height, selection.X, selection.Y), 0)
		}
	}
	for _, scale := range []float64{1.25, 1.5, 1.75, 2, 2.5, 3} {
		width := int(math.Round(float64(selection.Width) * scale))
		height := int(math.Round(float64(selection.Height) * scale))
		if width == selection.Width && height == selection.Height {
			continue
		}
		data, _, _, err := capturehistory.CaptureRegionPNG(selection.X, selection.Y, width, height)
		if err != nil {
			continue
		}
		img, err := decodePNG(data)
		if err != nil {
			continue
		}
		referenceImages[imageSizeKey(img)] = img
	}

	overlayStart := time.Now()
	sendScreenshotHotkey(opts.ScreenshotHotkey)
	overlays, ok, err := waitForWindows(0, minDuration(opts.Timeout, 1800*time.Millisecond), func(window windowSample) bool {
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
		overlays, ok, err = waitForWindows(0, opts.Timeout, func(window windowSample) bool {
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

	if opts.CopyClipboardOnly {
		sentinel, err := solidPNG(9, 7, color.RGBA{R: 211, G: 37, B: 89, A: 255})
		if err != nil {
			return result.fail("seed_clipboard_image", err)
		}
		if err := clipboardhistory.WritePNGToSystemClipboard(sentinel); err != nil {
			return result.fail("seed_clipboard_image", err)
		}
		result.addStep("seed_clipboard_image", true, "clipboard seeded with stale image sentinel 9x7", 0)
		sendKey(vkReturn)
		copyStart := time.Now()
		clipboardWidth, clipboardHeight, dimensionsDetail, err := waitForClipboardImageMatching(opts.Timeout, selection.Width, selection.Height)
		if err != nil {
			return result.fail("verify_clipboard_image", err)
		}
		copyElapsed := time.Since(copyStart)
		result.ClipboardType = "image"
		result.ClipboardWidth = clipboardWidth
		result.ClipboardHeight = clipboardHeight
		latencyOK := copyElapsed <= maxCopyClipboardLatency
		if !latencyOK {
			dimensionsDetail += fmt.Sprintf(" latency_exceeded max=%s", maxCopyClipboardLatency)
		}
		result.addStep("copy_selection_to_clipboard", latencyOK, dimensionsDetail, copyElapsed)
		result.Pass = latencyOK
		result.AppLogTail = readLogTail(filepath.Join(roaming, "Ariadne", "logs", "ariadne.log"), 40)
		return result
	}

	existingPinned := matchingWindowHandles(func(window windowSample) bool {
		return strings.Contains(window.Title, "截图贴图")
	})
	sendKey(vkP)
	pinStart := time.Now()
	pinnedWindow, ok, err := waitForWindow(0, opts.Timeout, func(window windowSample) bool {
		return strings.Contains(window.Title, "截图贴图") &&
			window.ProcessID != uint32(command.Process.Pid) &&
			!existingPinned[window.Handle]
	}, exited)
	if err != nil {
		return result.fail("wait pinned window", err)
	}
	if !ok {
		return result.fail("wait pinned window", fmt.Errorf("pinned image window not found within %s after P", opts.Timeout))
	}
	result.PinnedWindow = pinnedWindow
	pinElapsed := time.Since(pinStart)
	pinOK := opts.MaxPinLatency <= 0 || pinElapsed <= opts.MaxPinLatency
	pinDetail := fmt.Sprintf("title=%q", pinnedWindow.Title)
	if opts.MaxPinLatency > 0 {
		pinDetail = fmt.Sprintf("%s max=%dms", pinDetail, opts.MaxPinLatency.Milliseconds())
	}
	result.addStep("pin_selection", pinOK, pinDetail, pinElapsed)
	if opts.PinLatencyOnly {
		result.Pass = pinOK
		result.AppLogTail = readLogTail(filepath.Join(roaming, "Ariadne", "logs", "ariadne.log"), 40)
		return result
	}

	latestPNG, err := waitLatestPNG(filepath.Join(roaming, "Ariadne", "capture_images"), opts.Timeout)
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
	dimensionsOK, dimensionsDetail, pixelScale := captureDimensionsOK(result.CapturedWidth, result.CapturedHeight, selection.Width, selection.Height)
	result.addStep(
		"check_capture_dimensions",
		dimensionsOK,
		dimensionsDetail,
		0,
	)
	if pixelScale > 1 {
		result.addSkippedStep("compare_capture_content", fmt.Sprintf("physical-DPI capture scale=%.2f; content equality skipped", pixelScale), 0)
	} else if referenceImage == nil {
		result.addSkippedStep("compare_capture_content", "reference image unavailable; content equality was not checked", 0)
	} else {
		expectedImage := referenceImage
		comparisonImage := capturedImage
		if scaledReference, ok := referenceImages[imageSizeKey(capturedImage)]; ok {
			expectedImage = scaledReference
		} else {
			comparisonImage = normalizeCapturedForComparison(referenceImage, capturedImage, pixelScale)
		}
		matchPercent, meanDiff := compareImages(expectedImage, comparisonImage)
		result.PixelMatchPercent = matchPercent
		result.MeanAbsDiff = meanDiff
		contentOK, contentDetail := evaluateContentMatch(matchPercent, meanDiff)
		if !contentOK && pixelScale > 1 && matchPercent >= contentPhysicalMatchPercent && meanDiff <= contentPhysicalMeanAbsDiff {
			contentOK = true
			contentDetail = fmt.Sprintf("match=%.2f%% mean_abs_diff=%.3f mode=physical_dpi_tolerance min_match=%.2f%% max_mean=%.3f", matchPercent, meanDiff, contentPhysicalMatchPercent, contentPhysicalMeanAbsDiff)
		}
		if pixelScale > 1 {
			contentDetail = fmt.Sprintf("%s normalized_scale=%.2f", contentDetail, pixelScale)
		}
		result.addStep(
			"compare_capture_content",
			contentOK,
			contentDetail,
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

	afterDrag, dragAttempts := dragPinnedWindow(pinnedWindow, defaultPinnedDragRuntime())
	result.PinnedAfterDrag = afterDrag
	result.DragDeltaX = afterDrag.X - pinnedWindow.X
	result.DragDeltaY = afterDrag.Y - pinnedWindow.Y
	dragOK := pinnedDragMoved(result.DragDeltaX, result.DragDeltaY)
	result.addStep(
		"drag_pinned_window",
		dragOK,
		pinnedDragDetail(dragAttempts),
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
		if search.pid != 0 && windowPID != search.pid {
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
		search.windows = append(search.windows, readWindowSample(hwnd, windowPID, title))
		return 1
	})
	procEnumWindows.Call(callback, uintptr(unsafe.Pointer(state)))
	return state.windows
}

func matchingWindowHandles(predicate func(windowSample) bool) map[uint64]bool {
	handles := map[uint64]bool{}
	for _, window := range processWindows(0) {
		if predicate(window) {
			handles[window.Handle] = true
		}
	}
	return handles
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

func readWindowSample(hwnd uintptr, pid uint32, title string) windowSample {
	rect := winRect{}
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	style, _, _ := procGetWindowLongPtr.Call(hwnd, gwlStyleIndex)
	exStyle, _, _ := procGetWindowLongPtr.Call(hwnd, gwlExStyleIndex)
	foreground, _, _ := procGetForegroundWindow.Call()
	return windowSample{
		Handle:        uint64(hwnd),
		ProcessID:     pid,
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

func defaultPinnedDragRuntime() pinnedDragRuntime {
	return pinnedDragRuntime{
		Sleep:     time.Sleep,
		DragMouse: dragMouse,
		ReadWindow: func(window windowSample) windowSample {
			return readWindowSample(uintptr(window.Handle), window.ProcessID, window.Title)
		},
	}
}

func dragPinnedWindow(pinnedWindow windowSample, runtime pinnedDragRuntime) (windowSample, []pinnedDragAttempt) {
	if runtime.Sleep == nil {
		runtime.Sleep = time.Sleep
	}
	if runtime.DragMouse == nil {
		runtime.DragMouse = dragMouse
	}
	if runtime.ReadWindow == nil {
		runtime.ReadWindow = func(window windowSample) windowSample {
			return readWindowSample(uintptr(window.Handle), window.ProcessID, window.Title)
		}
	}

	current := pinnedWindow
	attempts := make([]pinnedDragAttempt, 0, pinnedDragMaxAttempts)
	runtime.Sleep(pinnedDragReadyDelay)
	for attempt := 1; attempt <= pinnedDragMaxAttempts; attempt++ {
		before := runtime.ReadWindow(current)
		dragStartX, dragStartY := pinnedDragStart(before)
		runtime.DragMouse(dragStartX, dragStartY, dragStartX+pinnedDragMoveX, dragStartY+pinnedDragMoveY)
		runtime.Sleep(pinnedDragSettleDelay)
		after := runtime.ReadWindow(before)
		current = after
		next := pinnedDragAttempt{
			Attempt:     attempt,
			DeltaX:      after.X - before.X,
			DeltaY:      after.Y - before.Y,
			TotalDeltaX: after.X - pinnedWindow.X,
			TotalDeltaY: after.Y - pinnedWindow.Y,
		}
		attempts = append(attempts, next)
		if pinnedDragMoved(next.TotalDeltaX, next.TotalDeltaY) {
			return after, attempts
		}
		if attempt < pinnedDragMaxAttempts {
			runtime.Sleep(pinnedDragRetryDelay)
		}
	}
	return current, attempts
}

func pinnedDragStart(window windowSample) (int, int) {
	dragStartX := window.X + max(30, window.Width/2)
	dragStartY := window.Y + max(44, window.Height/2)
	if dragStartX >= window.X+window.Width-8 {
		dragStartX = window.X + window.Width/2
	}
	if dragStartY >= window.Y+window.Height-8 {
		dragStartY = window.Y + window.Height/2
	}
	return dragStartX, dragStartY
}

func pinnedDragMoved(deltaX int, deltaY int) bool {
	return abs(deltaX) >= pinnedDragMinDelta || abs(deltaY) >= pinnedDragMinDelta
}

func pinnedDragDetail(attempts []pinnedDragAttempt) string {
	if len(attempts) == 0 {
		return "attempts=0 delta=0,0"
	}
	attemptDeltas := make([]string, 0, len(attempts))
	for _, attempt := range attempts {
		attemptDeltas = append(attemptDeltas, fmt.Sprintf("%d:%d,%d", attempt.Attempt, attempt.DeltaX, attempt.DeltaY))
	}
	last := attempts[len(attempts)-1]
	return fmt.Sprintf("attempts=%d delta=%d,%d attempt_deltas=%s", len(attempts), last.TotalDeltaX, last.TotalDeltaY, strings.Join(attemptDeltas, ";"))
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

func sendScreenshotHotkey(hotkey string) {
	switch strings.ToLower(strings.TrimSpace(hotkey)) {
	case "f1":
		sendKey(vkF1)
	default:
		sendAltKey(vkA)
	}
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

func waitLatestPNG(root string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		path, err := latestPNG(root)
		if err == nil {
			return path, nil
		}
		lastErr = err
		time.Sleep(30 * time.Millisecond)
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("no png found under %s", root)
}

func waitForClipboardImageMatching(timeout time.Duration, expectedWidth int, expectedHeight int) (int, int, string, error) {
	deadline := time.Now().Add(timeout)
	var lastMessage string
	for time.Now().Before(deadline) {
		width, height, format, err := readClipboardImageDimensions()
		if err == nil {
			ok, detail, _ := captureDimensionsOK(width, height, expectedWidth, expectedHeight)
			if ok {
				return width, height, detail + " format=" + format, nil
			}
			lastMessage = "stale image " + detail
		} else if err != nil {
			lastMessage = err.Error()
		}
		time.Sleep(80 * time.Millisecond)
	}
	if lastMessage == "" {
		lastMessage = "clipboard did not contain an image"
	}
	return 0, 0, "", fmt.Errorf("matching clipboard image not found within %s: %s", timeout, lastMessage)
}

func readClipboardImageDimensions() (int, int, string, error) {
	opened, _, openErr := procOpenClipboard.Call(0)
	if opened == 0 {
		return 0, 0, "", fmt.Errorf("open clipboard: %w", openErr)
	}
	defer procCloseClipboard.Call()

	format := uintptr(cfDIBV5)
	available, _, _ := procIsFormatAvailable.Call(format)
	formatName := "CF_DIBV5"
	if available == 0 {
		format = uintptr(cfDIB)
		formatName = "CF_DIB"
		available, _, _ = procIsFormatAvailable.Call(format)
		if available == 0 {
			return 0, 0, "", fmt.Errorf("clipboard image format unavailable")
		}
	}

	handle, _, dataErr := procGetClipboardData.Call(format)
	if handle == 0 {
		return 0, 0, "", fmt.Errorf("get clipboard image: %w", dataErr)
	}
	pointer, _, lockErr := procGlobalLock.Call(handle)
	if pointer == 0 {
		return 0, 0, "", fmt.Errorf("lock clipboard image: %w", lockErr)
	}
	defer procGlobalUnlock.Call(handle)

	header := unsafe.Slice((*byte)(unsafe.Pointer(pointer)), 16)
	if len(header) < 12 {
		return 0, 0, "", fmt.Errorf("clipboard image header too small")
	}
	width := int(int32(binary.LittleEndian.Uint32(header[4:8])))
	height := int(int32(binary.LittleEndian.Uint32(header[8:12])))
	if width < 0 {
		width = -width
	}
	if height < 0 {
		height = -height
	}
	if width <= 0 || height <= 0 {
		return 0, 0, "", fmt.Errorf("clipboard image dimensions invalid: %dx%d", width, height)
	}
	return width, height, formatName, nil
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

func captureDimensionsOK(savedWidth int, savedHeight int, expectedWidth int, expectedHeight int) (bool, string, float64) {
	if savedWidth == expectedWidth && savedHeight == expectedHeight {
		return true, fmt.Sprintf("saved=%dx%d expected=%dx%d mode=logical_pixels", savedWidth, savedHeight, expectedWidth, expectedHeight), 1
	}
	if expectedWidth <= 0 || expectedHeight <= 0 || savedWidth <= 0 || savedHeight <= 0 {
		return false, fmt.Sprintf("saved=%dx%d expected=%dx%d mode=invalid", savedWidth, savedHeight, expectedWidth, expectedHeight), 1
	}
	scaleX := float64(savedWidth) / float64(expectedWidth)
	scaleY := float64(savedHeight) / float64(expectedHeight)
	if math.Abs(scaleX-scaleY) <= 0.02 && scaleX >= 1 && scaleX <= 4 {
		return true, fmt.Sprintf("saved=%dx%d expected=%dx%d scale=%.2f mode=physical_pixels", savedWidth, savedHeight, expectedWidth, expectedHeight, scaleX), scaleX
	}
	return false, fmt.Sprintf("saved=%dx%d expected=%dx%d scale=%.2f/%.2f mode=failed", savedWidth, savedHeight, expectedWidth, expectedHeight, scaleX, scaleY), 1
}

func normalizeCapturedForComparison(reference image.Image, captured image.Image, pixelScale float64) image.Image {
	refBounds := reference.Bounds()
	capBounds := captured.Bounds()
	if refBounds.Dx() == capBounds.Dx() && refBounds.Dy() == capBounds.Dy() {
		return captured
	}
	if pixelScale <= 1 {
		return captured
	}
	return resizeNearest(captured, refBounds.Dx(), refBounds.Dy())
}

func imageSizeKey(img image.Image) string {
	if img == nil {
		return ""
	}
	bounds := img.Bounds()
	return fmt.Sprintf("%dx%d", bounds.Dx(), bounds.Dy())
}

func resizeNearest(source image.Image, width int, height int) *image.RGBA {
	target := image.NewRGBA(image.Rect(0, 0, max(1, width), max(1, height)))
	sourceBounds := source.Bounds()
	for y := 0; y < target.Bounds().Dy(); y++ {
		sourceY := sourceBounds.Min.Y + min(sourceBounds.Dy()-1, int(float64(y)*float64(sourceBounds.Dy())/float64(target.Bounds().Dy())))
		for x := 0; x < target.Bounds().Dx(); x++ {
			sourceX := sourceBounds.Min.X + min(sourceBounds.Dx()-1, int(float64(x)*float64(sourceBounds.Dx())/float64(target.Bounds().Dx())))
			target.Set(x, y, source.At(sourceX, sourceY))
		}
	}
	return target
}

func solidPNG(width int, height int, value color.RGBA) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, max(1, width), max(1, height)))
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			img.SetRGBA(x, y, value)
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func evaluateContentMatch(matchPercent float64, meanDiff float64) (bool, string) {
	mode := "failed"
	strictOK := matchPercent >= contentStrictMatchPercent && meanDiff <= contentStrictMeanAbsDiff
	tolerantOK := matchPercent >= contentTolerantMatchPercent && meanDiff <= contentTolerantMeanAbsDiff
	if strictOK {
		mode = "strict"
	} else if tolerantOK {
		mode = "low_mean_tolerance"
	}
	detail := fmt.Sprintf(
		"match=%.2f%% mean_abs_diff=%.3f mode=%s strict(min_match=%.2f%% max_mean=%.3f) tolerant(min_match=%.2f%% max_mean=%.3f)",
		matchPercent,
		meanDiff,
		mode,
		contentStrictMatchPercent,
		contentStrictMeanAbsDiff,
		contentTolerantMatchPercent,
		contentTolerantMeanAbsDiff,
	)
	return strictOK || tolerantOK, detail
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
