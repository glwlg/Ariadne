//go:build !windows

package aiclient

import "os/exec"

func configureFlowAgentCommand(_ *exec.Cmd) {}
