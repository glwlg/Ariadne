package shell

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

const (
	modAlt      uint32 = 0x0001
	modControl  uint32 = 0x0002
	modShift    uint32 = 0x0004
	modWin      uint32 = 0x0008
	modNoRepeat uint32 = 0x4000
)

type HotkeySpec struct {
	Raw       string
	Modifiers uint32
	KeyCode   uint32
	KeyName   string
}

type shortcutRegistry interface {
	Register(accelerator string, callback func()) error
	Unregister(accelerator string) error
	IsRegistered(accelerator string) bool
}

type ManagedHotkeyRegistration struct {
	mu          sync.Mutex
	registry    shortcutRegistry
	accelerator string
	stopped     bool
}

func RegisterManagedGlobalHotkey(registry shortcutRegistry, spec HotkeySpec, callback func()) (*ManagedHotkeyRegistration, error) {
	if registry == nil {
		return nil, fmt.Errorf("global shortcut manager is unavailable")
	}
	if callback == nil {
		return nil, fmt.Errorf("global hotkey callback is nil")
	}
	accelerator := spec.WailsAccelerator()
	if err := registry.Register(accelerator, callback); err != nil {
		return nil, err
	}
	return &ManagedHotkeyRegistration{registry: registry, accelerator: accelerator}, nil
}

func (r *ManagedHotkeyRegistration) Stop() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return nil
	}
	if !r.registry.IsRegistered(r.accelerator) {
		r.stopped = true
		return nil
	}
	if err := r.registry.Unregister(r.accelerator); err != nil {
		return err
	}
	r.stopped = true
	return nil
}

func (r *ManagedHotkeyRegistration) IsRegistered() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return !r.stopped && r.registry.IsRegistered(r.accelerator)
}

func (s HotkeySpec) WailsAccelerator() string {
	parts := make([]string, 0, 5)
	if s.Modifiers&modControl != 0 {
		parts = append(parts, "Ctrl")
	}
	if s.Modifiers&modAlt != 0 {
		parts = append(parts, "Alt")
	}
	if s.Modifiers&modShift != 0 {
		parts = append(parts, "Shift")
	}
	if s.Modifiers&modWin != 0 {
		parts = append(parts, "Super")
	}
	parts = append(parts, s.KeyName)
	return strings.Join(parts, "+")
}

func ParseHotkey(value string) (HotkeySpec, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return HotkeySpec{}, fmt.Errorf("hotkey is empty")
	}

	tokens := strings.FieldsFunc(strings.ToLower(raw), func(r rune) bool {
		return r == '+' || r == ' '
	})
	modifiers := uint32(modNoRepeat)
	var keyName string
	var keyCode uint32

	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		switch token {
		case "alt", "option":
			modifiers |= modAlt
		case "ctrl", "control":
			modifiers |= modControl
		case "shift":
			modifiers |= modShift
		case "win", "windows", "meta", "cmd", "command":
			modifiers |= modWin
		default:
			if keyCode != 0 {
				return HotkeySpec{}, fmt.Errorf("hotkey has multiple non-modifier keys: %q", raw)
			}
			code, normalized, err := parseVirtualKey(token)
			if err != nil {
				return HotkeySpec{}, err
			}
			keyCode = code
			keyName = normalized
		}
	}

	if keyCode == 0 {
		return HotkeySpec{}, fmt.Errorf("hotkey missing key: %q", raw)
	}
	if modifiers == modNoRepeat && !canUseBareHotkey(keyName) {
		return HotkeySpec{}, fmt.Errorf("hotkey requires at least one modifier: %q", raw)
	}

	return HotkeySpec{
		Raw:       raw,
		Modifiers: modifiers,
		KeyCode:   keyCode,
		KeyName:   keyName,
	}, nil
}

func canUseBareHotkey(keyName string) bool {
	keyName = strings.ToUpper(strings.TrimSpace(keyName))
	if !strings.HasPrefix(keyName, "F") {
		return false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(keyName, "F"))
	return err == nil && n >= 1 && n <= 24
}

func parseVirtualKey(token string) (uint32, string, error) {
	if len(token) == 1 {
		ch := token[0]
		if ch >= 'a' && ch <= 'z' {
			return uint32(ch - 'a' + 'A'), strings.ToUpper(token), nil
		}
		if ch >= '0' && ch <= '9' {
			return uint32(ch), token, nil
		}
	}

	switch token {
	case "space":
		return 0x20, "Space", nil
	case "tab":
		return 0x09, "Tab", nil
	case "enter", "return":
		return 0x0D, "Enter", nil
	case "esc", "escape":
		return 0x1B, "Escape", nil
	case "backspace":
		return 0x08, "Backspace", nil
	case "delete", "del":
		return 0x2E, "Delete", nil
	}

	if strings.HasPrefix(token, "f") {
		n, err := strconv.Atoi(strings.TrimPrefix(token, "f"))
		if err == nil && n >= 1 && n <= 24 {
			return uint32(0x70 + n - 1), strings.ToUpper(token), nil
		}
	}

	return 0, "", fmt.Errorf("unsupported hotkey key: %q", token)
}
