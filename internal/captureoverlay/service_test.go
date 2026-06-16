package captureoverlay

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"ariadne/internal/capturehistory"
	"ariadne/internal/pinnedimage"
)

type failingPinService struct{}

func (f failingPinService) OpenCapture(id string) pinnedimage.OpenResult {
	return pinnedimage.OpenResult{OK: false, Message: "pin failed", PinID: id}
}

type positionedPinRecorder struct {
	id string
	x  int
	y  int
}

func (p *positionedPinRecorder) OpenCapture(id string) pinnedimage.OpenResult {
	p.id = id
	return pinnedimage.OpenResult{OK: true, Message: "pinned", PinID: id}
}

func (p *positionedPinRecorder) OpenCaptureAt(id string, x int, y int) pinnedimage.OpenResult {
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

	var copiedPath string
	previousWriter := writeImageToClipboard
	writeImageToClipboard = func(path string) error {
		copiedPath = path
		return nil
	}
	defer func() { writeImageToClipboard = previousWriter }()

	result := service.CaptureSelection(SelectionRequest{SessionID: session.ID, X: 0, Y: 0, Width: 3, Height: 2, Action: "copy"})
	if !result.OK || result.CaptureID == "" {
		t.Fatalf("expected copied capture, got %#v", result)
	}
	if copiedPath != result.ImagePath {
		t.Fatalf("expected copied image path %q, got %q", result.ImagePath, copiedPath)
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

func testOverlayPNG(t *testing.T, width int, height int) []byte {
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

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func sameRGBA(a color.Color, b color.RGBA) bool {
	r, g, bl, alpha := a.RGBA()
	return uint8(r>>8) == b.R && uint8(g>>8) == b.G && uint8(bl>>8) == b.B && uint8(alpha>>8) == b.A
}

var _ PinService = failingPinService{}
