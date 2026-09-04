package shell

import (
	"fmt"
	"testing"
)

type fakeShortcutRegistry struct {
	callbacks map[string]func()
}

func newFakeShortcutRegistry() *fakeShortcutRegistry {
	return &fakeShortcutRegistry{callbacks: map[string]func(){}}
}

func (f *fakeShortcutRegistry) Register(accelerator string, callback func()) error {
	if _, exists := f.callbacks[accelerator]; exists {
		return fmt.Errorf("shortcut already registered")
	}
	f.callbacks[accelerator] = callback
	return nil
}

func (f *fakeShortcutRegistry) Unregister(accelerator string) error {
	if _, exists := f.callbacks[accelerator]; !exists {
		return fmt.Errorf("shortcut is not registered")
	}
	delete(f.callbacks, accelerator)
	return nil
}

func (f *fakeShortcutRegistry) IsRegistered(accelerator string) bool {
	_, exists := f.callbacks[accelerator]
	return exists
}

func TestParseHotkeyAltQ(t *testing.T) {
	spec, err := ParseHotkey("alt+q")
	if err != nil {
		t.Fatal(err)
	}
	if spec.KeyCode != 'Q' || spec.Modifiers&(modAlt|modNoRepeat) != (modAlt|modNoRepeat) {
		t.Fatalf("unexpected Alt+Q spec: %#v", spec)
	}
}

func TestParseHotkeyAltA(t *testing.T) {
	spec, err := ParseHotkey("alt+a")
	if err != nil {
		t.Fatal(err)
	}
	if spec.KeyCode != 'A' || spec.Modifiers&(modAlt|modNoRepeat) != (modAlt|modNoRepeat) {
		t.Fatalf("unexpected Alt+A spec: %#v", spec)
	}
}

func TestParseHotkeyRejectsMissingModifier(t *testing.T) {
	if _, err := ParseHotkey("q"); err == nil {
		t.Fatal("expected hotkey without modifier to fail")
	}
}

func TestParseHotkeySupportsFunctionKeys(t *testing.T) {
	spec, err := ParseHotkey("ctrl+shift+f12")
	if err != nil {
		t.Fatal(err)
	}
	if spec.KeyCode != 0x7B || spec.Modifiers&(modControl|modShift|modNoRepeat) != (modControl|modShift|modNoRepeat) {
		t.Fatalf("unexpected F12 spec: %#v", spec)
	}
}

func TestParseHotkeyAllowsBareFunctionKeys(t *testing.T) {
	spec, err := ParseHotkey("F1")
	if err != nil {
		t.Fatal(err)
	}
	if spec.KeyCode != 0x70 || spec.Modifiers != modNoRepeat || spec.KeyName != "F1" {
		t.Fatalf("unexpected bare F1 spec: %#v", spec)
	}
}

func TestManagedGlobalHotkeyUsesRegistryLifecycle(t *testing.T) {
	registry := newFakeShortcutRegistry()
	spec, err := ParseHotkey("ctrl shift f12")
	if err != nil {
		t.Fatal(err)
	}
	fired := false

	registration, err := RegisterManagedGlobalHotkey(registry, spec, func() { fired = true })
	if err != nil {
		t.Fatal(err)
	}
	callback, registered := registry.callbacks["Ctrl+Shift+F12"]
	if !registered {
		t.Fatalf("expected canonical Wails accelerator, got %#v", registry.callbacks)
	}
	callback()
	if !fired {
		t.Fatal("expected registered callback to run")
	}
	if err := registration.Stop(); err != nil {
		t.Fatal(err)
	}
	if registry.IsRegistered("Ctrl+Shift+F12") {
		t.Fatal("expected stopped registration to be removed")
	}
}
