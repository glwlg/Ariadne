//go:build windows

package toolwindows

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"
)

func networkMiniCursorPoint() (application.Point, bool) {
	x, y, ok := w32.GetCursorPos()
	return application.Point{X: x, Y: y}, ok
}
