//go:build !windows

package shell

import "fmt"

type HotkeyRegistration struct{}

func RegisterGlobalHotkey(spec HotkeySpec, callback func()) (*HotkeyRegistration, error) {
	return nil, fmt.Errorf("global hotkey %q is only implemented on Windows", spec.Raw)
}

func (r *HotkeyRegistration) Stop() error {
	return nil
}
