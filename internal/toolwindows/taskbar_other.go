//go:build !windows

package toolwindows

import "github.com/wailsapp/wails/v3/pkg/application"

func applyNetworkMiniTaskbarOwner(window application.Window) {}

func refreshNetworkMiniTaskbarLayer(window application.Window) {}

func networkMiniTaskbarForegroundActive() bool { return false }

func watchNetworkMiniTaskbarForeground(stop <-chan struct{}, onTaskbarForeground func()) error {
	<-stop
	return nil
}

func enableOrdinaryWindowTaskbarToggle(window application.Window) {}

func setOrdinaryWindowIcon(window application.Window, icon []byte) {}
