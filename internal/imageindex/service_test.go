package imageindex

import (
	"path/filepath"
	"testing"
	"time"

	"ariadne/internal/capturehistory"
	"ariadne/internal/clipboardhistory"
	"ariadne/internal/contracts"
	"ariadne/internal/ocr"
)

func TestIndexRecentIndexesCaptureAndClipboardImages(t *testing.T) {
	captures := fakeCaptures{items: []capturehistory.Entry{
		{ID: "cap-1", ImagePath: filepath.Join(t.TempDir(), "cap.png"), CreatedAt: 10, Width: 120, Height: 80},
	}}
	clipboard := fakeClipboard{items: []clipboardhistory.Entry{
		{ID: "clip-text", Type: clipboardhistory.EntryText, Text: "skip me"},
		{ID: "clip-1", Type: clipboardhistory.EntryImage, ImagePath: filepath.Join(t.TempDir(), "clip.png"), CreatedAt: 20, Width: 40, Height: 30},
	}}
	ocrProvider := &fakeOCR{
		captureText:   map[string]string{"cap-1": "router gateway error"},
		clipboardText: map[string]string{"clip-1": "clipboard invoice total"},
	}
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "image_index.json"), captures, clipboard, ocrProvider)

	result := service.IndexRecent(IndexRequest{Limit: 10})
	if !result.OK || result.Indexed != 2 || result.Skipped != 0 || result.Failed != 0 {
		t.Fatalf("unexpected batch result: %#v", result)
	}
	if len(service.Search("gateway")) != 1 {
		t.Fatal("capture OCR text should be searchable")
	}
	if len(service.Search("ocr invoice")) != 0 {
		t.Fatal("clipboard OCR text should not appear in launcher search")
	}

	second := service.IndexRecent(IndexRequest{Limit: 10})
	if second.Indexed != 0 || second.Skipped != 2 {
		t.Fatalf("expected existing entries to be skipped, got %#v", second)
	}
}

func TestSearchResultUsesExplicitActionsBySource(t *testing.T) {
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "image_index.json"), nil, nil, nil)
	service.recordEntry(Entry{
		ID:        entryID(SourceCaptureHistory, "cap-1"),
		Source:    SourceCaptureHistory,
		SourceID:  "cap-1",
		ImagePath: filepath.Join(t.TempDir(), "cap.png"),
		Text:      "deployment failed on server",
		OK:        true,
		Width:     100,
		Height:    50,
		IndexedAt: 100,
		CreatedAt: 90,
	})
	service.recordEntry(Entry{
		ID:        entryID(SourceClipboardHistory, "clip-1"),
		Source:    SourceClipboardHistory,
		SourceID:  "clip-1",
		ImagePath: filepath.Join(t.TempDir(), "clip.png"),
		Text:      "clipboard deployment note",
		OK:        true,
		Width:     80,
		Height:    40,
		IndexedAt: 110,
		CreatedAt: 95,
	})

	captureResults := service.Search("server")
	if len(captureResults) != 1 || captureResults[0].Type != contracts.ResultCapture {
		t.Fatalf("expected one capture result, got %#v", captureResults)
	}
	if err := contracts.ValidateActionSurface(captureResults[0]); err != nil {
		t.Fatalf("capture actions should be explicit and valid: %v", err)
	}
	if !hasAction(captureResults[0], "copy_image_ocr_text") || !hasAction(captureResults[0], "open_capture_ocr_parent") {
		t.Fatalf("capture OCR result missing expected actions: %#v", captureResults[0].Actions)
	}

	if clipboardResults := service.Search("clipboard deployment"); len(clipboardResults) != 0 {
		t.Fatalf("clipboard OCR entries should not appear in launcher search, got %#v", clipboardResults)
	}
}

func TestImageIndexCommandHasExecutableAction(t *testing.T) {
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "image_index.json"), nil, nil, nil)

	results := service.Search("img index")
	if len(results) != 1 || results[0].ID != "image-index-recent" {
		t.Fatalf("expected image index command result, got %#v", results)
	}
	if err := contracts.ValidateActionSurface(results[0]); err != nil {
		t.Fatalf("image index command action should be explicit and valid: %v", err)
	}
	if !hasAction(results[0], "image_index_recent") {
		t.Fatalf("image index command missing executable action: %#v", results[0].Actions)
	}
}

func TestSensitiveOCRTextIsRedactedFromSearch(t *testing.T) {
	captures := fakeCaptures{items: []capturehistory.Entry{
		{ID: "cap-secret", ImagePath: filepath.Join(t.TempDir(), "secret.png"), Width: 10, Height: 10},
	}}
	ocrProvider := &fakeOCR{
		captureText:      map[string]string{"cap-secret": "password token secret"},
		captureSensitive: map[string]bool{"cap-secret": true},
	}
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "image_index.json"), captures, nil, ocrProvider)

	result := service.IndexRecent(IndexRequest{Sources: []string{SourceCaptureHistory}})
	if !result.OK || result.Indexed != 1 {
		t.Fatalf("expected redacted sensitive entry to count as indexed, got %#v", result)
	}
	entries := service.List("", 10)
	if len(entries) != 1 || !entries[0].Redacted || entries[0].Text != "" {
		t.Fatalf("sensitive OCR text should be redacted, got %#v", entries)
	}
	if got := service.Search("password"); len(got) != 0 {
		t.Fatalf("sensitive OCR text must not be searchable, got %#v", got)
	}
}

func TestRetentionPolicyRemovesExpiredAndStaleEntries(t *testing.T) {
	now := time.Unix(1772000000, 0)
	captures := fakeCaptures{items: []capturehistory.Entry{
		{ID: "cap-live", ImagePath: "live.png"},
		{ID: "cap-expired", ImagePath: "expired.png"},
	}}
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "image_index.json"), captures, nil, nil)
	service.now = func() time.Time { return now }
	service.recordEntry(Entry{
		ID:        entryID(SourceCaptureHistory, "cap-live"),
		Source:    SourceCaptureHistory,
		SourceID:  "cap-live",
		ImagePath: "live.png",
		Text:      "recent live text",
		OK:        true,
		CreatedAt: now.Add(-2 * 24 * time.Hour).Unix(),
		IndexedAt: now.Add(-2 * 24 * time.Hour).Unix(),
	})
	service.recordEntry(Entry{
		ID:        entryID(SourceCaptureHistory, "cap-expired"),
		Source:    SourceCaptureHistory,
		SourceID:  "cap-expired",
		ImagePath: "expired.png",
		Text:      "old text",
		OK:        true,
		CreatedAt: now.Add(-40 * 24 * time.Hour).Unix(),
		IndexedAt: now.Add(-40 * 24 * time.Hour).Unix(),
	})
	service.recordEntry(Entry{
		ID:        entryID(SourceCaptureHistory, "cap-stale"),
		Source:    SourceCaptureHistory,
		SourceID:  "cap-stale",
		ImagePath: "stale.png",
		Text:      "stale text",
		OK:        true,
		CreatedAt: now.Add(-1 * 24 * time.Hour).Unix(),
		IndexedAt: now.Add(-1 * 24 * time.Hour).Unix(),
	})

	result := service.ApplyRetentionPolicy(30)

	if !result.OK || result.Removed != 2 || result.RemovedExpired != 1 || result.RemovedStale != 1 || result.RemainingCount != 1 {
		t.Fatalf("unexpected retention result: %#v", result)
	}
	entries := service.List("", 10)
	if len(entries) != 1 || entries[0].SourceID != "cap-live" {
		t.Fatalf("expected only live index entry to remain, got %#v", entries)
	}
}

type fakeCaptures struct {
	items []capturehistory.Entry
}

func (f fakeCaptures) List(string, int) []capturehistory.Entry {
	return append([]capturehistory.Entry{}, f.items...)
}

func (f fakeCaptures) Entry(id string) capturehistory.Entry {
	for _, item := range f.items {
		if item.ID == id {
			return item
		}
	}
	return capturehistory.Entry{}
}

type fakeClipboard struct {
	items []clipboardhistory.Entry
}

func (f fakeClipboard) List(string, int) []clipboardhistory.Entry {
	return append([]clipboardhistory.Entry{}, f.items...)
}

func (f fakeClipboard) Entry(id string) clipboardhistory.Entry {
	for _, item := range f.items {
		if item.ID == id {
			return item
		}
	}
	return clipboardhistory.Entry{}
}

type fakeOCR struct {
	captureText        map[string]string
	captureSensitive   map[string]bool
	clipboardText      map[string]string
	clipboardSensitive map[string]bool
}

func (f *fakeOCR) RecognizeCapture(captureID string) ocr.Result {
	return ocr.Result{
		OK:        true,
		Text:      f.captureText[captureID],
		Provider:  "test-ocr",
		Sensitive: f.captureSensitive[captureID],
	}
}

func (f *fakeOCR) RecognizeClipboardImage(clipboardID string) ocr.Result {
	return ocr.Result{
		OK:        true,
		Text:      f.clipboardText[clipboardID],
		Provider:  "test-ocr",
		Sensitive: f.clipboardSensitive[clipboardID],
	}
}

func (f *fakeOCR) Status() ocr.Status {
	return ocr.Status{Available: true, Provider: "test-ocr"}
}

func hasAction(result contracts.SearchResult, id string) bool {
	for _, action := range result.Actions {
		if action.ID == id {
			return true
		}
	}
	return false
}
