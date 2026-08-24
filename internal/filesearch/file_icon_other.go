//go:build !windows

package filesearch

import "errors"

func resolveWindowsShellIconPNG(_ string, _ int) ([]byte, error) {
	return nil, errors.New("Windows Shell 图标仅在 Windows 可用")
}
