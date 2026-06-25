//go:build windows

package capturehistory

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"syscall"
	"unsafe"
)

const (
	smCXScreen        = 0
	smCYScreen        = 1
	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCXVirtualScreen = 78
	smCYVirtualScreen = 79

	srccopy       = 0x00CC0020
	captureblt    = 0x40000000
	biRGB         = 0
	dibRGBColors  = 0
	screenDesktop = 0
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	gdi32                   = syscall.NewLazyDLL("gdi32.dll")
	procGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procGetWindowRect       = user32.NewProc("GetWindowRect")
	procEnumDisplayMonitors = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW     = user32.NewProc("GetMonitorInfoW")
	procGetDC               = user32.NewProc("GetDC")
	procReleaseDC           = user32.NewProc("ReleaseDC")
	procCreateCompatibleDC  = gdi32.NewProc("CreateCompatibleDC")
	procCreateBitmap        = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject        = gdi32.NewProc("SelectObject")
	procBitBlt              = gdi32.NewProc("BitBlt")
	procGetDIBits           = gdi32.NewProc("GetDIBits")
	procDeleteObject        = gdi32.NewProc("DeleteObject")
	procDeleteDC            = gdi32.NewProc("DeleteDC")
)

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [3]uint32
}

type ScreenBounds struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type winRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type monitorInfo struct {
	Size    uint32
	Monitor winRect
	Work    winRect
	Flags   uint32
}

func captureScreenPNG() ([]byte, int, int, error) {
	bounds := virtualScreenBounds()
	data, width, height, err := captureRegionPNG(bounds.X, bounds.Y, bounds.Width, bounds.Height)
	if err != nil {
		return nil, 0, 0, err
	}
	return data, width, height, nil
}

func CaptureScreenPNG() ([]byte, ScreenBounds, error) {
	bounds := virtualScreenBounds()
	data, _, _, err := captureRegionPNG(bounds.X, bounds.Y, bounds.Width, bounds.Height)
	return data, bounds, err
}

func CaptureScreenPNGFast() ([]byte, ScreenBounds, error) {
	bounds := virtualScreenBounds()
	data, _, _, err := captureRegionPNGWithCompression(bounds.X, bounds.Y, bounds.Width, bounds.Height, png.BestSpeed)
	return data, bounds, err
}

func captureScreenPNGs(options CaptureOptions) ([]capturedScreen, error) {
	options = normalizeCaptureOptions(options)
	bounds, err := captureBoundsForOptions(options)
	if err != nil {
		return nil, err
	}
	captures := make([]capturedScreen, 0, len(bounds))
	for index, bound := range bounds {
		data, width, height, err := captureRegionPNG(bound.X, bound.Y, bound.Width, bound.Height)
		if err != nil {
			return nil, err
		}
		captures = append(captures, capturedScreen{
			Data:    data,
			Width:   width,
			Height:  height,
			Bounds:  bound,
			Actions: captureActionTags(options, index, len(bounds)),
			Tags:    captureMetadataTags(options, bound, index, len(bounds)),
		})
	}
	return captures, nil
}

func CaptureRegionPNG(x int, y int, width int, height int) ([]byte, int, int, error) {
	return captureRegionPNG(x, y, width, height)
}

func CaptureRegionPNGFast(x int, y int, width int, height int) ([]byte, int, int, error) {
	return captureRegionPNGWithCompression(x, y, width, height, png.BestSpeed)
}

func VirtualScreenBounds() ScreenBounds {
	return virtualScreenBounds()
}

func MonitorBounds() []ScreenBounds {
	return monitorBounds()
}

func captureRegionPNG(x int, y int, width int, height int) ([]byte, int, int, error) {
	return captureRegionPNGWithCompression(x, y, width, height, png.DefaultCompression)
}

func captureRegionPNGWithCompression(x int, y int, width int, height int, compression png.CompressionLevel) ([]byte, int, int, error) {
	if width <= 0 || height <= 0 {
		return nil, 0, 0, fmt.Errorf("截图区域尺寸无效")
	}
	bounds := virtualScreenBounds()
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return nil, 0, 0, fmt.Errorf("无法读取虚拟屏幕尺寸")
	}
	region := image.Rect(x, y, x+width, y+height).Intersect(image.Rect(bounds.X, bounds.Y, bounds.X+bounds.Width, bounds.Y+bounds.Height))
	if region.Empty() {
		return nil, 0, 0, fmt.Errorf("截图区域不在虚拟屏幕内")
	}
	x = region.Min.X
	y = region.Min.Y
	width = region.Dx()
	height = region.Dy()

	screenDC, _, err := procGetDC.Call(screenDesktop)
	if screenDC == 0 {
		return nil, 0, 0, fmt.Errorf("GetDC 失败: %v", err)
	}
	defer procReleaseDC.Call(screenDesktop, screenDC)

	memDC, _, err := procCreateCompatibleDC.Call(screenDC)
	if memDC == 0 {
		return nil, 0, 0, fmt.Errorf("CreateCompatibleDC 失败: %v", err)
	}
	defer procDeleteDC.Call(memDC)

	bitmap, _, err := procCreateBitmap.Call(screenDC, uintptr(width), uintptr(height))
	if bitmap == 0 {
		return nil, 0, 0, fmt.Errorf("CreateCompatibleBitmap 失败: %v", err)
	}
	defer procDeleteObject.Call(bitmap)

	oldObject, _, _ := procSelectObject.Call(memDC, bitmap)
	if oldObject != 0 {
		defer procSelectObject.Call(memDC, oldObject)
	}

	ok, _, err := procBitBlt.Call(
		memDC,
		0,
		0,
		uintptr(width),
		uintptr(height),
		screenDC,
		uintptr(x),
		uintptr(y),
		srccopy|captureblt,
	)
	if ok == 0 {
		return nil, 0, 0, fmt.Errorf("BitBlt 失败: %v", err)
	}

	bmi := bitmapInfo{Header: bitmapInfoHeader{
		Size:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		Width:       int32(width),
		Height:      -int32(height),
		Planes:      1,
		BitCount:    32,
		Compression: biRGB,
		SizeImage:   uint32(width * height * 4),
	}}
	pixels := make([]byte, width*height*4)
	lines, _, err := procGetDIBits.Call(
		memDC,
		bitmap,
		0,
		uintptr(height),
		uintptr(unsafe.Pointer(&pixels[0])),
		uintptr(unsafe.Pointer(&bmi)),
		dibRGBColors,
	)
	if lines == 0 {
		return nil, 0, 0, fmt.Errorf("GetDIBits 失败: %v", err)
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < width*height; i++ {
		src := i * 4
		dst := i * 4
		img.Pix[dst+0] = pixels[src+2]
		img.Pix[dst+1] = pixels[src+1]
		img.Pix[dst+2] = pixels[src+0]
		alpha := pixels[src+3]
		if alpha == 0 {
			alpha = 255
		}
		img.Pix[dst+3] = alpha
	}

	var out bytes.Buffer
	if err := encodePNG(&out, img, compression); err != nil {
		return nil, 0, 0, err
	}
	return out.Bytes(), width, height, nil
}

func encodePNG(out *bytes.Buffer, img image.Image, compression png.CompressionLevel) error {
	if compression == png.DefaultCompression {
		return png.Encode(out, img)
	}
	encoder := png.Encoder{CompressionLevel: compression}
	return encoder.Encode(out, img)
}

func virtualScreenBounds() ScreenBounds {
	return ScreenBounds{
		X:      systemMetricInt(smXVirtualScreen),
		Y:      systemMetricInt(smYVirtualScreen),
		Width:  systemMetricInt(smCXVirtualScreen),
		Height: systemMetricInt(smCYVirtualScreen),
	}
}

func primaryScreenBounds() ScreenBounds {
	return ScreenBounds{X: 0, Y: 0, Width: systemMetricInt(smCXScreen), Height: systemMetricInt(smCYScreen)}
}

func activeWindowBounds() (ScreenBounds, error) {
	window, _, _ := procGetForegroundWindow.Call()
	if window == 0 {
		return ScreenBounds{}, fmt.Errorf("无法读取前台窗口")
	}
	var rect winRect
	ok, _, err := procGetWindowRect.Call(window, uintptr(unsafe.Pointer(&rect)))
	if ok == 0 {
		return ScreenBounds{}, fmt.Errorf("GetWindowRect 失败: %v", err)
	}
	bounds := rectToBounds(rect)
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return ScreenBounds{}, fmt.Errorf("前台窗口尺寸无效")
	}
	return intersectBounds(bounds, virtualScreenBounds())
}

func monitorBounds() []ScreenBounds {
	bounds := []ScreenBounds{}
	callback := syscall.NewCallback(func(monitor uintptr, hdc uintptr, rect uintptr, data uintptr) uintptr {
		info := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
		ok, _, _ := procGetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info)))
		if ok != 0 {
			bound := rectToBounds(info.Monitor)
			if bound.Width > 0 && bound.Height > 0 {
				bounds = append(bounds, bound)
			}
		}
		return 1
	})
	procEnumDisplayMonitors.Call(0, 0, callback, 0)
	if len(bounds) == 0 {
		bounds = append(bounds, virtualScreenBounds())
	}
	return bounds
}

func systemMetricInt(index int) int {
	value, _, _ := procGetSystemMetrics.Call(uintptr(index))
	return int(int32(value))
}
