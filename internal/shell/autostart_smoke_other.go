//go:build !windows

package shell

import "runtime"

func RunAutostartSmoke(options AutostartSmokeOptions) AutostartSmokeReport {
	return AutostartSmokeReport{
		OK:       false,
		Platform: runtime.GOOS,
		Error:    "autostart registry smoke is implemented for Windows only",
	}
}
