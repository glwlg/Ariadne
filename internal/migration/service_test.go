package migration

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ariadne/internal/capturehistory"
	"ariadne/internal/clipboardhistory"
	"ariadne/internal/workmemory"
)

func TestServiceImportsLegacyHistoryData(t *testing.T) {
	legacyRoot := t.TempDir()
	targetRoot := t.TempDir()

	clipboardImage := writeTestPNG(t, filepath.Join(legacyRoot, "clipboard_images", "clip.png"), 3, 2)
	captureImage := writeTestPNG(t, filepath.Join(legacyRoot, "capture_images", "capture.png"), 4, 3)
	memoryImage := writeTestPNG(t, filepath.Join(legacyRoot, "work_memory", "images", "memory.png"), 5, 4)

	writeJSON(t, filepath.Join(legacyRoot, "clipboard_history.json"), []legacyClipboardEntry{
		{
			ID:        "legacy-clip-text",
			Type:      "text",
			Text:      "legacy gateway token note",
			CreatedAt: 1710000000,
			Pinned:    true,
			Signature: "text:legacy-gateway",
		},
		{
			ID:        "legacy-clip-image",
			Type:      "image",
			ImagePath: clipboardImage,
			CreatedAt: 1710000010,
			Width:     3,
			Height:    2,
		},
	})
	writeJSON(t, filepath.Join(legacyRoot, "capture_history.json"), []legacyCaptureEntry{
		{
			ID:        "legacy-capture",
			ImagePath: captureImage,
			SavedPath: filepath.Join(legacyRoot, "manual", "capture.png"),
			CreatedAt: 1710000020,
			Source:    "region",
			Actions:   []string{"ocr"},
			Pinned:    true,
			Width:     4,
			Height:    3,
		},
	})
	writeJSON(t, filepath.Join(legacyRoot, "work_memory", "entries.json"), []legacyWorkMemoryEntry{
		{
			ID:          "legacy-memory",
			CreatedAt:   1710000030,
			Source:      "manual_note",
			SourceID:    "legacy-capture",
			ContentType: "screenshot",
			Title:       "Legacy memory",
			Summary:     "imported summary",
			Text:        "import this work memory",
			OCRText:     "gateway ocr",
			ImagePath:   memoryImage,
			AppName:     "legacy.exe",
			WindowTitle: "Legacy Window",
			Tags:        []string{"important"},
			Favorite:    true,
		},
	})

	clipboardService := clipboardhistory.NewServiceWithPaths(filepath.Join(targetRoot, "clipboard_history.json"), filepath.Join(targetRoot, "clipboard_images"))
	captureService := capturehistory.NewServiceWithPaths(filepath.Join(targetRoot, "capture_history.json"), filepath.Join(targetRoot, "capture_images"))
	memoryService := workmemory.NewServiceWithPath(filepath.Join(targetRoot, "work_memory.json"), nil)
	defer memoryService.Stop()
	clearWorkMemory(memoryService)
	service := NewServiceWithRoot(legacyRoot, clipboardService, captureService, memoryService)

	status := service.Status()
	if !status.Exists || status.TotalCount != 4 || status.TotalBytes <= 0 {
		t.Fatalf("unexpected legacy status: %#v", status)
	}
	if !status.NeedsImport {
		t.Fatalf("legacy status should require import before migration: %#v", status)
	}

	dryRun := service.ImportLegacyData(LegacyImportRequest{DryRun: true, Limit: 10})
	if !dryRun.OK || !dryRun.DryRun || totalImported(dryRun) != 4 {
		t.Fatalf("unexpected dry run: %#v", dryRun)
	}
	if clipboardService.Status().Count != 0 || captureService.Status().Count != 0 || memoryService.Status().EntryCount != 0 {
		t.Fatalf("dry run should not mutate target services")
	}

	result := service.ImportLegacyData(LegacyImportRequest{Limit: 10})
	if !result.OK || totalImported(result) != 4 {
		t.Fatalf("unexpected import result: %#v", result)
	}
	if clipboardService.Status().Count != 2 {
		t.Fatalf("expected two clipboard records, got %#v", clipboardService.Status())
	}
	if captureService.Status().Count != 1 {
		t.Fatalf("expected one capture record, got %#v", captureService.Status())
	}
	if memoryService.Status().EntryCount != 1 {
		t.Fatalf("expected one memory record, got %#v", memoryService.Status())
	}

	assertImportedClipboardImage(t, clipboardService, filepath.Join(targetRoot, "clipboard_images"))
	assertImportedCaptureImage(t, captureService, filepath.Join(targetRoot, "capture_images"))
	assertImportedMemoryImage(t, memoryService, filepath.Join(targetRoot, "work_memory_images"))
	importedStatus := service.Status()
	if importedStatus.NeedsImport {
		t.Fatalf("legacy status should be quiet after all sources are imported: %#v", importedStatus)
	}
	for _, source := range importedStatus.Sources {
		if source.NeedsImport || source.ImportedCount < source.Count {
			t.Fatalf("source should be marked imported: %#v", source)
		}
	}

	reimport := service.ImportLegacyData(LegacyImportRequest{Limit: 10})
	if !reimport.OK || totalImported(reimport) != 0 {
		t.Fatalf("reimport should dedupe existing records: %#v", reimport)
	}
}

func TestServiceImportIgnoresMissingLegacySources(t *testing.T) {
	legacyRoot := t.TempDir()
	targetRoot := t.TempDir()
	writeJSON(t, filepath.Join(legacyRoot, "clipboard_history.json"), []legacyClipboardEntry{
		{
			ID:        "legacy-clip-text",
			Type:      "text",
			Text:      "partial legacy data",
			CreatedAt: 1710000100,
		},
	})

	clipboardService := clipboardhistory.NewServiceWithPaths(filepath.Join(targetRoot, "clipboard_history.json"), filepath.Join(targetRoot, "clipboard_images"))
	captureService := capturehistory.NewServiceWithPaths(filepath.Join(targetRoot, "capture_history.json"), filepath.Join(targetRoot, "capture_images"))
	memoryService := workmemory.NewServiceWithPath(filepath.Join(targetRoot, "work_memory.json"), nil)
	defer memoryService.Stop()
	clearWorkMemory(memoryService)

	result := NewServiceWithRoot(legacyRoot, clipboardService, captureService, memoryService).ImportLegacyData(LegacyImportRequest{Limit: 10})
	if !result.OK || totalImported(result) != 1 {
		t.Fatalf("missing sources should not fail partial import: %#v", result)
	}
	if clipboardService.Status().Count != 1 || captureService.Status().Count != 0 || memoryService.Status().EntryCount != 0 {
		t.Fatalf("unexpected partial import state: clipboard=%#v capture=%#v memory=%#v", clipboardService.Status(), captureService.Status(), memoryService.Status())
	}
}

func writeJSON(t *testing.T, path string, value interface{}) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeTestPNG(t *testing.T, path string, width int, height int) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer file.Close()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(40 + x), G: uint8(90 + y), B: 180, A: 255})
		}
	}
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
	return path
}

func totalImported(result LegacyImportResult) int {
	total := 0
	for _, source := range result.Sources {
		total += source.Imported
	}
	return total
}

func clearWorkMemory(service *workmemory.Service) {
	for _, entry := range service.Timeline() {
		service.Delete(entry.ID)
	}
}

func assertImportedClipboardImage(t *testing.T, service *clipboardhistory.Service, expectedDir string) {
	t.Helper()
	for _, entry := range service.List("", 10) {
		if entry.Type != clipboardhistory.EntryImage {
			continue
		}
		assertPathUnder(t, entry.ImagePath, expectedDir)
		return
	}
	t.Fatal("expected imported clipboard image")
}

func assertImportedCaptureImage(t *testing.T, service *capturehistory.Service, expectedDir string) {
	t.Helper()
	entries := service.List("", 10)
	if len(entries) != 1 {
		t.Fatalf("expected one capture entry, got %#v", entries)
	}
	assertPathUnder(t, entries[0].ImagePath, expectedDir)
}

func assertImportedMemoryImage(t *testing.T, service *workmemory.Service, expectedDir string) {
	t.Helper()
	entries := service.Timeline()
	if len(entries) != 1 {
		t.Fatalf("expected one memory entry, got %#v", entries)
	}
	assertPathUnder(t, entries[0].ImagePath, expectedDir)
	if !contains(entries[0].Tags, "legacy_x_tools") || !entries[0].Favorite {
		t.Fatalf("expected legacy tags and favorite flag: %#v", entries[0])
	}
}

func assertPathUnder(t *testing.T, path string, dir string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected copied file %s: %v", path, err)
	}
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(path))
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		t.Fatalf("expected %s under %s", path, dir)
	}
}

func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
