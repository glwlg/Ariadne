//go:build !windows

package capturehistory

import "fmt"

type ScreenBounds struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

func captureScreenPNG() ([]byte, int, int, error) {
	return nil, 0, 0, fmt.Errorf("当前平台尚未接入屏幕捕获")
}

func CaptureScreenPNG() ([]byte, ScreenBounds, error) {
	return nil, ScreenBounds{}, fmt.Errorf("当前平台尚未接入屏幕捕获")
}

func CaptureScreenPNGFast() ([]byte, ScreenBounds, error) {
	return nil, ScreenBounds{}, fmt.Errorf("当前平台尚未接入屏幕捕获")
}

func CaptureRegionPNG(x int, y int, width int, height int) ([]byte, int, int, error) {
	return nil, 0, 0, fmt.Errorf("当前平台尚未接入屏幕捕获")
}

func CaptureRegionPNGFast(x int, y int, width int, height int) ([]byte, int, int, error) {
	return nil, 0, 0, fmt.Errorf("当前平台尚未接入屏幕捕获")
}

func captureScreenPNGs(options CaptureOptions) ([]capturedScreen, error) {
	return nil, fmt.Errorf("当前平台尚未接入屏幕捕获")
}

func VirtualScreenBounds() ScreenBounds {
	return ScreenBounds{}
}

func MonitorBounds() []ScreenBounds {
	return nil
}

func primaryScreenBounds() ScreenBounds {
	return ScreenBounds{}
}

func activeWindowBounds() (ScreenBounds, error) {
	return ScreenBounds{}, fmt.Errorf("当前平台尚未接入前台窗口捕获")
}

func monitorBounds() []ScreenBounds {
	return nil
}

type winRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}
