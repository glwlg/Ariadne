//go:build !windows

package setupstub

func setAutostart(productName string, exePath string) error {
	return nil
}

func cleanupAutostart(productName string) {}
