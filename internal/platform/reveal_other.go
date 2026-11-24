//go:build !windows

package platform

import (
	"os"
	"os/exec"
)

func revealInExplorer(path string) error {
	target, err := resolveRevealTarget(path, os.Stat)
	if err != nil {
		return err
	}
	if target.Mode == revealModeOpenDir {
		return exec.Command("explorer.exe", target.Path).Start()
	}
	return exec.Command("explorer.exe", "/select,"+target.Path).Start()
}
