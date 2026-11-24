package toolwindows

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"fancun/internal/networkmonitor"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestNormalizeViewAcceptsKnownToolViews(t *testing.T) {
	for _, view := range []string{"clipboard", "capture", "hosts", "workflow", "json-compare", "api-testing", "airplay", "network-monitor", "network-mini", "settings"} {
		if normalizeView(" "+view+" ") != view {
			t.Fatalf("expected %q to be accepted", view)
		}
	}
}

func TestNormalizeViewRejectsLauncherAndUnknownViews(t *testing.T) {
	for _, view := range []string{"launcher", "pinned-image", "capture-overlay", "process-network", "dashboard", ""} {
		if normalizeView(view) != "" {
			t.Fatalf("expected %q to be rejected", view)
		}
	}
}

func TestToolWindowSizingKeepsPaletteSeparateFromToolWindows(t *testing.T) {
	width, height := toolSize("capture")
	if width != 1120 || height != 720 {
		t.Fatalf("unexpected work memory size: %dx%d", width, height)
	}

	width, height = toolSize("json-compare")
	if width != 1180 || height != 760 {
		t.Fatalf("unexpected json compare size: %dx%d", width, height)
	}

	if minWidth("network-monitor") >= minWidth("capture") {
		t.Fatalf("network monitor should keep a smaller minimum width")
	}
	if width, height = toolSize("network-monitor"); width != 1240 || height != 760 {
		t.Fatalf("unexpected network monitor size: %dx%d", width, height)
	}

	width, height = toolSize("network-mini")
	if width != networkMiniWidth || height != networkMiniHeight {
		t.Fatalf("unexpected network mini size: %dx%d", width, height)
	}
	if !disableResize("network-mini") || !alwaysOnTop("network-mini") {
		t.Fatal("network mini should be locked and topmost")
	}
	if maxWidth("network-mini") != networkMiniWidth || maxHeight("network-mini") != networkMiniHeight {
		t.Fatalf("network mini should have fixed max size, got %dx%d", maxWidth("network-mini"), maxHeight("network-mini"))
	}
}

func TestSettingsAndHostsUseDesignSpecificWindowGeometry(t *testing.T) {
	settingsWidth, settingsHeight := toolSize("settings")
	if settingsWidth != 1240 || settingsHeight != 800 || minWidth("settings") != 980 || minHeight("settings") != 680 {
		t.Fatalf("unexpected settings geometry: %dx%d min %dx%d", settingsWidth, settingsHeight, minWidth("settings"), minHeight("settings"))
	}
	hostsWidth, hostsHeight := toolSize("hosts")
	if hostsWidth != 1360 || hostsHeight != 860 || minWidth("hosts") != 1120 || minHeight("hosts") != 720 {
		t.Fatalf("unexpected hosts geometry: %dx%d min %dx%d", hostsWidth, hostsHeight, minWidth("hosts"), minHeight("hosts"))
	}
}

func TestOnlyNetworkMiniUsesFramelessFixedUtilityWindow(t *testing.T) {
	for _, view := range []string{"clipboard", "capture", "hosts", "workflow", "json-compare", "api-testing", "airplay", "network-monitor", "settings"} {
		if frameless(view) {
			t.Fatalf("%s should use native window chrome", view)
		}
		if disableIcon(view) {
			t.Fatalf("%s should show the Fancun titlebar icon", view)
		}
		if hiddenOnTaskbar(view) {
			t.Fatalf("%s should have its own taskbar button", view)
		}
		if disableResize(view) {
			t.Fatalf("%s should remain resizable", view)
		}
	}
	if !frameless("network-mini") {
		t.Fatal("network mini should stay frameless")
	}
	if !disableIcon("network-mini") || !hiddenOnTaskbar("network-mini") {
		t.Fatal("network mini should hide titlebar icon and taskbar button")
	}
}

func TestAirPlayUsesIndependentOrdinaryToolWindow(t *testing.T) {
	if toolTitle("airplay") != "Fancun - iPhone 投屏" {
		t.Fatal("airplay should use the Fancun tool title")
	}
	width, height := toolSize("airplay")
	if width != 1080 || height != 760 || minWidth("airplay") != 820 || minHeight("airplay") != 560 {
		t.Fatalf("unexpected airplay window geometry: %dx%d", width, height)
	}
	if !ordinaryTaskbarToggleAllowed("airplay") || frameless("airplay") || hiddenOnTaskbar("airplay") {
		t.Fatal("airplay should use normal native window and taskbar controls")
	}
}

func TestOrdinaryWindowPolicyRetriesLateTaskbarRegistration(t *testing.T) {
	delays := ordinaryWindowPolicyRetryDelays()
	if len(delays) < 3 {
		t.Fatalf("expected multiple taskbar policy retries for late HWND shell registration, got %d", len(delays))
	}
	if delays[0] > 100*time.Millisecond {
		t.Fatalf("first retry should happen quickly after mount/show, got %s", delays[0])
	}
	if delays[len(delays)-1] < 800*time.Millisecond {
		t.Fatalf("last retry should cover delayed WebView/taskbar registration, got %s", delays[len(delays)-1])
	}
}

func TestTaskbarToggleOnlyAppliesToOrdinaryToolWindows(t *testing.T) {
	for _, view := range []string{"clipboard", "capture", "hosts", "workflow", "json-compare", "api-testing", "airplay", "network-monitor", "settings"} {
		if !ordinaryTaskbarToggleAllowed(view) {
			t.Fatalf("%s should keep ordinary taskbar minimise behaviour", view)
		}
	}
	for _, view := range []string{"launcher", "network-mini", "capture-overlay", "pinned-image", ""} {
		if ordinaryTaskbarToggleAllowed(view) {
			t.Fatalf("%s should not receive ordinary taskbar minimise styles", view)
		}
	}
}

func TestNetworkMiniPlacementDefaultsToTaskbarLeft(t *testing.T) {
	screen := &application.Screen{
		Bounds:   application.Rect{Width: 1920, Height: 1080},
		WorkArea: application.Rect{Width: 1920, Height: 1040},
	}

	position, x, y, target := toolPlacement("network-mini", networkMiniWidth, networkMiniHeight, screen, "taskbar-left")

	if position != application.WindowXY || target != screen {
		t.Fatalf("expected network mini to use screen-relative XY placement, got position=%v target=%#v", position, target)
	}
	if x != networkMiniMargin || y != 1043 {
		t.Fatalf("unexpected taskbar-left placement: %d,%d", x, y)
	}
}

func TestNetworkMiniTaskbarPlacementUsesWorkAreaRelativeCoordinates(t *testing.T) {
	tests := []struct {
		name     string
		screen   *application.Screen
		expected networkMiniWindowFrame
		absolute [2]int
	}{
		{
			name: "top taskbar",
			screen: &application.Screen{
				Bounds:   application.Rect{X: 0, Y: 0, Width: 1920, Height: 1080},
				WorkArea: application.Rect{X: 0, Y: 40, Width: 1920, Height: 1040},
			},
			expected: networkMiniWindowFrame{X: networkMiniMargin, Y: -37, Width: networkMiniWidth, Height: 33},
			absolute: [2]int{networkMiniMargin, 3},
		},
		{
			name: "left taskbar",
			screen: &application.Screen{
				Bounds:   application.Rect{X: 0, Y: 0, Width: 1920, Height: 1080},
				WorkArea: application.Rect{X: 40, Y: 0, Width: 1880, Height: 1080},
			},
			expected: networkMiniWindowFrame{X: -40, Y: networkMiniMargin, Width: networkMiniWidth, Height: 33},
			absolute: [2]int{0, networkMiniMargin},
		},
		{
			name: "secondary screen bottom taskbar",
			screen: &application.Screen{
				Bounds:   application.Rect{X: 1920, Y: 0, Width: 1280, Height: 720},
				WorkArea: application.Rect{X: 1920, Y: 0, Width: 1280, Height: 680},
			},
			expected: networkMiniWindowFrame{X: networkMiniMargin, Y: 683, Width: networkMiniWidth, Height: 33},
			absolute: [2]int{1926, 683},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := networkMiniTaskbarFrame(tt.screen, networkMiniWidth, networkMiniHeight)
			if frame != tt.expected {
				t.Fatalf("unexpected work-area-relative frame: %#v", frame)
			}
			x, y := networkMiniAbsolutePosition(tt.screen, frame)
			if x != tt.absolute[0] || y != tt.absolute[1] {
				t.Fatalf("unexpected absolute position: %d,%d", x, y)
			}
		})
	}
}

func TestNetworkMiniPlacementFallsBackWhenScreenUnavailable(t *testing.T) {
	position, x, y, target := toolPlacement("network-mini", 318, 168, nil, "bottom-right")

	if position != application.WindowCentered || x != 0 || y != 0 || target != nil {
		t.Fatalf("expected centered fallback, got position=%v x=%d y=%d target=%#v", position, x, y, target)
	}
}

func TestNetworkMiniPlacementSupportsAllAnchors(t *testing.T) {
	tests := map[string][2]int{
		"top-left":     {networkMiniMargin, networkMiniMargin},
		"top-right":    {1758, networkMiniMargin},
		"bottom-left":  {networkMiniMargin, 994},
		"bottom-right": {1758, 994},
	}

	for anchor, expected := range tests {
		x, y := networkMiniAnchorPosition(anchor, 1920, 1040, networkMiniWidth, networkMiniHeight)
		if x != expected[0] || y != expected[1] {
			t.Fatalf("unexpected placement for %s: %d,%d", anchor, x, y)
		}
	}
}

func TestNetworkMiniStatusDefaultsToLockedTaskbarLeftAutoHide(t *testing.T) {
	service := NewServiceWithOptions(filepath.Join(t.TempDir(), "network-mini.json"), nil)

	status := service.NetworkMiniStatus()

	if status.Anchor != "taskbar-left" || status.ScreenMode != "cursor" || !status.AutoHideFullscreen || !status.Locked {
		t.Fatalf("unexpected default status: %#v", status)
	}
	if status.LastError != "" {
		t.Fatalf("unexpected default status error: %s", status.LastError)
	}
}

func TestNetworkMiniAnchorPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "network-mini.json")
	service := NewServiceWithOptions(path, nil)

	status := service.SetNetworkMiniAnchor("top-left")
	if status.Anchor != "top-left" || status.LastError != "" {
		t.Fatalf("unexpected status after anchor save: %#v", status)
	}

	reloaded := NewServiceWithOptions(path, nil)
	if got := reloaded.NetworkMiniStatus().Anchor; got != "top-left" {
		t.Fatalf("expected persisted top-left anchor, got %q", got)
	}
}

func TestNetworkMiniRejectsInvalidAnchorWithoutPersisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "network-mini.json")
	service := NewServiceWithOptions(path, nil)

	status := service.SetNetworkMiniAnchor("middle")

	if status.Anchor != "taskbar-left" || status.LastError == "" {
		t.Fatalf("expected invalid anchor to preserve default and report error, got %#v", status)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("invalid anchor should not write config, stat err=%v", err)
	}
}

func TestNetworkMiniAutoHideSettingPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "network-mini.json")
	service := NewServiceWithOptions(path, nil)

	status := service.SetNetworkMiniAutoHideFullscreen(false)
	if status.AutoHideFullscreen {
		t.Fatalf("expected auto-hide to be disabled: %#v", status)
	}

	reloaded := NewServiceWithOptions(path, nil)
	if reloaded.NetworkMiniStatus().AutoHideFullscreen {
		t.Fatal("expected disabled auto-hide setting to persist")
	}
}

func TestNetworkMiniVisibleStatePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "network-mini.json")
	service := NewServiceWithOptions(path, nil)

	service.markNetworkMiniVisible()

	reloaded := NewServiceWithOptions(path, nil)
	if !reloaded.NetworkMiniStatus().Visible {
		t.Fatal("expected network mini visible state to persist after opening")
	}
}

func TestNetworkMiniLayerRefreshIsThrottled(t *testing.T) {
	service := NewServiceWithOptions(filepath.Join(t.TempDir(), "network-mini.json"), nil)

	if !service.shouldRefreshNetworkMiniLayer() {
		t.Fatal("first layer refresh should be allowed")
	}
	if service.shouldRefreshNetworkMiniLayer() {
		t.Fatal("immediate second layer refresh should be throttled")
	}
	service.mu.Lock()
	service.networkMiniLayerAt = time.Now().Add(-networkMiniLayerRefresh - time.Second)
	service.mu.Unlock()
	if !service.shouldRefreshNetworkMiniLayer() {
		t.Fatal("stale layer refresh should be allowed")
	}
}

func TestCaptureFocusSuspensionPausesNetworkMiniTopmostRefresh(t *testing.T) {
	service := NewServiceWithOptions(filepath.Join(t.TempDir(), "network-mini.json"), nil)
	if !service.shouldRefreshNetworkMiniLayerNow(true) {
		t.Fatal("taskbar foreground should refresh before capture starts")
	}

	service.SetCaptureFocusSuspended(true)
	if service.shouldRefreshNetworkMiniLayerNow(true) {
		t.Fatal("capture should pause taskbar-triggered topmost refresh")
	}
	if service.shouldRefreshNetworkMiniLayerNow(false) {
		t.Fatal("capture should pause periodic topmost refresh")
	}
	if service.networkMiniCaptureAction(true, false, true) != networkMiniCaptureHold {
		t.Fatal("fullscreen auto-hide must not restore the network mini during capture")
	}
	if service.networkMiniCaptureAction(false, true, true) != networkMiniCaptureHold {
		t.Fatal("disabled auto-hide must not show the network mini during capture")
	}

	service.SetCaptureFocusSuspended(false)
	if !service.shouldRefreshNetworkMiniLayerNow(true) {
		t.Fatal("ending capture should restore taskbar-triggered topmost refresh")
	}
	if service.networkMiniCaptureAction(false, true, true) != networkMiniCaptureShow {
		t.Fatal("a mini hidden by capture should be restored afterwards")
	}
}

func TestNetworkMiniTaskbarForegroundBypassesLayerRefreshThrottle(t *testing.T) {
	service := NewServiceWithOptions(filepath.Join(t.TempDir(), "network-mini.json"), nil)

	if !service.shouldRefreshNetworkMiniLayerNow(true) {
		t.Fatal("taskbar foreground should refresh the network mini layer immediately")
	}
	if !service.shouldRefreshNetworkMiniLayerNow(true) {
		t.Fatal("taskbar foreground should bypass normal layer refresh throttling")
	}
	if service.shouldRefreshNetworkMiniLayerNow(false) {
		t.Fatal("non-taskbar refresh should still use normal throttling")
	}
}

func TestNetworkMiniTaskbarClassNames(t *testing.T) {
	for _, className := range []string{"Shell_TrayWnd", " Shell_SecondaryTrayWnd "} {
		if !isNetworkMiniTaskbarClassName(className) {
			t.Fatalf("expected %q to be treated as a taskbar window class", className)
		}
	}
	for _, className := range []string{"", "WorkerW", "Progman", "Chrome_WidgetWin_1"} {
		if isNetworkMiniTaskbarClassName(className) {
			t.Fatalf("expected %q to be ignored", className)
		}
	}
}

func TestNetworkMiniLayoutSignatureTracksResolutionAndWorkAreaChanges(t *testing.T) {
	original := &application.Screen{
		ID:               "primary",
		IsPrimary:        true,
		Bounds:           application.Rect{X: 0, Y: 0, Width: 1920, Height: 1080},
		WorkArea:         application.Rect{X: 0, Y: 0, Width: 1920, Height: 1040},
		PhysicalBounds:   application.Rect{X: 0, Y: 0, Width: 1920, Height: 1080},
		PhysicalWorkArea: application.Rect{X: 0, Y: 0, Width: 1920, Height: 1040},
	}
	same := &application.Screen{
		ID:               "primary",
		IsPrimary:        true,
		Bounds:           application.Rect{X: 0, Y: 0, Width: 1920, Height: 1080},
		WorkArea:         application.Rect{X: 0, Y: 0, Width: 1920, Height: 1040},
		PhysicalBounds:   application.Rect{X: 0, Y: 0, Width: 1920, Height: 1080},
		PhysicalWorkArea: application.Rect{X: 0, Y: 0, Width: 1920, Height: 1040},
	}
	resized := &application.Screen{
		ID:               "primary",
		IsPrimary:        true,
		Bounds:           application.Rect{X: 0, Y: 0, Width: 1536, Height: 864},
		WorkArea:         application.Rect{X: 0, Y: 0, Width: 1536, Height: 824},
		PhysicalBounds:   application.Rect{X: 0, Y: 0, Width: 1536, Height: 864},
		PhysicalWorkArea: application.Rect{X: 0, Y: 0, Width: 1536, Height: 824},
	}
	workAreaMoved := &application.Screen{
		ID:               "primary",
		IsPrimary:        true,
		Bounds:           application.Rect{X: 0, Y: 0, Width: 1920, Height: 1080},
		WorkArea:         application.Rect{X: 0, Y: 40, Width: 1920, Height: 1040},
		PhysicalBounds:   application.Rect{X: 0, Y: 0, Width: 1920, Height: 1080},
		PhysicalWorkArea: application.Rect{X: 0, Y: 40, Width: 1920, Height: 1040},
	}

	layout := networkMiniScreenLayoutSignature([]*application.Screen{original}, original)
	if layout == "" {
		t.Fatal("expected populated screen layout signature")
	}
	if layout != networkMiniScreenLayoutSignature([]*application.Screen{same}, same) {
		t.Fatal("same screen geometry should keep the same layout signature")
	}
	if layout == networkMiniScreenLayoutSignature([]*application.Screen{resized}, resized) {
		t.Fatal("resolution changes should change the layout signature")
	}
	if layout == networkMiniScreenLayoutSignature([]*application.Screen{workAreaMoved}, workAreaMoved) {
		t.Fatal("taskbar/work-area changes should change the layout signature")
	}
}

func TestNetworkMiniPlacementRefreshDetectsKnownLayoutChanges(t *testing.T) {
	service := NewServiceWithOptions(filepath.Join(t.TempDir(), "network-mini.json"), nil)
	original := "primary=primary|screen=primary:true:0,0,1920,1080:0,0,1920,1040:0,0,1920,1080:0,0,1920,1040"
	resized := "primary=primary|screen=primary:true:0,0,1536,864:0,0,1536,824:0,0,1536,864:0,0,1536,824"

	if service.shouldReapplyNetworkMiniPlacementForLayout(original) {
		t.Fatal("unknown previous layout should not force an immediate reapply")
	}
	service.mu.Lock()
	service.networkMiniLayout = original
	service.mu.Unlock()
	if service.shouldReapplyNetworkMiniPlacementForLayout(original) {
		t.Fatal("same layout should not reapply placement")
	}
	if !service.shouldReapplyNetworkMiniPlacementForLayout(resized) {
		t.Fatal("changed layout should reapply placement")
	}
}

func TestNetworkMiniResetPlacementPreservesVisibleState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "network-mini.json")
	service := NewServiceWithOptions(path, nil)
	service.markNetworkMiniVisible()
	status := service.SetNetworkMiniAnchor("top-left")
	if status.Anchor != "top-left" || !status.Visible {
		t.Fatalf("expected visible custom placement before reset, got %#v", status)
	}

	status = service.ResetNetworkMiniPlacement()
	if status.Anchor != networkMiniDefaultAnchor || !status.Visible {
		t.Fatalf("reset should preserve visible state while restoring placement, got %#v", status)
	}

	reloaded := NewServiceWithOptions(path, nil)
	if !reloaded.NetworkMiniStatus().Visible {
		t.Fatal("expected reset visible state to persist")
	}
}

func TestNetworkMiniLegacyConfigWithoutAutoHideKeepsDefaultEnabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "network-mini.json")
	if err := os.WriteFile(path, []byte(`{"anchor":"top-right"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	service := NewServiceWithOptions(path, nil)
	status := service.NetworkMiniStatus()

	if status.Anchor != "top-right" || !status.AutoHideFullscreen {
		t.Fatalf("expected legacy config to keep explicit anchor and default auto-hide, got %#v", status)
	}
}

func TestNetworkMiniLegacyConfigWithScreenIDUsesSpecificScreenMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "network-mini.json")
	if err := os.WriteFile(path, []byte(`{"anchor":"top-right","screenId":"display-2"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	service := NewServiceWithOptions(path, nil)
	status := service.NetworkMiniStatus()

	if status.ScreenMode != "screen" || status.ScreenID != "display-2" {
		t.Fatalf("legacy screenId should be preserved as specific-screen mode, got %#v", status)
	}
}

func TestNetworkMiniScreenModePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "network-mini.json")
	service := NewServiceWithOptions(path, nil)

	status := service.SetNetworkMiniScreenMode("screen", "display-2")
	if status.ScreenMode != "screen" || status.ScreenID != "display-2" || status.LastError != "" {
		t.Fatalf("unexpected status after screen mode save: %#v", status)
	}

	reloaded := NewServiceWithOptions(path, nil)
	status = reloaded.NetworkMiniStatus()
	if status.ScreenMode != "screen" || status.ScreenID != "display-2" {
		t.Fatalf("expected persisted screen mode, got %#v", status)
	}

	status = reloaded.SetNetworkMiniScreenMode("primary", "display-2")
	if status.ScreenMode != "primary" || status.ScreenID != "" || status.LastError != "" {
		t.Fatalf("primary mode should clear specific screen ID, got %#v", status)
	}
}

func TestNetworkMiniRejectsInvalidScreenModeWithoutPersisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "network-mini.json")
	service := NewServiceWithOptions(path, nil)

	status := service.SetNetworkMiniScreenMode("teleport", "")

	if status.ScreenMode != "cursor" || status.LastError == "" {
		t.Fatalf("expected invalid mode to preserve default and report error, got %#v", status)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("invalid screen mode should not write config, stat err=%v", err)
	}
}

func TestNetworkMiniRejectsSpecificScreenModeWithoutID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "network-mini.json")
	service := NewServiceWithOptions(path, nil)

	status := service.SetNetworkMiniScreenMode("screen", "")

	if status.ScreenMode != "cursor" || status.LastError == "" {
		t.Fatalf("expected missing screen ID to preserve default and report error, got %#v", status)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("missing screen ID should not write config, stat err=%v", err)
	}
}

func TestNetworkMiniSelectsCursorScreenAcrossMonitors(t *testing.T) {
	primary := &application.Screen{
		ID:             "primary",
		Name:           "Primary",
		IsPrimary:      true,
		PhysicalBounds: application.Rect{X: 0, Y: 0, Width: 1920, Height: 1080},
	}
	secondary := &application.Screen{
		ID:             "secondary",
		Name:           "Secondary",
		PhysicalBounds: application.Rect{X: 1920, Y: 0, Width: 1280, Height: 1024},
	}

	selected := selectNetworkMiniScreen(
		networkMiniConfig{ScreenMode: "cursor"},
		[]*application.Screen{primary, secondary},
		primary,
		application.Point{X: 2200, Y: 240},
		true,
	)
	if selected != secondary {
		t.Fatalf("cursor mode should follow the monitor under the pointer, got %#v", selected)
	}

	selected = selectNetworkMiniScreen(
		networkMiniConfig{ScreenMode: "cursor"},
		[]*application.Screen{primary, secondary},
		primary,
		application.Point{},
		false,
	)
	if selected != primary {
		t.Fatalf("cursor mode should fall back to primary when cursor is unavailable, got %#v", selected)
	}
}

func TestNetworkMiniSpecificScreenFallsBackWhenMissing(t *testing.T) {
	primary := &application.Screen{
		ID:             "primary",
		Name:           "Primary",
		IsPrimary:      true,
		PhysicalBounds: application.Rect{X: 0, Y: 0, Width: 1920, Height: 1080},
	}
	secondary := &application.Screen{
		ID:             "secondary",
		Name:           "Secondary",
		PhysicalBounds: application.Rect{X: 1920, Y: 0, Width: 1280, Height: 1024},
	}

	selected := selectNetworkMiniScreen(
		networkMiniConfig{ScreenMode: "screen", ScreenID: "removed-monitor"},
		[]*application.Screen{primary, secondary},
		primary,
		application.Point{X: 2300, Y: 200},
		true,
	)
	if selected != primary {
		t.Fatalf("missing specific screen should fall back to primary, got %#v", selected)
	}
}

func TestLauncherToggleHideDecision(t *testing.T) {
	if !shouldToggleHideLauncher(true, true, false) {
		t.Fatal("visible focused launcher should toggle-hide")
	}
	if !shouldToggleHideLauncher(true, false, true) {
		t.Fatal("visible foreground launcher should toggle-hide even if WebView child holds focus")
	}
	if shouldToggleHideLauncher(true, false, false) {
		t.Fatal("visible unfocused background launcher should focus instead of hiding")
	}
	if shouldToggleHideLauncher(false, true, true) {
		t.Fatal("hidden launcher should show instead of hiding")
	}
}

func TestLauncherPreviousForegroundCaptureAndRestoreLifecycle(t *testing.T) {
	const (
		editorHWND   = uintptr(0x1001)
		launcherHWND = uintptr(0x2002)
	)
	now := time.Unix(1700000000, 0)
	currentFG := editorHWND
	var restored []uintptr

	service := NewServiceWithOptions("", nil)
	service.launcherNow = func() time.Time { return now }
	service.captureForegroundFn = func() uintptr { return currentFG }
	service.restoreForegroundFn = func(hwnd uintptr) { restored = append(restored, hwnd) }

	// Capturing when editor is foreground saves editorHWND.
	service.captureAndNoteLauncherShown(nil)
	if service.launcherPrevHWND != editorHWND {
		t.Fatalf("expected previous hwnd %#x, got %#x", editorHWND, service.launcherPrevHWND)
	}

	// Esc / toggle hide restores previous HWND once.
	service.hideLauncherWindow(nil, true)
	if len(restored) != 1 || restored[0] != editorHWND || service.launcherPrevHWND != 0 {
		t.Fatalf("expected restore of %#x once, got %#v", editorHWND, restored)
	}

	// Blur-hide clears previous HWND without stealing focus from newly clicked app.
	currentFG = editorHWND
	service.captureAndNoteLauncherShown(nil)
	service.hideLauncherWindow(nil, false)
	service.handleLauncherHidden()
	if len(restored) != 1 || service.launcherPrevHWND != 0 {
		t.Fatalf("blur-hide must not restore previous foreground window, restored=%#v", restored)
	}

	// Direct WindowHide (e.g. frontend Esc) restores saved HWND.
	currentFG = editorHWND
	service.captureAndNoteLauncherShown(nil)
	service.handleLauncherHidden()
	if len(restored) != 2 || restored[1] != editorHWND {
		t.Fatalf("WindowHide after Esc should restore %#x, got %#v", editorHWND, restored)
	}

	// Opening another tool window clears saved previous HWND so hiding launcher doesn't steal focus from the tool window.
	currentFG = editorHWND
	service.captureAndNoteLauncherShown(nil)
	service.clearLauncherPrevHWND()
	service.handleLauncherHidden()
	if len(restored) != 2 {
		t.Fatalf("clearing previous HWND should suppress restore, got %#v", restored)
	}

	// Stop suppresses further restores.
	currentFG = launcherHWND
	service.captureAndNoteLauncherShown(nil)
	service.Stop()
	service.handleLauncherHidden()
	if len(restored) != 2 {
		t.Fatalf("Stop should clear and suppress foreground restore, got %#v", restored)
	}
}

func TestLauncherLostFocusRespectsShowAndResizeGraceGuard(t *testing.T) {
	base := time.Unix(1700000000, 0)
	service := NewServiceWithOptions("", nil)
	service.launcherNow = func() time.Time { return base }

	service.captureAndNoteLauncherShown(nil)

	// Within show grace window -> must not hide immediately, returns remaining grace.
	hide, retry := service.shouldHideLauncherOnBlurLocked(base.Add(50*time.Millisecond), true, false, false)
	if hide || retry <= 0 {
		t.Fatalf("expected show grace deferral, got hide=%t retry=%s", hide, retry)
	}

	// Past show grace, but a resize just occurred -> must defer for resize grace.
	service.launcherNow = func() time.Time { return base.Add(250 * time.Millisecond) }
	service.noteLauncherResize()
	hide, retry = service.shouldHideLauncherOnBlurLocked(base.Add(300*time.Millisecond), true, false, false)
	if hide || retry <= 0 {
		t.Fatalf("expected resize grace deferral, got hide=%t retry=%s", hide, retry)
	}

	// If focus settled back onto launcher or child after grace -> do not hide.
	hide, retry = service.shouldHideLauncherOnBlurLocked(base.Add(600*time.Millisecond), true, false, true)
	if hide || retry != 0 {
		t.Fatalf("foreground child should prevent blur hide, got hide=%t retry=%s", hide, retry)
	}

	// After both grace periods expire and focus is elsewhere -> hide.
	hide, retry = service.shouldHideLauncherOnBlurLocked(base.Add(600*time.Millisecond), true, false, false)
	if !hide || retry != 0 {
		t.Fatalf("expected blur hide after grace expiry, got hide=%t retry=%s", hide, retry)
	}
}

func TestTickNetworkMiniAutoHideSkipsDetectorWhenHiddenOrAutoHideDisabled(t *testing.T) {
	detectorCalls := 0
	service := NewServiceWithOptions("", func() (bool, error) {
		detectorCalls++
		return true, nil
	})
	renderer := &recordingNativeMini{}
	ConfigureNativeNetworkMini(service, renderer, func() networkmonitor.TrafficSnapshot {
		return networkmonitor.TrafficSnapshot{}
	})
	service.mu.Lock()
	service.app = &application.App{}
	service.networkMiniConfig.Visible = false
	service.networkMiniConfig.AutoHideFullscreen = true
	service.mu.Unlock()

	// Hidden native mini must skip fullscreen detector and pipe updates.
	service.tickNetworkMiniAutoHide()
	if detectorCalls != 0 || len(renderer.states) != 0 {
		t.Fatalf("hidden mini should skip detector and pipe calls: detector=%d states=%d", detectorCalls, len(renderer.states))
	}

	// Visible but AutoHideFullscreen=false must skip fullscreen detector.
	service.mu.Lock()
	service.networkMiniConfig.Visible = true
	service.networkMiniConfig.AutoHideFullscreen = false
	service.mu.Unlock()
	service.tickNetworkMiniAutoHide()
	if detectorCalls != 0 {
		t.Fatalf("autoHide=false should skip fullscreen detector, got %d calls", detectorCalls)
	}

	// Visible with AutoHideFullscreen=true runs detector once.
	service.mu.Lock()
	service.networkMiniConfig.AutoHideFullscreen = true
	service.mu.Unlock()
	service.tickNetworkMiniAutoHide()
	if detectorCalls != 1 || !service.NetworkMiniStatus().FullscreenActive {
		t.Fatalf("expected fullscreen detector to run once and detect fullscreen, calls=%d status=%#v", detectorCalls, service.NetworkMiniStatus())
	}
}
