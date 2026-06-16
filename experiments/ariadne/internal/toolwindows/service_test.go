package toolwindows

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"
)

func TestNormalizeViewAcceptsKnownToolViews(t *testing.T) {
	for _, view := range []string{"work-memory", "clipboard", "capture", "hosts", "workflow", "json-compare", "network-monitor", "network-mini", "settings"} {
		if normalizeView(" "+view+" ") != view {
			t.Fatalf("expected %q to be accepted", view)
		}
	}
}

func TestNormalizeViewRejectsLauncherAndUnknownViews(t *testing.T) {
	for _, view := range []string{"launcher", "pinned-image", "capture-overlay", "dashboard", ""} {
		if normalizeView(view) != "" {
			t.Fatalf("expected %q to be rejected", view)
		}
	}
}

func TestToolWindowSizingKeepsPaletteSeparateFromToolWindows(t *testing.T) {
	width, height := toolSize("work-memory")
	if width != 1120 || height != 720 {
		t.Fatalf("unexpected work memory size: %dx%d", width, height)
	}

	width, height = toolSize("json-compare")
	if width != 1180 || height != 760 {
		t.Fatalf("unexpected json compare size: %dx%d", width, height)
	}

	if minWidth("network-monitor") >= minWidth("work-memory") {
		t.Fatalf("network monitor should keep a smaller minimum width")
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

func TestOnlyNetworkMiniUsesFramelessFixedUtilityWindow(t *testing.T) {
	for _, view := range []string{"work-memory", "clipboard", "capture", "hosts", "workflow", "json-compare", "network-monitor", "settings"} {
		if frameless(view) {
			t.Fatalf("%s should use native window chrome", view)
		}
		if disableIcon(view) {
			t.Fatalf("%s should show the Ariadne titlebar icon", view)
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

func TestToolTitleUsesFlowBrandForWorkMemoryWindow(t *testing.T) {
	if got := toolTitle("work-memory"); got != "Ariadne - 心流" {
		t.Fatalf("unexpected work memory title: %q", got)
	}
}

func TestOrdinaryTaskbarStyleRestoresOverlappedWindowSemantics(t *testing.T) {
	style := uintptr(w32.WS_VISIBLE | w32.WS_POPUP)
	next := ordinaryWindowTaskbarStyle(style)
	if next&uintptr(w32.WS_POPUP) != 0 {
		t.Fatalf("ordinary taskbar style should clear WS_POPUP: %#x", next)
	}
	for _, flag := range []struct {
		name string
		bit  uint
	}{
		{name: "WS_VISIBLE", bit: w32.WS_VISIBLE},
		{name: "WS_CAPTION", bit: w32.WS_CAPTION},
		{name: "WS_SYSMENU", bit: w32.WS_SYSMENU},
		{name: "WS_THICKFRAME", bit: w32.WS_THICKFRAME},
		{name: "WS_MINIMIZEBOX", bit: w32.WS_MINIMIZEBOX},
		{name: "WS_MAXIMIZEBOX", bit: w32.WS_MAXIMIZEBOX},
	} {
		if next&uintptr(flag.bit) == 0 {
			t.Fatalf("ordinary taskbar style should include %s: %#x", flag.name, next)
		}
	}
}

func TestOrdinaryTaskbarExStyleUsesAppWindow(t *testing.T) {
	exStyle := uintptr(w32.WS_EX_TOOLWINDOW | w32.WS_EX_NOACTIVATE | w32.WS_EX_TOPMOST)
	next := ordinaryWindowTaskbarExStyle(exStyle)
	if next&uintptr(w32.WS_EX_APPWINDOW) == 0 {
		t.Fatalf("ordinary taskbar exstyle should include WS_EX_APPWINDOW: %#x", next)
	}
	for _, flag := range []struct {
		name string
		bit  uint
	}{
		{name: "WS_EX_TOOLWINDOW", bit: w32.WS_EX_TOOLWINDOW},
		{name: "WS_EX_NOACTIVATE", bit: w32.WS_EX_NOACTIVATE},
		{name: "WS_EX_TOPMOST", bit: w32.WS_EX_TOPMOST},
	} {
		if next&uintptr(flag.bit) != 0 {
			t.Fatalf("ordinary taskbar exstyle should clear %s: %#x", flag.name, next)
		}
	}
}

func TestTaskbarToggleOnlyAppliesToOrdinaryToolWindows(t *testing.T) {
	for _, view := range []string{"work-memory", "clipboard", "capture", "hosts", "workflow", "json-compare", "network-monitor", "settings"} {
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
