//go:build !windows

package setupstub

import (
	"fmt"
	"os"
	"path/filepath"
)

func createShortcut(targetPath string, shortcutPath string, iconPath string, arguments string) error {
	if err := os.MkdirAll(filepath.Dir(shortcutPath), 0o755); err != nil {
		return err
	}
	content := fmt.Sprintf("target=%s\narguments=%s\nicon=%s\n", targetPath, arguments, iconPath)
	return os.WriteFile(shortcutPath, []byte(content), 0o644)
}

func refreshShell() {}
