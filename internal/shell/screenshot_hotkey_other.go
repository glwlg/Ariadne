//go:build !windows

package shell

type noopScreenshotHotkeyHook struct{}

func (noopScreenshotHotkeyHook) stop() {}

func (noopScreenshotHotkeyHook) active() bool { return true }

func startScreenshotHotkeyHook(spec HotkeySpec, freeze func()) (screenshotHotkeyHook, error) {
	_ = spec
	_ = freeze
	return noopScreenshotHotkeyHook{}, nil
}
