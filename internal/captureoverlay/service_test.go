package captureoverlay

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ariadne/internal/capturehistory"
	"ariadne/internal/nativecapture"
	"ariadne/internal/ocr"
	"ariadne/internal/pinnedimage"

	goqrcode "github.com/skip2/go-qrcode"
)

type failingPinService struct{}

type fakeOCRProvider struct {
	mu     sync.Mutex
	result ocr.Result
	paths  []string
	delay  time.Duration
}

func (f *fakeOCRProvider) RecognizeImagePath(path string) ocr.Result {
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.paths = append(f.paths, path)
	return f.result
}

func (f *fakeOCRProvider) pathCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.paths)
}

func (f failingPinService) OpenCapture(id string) pinnedimage.OpenResult {
	return pinnedimage.OpenResult{OK: false, Message: "pin failed", PinID: id}
}

type positionedPinRecorder struct {
	id    string
	x     int
	y     int
	calls int
}

type blockingCaptureSink struct {
	called  chan struct{}
	release chan struct{}
}

type panicCaptureSink struct{}

func (b *blockingCaptureSink) AddPNG(data []byte, width int, height int, source string, savedPath string, actions []string) capturehistory.Status {
	close(b.called)
	<-b.release
	return capturehistory.Status{
		Entries: []capturehistory.Entry{{
			ID:        "cap-async",
			ImagePath: "async.png",
			Width:     width,
			Height:    height,
		}},
	}
}

func (panicCaptureSink) AddPNG(data []byte, width int, height int, source string, savedPath string, actions []string) capturehistory.Status {
	panic("OCR text action must not write screenshot history")
}

func (p *positionedPinRecorder) OpenCapture(id string) pinnedimage.OpenResult {
	p.calls++
	p.id = id
	return pinnedimage.OpenResult{OK: true, Message: "pinned", PinID: id}
}

func (p *positionedPinRecorder) OpenCaptureAt(id string, x int, y int) pinnedimage.OpenResult {
	p.calls++
	p.id = id
	p.x = x
	p.y = y
	return pinnedimage.OpenResult{OK: true, Message: "pinned", PinID: id}
}

func TestCaptureSelectionCropsAndSavesHistory(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	service := NewService(captures, nil)
	session := service.stageTestSession(t, capturehistory.ScreenBounds{X: 10, Y: 20, Width: 4, Height: 3})

	result := service.CaptureSelection(SelectionRequest{
		SessionID: session.ID,
		X:         11,
		Y:         21,
		Width:     2,
		Height:    2,
		Action:    "capture",
	})

	if !result.OK || result.CaptureID == "" || result.Width != 2 || result.Height != 2 {
		t.Fatalf("expected saved selection, got %#v", result)
	}
	if _, err := os.Stat(result.ImagePath); err != nil {
		t.Fatalf("expected cropped image file: %v", err)
	}
	if service.GetSession(session.ID).ID != "" {
		t.Fatal("successful capture should finish the overlay session")
	}
}

func TestCaptureSelectionUsesNativeBoundsWhenDisplayBoundsDiffer(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	service := NewService(captures, nil)
	session := service.stageTestSessionWithNative(
		t,
		capturehistory.ScreenBounds{X: 0, Y: 0, Width: 100, Height: 80},
		capturehistory.ScreenBounds{X: 0, Y: 0, Width: 200, Height: 160},
	)

	result := service.CaptureSelection(SelectionRequest{
		SessionID: session.ID,
		X:         100,
		Y:         80,
		Width:     20,
		Height:    10,
		Action:    "capture",
	})

	if !result.OK || result.CaptureID == "" || result.Width != 20 || result.Height != 10 {
		t.Fatalf("expected native selection size, got %#v", result)
	}
	img := readPNG(t, result.ImagePath)
	if !sameRGBA(img.At(0, 0), color.RGBA{R: 100, G: 80, B: 120, A: 255}) {
		t.Fatalf("expected cropped native pixel at 100,80, got %#v", img.At(0, 0))
	}
}

func TestCaptureSelectionPinsFromOffsetNativeBounds(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	pins := &positionedPinRecorder{}
	service := NewService(captures, pins)
	session := service.stageTestSessionWithNative(
		t,
		capturehistory.ScreenBounds{X: 100, Y: 200, Width: 120, Height: 90},
		capturehistory.ScreenBounds{X: 2000, Y: 900, Width: 240, Height: 180},
	)

	result := service.CaptureSelection(SelectionRequest{
		SessionID: session.ID,
		X:         2020,
		Y:         930,
		Width:     12,
		Height:    8,
		Action:    "pin",
	})

	if !result.OK || result.CaptureID == "" || result.Pin == nil || !result.Pin.OK {
		t.Fatalf("expected pinned native-offset capture, got %#v", result)
	}
	if result.Width != 12 || result.Height != 8 {
		t.Fatalf("expected native selection size, got %dx%d", result.Width, result.Height)
	}
	img := readPNG(t, result.ImagePath)
	if !sameRGBA(img.At(0, 0), color.RGBA{R: 20, G: 30, B: 120, A: 255}) {
		t.Fatalf("expected offset native pixel 2020,930 to map to local 20,30, got %#v", img.At(0, 0))
	}
	if pins.id != result.CaptureID || pins.x != 2020 || pins.y != 930 {
		t.Fatalf("expected pin at native selection origin, got id=%q x=%d y=%d", pins.id, pins.x, pins.y)
	}
}

func TestCaptureSelectionUsesSessionCoordinateSpaceForDisplayLocalPixels(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	service := NewService(captures, nil)
	session := service.stageTestSessionWithNative(
		t,
		capturehistory.ScreenBounds{X: 67, Y: 83, Width: 120, Height: 90},
		capturehistory.ScreenBounds{X: 2000, Y: 900, Width: 240, Height: 180},
	)

	result := service.CaptureSelection(SelectionRequest{
		SessionID:       session.ID,
		CoordinateSpace: "session",
		X:               20,
		Y:               30,
		Width:           12,
		Height:          8,
		Action:          "capture",
	})

	if !result.OK || result.CaptureID == "" || result.Width != 12 || result.Height != 8 {
		t.Fatalf("expected local session crop, got %#v", result)
	}
	img := readPNG(t, result.ImagePath)
	if !sameRGBA(img.At(0, 0), color.RGBA{R: 20, G: 30, B: 120, A: 255}) {
		t.Fatalf("expected local pixel 20,30, got %#v", img.At(0, 0))
	}
}

func TestCaptureSelectionSessionCoordinatesStaySourceLocalWhenPinned(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	pins := &positionedPinRecorder{}
	service := NewService(captures, pins)
	session := service.stageTestSessionWithNative(
		t,
		capturehistory.ScreenBounds{X: 300, Y: 150, Width: 100, Height: 80},
		capturehistory.ScreenBounds{X: 2000, Y: 900, Width: 200, Height: 160},
	)

	result := service.CaptureSelection(SelectionRequest{
		SessionID:       session.ID,
		CoordinateSpace: "session",
		X:               100,
		Y:               40,
		Width:           60,
		Height:          30,
		Action:          "pin",
		PinPositioned:   true,
		PinX:            335,
		PinY:            175,
	})

	if !result.OK || result.CaptureID == "" || result.Pin == nil || !result.Pin.OK {
		t.Fatalf("expected source-local pinned capture, got %#v", result)
	}
	if result.Width != 60 || result.Height != 30 {
		t.Fatalf("expected source-local selection size, got %dx%d", result.Width, result.Height)
	}
	img := readPNG(t, result.ImagePath)
	if !sameRGBA(img.At(0, 0), color.RGBA{R: 100, G: 40, B: 120, A: 255}) {
		t.Fatalf("expected local source pixel 100,40, got %#v", img.At(0, 0))
	}
	if pins.id != result.CaptureID || pins.x != 335 || pins.y != 175 {
		t.Fatalf("expected explicit DIP pin position, got id=%q x=%d y=%d", pins.id, pins.x, pins.y)
	}
}

func TestCaptureSelectionUsesVisualCoordinateSpaceForDisplayedSelection(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	service := NewService(captures, nil)
	session := service.stageTestSessionWithNative(
		t,
		capturehistory.ScreenBounds{X: 300, Y: 150, Width: 100, Height: 80},
		capturehistory.ScreenBounds{X: 2000, Y: 900, Width: 200, Height: 160},
	)

	result := service.CaptureSelection(SelectionRequest{
		SessionID:       session.ID,
		CoordinateSpace: "visual",
		X:               50,
		Y:               40,
		Width:           20,
		Height:          10,
		DisplayWidth:    100,
		DisplayHeight:   80,
		Action:          "capture",
	})

	if !result.OK || result.CaptureID == "" || result.Width != 40 || result.Height != 20 {
		t.Fatalf("expected visual selection to scale to source pixels, got %#v", result)
	}
	img := readPNG(t, result.ImagePath)
	if !sameRGBA(img.At(0, 0), color.RGBA{R: 100, G: 80, B: 120, A: 255}) {
		t.Fatalf("expected displayed point 50,40 to map to source pixel 100,80, got %#v", img.At(0, 0))
	}
}

func TestCaptureSelectionVisualCoordinateSpacePinsFromResolvedNativeSelection(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	pins := &positionedPinRecorder{}
	service := NewService(captures, pins)
	session := service.stageTestSessionWithNative(
		t,
		capturehistory.ScreenBounds{X: 300, Y: 150, Width: 100, Height: 80},
		capturehistory.ScreenBounds{X: 2000, Y: 900, Width: 200, Height: 160},
	)

	result := service.CaptureSelection(SelectionRequest{
		SessionID:       session.ID,
		CoordinateSpace: "visual",
		X:               50,
		Y:               40,
		Width:           20,
		Height:          10,
		DisplayWidth:    100,
		DisplayHeight:   80,
		Action:          "pin",
	})

	if !result.OK || result.CaptureID == "" || result.Pin == nil || !result.Pin.OK {
		t.Fatalf("expected visual positioned pin, got %#v", result)
	}
	if result.Width != 40 || result.Height != 20 {
		t.Fatalf("expected visual selection to scale to source pixels, got %dx%d", result.Width, result.Height)
	}
	if pins.id != result.CaptureID || pins.x != 2100 || pins.y != 980 {
		t.Fatalf("expected pin at resolved native selection origin, got id=%q x=%d y=%d", pins.id, pins.x, pins.y)
	}
}

func TestCaptureSelectionVisualCoordinateSpaceUsesActualSurfaceSize(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	service := NewService(captures, nil)
	session := service.stageTestSessionWithNative(
		t,
		capturehistory.ScreenBounds{X: 300, Y: 150, Width: 90, Height: 70},
		capturehistory.ScreenBounds{X: 2000, Y: 900, Width: 200, Height: 160},
	)

	result := service.CaptureSelection(SelectionRequest{
		SessionID:       session.ID,
		CoordinateSpace: "visual",
		X:               50,
		Y:               40,
		Width:           20,
		Height:          10,
		DisplayWidth:    100,
		DisplayHeight:   80,
		Action:          "capture",
	})

	if !result.OK || result.CaptureID == "" || result.Width != 40 || result.Height != 20 {
		t.Fatalf("expected actual surface size to drive visual scaling, got %#v", result)
	}
	img := readPNG(t, result.ImagePath)
	if !sameRGBA(img.At(0, 0), color.RGBA{R: 100, G: 80, B: 120, A: 255}) {
		t.Fatalf("expected explicit surface size to override session bounds, got %#v", img.At(0, 0))
	}
}

func TestOverlaySessionsSplitAndCropPerDisplay(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	service := NewService(captures, nil)
	virtual := capturehistory.ScreenBounds{X: -100, Y: 0, Width: 300, Height: 80}
	raw := testOverlayPNG(t, virtual.Width, virtual.Height)
	displays := []capturehistory.ScreenBounds{
		{X: 0, Y: 0, Width: 200, Height: 80},
		{X: -100, Y: 0, Width: 100, Height: 80},
	}

	sessions, err := service.overlaySessionsForDisplayBounds(nil, raw, virtual, displayNativeBounds(virtual, displays), true)
	if err != nil {
		t.Fatalf("expected display sessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected two display sessions, got %d", len(sessions))
	}
	if sessions[0].Native.X != -100 || sessions[0].Native.Width != 100 || !sessions[0].restoreMain {
		t.Fatalf("expected sorted left display with restore flag, got %#v", sessions[0])
	}
	if sessions[1].Native.X != 0 || sessions[1].Native.Width != 200 || sessions[1].restoreMain {
		t.Fatalf("expected right display without restore flag, got %#v", sessions[1])
	}
	leftImage := decodePNGBytes(t, sessions[0].pngBytes)
	if leftImage.Bounds().Dx() != 100 || leftImage.Bounds().Dy() != 80 {
		t.Fatalf("expected left display crop size 100x80, got %v", leftImage.Bounds())
	}
	if !sameRGBA(leftImage.At(0, 0), color.RGBA{R: 0, G: 0, B: 120, A: 255}) {
		t.Fatalf("expected cropped display to start at source pixel 0,0, got %#v", leftImage.At(0, 0))
	}

	service.mu.Lock()
	for _, session := range sessions {
		service.sessions[session.ID] = session
	}
	service.mu.Unlock()

	result := service.CaptureSelection(SelectionRequest{
		SessionID: sessions[0].ID,
		X:         -90,
		Y:         10,
		Width:     5,
		Height:    4,
		Action:    "capture",
	})
	if !result.OK || result.CaptureID == "" || result.Width != 5 || result.Height != 4 {
		t.Fatalf("expected saved crop from left display, got %#v", result)
	}
	saved := readPNG(t, result.ImagePath)
	if !sameRGBA(saved.At(0, 0), color.RGBA{R: 10, G: 10, B: 120, A: 255}) {
		t.Fatalf("expected global -90,10 to map to source pixel 10,10, got %#v", saved.At(0, 0))
	}
	if service.GetSession(sessions[1].ID).ID != "" {
		t.Fatal("finishing one overlay session should clear sibling display sessions")
	}
}

func TestOverlaySessionsCaptureEachDisplayDirectly(t *testing.T) {
	service := NewService(nil, nil)
	displays := []capturehistory.ScreenBounds{
		{X: -100, Y: 0, Width: 100, Height: 80},
		{X: 0, Y: 0, Width: 200, Height: 80},
	}
	calls := []capturehistory.ScreenBounds{}
	originalCapture := captureOverlayRegionPNG
	captureOverlayRegionPNG = func(x int, y int, width int, height int) ([]byte, int, int, error) {
		calls = append(calls, capturehistory.ScreenBounds{X: x, Y: y, Width: width, Height: height})
		return testOverlayPNG(t, width, height), width, height, nil
	}
	defer func() {
		captureOverlayRegionPNG = originalCapture
	}()

	sessions, err := service.overlaySessionsForCapturedDisplays(nil, displays, true)
	if err != nil {
		t.Fatalf("expected captured display sessions: %v", err)
	}
	if len(sessions) != 2 || len(calls) != 2 {
		t.Fatalf("expected two display captures, sessions=%d calls=%d", len(sessions), len(calls))
	}
	for index := range displays {
		if calls[index] != displays[index] {
			t.Fatalf("expected direct display capture %d to use %#v, got %#v", index, displays[index], calls[index])
		}
		if sessions[index].Native != displays[index] || sessions[index].Bounds != displays[index] {
			t.Fatalf("expected session bounds to match display %d, got %#v", index, sessions[index])
		}
		if !strings.HasPrefix(sessions[index].ImageURL, "/capture-overlay-image/") {
			t.Fatalf("expected lightweight asset URL, got %q", sessions[index].ImageURL)
		}
		img := decodePNGBytes(t, sessions[index].pngBytes)
		if img.Bounds().Dx() != displays[index].Width || img.Bounds().Dy() != displays[index].Height {
			t.Fatalf("expected captured display image size %dx%d, got %v", displays[index].Width, displays[index].Height, img.Bounds())
		}
	}
	if !sessions[0].restoreMain || sessions[1].restoreMain {
		t.Fatalf("expected only the first display session to restore main, got %#v %#v", sessions[0], sessions[1])
	}
}

func TestOpenGuardRejectsConcurrentOpen(t *testing.T) {
	service := NewService(nil, nil)

	if !service.tryBeginOpen() {
		t.Fatal("first open should acquire the guard")
	}
	if service.tryBeginOpen() {
		t.Fatal("concurrent open should not acquire the guard")
	}
	service.finishOpen()
	if !service.tryBeginOpen() {
		t.Fatal("guard should be reusable after finishing open")
	}
	service.finishOpen()
}

func TestOpenNativeUsesOpenGuard(t *testing.T) {
	service := NewServiceWithNative(nil, nil, nativecapture.NewManager(nativecapture.Options{
		ExePath: filepath.Join(t.TempDir(), "missing-native-host.exe"),
	}))

	if !service.tryBeginOpen() {
		t.Fatal("test should acquire the open guard")
	}
	defer service.finishOpen()

	result := service.Open()
	if !result.OK || result.Message != "截图覆盖层正在打开" {
		t.Fatalf("native open should reuse open guard before starting host, got %#v", result)
	}
}

func TestCaptureNativeSelectionRedactsBeforeSaveAs(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	ocrProvider := &fakeOCRProvider{
		result: ocr.Result{
			OK: true,
			Lines: []ocr.Line{
				{Text: "联系电话 13800138000", Rect: ocr.Rect{X: 2, Y: 2, Width: 5, Height: 4}},
			},
		},
	}
	service := NewService(captures, nil, ocrProvider)
	savePath := filepath.Join(dir, "exports", "native-redacted.png")

	result := service.captureNativeSelection(nativecapture.Response{
		OK:        true,
		Action:    "save_as",
		PNGBase64: base64.StdEncoding.EncodeToString(testOverlayPNG(t, 12, 8)),
		Width:     12,
		Height:    8,
		SavedPath: savePath,
	}, ScreenshotPolicy{AutoRedact: true, RedactPhones: true})

	if !result.OK || result.CaptureID == "" || !strings.Contains(result.Message, "已打码") {
		t.Fatalf("expected redacted native save_as capture, got %#v", result)
	}
	for _, path := range []string{result.ImagePath, result.SavedPath} {
		img := readPNG(t, path)
		if !sameRGBA(img.At(3, 3), color.RGBA{R: 24, G: 26, B: 30, A: 255}) {
			t.Fatalf("expected redacted pixel in %s, got %#v", path, img.At(3, 3))
		}
	}
	entry := captures.Entry(result.CaptureID)
	if !containsString(entry.Actions, "redacted") || !containsString(entry.Actions, "save_as") || !containsString(entry.Actions, "save") {
		t.Fatalf("expected redacted save_as metadata, got %#v", entry.Actions)
	}
}

func TestCaptureNativeSelectionRedactCopyRedactsBeforeClipboard(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	ocrProvider := &fakeOCRProvider{
		result: ocr.Result{
			OK: true,
			Lines: []ocr.Line{
				{Text: "TOKEN=abcdef", Rect: ocr.Rect{X: 4, Y: 1, Width: 4, Height: 3}},
			},
		},
	}
	service := NewService(captures, nil, ocrProvider)
	copiedPNG := stubPNGClipboardWriter(t)

	result := service.captureNativeSelection(nativecapture.Response{
		OK:        true,
		Action:    "redact_copy",
		PNGBase64: base64.StdEncoding.EncodeToString(testOverlayPNG(t, 12, 8)),
		Width:     12,
		Height:    8,
	}, ScreenshotPolicy{RedactKeywords: []string{"token"}})

	if !result.OK || result.CaptureID == "" || len(*copiedPNG) == 0 {
		t.Fatalf("expected redacted native copy, got result=%#v copied_png=%d", result, len(*copiedPNG))
	}
	img := readPNG(t, result.ImagePath)
	if !sameRGBA(img.At(5, 2), color.RGBA{R: 24, G: 26, B: 30, A: 255}) {
		t.Fatalf("expected copied image to be redacted, got %#v", img.At(5, 2))
	}
}

func TestCaptureNativeSelectionTrustsNativeClipboardAndPersistsHistoryAsync(t *testing.T) {
	captures := &blockingCaptureSink{called: make(chan struct{}), release: make(chan struct{})}
	service := NewService(captures, nil)
	previousWriter := writeImageToClipboard
	copiedPNG := stubPNGClipboardWriter(t)
	imageWriterCalled := false
	writeImageToClipboard = func(path string) error {
		imageWriterCalled = true
		return nil
	}
	t.Cleanup(func() {
		writeImageToClipboard = previousWriter
	})

	result := service.captureNativeSelection(nativecapture.Response{
		OK:               true,
		Action:           "copy",
		PNGBase64:        base64.StdEncoding.EncodeToString(testOverlayPNG(t, 12, 8)),
		Width:            12,
		Height:           8,
		ClipboardWritten: true,
	}, ScreenshotPolicy{})

	if !result.OK || result.CaptureID != "" || !strings.Contains(result.Message, "已复制截图") {
		t.Fatalf("expected native clipboard copy result, got %#v", result)
	}
	if len(*copiedPNG) != 0 {
		t.Fatal("Go clipboard writer must not run after native host reports clipboardWritten")
	}
	if imageWriterCalled {
		t.Fatal("path-based clipboard writer must not run after native host reports clipboardWritten")
	}
	select {
	case <-captures.called:
	case <-time.After(time.Second):
		t.Fatal("expected native copy history to be persisted asynchronously")
	}
	close(captures.release)
}

func TestNativeCaptureRequestUsesNativeClipboardForPlainCopy(t *testing.T) {
	request := nativeCaptureRequest(ScreenshotPolicy{})
	if !request.DirectClipboardCopy {
		t.Fatal("plain native capture should write the clipboard in the native host")
	}
}

func TestNativeCaptureRequestKeepsPlainCopyInNativeHostWhenAutoRedactIsConfigured(t *testing.T) {
	request := nativeCaptureRequest(ScreenshotPolicy{AutoRedact: true, RedactKeywords: []string{"token"}})
	if !request.DirectClipboardCopy {
		t.Fatal("plain native copy should stay in the native host even when auto redaction is configured")
	}
}

func TestCaptureNativeSelectionTrustsNativeClipboardWhenAutoRedactingPlainCopy(t *testing.T) {
	captures := &blockingCaptureSink{called: make(chan struct{}), release: make(chan struct{})}
	ocrProvider := &fakeOCRProvider{
		result: ocr.Result{
			OK: true,
			Lines: []ocr.Line{
				{Text: "数字国联整体架构图", Rect: ocr.Rect{X: 0, Y: 0, Width: 90, Height: 10}},
			},
		},
	}
	service := NewService(captures, nil, ocrProvider)
	copiedPNG := stubPNGClipboardWriter(t)

	result := service.captureNativeSelection(nativecapture.Response{
		OK:               true,
		Action:           "copy",
		PNGBase64:        base64.StdEncoding.EncodeToString(testOverlayPNG(t, 90, 12)),
		Width:            90,
		Height:           12,
		ClipboardWritten: true,
	}, ScreenshotPolicy{AutoRedact: true, RedactKeywords: []string{"国联"}})

	if !result.OK || result.CaptureID != "" || len(*copiedPNG) != 0 {
		t.Fatalf("expected native clipboard result without Go copy, got result=%#v copied_png=%d", result, len(*copiedPNG))
	}
	if ocrProvider.pathCount() != 0 {
		t.Fatalf("plain native copy should not run OCR, got %d calls", ocrProvider.pathCount())
	}
	select {
	case <-captures.called:
	case <-time.After(time.Second):
		t.Fatal("expected native copy history to be persisted asynchronously")
	}
	close(captures.release)
}

func TestCaptureNativeSelectionWritesClipboardBeforeHistory(t *testing.T) {
	captures := &blockingCaptureSink{called: make(chan struct{}), release: make(chan struct{})}
	service := NewService(captures, nil)
	copiedPNG := stubPNGClipboardWriter(t)

	done := make(chan CaptureResult, 1)
	go func() {
		done <- service.captureNativeSelection(nativecapture.Response{
			OK:        true,
			Action:    "copy",
			PNGBase64: base64.StdEncoding.EncodeToString(testOverlayPNG(t, 12, 8)),
			Width:     12,
			Height:    8,
		}, ScreenshotPolicy{})
	}()

	waitForCondition(t, time.Second, func() bool {
		return len(*copiedPNG) > 0
	})
	select {
	case <-captures.called:
	case <-time.After(time.Second):
		t.Fatal("expected history write to start after clipboard write")
	}
	select {
	case result := <-done:
		t.Fatalf("copy result should wait on blocked history in this test, got %#v", result)
	default:
	}
	close(captures.release)
	result := <-done
	if !result.OK {
		t.Fatalf("expected copy to finish after history release, got %#v", result)
	}
}

func TestCaptureNativeSelectionDecodesQRFromHistoryEntry(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	service := NewService(captures, nil)
	copiedText := stubTextClipboardWriter(t)
	text := "https://ariadne.local/native-qr"
	data, err := goqrcode.Encode(text, goqrcode.Medium, 192)
	if err != nil {
		t.Fatal(err)
	}

	result := service.captureNativeSelection(nativecapture.Response{
		OK:        true,
		Action:    "qr",
		PNGBase64: base64.StdEncoding.EncodeToString(data),
	}, ScreenshotPolicy{})

	if !result.OK || result.QR == nil || !result.QR.OK || result.QR.Text != text {
		t.Fatalf("expected native QR decode result, got %#v", result)
	}
	if *copiedText != text || result.Message != "二维码已复制" {
		t.Fatalf("expected native QR text copied, got copied=%q message=%q", *copiedText, result.Message)
	}
	if result.QR.CaptureID != result.CaptureID || result.QR.Source != "capture_overlay" {
		t.Fatalf("expected QR evidence to reference history entry, got %#v", result.QR)
	}
	entry := captures.Entry(result.CaptureID)
	if !containsString(entry.Actions, "qr") {
		t.Fatalf("expected qr action metadata, got %#v", entry.Actions)
	}
}

func TestCaptureNativeSelectionPinsThroughPinnedService(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	pins := &positionedPinRecorder{}
	service := NewService(captures, pins)

	result := service.captureNativeSelection(nativecapture.Response{
		OK:        true,
		Action:    "pin",
		PNGBase64: base64.StdEncoding.EncodeToString(testOverlayPNG(t, 10, 6)),
		X:         50,
		Y:         70,
		Width:     10,
		Height:    6,
	}, ScreenshotPolicy{})

	if !result.OK || result.Pin == nil || !result.Pin.OK || result.Pin.PinID == "native" {
		t.Fatalf("expected manageable native pin, got %#v", result)
	}
	if pins.id != result.CaptureID || pins.x != 50 || pins.y != 70 {
		t.Fatalf("expected pinned service call at native selection origin, got id=%q x=%d y=%d", pins.id, pins.x, pins.y)
	}
}

func TestCaptureNativeSelectionDoesNotReopenWailsWhenNativeResponseReportsPinned(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	pins := &positionedPinRecorder{}
	service := NewService(captures, pins)

	result := service.captureNativeSelection(nativecapture.Response{
		OK:        true,
		Action:    "pin",
		PNGBase64: base64.StdEncoding.EncodeToString(testOverlayPNG(t, 10, 6)),
		X:         50,
		Y:         70,
		Width:     10,
		Height:    6,
		Pinned:    true,
	}, ScreenshotPolicy{})

	if !result.OK || result.CaptureID != "" || result.Pin == nil || !result.Pin.OK || !strings.HasPrefix(result.Pin.PinID, "native-") {
		t.Fatalf("expected native pinned image result, got %#v", result)
	}
	if pins.calls != 0 {
		t.Fatalf("native pinned response must not reopen Wails pinned service, got calls=%d", pins.calls)
	}
	waitCaptureHistoryCount(t, captures, 1)
}

func TestCaptureNativeSelectionReturnsBeforeHistoryWriteForNativePin(t *testing.T) {
	captures := &blockingCaptureSink{called: make(chan struct{}), release: make(chan struct{})}
	service := NewService(captures, &positionedPinRecorder{})
	defer close(captures.release)

	done := make(chan CaptureResult, 1)
	go func() {
		done <- service.captureNativeSelection(nativecapture.Response{
			OK:          true,
			Action:      "pin",
			PNGBase64:   base64.StdEncoding.EncodeToString(testOverlayPNG(t, 10, 6)),
			Width:       10,
			Height:      6,
			Pinned:      true,
			NativePinID: "native-test",
		}, ScreenshotPolicy{})
	}()

	select {
	case result := <-done:
		if !result.OK || result.CaptureID != "" || result.Pin == nil || result.Pin.PinID != "native-native-test" {
			t.Fatalf("expected immediate native pin result without capture id, got %#v", result)
		}
	case <-time.After(150 * time.Millisecond):
		t.Fatal("native pin result waited for screenshot history write")
	}

	select {
	case <-captures.called:
	case <-time.After(time.Second):
		t.Fatal("expected screenshot history write to run in background")
	}
}

func TestCaptureNativeSelectionOcrDoesNotRequirePNGOrHistory(t *testing.T) {
	service := NewService(panicCaptureSink{}, &positionedPinRecorder{})

	result := service.captureNativeSelection(nativecapture.Response{
		OK:      true,
		Action:  "ocr",
		Message: "OCR 文本已复制",
	}, ScreenshotPolicy{AutoCopy: true, AutoSave: true, AutoPin: true})

	if !result.OK || result.Message != "OCR 文本已复制" {
		t.Fatalf("expected OCR text result, got %#v", result)
	}
	if result.CaptureID != "" || result.ImagePath != "" || result.Pin != nil {
		t.Fatalf("OCR text action should not create image artifacts, got %#v", result)
	}
}

func TestOverlaySessionsReuseVirtualPNGForSingleDisplay(t *testing.T) {
	service := NewService(nil, nil)
	virtual := capturehistory.ScreenBounds{X: 0, Y: 0, Width: 1280, Height: 720}
	raw := []byte("already-encoded-png")

	sessions, err := service.overlaySessionsForDisplayBounds(nil, raw, virtual, []capturehistory.ScreenBounds{virtual}, false)
	if err != nil {
		t.Fatalf("expected single-display session to reuse existing PNG: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected one display session, got %d", len(sessions))
	}
	if !bytes.Equal(sessions[0].pngBytes, raw) {
		t.Fatal("expected single-display session to reuse the source PNG bytes")
	}
	if strings.HasPrefix(sessions[0].ImageURL, "data:image/") || strings.Contains(sessions[0].ImageURL, base64.StdEncoding.EncodeToString(raw)) {
		t.Fatalf("expected lightweight asset URL, got embedded image URL %q", sessions[0].ImageURL)
	}
	if !strings.HasPrefix(sessions[0].ImageURL, "/capture-overlay-image/") {
		t.Fatalf("expected capture overlay asset URL, got %q", sessions[0].ImageURL)
	}

	service.mu.Lock()
	service.sessions[sessions[0].ID] = sessions[0]
	service.mu.Unlock()

	handler := CaptureOverlayAssetHandler(service, http.NotFoundHandler())
	request := httptest.NewRequest(http.MethodGet, sessions[0].ImageURL, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected overlay image asset response, got status %d", response.Code)
	}
	if !bytes.Equal(response.Body.Bytes(), raw) {
		t.Fatal("expected overlay image asset response to serve the session PNG bytes")
	}
}

func BenchmarkOverlaySessionsForSingleDisplay(b *testing.B) {
	captures := capturehistory.NewServiceWithPaths(filepath.Join(b.TempDir(), "capture_history.json"), filepath.Join(b.TempDir(), "capture_images"))
	service := NewService(captures, nil)
	virtual := capturehistory.ScreenBounds{X: 0, Y: 0, Width: 1280, Height: 720}
	raw := testOverlayPNG(b, virtual.Width, virtual.Height)
	displays := []capturehistory.ScreenBounds{virtual}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sessions, err := service.overlaySessionsForDisplayBounds(nil, raw, virtual, displays, false)
		if err != nil {
			b.Fatalf("expected display sessions: %v", err)
		}
		if len(sessions) != 1 || len(sessions[0].pngBytes) == 0 {
			b.Fatalf("expected one populated session, got %#v", sessions)
		}
	}
}

func TestDisplayNativeBoundsFallsBackToVirtualScreen(t *testing.T) {
	virtual := capturehistory.ScreenBounds{X: -50, Y: 20, Width: 120, Height: 90}
	displays := displayNativeBounds(virtual, nil)
	if len(displays) != 1 || displays[0] != virtual {
		t.Fatalf("expected virtual fallback, got %#v", displays)
	}
}

func TestCaptureWindowShouldNotHideAriadneWindowsBeforeScreenshot(t *testing.T) {
	cases := []struct {
		name string
		hide bool
	}{
		{name: "main", hide: false},
		{name: "pinned-image-capture-1", hide: false},
		{name: "tool-hosts", hide: false},
		{name: "capture-overlay-active", hide: false},
		{name: "", hide: false},
	}
	for _, test := range cases {
		if got := captureWindowShouldHide(test.name); got != test.hide {
			t.Fatalf("captureWindowShouldHide(%q) = %v, want %v", test.name, got, test.hide)
		}
	}
}

func TestCaptureSelectionRejectsTinyRegion(t *testing.T) {
	captures := capturehistory.NewServiceWithPaths(filepath.Join(t.TempDir(), "capture_history.json"), filepath.Join(t.TempDir(), "capture_images"))
	service := NewService(captures, nil)
	session := service.stageTestSession(t, capturehistory.ScreenBounds{X: 0, Y: 0, Width: 4, Height: 3})

	result := service.CaptureSelection(SelectionRequest{SessionID: session.ID, X: 1, Y: 1, Width: 1, Height: 1})
	if result.OK || result.Message != "截图区域太小" {
		t.Fatalf("expected tiny region failure, got %#v", result)
	}
	if service.GetSession(session.ID).ID == "" {
		t.Fatal("tiny selection should keep the overlay session alive")
	}
}

func TestCaptureSelectionReportsPinFailureAfterSaving(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	service := NewService(captures, failingPinService{})
	session := service.stageTestSession(t, capturehistory.ScreenBounds{X: 0, Y: 0, Width: 4, Height: 3})

	result := service.CaptureSelection(SelectionRequest{SessionID: session.ID, X: 0, Y: 0, Width: 3, Height: 2, Action: "pin"})
	if result.OK || result.CaptureID == "" || result.Pin == nil || result.Pin.OK {
		t.Fatalf("expected saved capture plus pin failure, got %#v", result)
	}
	if _, err := os.Stat(result.ImagePath); err != nil {
		t.Fatalf("expected saved image despite pin failure: %v", err)
	}
}

func TestCaptureSelectionPinsAtSelectionOrigin(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	pins := &positionedPinRecorder{}
	service := NewService(captures, pins)
	session := service.stageTestSession(t, capturehistory.ScreenBounds{X: 100, Y: 200, Width: 12, Height: 10})

	result := service.CaptureSelection(SelectionRequest{SessionID: session.ID, X: 104, Y: 206, Width: 5, Height: 3, Action: "pin"})
	if !result.OK || result.CaptureID == "" || result.Pin == nil || !result.Pin.OK {
		t.Fatalf("expected pinned capture, got %#v", result)
	}
	if pins.id != result.CaptureID || pins.x != 104 || pins.y != 206 {
		t.Fatalf("expected pin at selection origin, got id=%q x=%d y=%d", pins.id, pins.x, pins.y)
	}
}

func TestCaptureSelectionUsesExplicitPinPositionForSessionCoordinates(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	pins := &positionedPinRecorder{}
	service := NewService(captures, pins)
	session := service.stageTestSessionWithNative(
		t,
		capturehistory.ScreenBounds{X: 100, Y: 200, Width: 120, Height: 90},
		capturehistory.ScreenBounds{X: 2000, Y: 900, Width: 240, Height: 180},
	)

	result := service.CaptureSelection(SelectionRequest{
		SessionID:       session.ID,
		CoordinateSpace: "session",
		X:               20,
		Y:               30,
		Width:           12,
		Height:          8,
		Action:          "pin",
		PinPositioned:   true,
		PinX:            105,
		PinY:            215,
	})

	if !result.OK || result.CaptureID == "" || result.Pin == nil || !result.Pin.OK {
		t.Fatalf("expected positioned pin, got %#v", result)
	}
	if pins.id != result.CaptureID || pins.x != 105 || pins.y != 215 {
		t.Fatalf("expected explicit pin position, got id=%q x=%d y=%d", pins.id, pins.x, pins.y)
	}
}

func TestCaptureSelectionCopiesToClipboardAndSavesHistory(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	service := NewService(captures, nil)
	session := service.stageTestSession(t, capturehistory.ScreenBounds{X: 0, Y: 0, Width: 4, Height: 3})

	copiedPNG := stubPNGClipboardWriter(t)
	result := service.CaptureSelection(SelectionRequest{SessionID: session.ID, X: 0, Y: 0, Width: 3, Height: 2, Action: "copy"})
	if !result.OK || result.CaptureID == "" {
		t.Fatalf("expected copied capture, got %#v", result)
	}
	if len(*copiedPNG) == 0 {
		t.Fatal("expected copied image bytes")
	}
	entry := captures.Entry(result.CaptureID)
	if !containsString(entry.Actions, "copy") {
		t.Fatalf("expected copy metadata, got %#v", entry.Actions)
	}
}

func TestCaptureSelectionAppliesAnnotationOperations(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	service := NewService(captures, nil)
	session := service.stageTestSession(t, capturehistory.ScreenBounds{X: 0, Y: 0, Width: 8, Height: 8})

	result := service.CaptureSelection(SelectionRequest{
		SessionID: session.ID,
		X:         0,
		Y:         0,
		Width:     8,
		Height:    8,
		Operations: []AnnotationOperation{
			{Kind: "rect", X: 1, Y: 1, Width: 5, Height: 5, Color: "#dc2626", StrokeWidth: 2},
		},
	})

	if !result.OK || result.CaptureID == "" {
		t.Fatalf("expected annotated capture, got %#v", result)
	}
	img := readPNG(t, result.ImagePath)
	if !sameRGBA(img.At(1, 1), color.RGBA{R: 220, G: 38, B: 38, A: 255}) {
		t.Fatalf("expected red annotation pixel, got %#v", img.At(1, 1))
	}
	entry := captures.Entry(result.CaptureID)
	if !containsString(entry.Actions, "annotated") {
		t.Fatalf("expected annotated action, got %#v", entry.Actions)
	}
}

func TestCaptureSelectionUsesRenderedImageWhenProvided(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	service := NewService(captures, nil)
	session := service.stageTestSession(t, capturehistory.ScreenBounds{X: 0, Y: 0, Width: 8, Height: 8})
	rendered := image.NewRGBA(image.Rect(0, 0, 4, 4))
	drawSolid(rendered, color.RGBA{R: 1, G: 2, B: 3, A: 255})
	rendered.SetRGBA(0, 0, color.RGBA{R: 9, G: 8, B: 7, A: 255})

	result := service.CaptureSelection(SelectionRequest{
		SessionID:     session.ID,
		X:             0,
		Y:             0,
		Width:         4,
		Height:        4,
		RenderedImage: encodePNGBase64(t, rendered),
		Operations: []AnnotationOperation{
			{Kind: "text", X: 1, Y: 1, Text: "hello", Color: "#dc2626", FontSize: 18},
		},
	})

	if !result.OK || result.CaptureID == "" {
		t.Fatalf("expected rendered capture, got %#v", result)
	}
	img := readPNG(t, result.ImagePath)
	if !sameRGBA(img.At(0, 0), color.RGBA{R: 9, G: 8, B: 7, A: 255}) {
		t.Fatalf("expected rendered PNG pixel, got %#v", img.At(0, 0))
	}
	entry := captures.Entry(result.CaptureID)
	if !containsString(entry.Actions, "text") {
		t.Fatalf("expected text metadata, got %#v", entry.Actions)
	}
}

func TestCaptureSelectionUsesAutoSavePolicy(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	service := NewService(captures, nil)
	service.ApplyScreenshotPolicy(ScreenshotPolicy{
		AutoSave:         true,
		SaveDir:          filepath.Join(dir, "auto"),
		FilenameTemplate: "shot_{datetime}",
	})
	session := service.stageTestSession(t, capturehistory.ScreenBounds{X: 0, Y: 0, Width: 8, Height: 8})

	result := service.CaptureSelection(SelectionRequest{SessionID: session.ID, X: 0, Y: 0, Width: 5, Height: 4, Action: "capture"})
	if !result.OK || result.SavedPath == "" {
		t.Fatalf("expected auto-saved capture, got %#v", result)
	}
	if _, err := os.Stat(result.SavedPath); err != nil {
		t.Fatalf("expected auto-saved PNG: %v", err)
	}
	entry := captures.Entry(result.CaptureID)
	if entry.SavedPath != result.SavedPath || !containsString(entry.Actions, "save") {
		t.Fatalf("expected auto-save metadata, got %#v", entry)
	}
}

func drawSolid(img *image.RGBA, col color.RGBA) {
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			img.SetRGBA(x, y, col)
		}
	}
}

func encodePNGBase64(t *testing.T, img image.Image) string {
	t.Helper()
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(out.Bytes())
}

func TestCaptureSelectionMosaicAndSaveAs(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	service := NewService(captures, nil)
	session := service.stageTestSession(t, capturehistory.ScreenBounds{X: 0, Y: 0, Width: 8, Height: 8})
	savePath := filepath.Join(dir, "exports", "annotated-selection")

	result := service.CaptureSelection(SelectionRequest{
		SessionID: session.ID,
		X:         0,
		Y:         0,
		Width:     8,
		Height:    8,
		Action:    "save_as",
		SavedPath: savePath,
		Operations: []AnnotationOperation{
			{Kind: "mosaic", X: 0, Y: 0, Width: 4, Height: 4, PixelSize: 4},
		},
	})

	expectedSavePath := savePath + ".png"
	if !result.OK || result.SavedPath != expectedSavePath {
		t.Fatalf("expected saved-as annotated capture, got %#v want path %q", result, expectedSavePath)
	}
	if _, err := os.Stat(result.SavedPath); err != nil {
		t.Fatalf("expected external saved PNG: %v", err)
	}
	entry := captures.Entry(result.CaptureID)
	if entry.SavedPath != result.SavedPath || !containsString(entry.Actions, "mosaic") || !containsString(entry.Actions, "save_as") {
		t.Fatalf("expected save_as and mosaic metadata, got entry %#v", entry)
	}
}

func TestCaptureSelectionRedactsPhoneMatchesBeforeSaving(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	ocrProvider := &fakeOCRProvider{
		result: ocr.Result{
			OK: true,
			Lines: []ocr.Line{
				{Text: "联系电话 13800138000", Rect: ocr.Rect{X: 2, Y: 2, Width: 5, Height: 4}},
			},
		},
	}
	service := NewService(captures, nil, ocrProvider)
	service.ApplyScreenshotPolicy(ScreenshotPolicy{AutoRedact: true, RedactPhones: true})
	session := service.stageTestSession(t, capturehistory.ScreenBounds{X: 0, Y: 0, Width: 12, Height: 8})

	result := service.CaptureSelection(SelectionRequest{
		SessionID: session.ID,
		X:         0,
		Y:         0,
		Width:     12,
		Height:    8,
		Action:    "capture",
	})

	if !result.OK || !strings.Contains(result.Message, "已打码") {
		t.Fatalf("expected redacted capture, got %#v", result)
	}
	img := readPNG(t, result.ImagePath)
	if !sameRGBA(img.At(3, 3), color.RGBA{R: 24, G: 26, B: 30, A: 255}) {
		t.Fatalf("expected redacted pixel, got %#v", img.At(3, 3))
	}
	entry := captures.Entry(result.CaptureID)
	if !containsString(entry.Actions, "redacted") {
		t.Fatalf("expected redacted metadata, got %#v", entry.Actions)
	}
}

func TestCaptureSelectionRedactsKeywordMatches(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	ocrProvider := &fakeOCRProvider{
		result: ocr.Result{
			OK: true,
			Lines: []ocr.Line{
				{Text: "TOKEN=abcdef", Rect: ocr.Rect{X: 6, Y: 1, Width: 4, Height: 3}},
			},
		},
	}
	service := NewService(captures, nil, ocrProvider)
	service.ApplyScreenshotPolicy(ScreenshotPolicy{AutoRedact: true, RedactKeywords: []string{"token"}})
	session := service.stageTestSession(t, capturehistory.ScreenBounds{X: 0, Y: 0, Width: 12, Height: 8})

	result := service.CaptureSelection(SelectionRequest{
		SessionID: session.ID,
		X:         0,
		Y:         0,
		Width:     12,
		Height:    8,
		Action:    "capture",
	})

	if !result.OK {
		t.Fatalf("expected keyword redaction capture, got %#v", result)
	}
	img := readPNG(t, result.ImagePath)
	if !sameRGBA(img.At(7, 2), color.RGBA{R: 24, G: 26, B: 30, A: 255}) {
		t.Fatalf("expected keyword redacted pixel, got %#v", img.At(7, 2))
	}
}

func TestCaptureSelectionRedactsOnlyKeywordSegment(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	ocrProvider := &fakeOCRProvider{
		result: ocr.Result{
			OK: true,
			Lines: []ocr.Line{
				{Text: "数字国联整体架构图", Rect: ocr.Rect{X: 0, Y: 0, Width: 90, Height: 10}},
			},
		},
	}
	service := NewService(captures, nil, ocrProvider)
	service.ApplyScreenshotPolicy(ScreenshotPolicy{AutoRedact: false, RedactKeywords: []string{"国联"}})
	session := service.stageTestSession(t, capturehistory.ScreenBounds{X: 0, Y: 0, Width: 90, Height: 12})
	stubPNGClipboardWriter(t)

	result := service.CaptureSelection(SelectionRequest{
		SessionID: session.ID,
		X:         0,
		Y:         0,
		Width:     90,
		Height:    12,
		Action:    "redact_copy",
	})

	if !result.OK {
		t.Fatalf("expected keyword redaction capture, got %#v", result)
	}
	img := readPNG(t, result.ImagePath)
	if sameRGBA(img.At(8, 5), color.RGBA{R: 24, G: 26, B: 30, A: 255}) {
		t.Fatalf("non-matching text should not be redacted, got %#v", img.At(8, 5))
	}
	if !sameRGBA(img.At(35, 5), color.RGBA{R: 24, G: 26, B: 30, A: 255}) {
		t.Fatalf("keyword segment should be redacted, got %#v", img.At(35, 5))
	}
	if sameRGBA(img.At(75, 5), color.RGBA{R: 24, G: 26, B: 30, A: 255}) {
		t.Fatalf("text after keyword should not be redacted, got %#v", img.At(75, 5))
	}
}

func TestCaptureSelectionRedactsKeywordSplitAcrossAdjacentOCRLines(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	ocrProvider := &fakeOCRProvider{
		result: ocr.Result{
			OK: true,
			Lines: []ocr.Line{
				{Text: "数字国", Rect: ocr.Rect{X: 0, Y: 0, Width: 60, Height: 10}},
				{Text: "联APP", Rect: ocr.Rect{X: 0, Y: 12, Width: 80, Height: 10}},
			},
		},
	}
	service := NewService(captures, nil, ocrProvider)
	service.ApplyScreenshotPolicy(ScreenshotPolicy{AutoRedact: false, RedactKeywords: []string{"国联"}})
	session := service.stageTestSession(t, capturehistory.ScreenBounds{X: 0, Y: 0, Width: 90, Height: 28})
	stubPNGClipboardWriter(t)

	result := service.CaptureSelection(SelectionRequest{
		SessionID: session.ID,
		X:         0,
		Y:         0,
		Width:     90,
		Height:    28,
		Action:    "redact_copy",
	})

	if !result.OK {
		t.Fatalf("expected split keyword redaction capture, got %#v", result)
	}
	img := readPNG(t, result.ImagePath)
	if sameRGBA(img.At(10, 5), color.RGBA{R: 24, G: 26, B: 30, A: 255}) {
		t.Fatalf("first line prefix should not be redacted, got %#v", img.At(10, 5))
	}
	if !sameRGBA(img.At(50, 5), color.RGBA{R: 24, G: 26, B: 30, A: 255}) {
		t.Fatalf("first line split keyword segment should be redacted, got %#v", img.At(50, 5))
	}
	if !sameRGBA(img.At(10, 17), color.RGBA{R: 24, G: 26, B: 30, A: 255}) {
		t.Fatalf("second line split keyword segment should be redacted, got %#v", img.At(10, 17))
	}
	if sameRGBA(img.At(50, 17), color.RGBA{R: 24, G: 26, B: 30, A: 255}) {
		t.Fatalf("second line suffix should not be redacted, got %#v", img.At(50, 17))
	}
}

func TestCaptureSelectionPlainCopyUsesAutoRedactionPolicy(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	ocrProvider := &fakeOCRProvider{
		result: ocr.Result{
			OK: true,
			Lines: []ocr.Line{
				{Text: "数字国联整体架构图", Rect: ocr.Rect{X: 0, Y: 0, Width: 90, Height: 10}},
			},
		},
	}
	service := NewService(captures, nil, ocrProvider)
	service.ApplyScreenshotPolicy(ScreenshotPolicy{AutoRedact: true, RedactKeywords: []string{"国联"}})
	session := service.stageTestSession(t, capturehistory.ScreenBounds{X: 0, Y: 0, Width: 90, Height: 12})
	stubPNGClipboardWriter(t)

	result := service.CaptureSelection(SelectionRequest{
		SessionID: session.ID,
		X:         0,
		Y:         0,
		Width:     90,
		Height:    12,
		Action:    "copy",
	})

	if !result.OK {
		t.Fatalf("expected auto-redacted plain copy capture, got %#v", result)
	}
	if ocrProvider.pathCount() != 1 {
		t.Fatalf("plain copy should call OCR for auto redaction, got %d OCR calls", ocrProvider.pathCount())
	}
	img := readPNG(t, result.ImagePath)
	if !sameRGBA(img.At(35, 5), color.RGBA{R: 24, G: 26, B: 30, A: 255}) {
		t.Fatalf("plain copy should save redacted pixels when auto redaction is enabled, got %#v", img.At(35, 5))
	}
	entry := captures.Entry(result.CaptureID)
	if !containsString(entry.Actions, "redacted") {
		t.Fatalf("plain copy should record redacted action, got %#v", entry.Actions)
	}
}

func TestCaptureSelectionPinUsesAutoRedactionPolicy(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	ocrProvider := &fakeOCRProvider{
		result: ocr.Result{
			OK: true,
			Lines: []ocr.Line{
				{Text: "数字国联整体架构图", Rect: ocr.Rect{X: 0, Y: 0, Width: 90, Height: 10}},
			},
		},
	}
	pins := &positionedPinRecorder{}
	service := NewService(captures, pins, ocrProvider)
	service.ApplyScreenshotPolicy(ScreenshotPolicy{AutoRedact: true, RedactKeywords: []string{"国联"}})
	session := service.stageTestSession(t, capturehistory.ScreenBounds{X: 0, Y: 0, Width: 90, Height: 12})

	result := service.CaptureSelection(SelectionRequest{
		SessionID: session.ID,
		X:         0,
		Y:         0,
		Width:     90,
		Height:    12,
		Action:    "pin",
	})

	if !result.OK || result.Pin == nil || !result.Pin.OK {
		t.Fatalf("expected pinned capture, got %#v", result)
	}
	if ocrProvider.pathCount() != 1 {
		t.Fatalf("pin should call OCR for auto redaction, got %d OCR calls", ocrProvider.pathCount())
	}
	img := readPNG(t, result.ImagePath)
	if !sameRGBA(img.At(35, 5), color.RGBA{R: 24, G: 26, B: 30, A: 255}) {
		t.Fatalf("pin should save redacted pixels when auto redaction is enabled, got %#v", img.At(35, 5))
	}
	entry := captures.Entry(result.CaptureID)
	if !containsString(entry.Actions, "redacted") {
		t.Fatalf("pin should record redacted action, got %#v", entry.Actions)
	}
}

func TestPrepareSelectionRedactionWarmsOCRForRedactCopy(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	ocrProvider := &fakeOCRProvider{
		result: ocr.Result{
			OK: true,
			Lines: []ocr.Line{
				{Text: "数字国联整体架构图", Rect: ocr.Rect{X: 0, Y: 0, Width: 90, Height: 10}},
			},
		},
	}
	service := NewService(captures, nil, ocrProvider)
	service.ApplyScreenshotPolicy(ScreenshotPolicy{AutoRedact: false, RedactKeywords: []string{"国联"}})
	session := service.stageTestSession(t, capturehistory.ScreenBounds{X: 0, Y: 0, Width: 90, Height: 12})
	stubPNGClipboardWriter(t)
	request := SelectionRequest{
		SessionID: session.ID,
		X:         0,
		Y:         0,
		Width:     90,
		Height:    12,
		Action:    "redact_copy",
	}

	prepared := service.PrepareSelectionRedaction(request)
	if !prepared.OK {
		t.Fatalf("expected redaction prepare to start, got %#v", prepared)
	}
	waitForCondition(t, time.Second, func() bool {
		return ocrProvider.pathCount() == 1
	})

	result := service.CaptureSelection(request)
	if !result.OK {
		t.Fatalf("expected redacted copy to use warmed OCR, got %#v", result)
	}
	if ocrProvider.pathCount() != 1 {
		t.Fatalf("redacted copy should reuse warmed OCR, got %d OCR calls", ocrProvider.pathCount())
	}
	img := readPNG(t, result.ImagePath)
	if !sameRGBA(img.At(35, 5), color.RGBA{R: 24, G: 26, B: 30, A: 255}) {
		t.Fatalf("keyword segment should be redacted from warmed OCR, got %#v", img.At(35, 5))
	}
}

func TestCaptureSelectionRedactionFailsWhenOCRHasNoGeometry(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	ocrProvider := &fakeOCRProvider{
		result: ocr.Result{
			OK:    true,
			Lines: []ocr.Line{{Text: "联系电话 13800138000"}},
		},
	}
	service := NewService(captures, nil, ocrProvider)
	service.ApplyScreenshotPolicy(ScreenshotPolicy{AutoRedact: true, RedactPhones: true})
	session := service.stageTestSession(t, capturehistory.ScreenBounds{X: 0, Y: 0, Width: 12, Height: 8})

	result := service.CaptureSelection(SelectionRequest{
		SessionID: session.ID,
		X:         0,
		Y:         0,
		Width:     12,
		Height:    8,
		Action:    "capture",
	})

	if result.OK || !strings.Contains(result.Message, "OCR 未返回可打码位置") {
		t.Fatalf("expected missing geometry error, got %#v", result)
	}
	if service.GetSession(session.ID).ID == "" {
		t.Fatal("failed redaction should keep the overlay session open")
	}
}

func (s *Service) stageTestSession(t *testing.T, bounds capturehistory.ScreenBounds) Session {
	return s.stageTestSessionWithNative(t, bounds, bounds)
}

func (s *Service) stageTestSessionWithNative(t *testing.T, bounds capturehistory.ScreenBounds, nativeBounds capturehistory.ScreenBounds) Session {
	t.Helper()
	raw := testOverlayPNG(t, nativeBounds.Width, nativeBounds.Height)
	session := overlaySession{
		Session: Session{
			ID:        "test-session",
			Bounds:    bounds,
			Native:    nativeBounds,
			ImageURL:  "data:image/png;base64,test",
			CreatedAt: 1,
		},
		pngBytes: raw,
	}
	s.mu.Lock()
	s.sessions[session.ID] = session
	s.mu.Unlock()
	return session.Session
}

func testOverlayPNG(t testing.TB, width int, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 120, A: 255})
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func readPNG(t *testing.T, path string) image.Image {
	t.Helper()
	raw, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	img, err := png.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func decodePNGBytes(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func stubImageClipboardWriter(t *testing.T) *string {
	t.Helper()
	previousWriter := writeImageToClipboard
	copiedPath := ""
	writeImageToClipboard = func(path string) error {
		copiedPath = path
		return nil
	}
	t.Cleanup(func() {
		writeImageToClipboard = previousWriter
	})
	return &copiedPath
}

func stubPNGClipboardWriter(t *testing.T) *[]byte {
	t.Helper()
	previousWriter := writePNGToClipboard
	var copied []byte
	writePNGToClipboard = func(data []byte) error {
		copied = append([]byte(nil), data...)
		return nil
	}
	t.Cleanup(func() {
		writePNGToClipboard = previousWriter
	})
	return &copied
}

func stubTextClipboardWriter(t *testing.T) *string {
	t.Helper()
	previousWriter := writeTextToClipboard
	copiedText := ""
	writeTextToClipboard = func(text string) error {
		copiedText = text
		return nil
	}
	t.Cleanup(func() {
		writeTextToClipboard = previousWriter
	})
	return &copiedText
}

func waitCaptureHistoryCount(t *testing.T, captures *capturehistory.Service, expected int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if captures.Status().Count >= expected {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected capture history count to reach %d, got %d", expected, captures.Status().Count)
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}

func sameRGBA(a color.Color, b color.RGBA) bool {
	r, g, bl, alpha := a.RGBA()
	return uint8(r>>8) == b.R && uint8(g>>8) == b.G && uint8(bl>>8) == b.B && uint8(alpha>>8) == b.A
}

var _ PinService = failingPinService{}
