//go:build !windows

package perfcheck

func probeStartup(_ Options, iteration int) StartupSample {
	return StartupSample{
		Iteration: iteration,
		Error:     "desktop startup probing is currently implemented for Windows only",
	}
}

func probeHotkey(options Options) []HotkeySample {
	samples := make([]HotkeySample, 0, options.HotkeyIterations)
	for iteration := 1; iteration <= options.HotkeyIterations; iteration++ {
		samples = append(samples, HotkeySample{
			Iteration: iteration,
			Error:     "desktop hotkey probing is currently implemented for Windows only",
		})
	}
	return samples
}

func probeHotkeyRegistration(_ Options) HotkeyRegistrationProbe {
	return HotkeyRegistrationProbe{
		Note: "hotkey registration probing is currently implemented for Windows only",
	}
}
