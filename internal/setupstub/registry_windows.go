//go:build windows

package setupstub

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func setAutostart(productName string, exePath string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	if strings.EqualFold(strings.TrimSpace(productName), "Fancun") {
		if err := key.DeleteValue("Fancun"); err != nil && err != registry.ErrNotExist {
			return err
		}
	}
	return key.SetStringValue(autostartValueName(productName), fmt.Sprintf("%q --hidden", exePath))
}

func cleanupAutostart(productName string) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return
	}
	defer key.Close()
	_ = key.DeleteValue(autostartValueName(productName))
	if strings.EqualFold(strings.TrimSpace(productName), "Fancun") {
		_ = key.DeleteValue("Fancun")
	}
}

func autostartValueName(productName string) string {
	if strings.TrimSpace(productName) == "" {
		productName = "Fancun"
	}
	if strings.EqualFold(strings.TrimSpace(productName), "Fancun") {
		return "com.glwlg.fancun"
	}
	return productName
}
