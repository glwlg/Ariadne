package capturehistory

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ariadne/internal/contracts"
)

const onePixelPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgwJ/lkD1JwAAAABJRU5ErkJggg=="

func TestCaptureHistoryPersistsSearchesAndPins(t *testing.T) {
	root := t.TempDir()
	service := NewServiceWithPaths(filepath.Join(root, "capture_history.json"), filepath.Join(root, "capture_images"))
	pngData := mustPNG(t)

	status := service.AddPNG(pngData, 1920, 1080, "test-screen", "", []string{"screen"})
	if status.Count != 1 || status.LastSaveError != "" || status.LastCaptureError != "" {
		t.Fatalf("unexpected status: %#v", status)
	}
	if _, err := os.Stat(status.Entries[0].ImagePath); err != nil {
		t.Fatalf("expected image to be written: %v", err)
	}

	reloaded := NewServiceWithPaths(filepath.Join(root, "capture_history.json"), filepath.Join(root, "capture_images"))
	results := reloaded.Search("cap 1920x1080")
	if len(results) != 1 || results[0].Type != contracts.ResultCapture {
		t.Fatalf("expected capture result, got %#v", results)
	}
	if err := contracts.ValidateActionSurfaces(results); err != nil {
		t.Fatalf("invalid action surface: %v", err)
	}
	if !hasActionKind(results[0], contracts.ActionOpenParent) {
		t.Fatal("capture results should expose open parent")
	}

	pinned := reloaded.TogglePin(status.Entries[0].ID)
	if pinned.PinnedCount != 1 {
		t.Fatalf("expected pinned entry, got %#v", pinned)
	}
}

func TestCaptureHistoryNotifiesEntryObserver(t *testing.T) {
	root := t.TempDir()
	service := NewServiceWithPaths(filepath.Join(root, "capture_history.json"), filepath.Join(root, "capture_images"))
	var observed Entry
	RegisterEntryObserver(service, func(entry Entry) {
		observed = entry
	})

	service.AddPNG(mustPNG(t), 320, 180, "overlay_selection", "", []string{"copy"})

	if observed.ID == "" || observed.Source != "overlay_selection" || observed.Width != 320 {
		t.Fatalf("expected observer to receive added capture entry, got %#v", observed)
	}
}

func TestCaptureHistoryDeleteAndClearUnpinnedRemoveStoredImages(t *testing.T) {
	root := t.TempDir()
	service := NewServiceWithPaths(filepath.Join(root, "capture_history.json"), filepath.Join(root, "capture_images"))

	first := service.AddPNG(testPNG(t, 100, 80), 100, 80, "first", "", nil).Entries[0]
	second := service.AddPNG(testPNG(t, 200, 160), 200, 160, "second", "", nil).Entries[0]
	service.TogglePin(second.ID)

	status := service.Delete(first.ID)
	if status.Count != 1 {
		t.Fatalf("expected one entry after delete, got %#v", status)
	}
	if _, err := os.Stat(first.ImagePath); !os.IsNotExist(err) {
		t.Fatalf("expected first image removed, stat err=%v", err)
	}

	status = service.ClearUnpinned()
	if status.Count != 1 || status.PinnedCount != 1 {
		t.Fatalf("pinned entry should survive clear: %#v", status)
	}
	if _, err := os.Stat(second.ImagePath); err != nil {
		t.Fatalf("pinned image should remain: %v", err)
	}
}

func TestCaptureHistoryRetentionRemovesOldUnpinnedImages(t *testing.T) {
	root := t.TempDir()
	service := NewServiceWithPaths(filepath.Join(root, "capture_history.json"), filepath.Join(root, "capture_images"))
	now := time.Now()

	old := service.AddPNG(testPNG(t, 100, 80), 100, 80, "old", "", nil).Entries[0]
	pinnedOld := service.AddPNG(testPNG(t, 120, 90), 120, 90, "pinned-old", "", nil).Entries[0]
	recent := service.AddPNG(testPNG(t, 140, 100), 140, 100, "recent", "", nil).Entries[0]
	service.TogglePin(pinnedOld.ID)
	service.setCreatedAtForTest(old.ID, now.Add(-40*24*time.Hour).Unix())
	service.setCreatedAtForTest(pinnedOld.ID, now.Add(-50*24*time.Hour).Unix())
	service.setCreatedAtForTest(recent.ID, now.Add(-2*24*time.Hour).Unix())

	result := service.ApplyRetentionPolicy(30, true)

	if !result.OK || result.Removed != 1 || result.KeptPinned != 1 || result.RemainingCount != 2 {
		t.Fatalf("unexpected retention result: %#v", result)
	}
	if _, err := os.Stat(old.ImagePath); !os.IsNotExist(err) {
		t.Fatalf("old unpinned image should be removed, err=%v", err)
	}
	if _, err := os.Stat(pinnedOld.ImagePath); err != nil {
		t.Fatalf("old pinned image should remain: %v", err)
	}
	if service.Entry(old.ID).ID != "" || service.Entry(pinnedOld.ID).ID == "" || service.Entry(recent.ID).ID == "" {
		t.Fatalf("unexpected entries after retention: %#v", service.List("", 10))
	}
}

func TestCaptureHistoryImageDataURL(t *testing.T) {
	root := t.TempDir()
	service := NewServiceWithPaths(filepath.Join(root, "capture_history.json"), filepath.Join(root, "capture_images"))
	entry := service.AddPNG(mustPNG(t), 1, 1, "test", "", nil).Entries[0]

	dataURL := service.ImageDataURL(entry.ID)
	if len(dataURL) < len("data:image/png;base64,") || dataURL[:22] != "data:image/png;base64," {
		t.Fatalf("expected png data url, got %q", dataURL)
	}
}

func TestCaptureHistoryCreatesThumbnailAndDeletesItWithEntry(t *testing.T) {
	root := t.TempDir()
	service := NewServiceWithPaths(filepath.Join(root, "capture_history.json"), filepath.Join(root, "capture_images"))
	entry := service.AddPNG(testPNG(t, 900, 600), 900, 600, "large", "", nil).Entries[0]

	if entry.ThumbnailPath == "" || entry.ThumbnailWidth != 512 || entry.ThumbnailHeight != 341 || entry.ThumbnailBytes <= 0 {
		t.Fatalf("expected thumbnail metadata, got %#v", entry)
	}
	if _, err := os.Stat(entry.ThumbnailPath); err != nil {
		t.Fatalf("expected thumbnail file: %v", err)
	}
	if dataURL := service.ThumbnailDataURL(entry.ID); len(dataURL) < len("data:image/png;base64,") || dataURL[:22] != "data:image/png;base64," {
		t.Fatalf("expected thumbnail data url, got %q", dataURL)
	}
	status := service.Status()
	if status.ThumbnailCount != 1 || status.ThumbnailBytes <= 0 || status.ThumbnailDir == "" {
		t.Fatalf("expected thumbnail status, got %#v", status)
	}

	service.Delete(entry.ID)
	if _, err := os.Stat(entry.ImagePath); !os.IsNotExist(err) {
		t.Fatalf("expected original image removed, stat err=%v", err)
	}
	if _, err := os.Stat(entry.ThumbnailPath); !os.IsNotExist(err) {
		t.Fatalf("expected thumbnail removed, stat err=%v", err)
	}
}

func TestCaptureHistoryBackfillsMissingThumbnail(t *testing.T) {
	root := t.TempDir()
	service := NewServiceWithPaths(filepath.Join(root, "capture_history.json"), filepath.Join(root, "capture_images"))
	entry := service.AddPNG(testPNG(t, 900, 600), 900, 600, "large", "", nil).Entries[0]
	if err := os.Remove(entry.ThumbnailPath); err != nil {
		t.Fatalf("remove thumbnail: %v", err)
	}
	service.clearThumbnailForTest(entry.ID)

	result := service.ensureThumbnails()
	backfilled := service.Entry(entry.ID)

	if result.Created != 1 || backfilled.ThumbnailPath == "" || backfilled.ThumbnailBytes <= 0 {
		t.Fatalf("expected thumbnail backfill, result=%#v entry=%#v", result, backfilled)
	}
	if _, err := os.Stat(backfilled.ThumbnailPath); err != nil {
		t.Fatalf("expected backfilled thumbnail file: %v", err)
	}
}

func TestCaptureHistoryResultExposesPinnedImageAction(t *testing.T) {
	root := t.TempDir()
	service := NewServiceWithPaths(filepath.Join(root, "capture_history.json"), filepath.Join(root, "capture_images"))
	service.AddPNG(mustPNG(t), 120, 80, "test", "", nil)

	results := service.Search("test")
	if len(results) != 1 {
		t.Fatalf("expected one capture result, got %#v", results)
	}
	if !hasActionID(results[0], "pin_capture_image") {
		t.Fatalf("capture result should expose pinned image action: %#v", results[0].Actions)
	}
}

func TestCaptureScreenWithOptionsRecordsStrategyMetadata(t *testing.T) {
	restore := replaceCaptureArtifactsForTest(func(options CaptureOptions) ([]capturedScreen, error) {
		if options.CaptureScope != "active_window" || options.MultiMonitor != "primary_only" {
			t.Fatalf("unexpected capture options: %#v", options)
		}
		data := testPNG(t, 40, 30)
		return []capturedScreen{
			{
				Data:    data,
				Width:   40,
				Height:  30,
				Bounds:  ScreenBounds{X: 10, Y: 20, Width: 40, Height: 30},
				Actions: []string{"active_window", "primary_only"},
				Tags:    []string{"范围:前台窗口", "多屏:仅主屏", "区域:10,20,40x30"},
			},
		}, nil
	})
	defer restore()

	root := t.TempDir()
	service := NewServiceWithPaths(filepath.Join(root, "capture_history.json"), filepath.Join(root, "capture_images"))
	status := service.CaptureScreenWithOptions("work_memory_time_machine", CaptureOptions{CaptureScope: "active_window", MultiMonitor: "primary_only"})

	if status.LastCaptureError != "" || len(status.Entries) != 1 {
		t.Fatalf("unexpected status: %#v", status)
	}
	entry := status.Entries[0]
	if entry.Source != "work_memory_time_machine" || !containsString(entry.Actions, "active_window") || !containsString(entry.Tags, "范围:前台窗口") || !containsString(entry.Tags, "区域:10,20,40x30") {
		t.Fatalf("expected strategy metadata, got %#v", entry)
	}
	if _, err := os.Stat(entry.ImagePath); err != nil {
		t.Fatalf("expected image written: %v", err)
	}
}

func TestCaptureScreenWithOptionsRecordsPerMonitorEntries(t *testing.T) {
	restore := replaceCaptureArtifactsForTest(func(options CaptureOptions) ([]capturedScreen, error) {
		if options.MultiMonitor != "per_monitor" {
			t.Fatalf("unexpected capture options: %#v", options)
		}
		return []capturedScreen{
			{
				Data:    testPNG(t, 20, 15),
				Width:   20,
				Height:  15,
				Bounds:  ScreenBounds{X: 0, Y: 0, Width: 20, Height: 15},
				Actions: []string{"all_screens", "per_monitor", "monitor_1"},
				Tags:    []string{"范围:全部屏幕", "多屏:按屏幕分条", "显示器:1/2"},
			},
			{
				Data:    testPNG(t, 30, 15),
				Width:   30,
				Height:  15,
				Bounds:  ScreenBounds{X: 20, Y: 0, Width: 30, Height: 15},
				Actions: []string{"all_screens", "per_monitor", "monitor_2"},
				Tags:    []string{"范围:全部屏幕", "多屏:按屏幕分条", "显示器:2/2"},
			},
		}, nil
	})
	defer restore()

	root := t.TempDir()
	service := NewServiceWithPaths(filepath.Join(root, "capture_history.json"), filepath.Join(root, "capture_images"))
	status := service.CaptureScreenWithOptions("time_machine", CaptureOptions{MultiMonitor: "per_monitor"})

	if status.LastCaptureError != "" || len(status.Entries) != 2 || status.Count != 2 {
		t.Fatalf("expected two monitor entries, got %#v", status)
	}
	if !containsString(status.Entries[0].Actions, "monitor_2") || !containsString(status.Entries[1].Actions, "monitor_1") {
		t.Fatalf("expected newest monitor first with actions, got %#v", status.Entries)
	}
	if !containsString(status.Entries[0].Tags, "多屏:按屏幕分条") || !containsString(status.Entries[1].Tags, "显示器:1/2") {
		t.Fatalf("expected monitor tags, got %#v", status.Entries)
	}
}

func mustPNG(t *testing.T) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(onePixelPNG)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func testPNG(t *testing.T, width int, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x % 251), G: uint8(y % 241), B: uint8((x + y) % 239), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func hasActionKind(result contracts.SearchResult, kind contracts.PreviewActionKind) bool {
	for _, action := range result.Actions {
		if action.Kind == kind {
			return true
		}
	}
	return false
}

func hasActionID(result contracts.SearchResult, id string) bool {
	for _, action := range result.Actions {
		if action.ID == id {
			return true
		}
	}
	return false
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func replaceCaptureArtifactsForTest(fn func(CaptureOptions) ([]capturedScreen, error)) func() {
	original := captureScreenArtifacts
	captureScreenArtifacts = fn
	return func() {
		captureScreenArtifacts = original
	}
}

func (s *Service) setCreatedAtForTest(id string, createdAt int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.entries {
		if s.entries[i].ID == id {
			s.entries[i].CreatedAt = createdAt
		}
	}
}

func (s *Service) clearThumbnailForTest(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.entries {
		if s.entries[i].ID == id {
			clearThumbnailFields(&s.entries[i])
		}
	}
}
