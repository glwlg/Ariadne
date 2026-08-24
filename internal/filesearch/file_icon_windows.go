//go:build windows

package filesearch

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	shgfiIcon         = 0x000000100
	shgfiSysIconIndex = 0x000004000
	shilExtraLarge    = 0x2
	ildTransparent    = 0x1
	diNormal          = 0x0003
	biRGB             = 0
)

var (
	shell32                = windows.NewLazySystemDLL("shell32.dll")
	user32                 = windows.NewLazySystemDLL("user32.dll")
	gdi32                  = windows.NewLazySystemDLL("gdi32.dll")
	procSHGetFileInfoW     = shell32.NewProc("SHGetFileInfoW")
	procSHGetImageList     = shell32.NewProc("SHGetImageList")
	procDestroyIcon        = user32.NewProc("DestroyIcon")
	procDrawIconEx         = user32.NewProc("DrawIconEx")
	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
	procCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
)

var iidImageList = windows.GUID{
	Data1: 0x46EB5926,
	Data2: 0x582E,
	Data3: 0x4017,
	Data4: [8]byte{0x9F, 0xDF, 0xE8, 0x99, 0x8D, 0xAA, 0x09, 0x50},
}

type imageList struct {
	Vtbl *imageListVtbl
}

type imageListVtbl struct {
	QueryInterface  uintptr
	AddRef          uintptr
	Release         uintptr
	Add             uintptr
	ReplaceIcon     uintptr
	SetOverlayImage uintptr
	Replace         uintptr
	AddMasked       uintptr
	Draw            uintptr
	Remove          uintptr
	GetIcon         uintptr
}

type shellFileInfo struct {
	Icon        windows.Handle
	IconIndex   int32
	Attributes  uint32
	DisplayName [windows.MAX_PATH]uint16
	TypeName    [80]uint16
}

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
	Colors [1]uint32
}

func resolveWindowsShellIconPNG(path string, size int) ([]byte, error) {
	if size <= 0 || size > 256 {
		return nil, errors.New("invalid icon size")
	}
	icon, err := loadWindowsShellIcon(path)
	if err != nil {
		return nil, err
	}
	defer procDestroyIcon.Call(uintptr(icon))

	dc, _, _ := procCreateCompatibleDC.Call(0)
	if dc == 0 {
		return nil, errors.New("无法创建图标绘制上下文")
	}
	defer procDeleteDC.Call(dc)

	bitmapInfo := bitmapInfo{Header: bitmapInfoHeader{
		Size:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		Width:       int32(size),
		Height:      -int32(size),
		Planes:      1,
		BitCount:    32,
		Compression: biRGB,
		SizeImage:   uint32(size * size * 4),
	}}
	var pixels unsafe.Pointer
	bitmap, _, _ := procCreateDIBSection.Call(
		dc,
		uintptr(unsafe.Pointer(&bitmapInfo)),
		0,
		uintptr(unsafe.Pointer(&pixels)),
		0,
		0,
	)
	if bitmap == 0 || pixels == nil {
		return nil, errors.New("无法创建图标位图")
	}
	defer procDeleteObject.Call(bitmap)
	previous, _, _ := procSelectObject.Call(dc, bitmap)
	if previous != 0 {
		defer procSelectObject.Call(dc, previous)
	}

	drawn, _, drawErr := procDrawIconEx.Call(dc, 0, 0, uintptr(icon), uintptr(size), uintptr(size), 0, 0, diNormal)
	if drawn == 0 {
		if drawErr != windows.ERROR_SUCCESS {
			return nil, drawErr
		}
		return nil, errors.New("Windows Shell 图标绘制失败")
	}

	bgra := unsafe.Slice((*byte)(pixels), size*size*4)
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for offset := 0; offset < len(bgra); offset += 4 {
		blue, green, red, alpha := bgra[offset], bgra[offset+1], bgra[offset+2], bgra[offset+3]
		if alpha == 0 && (red != 0 || green != 0 || blue != 0) {
			alpha = 255
		}
		img.Pix[offset], img.Pix[offset+1], img.Pix[offset+2], img.Pix[offset+3] = red, green, blue, alpha
	}
	var output bytes.Buffer
	if err := png.Encode(&output, img); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func loadWindowsShellIcon(path string) (windows.Handle, error) {
	pathUTF16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var info shellFileInfo
	result, _, callErr := procSHGetFileInfoW.Call(
		uintptr(unsafe.Pointer(pathUTF16)),
		0,
		uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info),
		shgfiSysIconIndex,
	)
	if result == 0 {
		if callErr != windows.ERROR_SUCCESS {
			return 0, callErr
		}
		return 0, errors.New("Windows Shell 未返回文件图标索引")
	}

	var images *imageList
	hresult, _, _ := procSHGetImageList.Call(
		shilExtraLarge,
		uintptr(unsafe.Pointer(&iidImageList)),
		uintptr(unsafe.Pointer(&images)),
	)
	if int32(hresult) < 0 || images == nil || images.Vtbl == nil {
		return 0, errors.New("Windows Shell 未返回高清图标列表")
	}
	defer syscall.SyscallN(images.Vtbl.Release, uintptr(unsafe.Pointer(images)))

	var icon windows.Handle
	hresult, _, _ = syscall.SyscallN(
		images.Vtbl.GetIcon,
		uintptr(unsafe.Pointer(images)),
		uintptr(info.IconIndex),
		ildTransparent,
		uintptr(unsafe.Pointer(&icon)),
	)
	if int32(hresult) < 0 || icon == 0 {
		return 0, errors.New("Windows Shell 未返回高清文件图标")
	}
	return icon, nil
}
