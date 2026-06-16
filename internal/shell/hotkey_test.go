package shell

import "testing"

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
