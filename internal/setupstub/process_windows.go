//go:build windows

package setupstub

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

func configureCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
