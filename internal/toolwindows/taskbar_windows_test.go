//go:build windows

package toolwindows

import (
	"testing"

	"github.com/wailsapp/wails/v3/pkg/w32"
)

func TestNetworkMiniForegroundHookRefreshesForAnyForegroundWindow(t *testing.T) {
	const hook = uintptr(713)
	called := false
	networkMiniTaskbarForegroundHooks.Store(hook, func() {
		called = true
	})
	defer networkMiniTaskbarForegroundHooks.Delete(hook)

	networkMiniTaskbarForegroundWinEventProc(hook, networkMiniEventSystemForeground, 1, 0, 0, 0, 0)

	if !called {
		t.Fatal("foreground changes should immediately re-raise the Network Mini")
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

func TestOrdinaryTaskbarSetWindowPosFlagsForceShellRefresh(t *testing.T) {
	flags := ordinaryWindowTaskbarSetWindowPosFlags(true)
	if flags&w32.SWP_SHOWWINDOW == 0 {
		t.Fatalf("ordinary taskbar policy should force shell visibility refresh: %#x", flags)
	}
	if flags&w32.SWP_FRAMECHANGED == 0 {
		t.Fatalf("ordinary taskbar policy should report frame changes: %#x", flags)
	}
	if flags&w32.SWP_NOACTIVATE != 0 {
		t.Fatalf("ordinary taskbar policy should not make normal tool windows no-activate: %#x", flags)
	}
}
