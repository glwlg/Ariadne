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
	"strings"
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
)

var (
	user32                = windows.NewLazySystemDLL("user32.dll")
	kernel32              = windows.NewLazySystemDLL("kernel32.dll")
	procOpenClipboard     = user32.NewProc("OpenClipboard")
	procCloseClipboard    = user32.NewProc("CloseClipboard")
	procEmptyClipboard    = user32.NewProc("EmptyClipboard")
	procGetClipboardData  = user32.NewProc("GetClipboardData")
	procIsFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	procSetClipboardData  = user32.NewProc("SetClipboardData")
	procGlobalAlloc       = kernel32.NewProc("GlobalAlloc")
	procGlobalFree        = kernel32.NewProc("GlobalFree")
	procGlobalLock        = kernel32.NewProc("GlobalLock")
	procGlobalUnlock      = kernel32.NewProc("GlobalUnlock")
	procGlobalSize        = kernel32.NewProc("GlobalSize")
)

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
	defer procCloseClipboard.Call()

	if available, _, _ := procIsFormatAvailable.Call(cfDIBV5); available != 0 {
		return readImageEntryFromOpenClipboard(cfDIBV5, imageDir, source)
	}
	if available, _, _ := procIsFormatAvailable.Call(cfDIB); available != 0 {
		return readImageEntryFromOpenClipboard(cfDIB, imageDir, source)
	}
	text, err := readUnicodeTextFromOpenClipboard()
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

func readImageEntryFromOpenClipboard(format uintptr, imageDir string, source string) (Entry, error) {
	handle, _, err := procGetClipboardData.Call(format)
	if handle == 0 {
		return Entry{}, fmt.Errorf("get clipboard image: %w", err)
	}
	ptr, _, err := procGlobalLock.Call(handle)
	if ptr == 0 {
		return Entry{}, fmt.Errorf("lock clipboard image: %w", err)
	}
	defer procGlobalUnlock.Call(handle)

	size, _, _ := procGlobalSize.Call(handle)
	if size == 0 {
		return Entry{}, nil
	}
	dib := append([]byte{}, unsafe.Slice((*byte)(unsafe.Pointer(ptr)), int(size))...)
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
