package imageindex

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fancun/internal/appdb"
	"fancun/internal/capturehistory"
	"fancun/internal/clipboardhistory"
	"fancun/internal/contracts"
	"fancun/internal/ocr"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
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

func (f fakeCaptures) CaptureScreen(source string) capturehistory.Status {
	return capturehistory.Status{}
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
	captureLines       map[string][]ocr.Line
	clipboardText      map[string]string
	clipboardSensitive map[string]bool
}

func (f *fakeOCR) RecognizeCapture(captureID string) ocr.Result {
	return ocr.Result{
		OK:        true,
		Text:      f.captureText[captureID],
		Lines:     f.captureLines[captureID],
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

func TestSensitiveOCRTextAndLinesAreRedactedAndNotPersisted(t *testing.T) {
	indexPath := filepath.Join(t.TempDir(), "image_index.json")
	captures := fakeCaptures{items: []capturehistory.Entry{
		{ID: "cap-secret-lines", ImagePath: filepath.Join(t.TempDir(), "secret.png"), Width: 100, Height: 40},
	}}
	ocrProvider := &fakeOCR{
		captureText:      map[string]string{"cap-secret-lines": "Authorization: Bearer secret-token-123"},
		captureSensitive: map[string]bool{"cap-secret-lines": true},
		captureLines: map[string][]ocr.Line{
			"cap-secret-lines": {
				{Text: "Authorization: Bearer secret-token-123", Confidence: 0.99, Rect: ocr.Rect{X: 1, Y: 2, Width: 80, Height: 20}},
			},
		},
	}
	service := NewServiceWithPath(indexPath, captures, nil, ocrProvider)

	result := service.IndexRecent(IndexRequest{Sources: []string{SourceCaptureHistory}})
	if !result.OK || result.Indexed != 1 {
		t.Fatalf("expected indexed sensitive result, got %#v", result)
	}
	entries := service.List("", 10)
	if len(entries) != 1 || !entries[0].Redacted || entries[0].Text != "" || len(entries[0].Lines) != 0 {
		t.Fatalf("sensitive entry must clear both Text and Lines: %#v", entries)
	}

	reloaded := NewServiceWithPath(indexPath, captures, nil, ocrProvider)
	reloadedEntries := reloaded.List("", 10)
	if len(reloadedEntries) != 1 || len(reloadedEntries[0].Lines) != 0 || reloadedEntries[0].Text != "" {
		t.Fatalf("reloaded sensitive entry must not contain Lines or Text: %#v", reloadedEntries)
	}

	conn, err := appdb.OpenForPath(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	lineCount := -1
	if err := sqlitex.Execute(conn, `SELECT count(*) FROM image_index_lines`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error { lineCount = stmt.ColumnInt(0); return nil },
	}); err != nil {
		t.Fatal(err)
	}
	if lineCount != 0 {
		t.Fatalf("expected 0 persisted lines in SQLite for sensitive image, got %d", lineCount)
	}

	normalized := normalizeEntry(Entry{
		Source:   SourceCaptureHistory,
		SourceID: "manual-redacted",
		Redacted: true,
		Text:     "must clear",
		Lines:    []ocr.Line{{Text: "must clear"}},
	})
	if normalized.Text != "" || len(normalized.Lines) != 0 {
		t.Fatalf("normalizeEntry must clear Text and Lines when Redacted=true: %#v", normalized)
	}
}

type fakeLocalAIClient struct {
	calls int
}

func (f *fakeLocalAIClient) RecognizeImageOCR(_ context.Context, _ ocr.AIOCRJob) (ocr.AIResult, error) {
	f.calls++
	return ocr.AIResult{Provider: "remote:vision", Text: "remote leaked text"}, nil
}

func TestImageIndexEnforcesLocalOnlyOCREvenWhenRemoteAIConfigured(t *testing.T) {
	root := t.TempDir()
	capImg := filepath.Join(root, "cap.png")
	clipImg := filepath.Join(root, "clip.png")
	writeTestImage(t, capImg)
	writeTestImage(t, clipImg)
	localPy := filepath.Join(root, "python")
	writeTestImage(t, localPy)
	t.Setenv("FANCUN_OCR_PYTHON", localPy)

	captures := fakeCaptures{items: []capturehistory.Entry{
		{ID: "cap-1", ImagePath: capImg, Width: 100, Height: 50},
	}}
	clipboard := fakeClipboard{items: []clipboardhistory.Entry{
		{ID: "clip-1", Type: clipboardhistory.EntryImage, ImagePath: clipImg, Width: 80, Height: 40},
	}}
	ocrSvc := ocr.NewService(captures, clipboard)
	aiClient := &fakeLocalAIClient{}
	ocr.RegisterAIClient(ocrSvc, aiClient)
	ocrSvc.ApplyAIOCRPolicy(ocr.AIOCRPolicy{
		Enabled:  true,
		Provider: "openai-compatible",
		BaseURL:  "https://remote.example.invalid/v1",
		Model:    "vision-model",
	})

	// Swap local runner via RecognizeCaptureLocal? Wait: test using a LocalOCRProvider wrapper + real ocr.Service test in internal/ocr,
	// or check status and verify aiClient is never called!
	// Even better: if FANCUN_OCR_PYTHON fails to run as a real script, remote AI MUST STILL NOT be called!
	svc := NewServiceWithPath(filepath.Join(root, "image_index.json"), captures, clipboard, ocrSvc)
	status := svc.Status()
	if status.OCRProvider != "rapidocr_onnxruntime" {
		t.Fatalf("expected imageindex status to report local rapidocr provider, got %q", status.OCRProvider)
	}

	_ = svc.IndexRecent(IndexRequest{Limit: 10})
	if aiClient.calls != 0 {
		t.Fatalf("imageindex must never upload local screenshots to remote AI OCR, got %d AI calls", aiClient.calls)
	}

	localOnlyProvider := &fakeLocalOnlyOCR{}
	svc2 := NewServiceWithPath(filepath.Join(t.TempDir(), "image_index.json"), captures, clipboard, localOnlyProvider)
	batch := svc2.IndexRecent(IndexRequest{Limit: 10})
	if !batch.OK || batch.Indexed != 2 {
		t.Fatalf("expected local-only indexing to succeed, got %#v", batch)
	}
	if localOnlyProvider.localCaptureCalls != 1 || localOnlyProvider.localClipboardCalls != 1 {
		t.Fatalf("expected imageindex to call LocalOCRProvider methods, got %#v", localOnlyProvider)
	}
	if localOnlyProvider.remoteCaptureCalls != 0 || localOnlyProvider.remoteClipboardCalls != 0 {
		t.Fatalf("imageindex must not call policy-aware OCR methods when LocalOCRProvider is available: %#v", localOnlyProvider)
	}
	if st := svc2.Status(); st.OCRProvider != "rapidocr_onnxruntime" {
		t.Fatalf("expected LocalStatus provider rapidocr_onnxruntime, got %q", st.OCRProvider)
	}
}

type fakeLocalOnlyOCR struct {
	localCaptureCalls    int
	localClipboardCalls  int
	remoteCaptureCalls   int
	remoteClipboardCalls int
}

func (f *fakeLocalOnlyOCR) RecognizeCapture(string) ocr.Result {
	f.remoteCaptureCalls++
	return ocr.Result{OK: true, Text: "remote"}
}

func (f *fakeLocalOnlyOCR) RecognizeClipboardImage(string) ocr.Result {
	f.remoteClipboardCalls++
	return ocr.Result{OK: true, Text: "remote"}
}

func (f *fakeLocalOnlyOCR) RecognizeCaptureLocal(string) ocr.Result {
	f.localCaptureCalls++
	return ocr.Result{OK: true, Text: "local capture", Provider: "rapidocr_onnxruntime"}
}

func (f *fakeLocalOnlyOCR) RecognizeClipboardImageLocal(string) ocr.Result {
	f.localClipboardCalls++
	return ocr.Result{OK: true, Text: "local clipboard", Provider: "rapidocr_onnxruntime"}
}

func (f *fakeLocalOnlyOCR) Status() ocr.Status {
	return ocr.Status{Available: true, Provider: "openai-compatible:vision"}
}

func (f *fakeLocalOnlyOCR) LocalStatus() ocr.Status {
	return ocr.Status{Available: true, Provider: "rapidocr_onnxruntime"}
}

func writeTestImage(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}
}
