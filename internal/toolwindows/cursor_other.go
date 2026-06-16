//go:build !windows

package toolwindows

import "github.com/wailsapp/wails/v3/pkg/application"

func networkMiniCursorPoint() (application.Point, bool) {
	return application.Point{}, false
}
