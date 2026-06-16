//go:build !windows

package aiclient

import "os/exec"

func configureAgentCommand(cmd *exec.Cmd) {}
