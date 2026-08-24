//go:build windows

package toolwindows

import "testing"

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
