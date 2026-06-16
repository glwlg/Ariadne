package migration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ariadne/internal/capturehistory"
	"ariadne/internal/clipboardhistory"
	"ariadne/internal/workmemory"
)

type LegacySourceStatus struct {
	Source        string `json:"source"`
	Path          string `json:"path"`
	Exists        bool   `json:"exists"`
	Count         int    `json:"count"`
	Bytes         int64  `json:"bytes"`
	ImageDir      string `json:"imageDir,omitempty"`
	ImageCount    int    `json:"imageCount,omitempty"`
	ImageBytes    int64  `json:"imageBytes,omitempty"`
	ImportedCount int    `json:"importedCount"`
	NeedsImport   bool   `json:"needsImport"`
	LastError     string `json:"lastError,omitempty"`
}

type LegacyDataStatus struct {
	Root        string               `json:"root"`
	Exists      bool                 `json:"exists"`
	NeedsImport bool                 `json:"needsImport"`
	Sources     []LegacySourceStatus `json:"sources"`
	TotalCount  int                  `json:"totalCount"`
	TotalBytes  int64                `json:"totalBytes"`
	Notes       []string             `json:"notes"`
}

type LegacyImportRequest struct {
	Sources []string `json:"sources,omitempty"`
	Limit   int      `json:"limit,omitempty"`
	DryRun  bool     `json:"dryRun,omitempty"`
}

type LegacyImportSourceResult struct {
	Source      string `json:"source"`
	Path        string `json:"path"`
	Found       int    `json:"found"`
	Imported    int    `json:"imported"`
	Skipped     int    `json:"skipped"`
	Failed      int    `json:"failed"`
	BeforeCount int    `json:"beforeCount"`
	AfterCount  int    `json:"afterCount"`
	Error       string `json:"error,omitempty"`
}

type LegacyImportResult struct {
	OK         bool                       `json:"ok"`
	Message    string                     `json:"message"`
	StartedAt  int64                      `json:"startedAt"`
	FinishedAt int64                      `json:"finishedAt"`
	DryRun     bool                       `json:"dryRun"`
	Sources    []LegacyImportSourceResult `json:"sources"`
}

type Service struct {
	root      string
	clipboard *clipboardhistory.Service
	capture   *capturehistory.Service
	memory    *workmemory.Service
}

func NewService(clipboard *clipboardhistory.Service, capture *capturehistory.Service, memory *workmemory.Service) *Service {
	return NewServiceWithRoot(defaultLegacyRoot(), clipboard, capture, memory)
}

func NewServiceWithRoot(root string, clipboard *clipboardhistory.Service, capture *capturehistory.Service, memory *workmemory.Service) *Service {
	return &Service{
		root:      strings.TrimSpace(root),
		clipboard: clipboard,
		capture:   capture,
		memory:    memory,
	}
}

func (s *Service) Status() LegacyDataStatus {
	root := s.root
	status := LegacyDataStatus{
		Root:  root,
		Notes: []string{"历史数据迁移只复制旧版本地历史，不删除或改写旧 x-tools 数据。"},
	}
	if root == "" {
		status.Notes = append(status.Notes, "未能定位旧版 x-tools 数据目录。")
		return status
	}
	if info, err := os.Stat(root); err == nil && info.IsDir() {
		status.Exists = true
	}
	status.Sources = []LegacySourceStatus{
		s.sourceStatus("clipboard_history", filepath.Join(root, "clipboard_history.json"), filepath.Join(root, "clipboard_images")),
		s.sourceStatus("capture_history", filepath.Join(root, "capture_history.json"), filepath.Join(root, "capture_images")),
		s.sourceStatus("work_memory", filepath.Join(root, "work_memory", "entries.json"), filepath.Join(root, "work_memory", "images")),
	}
	s.annotateImportProgress(status.Sources)
	for _, source := range status.Sources {
		status.TotalCount += source.Count
		status.TotalBytes += source.Bytes + source.ImageBytes
		if source.NeedsImport {
			status.NeedsImport = true
		}
	}
	if status.Exists && status.TotalCount > 0 && !status.NeedsImport {
		status.Notes = append(status.Notes, "旧历史数据已在 Ariadne 中发现对应导入记录，迁移入口默认隐藏。")
	}
	return status
}

func (s *Service) annotateImportProgress(sources []LegacySourceStatus) {
	for index := range sources {
		if sources[index].Count <= 0 {
			continue
		}
		switch sources[index].Source {
		case "clipboard_history":
			if s.clipboard != nil {
				sources[index].ImportedCount = s.clipboard.LegacyEntryCount()
			}
		case "capture_history":
			if s.capture != nil {
				sources[index].ImportedCount = s.capture.LegacyEntryCount()
			}
		case "work_memory":
			if s.memory != nil {
				sources[index].ImportedCount = s.memory.LegacyEntryCount()
			}
		}
		sources[index].NeedsImport = sources[index].ImportedCount < sources[index].Count
	}
}

func (s *Service) ImportLegacyData(request LegacyImportRequest) LegacyImportResult {
	started := time.Now()
	result := LegacyImportResult{
		OK:        true,
		StartedAt: started.Unix(),
		DryRun:    request.DryRun,
	}
	sources := normalizeSources(request.Sources)
	if len(sources) == 0 {
		sources = []string{"clipboard_history", "capture_history", "work_memory"}
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 1000
	}
	if limit > 5000 {
		limit = 5000
	}
	for _, source := range sources {
		switch source {
		case "clipboard_history":
			result.Sources = append(result.Sources, s.importClipboard(limit, request.DryRun))
		case "capture_history":
			result.Sources = append(result.Sources, s.importCapture(limit, request.DryRun))
		case "work_memory":
			result.Sources = append(result.Sources, s.importWorkMemory(limit, request.DryRun))
		default:
			result.Sources = append(result.Sources, LegacyImportSourceResult{
				Source: source,
				Error:  "不支持的迁移来源",
			})
			result.OK = false
		}
	}
	for _, source := range result.Sources {
		if source.Error != "" || source.Failed > 0 {
			result.OK = false
			break
		}
	}
	result.FinishedAt = time.Now().Unix()
	if request.DryRun {
		result.Message = "旧版历史数据预览完成"
	} else if result.OK {
		result.Message = "旧版历史数据迁移完成"
	} else {
		result.Message = "旧版历史数据迁移完成，但存在跳过或失败项"
	}
	return result
}

func (s *Service) sourceStatus(source string, path string, imageDir string) LegacySourceStatus {
	status := LegacySourceStatus{Source: source, Path: path, ImageDir: imageDir}
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		status.Exists = true
		status.Bytes = info.Size()
		status.Count, status.LastError = countJSONList(path)
	}
	status.ImageCount, status.ImageBytes = countFiles(imageDir)
	return status
}

func (s *Service) importClipboard(limit int, dryRun bool) LegacyImportSourceResult {
	path := filepath.Join(s.root, "clipboard_history.json")
	result := LegacyImportSourceResult{Source: "clipboard_history", Path: path}
	entries, skipped, failed, err := readLegacyClipboard(path, limit)
	result.Found = len(entries) + skipped + failed
	result.Skipped = skipped
	result.Failed = failed
	if err != nil {
		if os.IsNotExist(err) {
			return result
		}
		result.Error = err.Error()
		return result
	}
	if s.clipboard == nil || dryRun {
		result.Imported = len(entries)
		return result
	}
	before := s.clipboard.Status().Count
	result.BeforeCount = before
	s.clipboard.ImportLegacyEntries(entries)
	after := s.clipboard.Status().Count
	result.AfterCount = after
	result.Imported = maxInt(0, after-before)
	result.Skipped += maxInt(0, len(entries)-result.Imported)
	return result
}

func (s *Service) importCapture(limit int, dryRun bool) LegacyImportSourceResult {
	path := filepath.Join(s.root, "capture_history.json")
	result := LegacyImportSourceResult{Source: "capture_history", Path: path}
	entries, skipped, failed, err := readLegacyCapture(path, limit)
	result.Found = len(entries) + skipped + failed
	result.Skipped = skipped
	result.Failed = failed
	if err != nil {
		if os.IsNotExist(err) {
			return result
		}
		result.Error = err.Error()
		return result
	}
	if s.capture == nil || dryRun {
		result.Imported = len(entries)
		return result
	}
	before := s.capture.Status().Count
	result.BeforeCount = before
	s.capture.ImportLegacyEntries(entries)
	after := s.capture.Status().Count
	result.AfterCount = after
	result.Imported = maxInt(0, after-before)
	result.Skipped += maxInt(0, len(entries)-result.Imported)
	return result
}

func (s *Service) importWorkMemory(limit int, dryRun bool) LegacyImportSourceResult {
	path := filepath.Join(s.root, "work_memory", "entries.json")
	result := LegacyImportSourceResult{Source: "work_memory", Path: path}
	entries, skipped, failed, err := readLegacyWorkMemory(path, limit)
	result.Found = len(entries) + skipped + failed
	result.Skipped = skipped
	result.Failed = failed
	if err != nil {
		if os.IsNotExist(err) {
			return result
		}
		result.Error = err.Error()
		return result
	}
	if s.memory == nil || dryRun {
		result.Imported = len(entries)
		return result
	}
	before := s.memory.Status().EntryCount
	result.BeforeCount = before
	s.memory.ImportLegacyEntries(entries)
	after := s.memory.Status().EntryCount
	result.AfterCount = after
	result.Imported = maxInt(0, after-before)
	result.Skipped += maxInt(0, len(entries)-result.Imported)
	return result
}

func readLegacyClipboard(path string, limit int) ([]clipboardhistory.Entry, int, int, error) {
	var raw []legacyClipboardEntry
	if err := readJSONList(path, &raw); err != nil {
		return nil, 0, 0, err
	}
	entries := []clipboardhistory.Entry{}
	skipped := 0
	failed := 0
	for _, item := range raw {
		if len(entries) >= limit {
			skipped++
			continue
		}
		entryType := strings.TrimSpace(item.Type)
		switch entryType {
		case "text":
			text := strings.TrimSpace(item.Text)
			if text == "" {
				skipped++
				continue
			}
			entries = append(entries, clipboardhistory.Entry{
				ID:        strings.TrimSpace(item.ID),
				Type:      clipboardhistory.EntryText,
				Text:      text,
				CreatedAt: unixSeconds(item.CreatedAt),
				Pinned:    item.Pinned,
				Signature: strings.TrimSpace(item.Signature),
				Source:    "legacy_x_tools",
				Tags:      []string{"legacy_x_tools"},
			})
		case "image":
			imagePath := strings.TrimSpace(item.ImagePath)
			if imagePath == "" || !fileExists(imagePath) {
				failed++
				continue
			}
			entries = append(entries, clipboardhistory.Entry{
				ID:        strings.TrimSpace(item.ID),
				Type:      clipboardhistory.EntryImage,
				ImagePath: imagePath,
				CreatedAt: unixSeconds(item.CreatedAt),
				Pinned:    item.Pinned,
				Signature: strings.TrimSpace(item.Signature),
				Source:    "legacy_x_tools",
				Width:     item.Width,
				Height:    item.Height,
				Tags:      []string{"legacy_x_tools"},
			})
		default:
			skipped++
		}
	}
	return entries, skipped, failed, nil
}

func readLegacyCapture(path string, limit int) ([]capturehistory.Entry, int, int, error) {
	var raw []legacyCaptureEntry
	if err := readJSONList(path, &raw); err != nil {
		return nil, 0, 0, err
	}
	entries := []capturehistory.Entry{}
	skipped := 0
	failed := 0
	for _, item := range raw {
		if len(entries) >= limit {
			skipped++
			continue
		}
		imagePath := strings.TrimSpace(item.ImagePath)
		if imagePath == "" || !fileExists(imagePath) {
			failed++
			continue
		}
		entries = append(entries, capturehistory.Entry{
			ID:        strings.TrimSpace(item.ID),
			ImagePath: imagePath,
			SavedPath: strings.TrimSpace(item.SavedPath),
			CreatedAt: unixSeconds(item.CreatedAt),
			Source:    firstNonEmpty(item.Source, "legacy_x_tools"),
			Actions:   cleanStrings(item.Actions),
			Pinned:    item.Pinned,
			Width:     item.Width,
			Height:    item.Height,
			Tags:      []string{"legacy_x_tools"},
		})
	}
	return entries, skipped, failed, nil
}

func readLegacyWorkMemory(path string, limit int) ([]workmemory.Entry, int, int, error) {
	var raw []legacyWorkMemoryEntry
	if err := readJSONList(path, &raw); err != nil {
		return nil, 0, 0, err
	}
	entries := []workmemory.Entry{}
	skipped := 0
	failed := 0
	for _, item := range raw {
		if len(entries) >= limit {
			skipped++
			continue
		}
		imagePath := strings.TrimSpace(item.ImagePath)
		if imagePath != "" && !fileExists(imagePath) {
			failed++
			imagePath = ""
		}
		text := strings.TrimSpace(item.Text)
		ocrText := strings.TrimSpace(item.OCRText)
		if text == "" && ocrText == "" && imagePath == "" && strings.TrimSpace(item.Title+item.Summary) == "" {
			skipped++
			continue
		}
		source := firstNonEmpty(item.Source, "legacy_x_tools")
		contentType := firstNonEmpty(item.ContentType, "text")
		id := strings.TrimSpace(item.ID)
		if id == "" {
			id = fmt.Sprintf("legacy-memory-%d", len(entries)+1)
		}
		entries = append(entries, workmemory.Entry{
			ID:          id,
			Source:      source,
			ContentType: contentType,
			Title:       strings.TrimSpace(item.Title),
			Summary:     strings.TrimSpace(item.Summary),
			Text:        text,
			OCRText:     ocrText,
			WindowTitle: strings.TrimSpace(item.WindowTitle),
			AppName:     firstNonEmpty(item.AppName, item.ProcessName),
			CaptureID:   strings.TrimSpace(item.SourceID),
			ImagePath:   imagePath,
			Tags:        append([]string{"legacy_x_tools"}, cleanStrings(item.Tags)...),
			Favorite:    item.Favorite || item.Pinned,
			Sensitive:   item.Sensitive,
			CreatedAt:   unixSeconds(item.CreatedAt),
		})
	}
	return entries, skipped, failed, nil
}

type legacyClipboardEntry struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	Text      string  `json:"text"`
	ImagePath string  `json:"image_path"`
	CreatedAt float64 `json:"created_at"`
	Pinned    bool    `json:"pinned"`
	Signature string  `json:"signature"`
	Width     int     `json:"width"`
	Height    int     `json:"height"`
}

type legacyCaptureEntry struct {
	ID        string   `json:"id"`
	ImagePath string   `json:"image_path"`
	SavedPath string   `json:"saved_path"`
	CreatedAt float64  `json:"created_at"`
	Source    string   `json:"source"`
	Actions   []string `json:"actions"`
	Pinned    bool     `json:"pinned"`
	Width     int      `json:"width"`
	Height    int      `json:"height"`
}

type legacyWorkMemoryEntry struct {
	ID          string   `json:"id"`
	CreatedAt   float64  `json:"created_at"`
	Source      string   `json:"source"`
	SourceID    string   `json:"source_id"`
	ContentType string   `json:"content_type"`
	Title       string   `json:"title"`
	Summary     string   `json:"summary"`
	Text        string   `json:"text"`
	OCRText     string   `json:"ocr_text"`
	ImagePath   string   `json:"image_path"`
	AppName     string   `json:"app_name"`
	ProcessName string   `json:"process_name"`
	WindowTitle string   `json:"window_title"`
	Tags        []string `json:"tags"`
	Favorite    bool     `json:"favorite"`
	Pinned      bool     `json:"pinned"`
	Sensitive   bool     `json:"sensitive"`
}

func readJSONList(path string, target interface{}) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

func countJSONList(path string) (int, string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err.Error()
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return 0, err.Error()
	}
	return len(items), ""
}

func countFiles(dir string) (int, int64) {
	if strings.TrimSpace(dir) == "" {
		return 0, 0
	}
	count := 0
	var bytes int64
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry == nil || entry.IsDir() {
			return nil
		}
		info, statErr := entry.Info()
		if statErr == nil {
			count++
			bytes += info.Size()
		}
		return nil
	})
	return count, bytes
}

func defaultLegacyRoot() string {
	base := os.Getenv("APPDATA")
	if strings.TrimSpace(base) == "" {
		if dir, err := os.UserConfigDir(); err == nil {
			base = dir
		}
	}
	if strings.TrimSpace(base) == "" {
		base = "."
	}
	return filepath.Join(base, "x-tools")
}

func normalizeSources(sources []string) []string {
	seen := map[string]bool{}
	cleaned := []string{}
	for _, source := range sources {
		source = strings.ToLower(strings.TrimSpace(source))
		if source == "" || seen[source] {
			continue
		}
		seen[source] = true
		cleaned = append(cleaned, source)
	}
	sort.Strings(cleaned)
	return cleaned
}

func unixSeconds(value float64) int64 {
	if value <= 0 {
		return time.Now().Unix()
	}
	return int64(value)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func cleanStrings(items []string) []string {
	seen := map[string]bool{}
	cleaned := []string{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		cleaned = append(cleaned, item)
	}
	return cleaned
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
