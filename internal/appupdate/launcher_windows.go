//go:build windows

package appupdate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func launchInstaller(path string) error {
	currentExecutable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate current Ariadne installation: %w", err)
	}
	args := installerArguments(currentExecutable)
	if len(args) == 0 {
		return fmt.Errorf("current Ariadne installation directory is unavailable")
	}
	command := exec.Command(path, args...)
	command.Stdin = nil
	command.Stdout = nil
	command.Stderr = nil
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}

func installerArguments(currentExecutable string) []string {
	currentExecutable = strings.TrimSpace(currentExecutable)
	if currentExecutable == "" {
		return nil
	}
	return []string{"--update-from", filepath.Dir(currentExecutable)}
}
