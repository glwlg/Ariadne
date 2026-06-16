package pinnedimage

import (
	"encoding/base64"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"ariadne/internal/capturehistory"
	"ariadne/internal/clipboardhistory"
)

const testOnePixelPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgwJ/lkD1JwAAAABJRU5ErkJggg=="

func TestOpenCaptureStagesPinnedImage(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	entry := captures.AddPNG(testPNG(t), 64, 32, "test-screen", "", nil).Entries[0]
	service := NewService(captures, nil)
	var opened PinnedImage
	service.openWindow = func(pin PinnedImage) error {
		opened = pin
		return nil
	}

	result := service.OpenCapture(entry.ID)
	if !result.OK || result.PinID == "" {
		t.Fatalf("expected capture pin to open, got %#v", result)
	}
	if opened.Source != "capture" || opened.SourceID != entry.ID || opened.ImagePath != entry.ImagePath || !opened.CanOCR {
		t.Fatalf("unexpected opened pin: %#v", opened)
	}
	pin := service.GetPinned(result.PinID)
	if pin.ID != result.PinID || !strings.HasPrefix(pin.DataURL, "data:image/png;base64,") {
		t.Fatalf("expected stored capture pin with data url, got %#v", pin)
	}
}

func TestOpenCaptureAtStagesPinnedImagePosition(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	entry := captures.AddPNG(testPNG(t), 64, 32, "test-screen", "", nil).Entries[0]
	service := NewService(captures, nil)
	var opened PinnedImage
	service.openWindow = func(pin PinnedImage) error {
		opened = pin
		return nil
	}

	result := service.OpenCaptureAt(entry.ID, 320, 240)
	if !result.OK || result.PinID == "" {
		t.Fatalf("expected positioned capture pin to open, got %#v", result)
	}
	if !opened.Positioned || opened.WindowX != 320 || opened.WindowY != 240 {
		t.Fatalf("expected positioned pin, got %#v", opened)
	}
	pin := service.GetPinned(result.PinID)
	if !pin.Positioned || pin.WindowX != 320 || pin.WindowY != 240 {
		t.Fatalf("expected stored positioned pin, got %#v", pin)
	}
}

func TestPinnedImageWindowUsesExactImageSize(t *testing.T) {
	pin := newPinnedImage("capture", "capture-id", "截图贴图 64x32", "capture.png", "data:image/png;base64,test", 64, 32, 128, false, "")
	if pin.WindowW != 64 || pin.WindowH != 32 {
		t.Fatalf("pin window should match source image size, got %dx%d", pin.WindowW, pin.WindowH)
	}
}

func TestSetPinnedPositionUpdatesStoredPositionWithoutWindow(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	entry := captures.AddPNG(testPNG(t), 64, 32, "test-screen", "", nil).Entries[0]
	service := NewService(captures, nil)
	service.openWindow = func(PinnedImage) error { return nil }

	result := service.OpenCapture(entry.ID)
	if !result.OK || result.PinID == "" {
		t.Fatalf("expected capture pin to open, got %#v", result)
	}
	moved := service.SetPinnedPosition(result.PinID, 420, 260)
	if !moved.OK {
		t.Fatalf("expected position sync, got %#v", moved)
	}
	pin := service.GetPinned(result.PinID)
	if !pin.Positioned || pin.WindowX != 420 || pin.WindowY != 260 {
		t.Fatalf("expected stored synced position, got %#v", pin)
	}
}

func TestOpenClipboardImageStagesPinnedImage(t *testing.T) {
	dir := t.TempDir()
	clipboards := clipboardhistory.NewServiceWithPaths(filepath.Join(dir, "clipboard_history.json"), filepath.Join(dir, "clipboard_images"))
	entry := clipboards.AddPNG(testPNG(t), "test").Entries[0]
	service := NewService(nil, clipboards)
	var opened PinnedImage
	service.openWindow = func(pin PinnedImage) error {
		opened = pin
		return nil
	}

	result := service.OpenClipboardImage(entry.ID)
	if !result.OK || result.PinID == "" {
		t.Fatalf("expected clipboard pin to open, got %#v", result)
	}
	if opened.Source != "clipboard" || opened.SourceID != entry.ID || !opened.CanCopy || opened.CopyAction != "copy_clipboard_image" || !opened.CanOCR {
		t.Fatalf("unexpected clipboard pin: %#v", opened)
	}
}

func TestOpenCurrentClipboardImageStagesPinnedImage(t *testing.T) {
	entry := clipboardhistory.Entry{
		ID:        "clip-image",
		Type:      clipboardhistory.EntryImage,
		ImagePath: "clipboard.png",
		Width:     80,
		Height:    40,
		Bytes:     128,
	}
	source := fakeClipboardSource{
		collected: clipboardhistory.CollectCurrentResult{OK: true, Message: "ok", Entry: entry},
		entry:     entry,
		dataURL:   "data:image/png;base64,test",
	}
	service := NewService(nil, source)
	var opened PinnedImage
	service.openWindow = func(pin PinnedImage) error {
		opened = pin
		return nil
	}

	result := service.OpenCurrentClipboard()
	if !result.OK || result.PinID == "" {
		t.Fatalf("expected current clipboard image pin to open, got %#v", result)
	}
	if opened.Source != "clipboard" || opened.SourceID != entry.ID || opened.Width != 80 || opened.Height != 40 {
		t.Fatalf("unexpected current clipboard image pin: %#v", opened)
	}
}

func TestOpenCurrentClipboardTextStagesTextPinnedImage(t *testing.T) {
	entry := clipboardhistory.Entry{
		ID:   "clip-text",
		Type: clipboardhistory.EntryText,
		Text: "hello\nAriadne",
	}
	source := fakeClipboardSource{
		collected: clipboardhistory.CollectCurrentResult{OK: true, Message: "ok", Entry: entry},
		entry:     entry,
	}
	service := NewService(nil, source)
	var opened PinnedImage
	service.openWindow = func(pin PinnedImage) error {
		opened = pin
		return nil
	}

	result := service.OpenCurrentClipboard()
	if !result.OK || result.PinID == "" {
		t.Fatalf("expected current clipboard text pin to open, got %#v", result)
	}
	if opened.Source != "clipboard_text" || opened.SourceID != entry.ID || opened.Text != entry.Text || !strings.HasPrefix(opened.DataURL, "data:image/svg+xml;base64,") {
		t.Fatalf("unexpected current clipboard text pin: %#v", opened)
	}
	if opened.WindowW <= 0 || opened.WindowH <= 0 || opened.CanOCR {
		t.Fatalf("unexpected text pin sizing/OCR flags: %#v", opened)
	}
}

func TestOpenQRTextStagesPinnedImage(t *testing.T) {
	service := NewService(nil, nil)
	service.openWindow = func(pin PinnedImage) error {
		if pin.Source != "qr" || pin.Width != 320 || pin.Height != 320 || pin.CanOCR {
			t.Fatalf("unexpected QR pin: %#v", pin)
		}
		return nil
	}

	result := service.OpenQRText("ariadne")
	if !result.OK || result.PinID == "" {
		t.Fatalf("expected QR pin to open, got %#v", result)
	}
	if pin := service.GetPinned(result.PinID); !strings.HasPrefix(pin.DataURL, "data:image/png;base64,") {
		t.Fatalf("expected QR data url, got %#v", pin)
	}
}

func TestOpenFailureRemovesPinnedImage(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	entry := captures.AddPNG(testPNG(t), 64, 32, "test-screen", "", nil).Entries[0]
	service := NewService(captures, nil)
	service.openWindow = func(PinnedImage) error {
		return errors.New("window failed")
	}

	result := service.OpenCapture(entry.ID)
	if result.OK || result.Message != "window failed" {
		t.Fatalf("expected open failure, got %#v", result)
	}
	if result.PinID != "" {
		t.Fatalf("failed open must not return pin id: %#v", result)
	}
}

type fakeClipboardSource struct {
	collected clipboardhistory.CollectCurrentResult
	entry     clipboardhistory.Entry
	dataURL   string
}

func (f fakeClipboardSource) CollectCurrentEntry(string) clipboardhistory.CollectCurrentResult {
	return f.collected
}

func (f fakeClipboardSource) Entry(string) clipboardhistory.Entry {
	return f.entry
}

func (f fakeClipboardSource) ImageDataURL(string) string {
	return f.dataURL
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(testOnePixelPNG)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
