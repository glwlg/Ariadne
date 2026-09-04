//go:build !windows

package appupdate

import "fmt"

func launchInstaller(string) error {
	return fmt.Errorf("Ariadne installer updates are only supported on Windows")
}
