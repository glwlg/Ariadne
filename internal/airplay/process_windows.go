//go:build windows

package airplay

import (
	"os/exec"
	"syscall"
)

func hideConsole(cmd *exec.Cmd) {
	// SW_HIDE also hides GStreamer's first video window; suppress only the console.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
}
