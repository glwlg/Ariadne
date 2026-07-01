//go:build !windows

package setupstub

import "os/exec"

func configureCommand(cmd *exec.Cmd) {}
