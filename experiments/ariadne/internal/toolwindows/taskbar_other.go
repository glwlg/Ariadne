//go:build !windows

package toolwindows

import "github.com/wailsapp/wails/v3/pkg/application"

func applyNetworkMiniTaskbarOwner(window application.Window) {}

func refreshNetworkMiniTaskbarLayer(window application.Window) {}
