//go:build windows

package filesearch

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procGetIconInfo = user32.NewProc("GetIconInfo")
	procGetObjectW  = gdi32.NewProc("GetObjectW")
)

type iconInfo struct {
	IsIcon   int32
	XHotspot uint32
	YHotspot uint32
	Mask     windows.Handle
	Color    windows.Handle
}

type bitmap struct {
	Type       int32
	Width      int32
	Height     int32
	WidthBytes int32
	Planes     uint16
	BitsPixel  uint16
	Bits       unsafe.Pointer
}

func TestWindowsShellIconResolverReturnsPNGForAssociatedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workbook.xlsx")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	asset, err := resolveWindowsShellIconPNG(path, 48)
	if err != nil {
		t.Fatalf("resolve Windows Shell icon: %v", err)
	}
	image, err := png.Decode(bytes.NewReader(asset))
	if err != nil {
		t.Fatalf("decode icon PNG: %v", err)
	}
	if image.Bounds().Dx() != 48 || image.Bounds().Dy() != 48 {
		t.Fatalf("unexpected icon dimensions: %v", image.Bounds())
	}
	visible := false
	for y := image.Bounds().Min.Y; y < image.Bounds().Max.Y && !visible; y++ {
		for x := image.Bounds().Min.X; x < image.Bounds().Max.X; x++ {
			_, _, _, alpha := image.At(x, y).RGBA()
			if alpha != 0 {
				visible = true
				break
			}
		}
	}
	if !visible {
		t.Fatal("Windows Shell icon PNG is fully transparent")
	}
}

func TestWindowsShellIconSourceIsNotUpscaled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workbook.xlsx")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	icon, err := loadWindowsShellIcon(path)
	if err != nil {
		t.Fatalf("load Windows Shell icon: %v", err)
	}
	defer procDestroyIcon.Call(uintptr(icon))

	var info iconInfo
	if ok, _, _ := procGetIconInfo.Call(uintptr(icon), uintptr(unsafe.Pointer(&info))); ok == 0 {
		t.Fatal("GetIconInfo failed")
	}
	defer procDeleteObject.Call(uintptr(info.Mask))
	defer procDeleteObject.Call(uintptr(info.Color))

	var native bitmap
	if result, _, _ := procGetObjectW.Call(uintptr(info.Color), unsafe.Sizeof(native), uintptr(unsafe.Pointer(&native))); result == 0 {
		t.Fatal("GetObjectW failed")
	}
	if native.Width < 48 || native.Height < 48 {
		t.Fatalf("Shell icon source is being upscaled from %dx%d to 48x48", native.Width, native.Height)
	}
}
