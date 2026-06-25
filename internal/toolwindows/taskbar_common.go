package toolwindows

import "strings"

func isNetworkMiniTaskbarClassName(className string) bool {
	switch strings.TrimSpace(className) {
	case "Shell_TrayWnd", "Shell_SecondaryTrayWnd":
		return true
	default:
		return false
	}
}
