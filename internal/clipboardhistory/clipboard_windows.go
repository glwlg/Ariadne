//go:build windows

package clipboardhistory

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	cfDIB         = 8
	cfUnicodeText = 13
	cfDIBV5       = 17

	biRGB       = 0
	biBitFields = 3

	gmemMoveable = 0x0002

	wmClose           = 0x0010
	wmDestroy         = 0x0002
	wmClipboardUpdate = 0x031D

	errorClassAlreadyExists = 1410
)

var (
	user32                          = windows.NewLazySystemDLL("user32.dll")
	kernel32                        = windows.NewLazySystemDLL("kernel32.dll")
	procOpenClipboard               = user32.NewProc("OpenClipboard")
	procCloseClipboard              = user32.NewProc("CloseClipboard")
	procEmptyClipboard              = user32.NewProc("EmptyClipboard")
	procGetClipboardData            = user32.NewProc("GetClipboardData")
	procIsFormatAvailable           = user32.NewProc("IsClipboardFormatAvailable")
	procSetClipboardData            = user32.NewProc("SetClipboardData")
	procAddClipboardFormatListener  = user32.NewProc("AddClipboardFormatListener")
	procRemoveClipboardFormatListen = user32.NewProc("RemoveClipboardFormatListener")
	procCreateWindowExW             = user32.NewProc("CreateWindowExW")
	procDefWindowProcW              = user32.NewProc("DefWindowProcW")
	procDestroyWindow               = user32.NewProc("DestroyWindow")
	procDispatchMessageW            = user32.NewProc("DispatchMessageW")
	procGetMessageW                 = user32.NewProc("GetMessageW")
	procPostMessageW                = user32.NewProc("PostMessageW")
	procPostQuitMessage             = user32.NewProc("PostQuitMessage")
	procRegisterClassExW            = user32.NewProc("RegisterClassExW")
	procTranslateMessage            = user32.NewProc("TranslateMessage")
	procGetModuleHandleW            = kernel32.NewProc("GetModuleHandleW")
	procGlobalAlloc                 = kernel32.NewProc("GlobalAlloc")
	procGlobalFree                  = kernel32.NewProc("GlobalFree")
	procGlobalLock                  = kernel32.NewProc("GlobalLock")
	procGlobalUnlock                = kernel32.NewProc("GlobalUnlock")
	procGlobalSize                  = kernel32.NewProc("GlobalSize")

	clipboardWatcherClassName = windows.StringToUTF16Ptr("AriadneClipboardWatcherWindow")
	clipboardWatcherWndProc   = syscall.NewCallback(clipboardWatcherWindowProc)
	clipboardWatcherCallbacks sync.Map
)

type point struct {
	X int32
	Y int32
}

type message struct {
	HWnd    windows.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   windows.Handle
	Icon       windows.Handle
	Cursor     windows.Handle
	Background windows.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     windows.Handle
}

func watchSystemClipboard(stop <-chan struct{}, onChange func()) error {
	if onChange == nil {
		return nil
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hwnd, err := createClipboardWatcherWindow(onChange)
	if err != nil {
		return err
	}
	defer func() {
		clipboardWatcherCallbacks.Delete(hwnd)
		procRemoveClipboardFormatListen.Call(uintptr(hwnd))
		procDestroyWindow.Call(uintptr(hwnd))
	}()
	added, _, addErr := procAddClipboardFormatListener.Call(uintptr(hwnd))
	if added == 0 {
		return fmt.Errorf("add clipboard listener: %w", addErr)
	}

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-stop:
			procPostMessageW.Call(uintptr(hwnd), wmClose, 0, 0)
		case <-done:
		}
	}()

	var msg message
	for {
		result, _, getErr := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if result == ^uintptr(0) {
			return fmt.Errorf("clipboard listener message loop: %w", getErr)
		}
		if result == 0 {
			return nil
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func createClipboardWatcherWindow(onChange func()) (windows.Handle, error) {
	module, _, moduleErr := procGetModuleHandleW.Call(0)
	if module == 0 {
		return 0, fmt.Errorf("get module handle: %w", moduleErr)
	}
	class := wndClassEx{
		Size:      uint32(unsafe.Sizeof(wndClassEx{})),
		WndProc:   clipboardWatcherWndProc,
		Instance:  windows.Handle(module),
		ClassName: clipboardWatcherClassName,
	}
	atom, _, registerErr := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&class)))
	if atom == 0 && registerErr != windows.Errno(errorClassAlreadyExists) {
		return 0, fmt.Errorf("register clipboard listener window: %w", registerErr)
	}
	const hwndMessage = ^uintptr(2)
	hwnd, _, createErr := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(clipboardWatcherClassName)),
		uintptr(unsafe.Pointer(clipboardWatcherClassName)),
		0,
		0,
		0,
		0,
		0,
		hwndMessage,
		0,
		module,
		0,
	)
	if hwnd == 0 {
		return 0, fmt.Errorf("create clipboard listener window: %w", createErr)
	}
	handle := windows.Handle(hwnd)
	clipboardWatcherCallbacks.Store(handle, onChange)
	return handle, nil
}

func clipboardWatcherWindowProc(hwnd windows.Handle, msg uint32, wParam uintptr, lParam uintptr) uintptr {
	switch msg {
	case wmClipboardUpdate:
		if callback, ok := clipboardWatcherCallbacks.Load(hwnd); ok {
			callback.(func())()
		}
		return 0
	case wmClose:
		procDestroyWindow.Call(uintptr(hwnd))
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	default:
		result, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
		return result
	}
}

func readSystemClipboardEntry(imageDir string, source string) (Entry, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		entry, err := readSystemClipboardEntryOnce(imageDir, source)
		if err == nil {
			return entry, nil
		}
		lastErr = err
		time.Sleep(15 * time.Millisecond)
	}
	return Entry{}, lastErr
}

func readSystemClipboardText() (string, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		text, err := readSystemClipboardTextOnce()
		if err == nil {
			return text, nil
		}
		lastErr = err
		time.Sleep(15 * time.Millisecond)
	}
	return "", lastErr
}

func readSystemClipboardEntryOnce(imageDir string, source string) (Entry, error) {
	opened, _, err := procOpenClipboard.Call(0)
	if opened == 0 {
		return Entry{}, fmt.Errorf("open clipboard: %w", err)
	}

	if available, _, _ := procIsFormatAvailable.Call(cfDIBV5); available != 0 {
		dib, err := copyClipboardData(cfDIBV5)
		procCloseClipboard.Call()
		if err != nil {
			return Entry{}, err
		}
		return makeImageEntryFromDIB(dib, imageDir, source)
	}
	if available, _, _ := procIsFormatAvailable.Call(cfDIB); available != 0 {
		dib, err := copyClipboardData(cfDIB)
		procCloseClipboard.Call()
		if err != nil {
			return Entry{}, err
		}
		return makeImageEntryFromDIB(dib, imageDir, source)
	}
	text, err := readUnicodeTextFromOpenClipboard()
	procCloseClipboard.Call()
	if err != nil {
		return Entry{}, err
	}
	return makeTextEntry(text, source), nil
}

func readSystemClipboardTextOnce() (string, error) {
	opened, _, err := procOpenClipboard.Call(0)
	if opened == 0 {
		return "", fmt.Errorf("open clipboard: %w", err)
	}
	defer procCloseClipboard.Call()

	return readUnicodeTextFromOpenClipboard()
}

func readUnicodeTextFromOpenClipboard() (string, error) {
	available, _, _ := procIsFormatAvailable.Call(cfUnicodeText)
	if available == 0 {
		return "", nil
	}

	handle, _, err := procGetClipboardData.Call(cfUnicodeText)
	if handle == 0 {
		return "", fmt.Errorf("get clipboard data: %w", err)
	}

	ptr, _, err := procGlobalLock.Call(handle)
	if ptr == 0 {
		return "", fmt.Errorf("lock clipboard data: %w", err)
	}
	defer procGlobalUnlock.Call(handle)

	size, _, _ := procGlobalSize.Call(handle)
	if size == 0 {
		return "", nil
	}
	words := int(size / 2)
	if words <= 0 {
		return "", nil
	}
	data := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), words)
	end := 0
	for end < len(data) && data[end] != 0 {
		end++
	}
	return strings.TrimSpace(windows.UTF16ToString(data[:end])), nil
}

func copyClipboardData(format uintptr) ([]byte, error) {
	handle, _, err := procGetClipboardData.Call(format)
	if handle == 0 {
		return nil, fmt.Errorf("get clipboard image: %w", err)
	}
	ptr, _, err := procGlobalLock.Call(handle)
	if ptr == 0 {
		return nil, fmt.Errorf("lock clipboard image: %w", err)
	}
	defer procGlobalUnlock.Call(handle)

	size, _, _ := procGlobalSize.Call(handle)
	if size == 0 {
		return nil, nil
	}
	return append([]byte{}, unsafe.Slice((*byte)(unsafe.Pointer(ptr)), int(size))...), nil
}

func makeImageEntryFromDIB(dib []byte, imageDir string, source string) (Entry, error) {
	if len(dib) == 0 {
		return Entry{}, nil
	}
	pngBytes, err := dibToPNG(dib)
	if err != nil {
		return Entry{}, err
	}
	return makeImageEntryFromPNG(pngBytes, imageDir, source)
}

func writeImageToSystemClipboard(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取剪贴板图片失败: %w", err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("解码剪贴板图片失败: %w", err)
	}
	dib := imageToDIB(img)

	opened, _, openErr := procOpenClipboard.Call(0)
	if opened == 0 {
		return fmt.Errorf("open clipboard: %w", openErr)
	}
	defer procCloseClipboard.Call()

	if ok, _, emptyErr := procEmptyClipboard.Call(); ok == 0 {
		return fmt.Errorf("empty clipboard: %w", emptyErr)
	}

	handle, _, allocErr := procGlobalAlloc.Call(gmemMoveable, uintptr(len(dib)))
	if handle == 0 {
		return fmt.Errorf("alloc clipboard image: %w", allocErr)
	}
	ptr, _, lockErr := procGlobalLock.Call(handle)
	if ptr == 0 {
		procGlobalFree.Call(handle)
		return fmt.Errorf("lock clipboard image: %w", lockErr)
	}
	copy(unsafe.Slice((*byte)(unsafe.Pointer(ptr)), len(dib)), dib)
	procGlobalUnlock.Call(handle)

	if result, _, setErr := procSetClipboardData.Call(cfDIB, handle); result == 0 {
		procGlobalFree.Call(handle)
		return fmt.Errorf("set clipboard image: %w", setErr)
	}
	return nil
}

func dibToPNG(dib []byte) ([]byte, error) {
	if len(dib) < 40 {
		return nil, fmt.Errorf("剪贴板图片数据太短")
	}
	headerSize := int(binary.LittleEndian.Uint32(dib[0:4]))
	if headerSize < 40 || headerSize > len(dib) {
		return nil, fmt.Errorf("不支持的剪贴板图片头")
	}
	width := int(int32(binary.LittleEndian.Uint32(dib[4:8])))
	rawHeight := int(int32(binary.LittleEndian.Uint32(dib[8:12])))
	if width <= 0 || rawHeight == 0 {
		return nil, fmt.Errorf("剪贴板图片尺寸无效")
	}
	topDown := rawHeight < 0
	height := rawHeight
	if height < 0 {
		height = -height
	}
	bpp := int(binary.LittleEndian.Uint16(dib[14:16]))
	compression := binary.LittleEndian.Uint32(dib[16:20])
	if bpp != 24 && bpp != 32 {
		return nil, fmt.Errorf("暂不支持 %d-bit 剪贴板图片", bpp)
	}
	if compression != biRGB && compression != biBitFields {
		return nil, fmt.Errorf("暂不支持压缩剪贴板图片")
	}

	offset := headerSize
	if headerSize == 40 && compression == biBitFields {
		offset += 12
	}
	if offset >= len(dib) {
		return nil, fmt.Errorf("剪贴板图片像素数据缺失")
	}
	stride := ((width*bpp + 31) / 32) * 4
	if offset+stride*height > len(dib) {
		return nil, fmt.Errorf("剪贴板图片像素数据不完整")
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	hasAlpha := false
	for y := 0; y < height; y++ {
		sourceY := y
		if !topDown {
			sourceY = height - 1 - y
		}
		row := dib[offset+sourceY*stride : offset+(sourceY+1)*stride]
		for x := 0; x < width; x++ {
			idx := x * (bpp / 8)
			b := row[idx]
			g := row[idx+1]
			r := row[idx+2]
			a := uint8(255)
			if bpp == 32 {
				a = row[idx+3]
				if a != 0 {
					hasAlpha = true
				}
			}
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}
	if bpp == 32 && !hasAlpha {
		for i := 3; i < len(img.Pix); i += 4 {
			img.Pix[i] = 255
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func imageToDIB(src image.Image) []byte {
	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	rgba := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(rgba, rgba.Bounds(), src, bounds.Min, draw.Src)

	stride := width * 4
	pixelBytes := stride * height
	dib := make([]byte, 40+pixelBytes)
	binary.LittleEndian.PutUint32(dib[0:4], 40)
	binary.LittleEndian.PutUint32(dib[4:8], uint32(width))
	binary.LittleEndian.PutUint32(dib[8:12], uint32(height))
	binary.LittleEndian.PutUint16(dib[12:14], 1)
	binary.LittleEndian.PutUint16(dib[14:16], 32)
	binary.LittleEndian.PutUint32(dib[16:20], biRGB)
	binary.LittleEndian.PutUint32(dib[20:24], uint32(pixelBytes))

	offset := 40
	for y := 0; y < height; y++ {
		sourceY := height - 1 - y
		for x := 0; x < width; x++ {
			r, g, b, a := rgba.At(x, sourceY).RGBA()
			idx := offset + y*stride + x*4
			dib[idx] = byte(b >> 8)
			dib[idx+1] = byte(g >> 8)
			dib[idx+2] = byte(r >> 8)
			dib[idx+3] = byte(a >> 8)
		}
	}
	return dib
}
