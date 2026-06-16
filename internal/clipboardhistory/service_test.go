package clipboardhistory

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ariadne/internal/capturehistory"
	"ariadne/internal/contracts"

	goqrcode "github.com/skip2/go-qrcode"
)

func TestClipboardHistoryPersistsAndSearchesText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clipboard_history.json")
	service := NewServiceWithPath(path)

	status := service.AddText(`{"service":"gateway","status":"degraded"}`, "test")
	if status.LastSaveError != "" {
		t.Fatalf("unexpected save error: %s", status.LastSaveError)
	}

	reloaded := NewServiceWithPath(path)
	results := reloaded.Search("gateway")

	if len(results) != 1 {
		t.Fatalf("expected clipboard search result, got %#v", results)
	}
	result := results[0]
	if result.Type != contracts.ResultClipboard {
		t.Fatalf("expected clipboard result, got %s", result.Type)
	}
	if err := contracts.ValidateActionSurface(result); err != nil {
		t.Fatalf("invalid action surface: %v", err)
	}
	if hasActionKind(result, contracts.ActionOpenParent) {
		t.Fatalf("clipboard result must not expose file-only open_parent: %#v", result.Actions)
	}
}

func TestClipboardHistoryNotifiesEntryObserver(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clipboard_history.json")
	service := NewServiceWithPath(path)
	var observed Entry
	RegisterEntryObserver(service, func(entry Entry) {
		observed = entry
	})

	service.AddText("proactive clipboard memory", "test")

	if observed.ID == "" || observed.Text != "proactive clipboard memory" {
		t.Fatalf("expected observer to receive added clipboard entry, got %#v", observed)
	}
}

func TestClipboardHistoryPersistsAndSearchesImage(t *testing.T) {
	dir := t.TempDir()
	service := NewServiceWithPaths(filepath.Join(dir, "clipboard_history.json"), filepath.Join(dir, "clipboard_images"))

	status := service.AddPNG(testPNG(t, 3, 2), "test")
	if status.LastSaveError != "" {
		t.Fatalf("unexpected save error: %s", status.LastSaveError)
	}
	if status.Count != 1 || status.ImageCount != 1 {
		t.Fatalf("expected one image entry, got %#v", status)
	}

	reloaded := NewServiceWithPaths(filepath.Join(dir, "clipboard_history.json"), filepath.Join(dir, "clipboard_images"))
	results := reloaded.Search("图片")
	if len(results) != 1 {
		t.Fatalf("expected image clipboard search result, got %#v", results)
	}
	result := results[0]
	if result.Type != contracts.ResultClipboard || result.Preview.Kind != contracts.PreviewImage {
		t.Fatalf("expected image clipboard result, got %#v", result)
	}
	if err := contracts.ValidateActionSurface(result); err != nil {
		t.Fatalf("invalid image action surface: %v", err)
	}
	if hasActionKind(result, contracts.ActionOpenParent) {
		t.Fatalf("clipboard image result must not expose file-only open_parent: %#v", result.Actions)
	}
}

func TestClipboardHistoryImageDataURL(t *testing.T) {
	dir := t.TempDir()
	service := NewServiceWithPaths(filepath.Join(dir, "clipboard_history.json"), filepath.Join(dir, "clipboard_images"))
	service.AddPNG(testPNG(t, 3, 2), "test")
	entry := service.List("图片", 10)[0]

	dataURL := service.ImageDataURL(entry.ID)
	if len(dataURL) < len("data:image/png;base64,") || dataURL[:22] != "data:image/png;base64," {
		t.Fatalf("expected png data url, got %q", dataURL)
	}
}

func TestClipboardHistoryCreatesThumbnailAndDeletesItWithImage(t *testing.T) {
	dir := t.TempDir()
	service := NewServiceWithPaths(filepath.Join(dir, "clipboard_history.json"), filepath.Join(dir, "clipboard_images"))
	service.AddPNG(testPNG(t, 900, 600), "test")
	entry := service.List("图片", 10)[0]

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
	if _, err := os.Stat(entry.ImagePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected image removed, err=%v", err)
	}
	if _, err := os.Stat(entry.ThumbnailPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected thumbnail removed, err=%v", err)
	}
}

func TestClipboardHistoryBackfillsMissingThumbnail(t *testing.T) {
	dir := t.TempDir()
	service := NewServiceWithPaths(filepath.Join(dir, "clipboard_history.json"), filepath.Join(dir, "clipboard_images"))
	service.AddPNG(testPNG(t, 900, 600), "test")
	entry := service.List("图片", 10)[0]
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

func TestClipboardImageResultExposesPinnedImageAction(t *testing.T) {
	dir := t.TempDir()
	service := NewServiceWithPaths(filepath.Join(dir, "clipboard_history.json"), filepath.Join(dir, "clipboard_images"))
	service.AddPNG(testPNG(t, 3, 2), "test")

	results := service.Search("图片")
	if len(results) != 1 {
		t.Fatalf("expected one clipboard image result, got %#v", results)
	}
	if !hasActionID(results[0], "pin_clipboard_image") {
		t.Fatalf("clipboard image result should expose pinned image action: %#v", results[0].Actions)
	}
}

func TestClipboardImageCanBeAddedToCaptureHistory(t *testing.T) {
	dir := t.TempDir()
	captures := capturehistory.NewServiceWithPaths(filepath.Join(dir, "capture_history.json"), filepath.Join(dir, "capture_images"))
	service := NewServiceWithPaths(filepath.Join(dir, "clipboard_history.json"), filepath.Join(dir, "clipboard_images"))
	service.captureSink = captures
	service.AddPNG(testPNG(t, 5, 4), "test")
	entry := service.List("图片", 10)[0]

	status := service.AddImageToCapture(entry.ID)
	if status.LastCaptureError != "" || status.Count != 1 || len(status.Entries) != 1 {
		t.Fatalf("expected image to be added to capture history, got %#v", status)
	}
	capture := status.Entries[0]
	if capture.Source != "clipboard_image" || capture.Width != 5 || capture.Height != 4 {
		t.Fatalf("unexpected capture entry: %#v", capture)
	}
	if _, err := os.Stat(capture.ImagePath); err != nil {
		t.Fatalf("expected capture png to exist: %v", err)
	}
}

func TestClipboardImageQRCodeDecode(t *testing.T) {
	service := NewServiceWithPaths(filepath.Join(t.TempDir(), "clipboard_history.json"), filepath.Join(t.TempDir(), "clipboard_images"))
	pngData, err := goqrcode.Encode("ariadne clipboard qr", goqrcode.Medium, 128)
	if err != nil {
		t.Fatalf("encode qr: %v", err)
	}
	service.AddPNG(pngData, "test")
	entry := service.List("图片", 10)[0]

	result := service.DecodeImageQRCode(entry.ID)
	if !result.OK || result.Text != "ariadne clipboard qr" || result.Source != "clipboard_history" {
		t.Fatalf("expected clipboard qr decode, got %#v", result)
	}
}

func TestClipboardHistoryDeleteAndClearRemoveImageFiles(t *testing.T) {
	dir := t.TempDir()
	service := NewServiceWithPaths(filepath.Join(dir, "clipboard_history.json"), filepath.Join(dir, "clipboard_images"))
	service.AddPNG(testPNG(t, 2, 2), "test")
	first := service.List("图片", 10)[0]
	if _, err := os.Stat(first.ImagePath); err != nil {
		t.Fatalf("expected image file to exist: %v", err)
	}

	service.Delete(first.ID)
	if _, err := os.Stat(first.ImagePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected deleted image file to be removed, err=%v", err)
	}

	service.AddPNG(testPNG(t, 4, 3), "test")
	second := service.List("图片", 10)[0]
	service.ClearUnpinned()
	if _, err := os.Stat(second.ImagePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected cleared image file to be removed, err=%v", err)
	}
}

func TestClipboardHistoryDeduplicatesAndPreservesPin(t *testing.T) {
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "clipboard_history.json"))
	service.AddText("first", "test")
	service.AddText("second", "test")
	first := service.List("first", 10)[0]
	service.TogglePin(first.ID)

	service.AddText("first", "test")
	entries := service.List("", 10)

	if len(entries) != 2 {
		t.Fatalf("expected duplicate to refresh existing entry, got %#v", entries)
	}
	if entries[0].Text != "first" || !entries[0].Pinned {
		t.Fatalf("expected refreshed pinned duplicate first, got %#v", entries[0])
	}
}

func TestClipboardHistorySearchUnderstandsClipPrefix(t *testing.T) {
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "clipboard_history.json"))
	service.AddText("token value from gateway", "test")

	results := service.Search("clip token")

	if len(results) != 1 {
		t.Fatalf("expected clip-prefixed query to find entry, got %#v", results)
	}
}

func TestClipboardHistoryClearUnpinnedKeepsPinned(t *testing.T) {
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "clipboard_history.json"))
	service.AddText("keep me", "test")
	service.AddText("remove me", "test")
	keep := service.List("keep", 10)[0]
	service.TogglePin(keep.ID)

	status := service.ClearUnpinned()
	entries := service.List("", 10)

	if status.Count != 1 || len(entries) != 1 || entries[0].Text != "keep me" {
		t.Fatalf("expected only pinned entry to remain, status=%#v entries=%#v", status, entries)
	}
}

func TestClipboardHistoryRetentionRemovesOldUnpinnedEntriesAndImages(t *testing.T) {
	dir := t.TempDir()
	service := NewServiceWithPaths(filepath.Join(dir, "clipboard_history.json"), filepath.Join(dir, "clipboard_images"))
	now := time.Now()
	service.AddText("old text", "test")
	oldText := service.List("old text", 10)[0]
	service.AddPNG(testPNG(t, 2, 2), "test")
	oldImage := service.List("图片", 10)[0]
	service.AddText("old pinned", "test")
	oldPinned := service.List("old pinned", 10)[0]
	service.TogglePin(oldPinned.ID)
	service.AddText("recent text", "test")
	recent := service.List("recent text", 10)[0]
	service.setCreatedAtForTest(oldText.ID, now.Add(-40*24*time.Hour).Unix())
	service.setCreatedAtForTest(oldImage.ID, now.Add(-45*24*time.Hour).Unix())
	service.setCreatedAtForTest(oldPinned.ID, now.Add(-50*24*time.Hour).Unix())
	service.setCreatedAtForTest(recent.ID, now.Add(-2*24*time.Hour).Unix())

	result := service.ApplyRetentionPolicy(30, true)

	if !result.OK || result.Removed != 2 || result.RemovedImages != 1 || result.KeptPinned != 1 || result.RemainingCount != 2 {
		t.Fatalf("unexpected retention result: %#v", result)
	}
	if _, err := os.Stat(oldImage.ImagePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old clipboard image should be removed, err=%v", err)
	}
	remaining := service.List("", 10)
	if len(remaining) != 2 {
		t.Fatalf("expected two retained entries, got %#v", remaining)
	}
	if service.Entry(oldText.ID).ID != "" || service.Entry(oldImage.ID).ID != "" {
		t.Fatalf("old unpinned entries should be removed, got %#v", remaining)
	}
	if service.Entry(oldPinned.ID).ID == "" || service.Entry(recent.ID).ID == "" {
		t.Fatalf("pinned and recent entries should remain, got %#v", remaining)
	}
}

func TestClipboardHistoryReportsSaveErrors(t *testing.T) {
	service := NewServiceWithPath(t.TempDir())

	status := service.AddText("cannot save to directory path", "test")

	if status.LastSaveError == "" {
		t.Fatal("expected save error for directory path")
	}
}

func TestClipboardWatcherPrimesWithoutRecordingExistingText(t *testing.T) {
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "clipboard_history.json"))
	values := []string{"already on clipboard", "new copied token"}
	index := 0
	service.clipboardReader = func(_ string, source string) (Entry, error) {
		if index >= len(values) {
			return makeTextEntry(values[len(values)-1], source), nil
		}
		value := values[index]
		index++
		return makeTextEntry(value, source), nil
	}

	status := service.pollClipboardOnce(false)
	if status.Count != 0 {
		t.Fatalf("initial clipboard baseline should not be recorded: %#v", status)
	}

	status = service.pollClipboardOnce(true)
	entries := service.List("", 10)
	if status.Count != 1 || len(entries) != 1 {
		t.Fatalf("expected one watched clipboard entry, status=%#v entries=%#v", status, entries)
	}
	if entries[0].Text != "new copied token" || entries[0].Source != "clipboard_watcher" {
		t.Fatalf("unexpected watched entry: %#v", entries[0])
	}
}

func TestClipboardWatcherSkipsUnchangedTextAndReportsErrors(t *testing.T) {
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "clipboard_history.json"))
	service.clipboardReader = func(_ string, source string) (Entry, error) {
		return makeTextEntry("same text", source), nil
	}

	service.pollClipboardOnce(false)
	status := service.pollClipboardOnce(true)
	if status.Count != 0 {
		t.Fatalf("unchanged clipboard should not be recorded: %#v", status)
	}

	service.clipboardReader = func(_ string, _ string) (Entry, error) {
		return Entry{}, errors.New("clipboard locked")
	}
	status = service.pollClipboardOnce(true)
	if status.LastWatcherError == "" {
		t.Fatalf("watcher errors should be reported: %#v", status)
	}
}

func TestClipboardWatcherPrimesImageWithoutRecordingBaseline(t *testing.T) {
	dir := t.TempDir()
	service := NewServiceWithPaths(filepath.Join(dir, "clipboard_history.json"), filepath.Join(dir, "clipboard_images"))
	values := [][]byte{testPNG(t, 1, 1), testPNG(t, 2, 2)}
	index := 0
	service.clipboardReader = func(imageDir string, source string) (Entry, error) {
		if index >= len(values) {
			return makeImageEntryFromPNG(values[len(values)-1], imageDir, source)
		}
		value := values[index]
		index++
		return makeImageEntryFromPNG(value, imageDir, source)
	}

	status := service.pollClipboardOnce(false)
	if status.Count != 0 {
		t.Fatalf("initial image baseline should not be recorded: %#v", status)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "clipboard_images", "*.png"))
	if len(files) != 0 {
		t.Fatalf("baseline image file should be removed, got %#v", files)
	}

	status = service.pollClipboardOnce(true)
	entries := service.List("图片", 10)
	if status.ImageCount != 1 || len(entries) != 1 || entries[0].Type != EntryImage {
		t.Fatalf("expected watched image entry, status=%#v entries=%#v", status, entries)
	}
}

func TestClipboardWatcherSettingsPauseForPrivacyOrDisabledSource(t *testing.T) {
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "clipboard_history.json"))

	status := service.ApplyWatcherSettings(true, true)
	if status.WatcherEnabled || status.WatcherRunning || status.LastWatcherError == "" {
		t.Fatalf("privacy mode should pause watcher with visible reason: %#v", status)
	}

	status = service.ApplyWatcherSettings(false, false)
	if status.WatcherEnabled || status.WatcherRunning {
		t.Fatalf("disabled clipboard source should stop watcher: %#v", status)
	}
}

func TestCollectCurrentReportsEmptyClipboard(t *testing.T) {
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "clipboard_history.json"))
	service.clipboardReader = func(_ string, source string) (Entry, error) {
		return makeTextEntry("", source), nil
	}

	status := service.CollectCurrent("manual")
	if status.LastSaveError == "" || status.Count != 0 {
		t.Fatalf("empty clipboard should report a local error without adding entries: %#v", status)
	}
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

func testPNG(t *testing.T, width int, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(20 + x), G: uint8(40 + y), B: 160, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}
