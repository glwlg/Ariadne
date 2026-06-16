//go:build !windows

package toolwindows

func defaultFullscreenDetector() (bool, error) {
	return false, nil
}
