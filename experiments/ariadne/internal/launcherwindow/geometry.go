package launcherwindow

import "github.com/wailsapp/wails/v3/pkg/application"

const (
	Width           = 860
	CollapsedHeight = 96
	ExpandedHeight  = 468
)

func Size(expanded bool) (int, int) {
	if expanded {
		return Width, ExpandedHeight
	}
	return Width, CollapsedHeight
}

func ReservedRelativePosition(screen *application.Screen) (int, int, bool) {
	if screen == nil {
		return 0, 0, false
	}
	workWidth := screen.WorkArea.Width
	workHeight := screen.WorkArea.Height
	if workWidth <= 0 || workHeight <= 0 {
		workWidth = screen.Bounds.Width
		workHeight = screen.Bounds.Height
	}
	if workWidth <= 0 || workHeight <= 0 {
		workWidth = screen.Size.Width
		workHeight = screen.Size.Height
	}
	if workWidth <= 0 || workHeight <= 0 {
		return 0, 0, false
	}
	x := (workWidth - Width) / 2
	y := (workHeight - ExpandedHeight) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return x, y, true
}

func InitialPlacement(screen *application.Screen) (application.WindowStartPosition, int, int, *application.Screen) {
	x, y, ok := ReservedRelativePosition(screen)
	if !ok {
		return application.WindowCentered, 0, 0, nil
	}
	return application.WindowXY, x, y, screen
}

func ApplyCollapsed(window application.Window, screen *application.Screen) {
	if window == nil {
		return
	}
	window.SetSize(Width, CollapsedHeight)
	if current, err := window.GetScreen(); err == nil {
		if x, y, ok := ReservedRelativePosition(current); ok {
			window.SetRelativePosition(x, y)
			return
		}
	}
	if x, y, ok := ReservedRelativePosition(screen); ok {
		window.SetScreen(screen)
		window.SetRelativePosition(x, y)
		return
	}
	window.Center()
}
