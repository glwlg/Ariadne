package shell

import (
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestSecondInstanceLaunchDoesNotOpenWindow(t *testing.T) {
	opened := []string{}
	manager := NewManager("", "", "", func(view string) bool {
		opened = append(opened, view)
		return true
	}, nil, nil)

	options := manager.SingleInstanceOptions()
	if options.OnSecondInstanceLaunch == nil {
		t.Fatal("expected second instance callback")
	}

	options.OnSecondInstanceLaunch(application.SecondInstanceData{Args: []string{"ariadne.exe"}})
	if len(opened) > 0 {
		t.Fatalf("second instance launch should stay in background, opened %v", opened)
	}
}

func TestOpenWorkMemoryUsesToolOpenerWithoutStartupWindow(t *testing.T) {
	opened := []string{}
	manager := NewManager("", "", "", func(view string) bool {
		opened = append(opened, view)
		return true
	}, nil, nil)

	manager.OpenWorkMemory()

	if len(opened) != 1 || opened[0] != "work-memory" {
		t.Fatalf("work memory should open through tool opener, got %v", opened)
	}
}

func TestManagerUsesWailsRegistryForDynamicGlobalShortcuts(t *testing.T) {
	registry := newFakeShortcutRegistry()
	opened := []string{}
	screenshots := 0
	pins := 0
	manager := NewManager("alt+q", "alt+a", "alt+v", func(view string) bool {
		opened = append(opened, view)
		return true
	}, func() bool {
		screenshots++
		return true
	}, func() bool {
		pins++
		return true
	})

	manager.attachShortcutRegistry(registry)
	for _, accelerator := range []string{"Alt+Q", "Alt+A", "Alt+V"} {
		if !registry.IsRegistered(accelerator) {
			t.Fatalf("expected %s to be registered, got %#v", accelerator, registry.callbacks)
		}
	}
	registry.callbacks["Alt+Q"]()
	registry.callbacks["Alt+A"]()
	registry.callbacks["Alt+V"]()
	if len(opened) != 1 || opened[0] != "launcher" || screenshots != 1 || pins != 1 {
		t.Fatalf("unexpected shortcut callbacks: opened=%v screenshots=%d pins=%d", opened, screenshots, pins)
	}

	status := manager.ApplyHotkeys("ctrl+q", "ctrl+a", "ctrl+v")
	if !status.GlobalHotkeyRegistered || !status.ScreenshotHotkeyRegistered || !status.PinClipboardHotkeyRegistered {
		t.Fatalf("expected replacement shortcuts to register, got %#v", status)
	}
	for _, accelerator := range []string{"Alt+Q", "Alt+A", "Alt+V"} {
		if registry.IsRegistered(accelerator) {
			t.Fatalf("expected old shortcut %s to be removed", accelerator)
		}
	}
	for _, accelerator := range []string{"Ctrl+Q", "Ctrl+A", "Ctrl+V"} {
		if !registry.IsRegistered(accelerator) {
			t.Fatalf("expected replacement shortcut %s, got %#v", accelerator, registry.callbacks)
		}
	}

	if err := manager.Stop(); err != nil {
		t.Fatal(err)
	}
	if len(registry.callbacks) != 0 {
		t.Fatalf("expected shutdown to unregister shortcuts, got %#v", registry.callbacks)
	}
}

func TestManagerRetriesShortcutRejectedDuringWailsStartup(t *testing.T) {
	registry := newFakeShortcutRegistry()
	manager := NewManager("alt+q", "alt+a", "alt+v", nil, nil, nil)
	manager.attachShortcutRegistry(registry)

	delete(registry.callbacks, "Alt+Q")
	if status := manager.Status(); status.GlobalHotkeyRegistered {
		t.Fatalf("expected registry state to expose rejected startup shortcut, got %#v", status)
	}

	status := manager.RetryHotkeyRegistration()
	if !status.GlobalHotkeyRegistered || !registry.IsRegistered("Alt+Q") {
		t.Fatalf("expected retry to register rejected shortcut, got status=%#v registry=%#v", status, registry.callbacks)
	}
}
