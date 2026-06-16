//go:build windows

package aiclient

import (
	"os/exec"
	"syscall"
)

const agentCreateNoWindow = 0x08000000

func configureAgentCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: agentCreateNoWindow,
	}
}
