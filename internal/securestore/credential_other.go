//go:build !windows

package securestore

import "fmt"

func defaultAvailable() bool {
	return false
}

func defaultBackend() string {
	return "unsupported"
}

func Read(target string) (string, bool, error) {
	return "", false, fmt.Errorf("secure credential store is only implemented on Windows")
}

func Write(target string, secret string) error {
	return fmt.Errorf("secure credential store is only implemented on Windows")
}

func Delete(target string) error {
	return fmt.Errorf("secure credential store is only implemented on Windows")
}
