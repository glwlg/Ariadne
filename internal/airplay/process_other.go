//go:build !windows

package airplay

import "os/exec"

func hideConsole(cmd *exec.Cmd) {}
