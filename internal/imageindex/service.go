package imageindex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"ariadne/internal/capturehistory"
	"ariadne/internal/clipboardhistory"
	"ariadne/internal/contracts"
	"ariadne/internal/ocr"
)

const (
	SourceCaptureHistory   = "capture_history"
	SourceClipboardHistory = "clipboard_history"
)

type CaptureProvider interface {
	List(query string, limit int) []capturehistory.Entry
	Entry(id string) capturehistory.Entry
}

type ClipboardProvider interface {
	List(query string, limit int) []clipboardhistory.Entry
	Entry(id string) clipboardhistory.Entry
}

type OCRProvider interface {
	RecognizeCapture(captureID string) ocr.Result
	RecognizeClipboardImage(clipboardID string) ocr.Result
	Status() ocr.Status
}

type Entry struct {
	ID           string     `json:"id"`
	Source       string     `json:"source"`
	SourceID     string     `json:"sourceId"`
	ImagePath    string     `json:"imagePath"`
	Text         string     `json:"text,omitempty"`
	Lines        []ocr.Line `json:"lines,omitempty"`
	Provider     string     `json:"provider,omitempty"`
	CreatedAt    int64      `json:"createdAt,omitempty"`
	IndexedAt    int64      `json:"indexedAt"`
	Width        int        `json:"width,omitempty"`
	Height       int        `json:"height,omitempty"`
	OK           bool       `json:"ok"`
	Sensitive    bool       `json:"sensitive"`
	Redacted     bool       `json:"redacted"`
	Error        string     `json:"error,omitempty"`
	RecognizedAt int64      `json:"recognizedAt,omitempty"`
}

type Status struct {
	Path           string  `json:"path"`
	Count          int     `json:"count"`
	IndexedCount   int     `json:"indexedCount"`
	SensitiveCount int     `json:"sensitiveCount"`
	FailedCount    int     `json:"failedCount"`
	LastRunAt      int64   `json:"lastRunAt,omitempty"`
	LastIndexedAt  int64   `json:"lastIndexedAt,omitempty"`
	LastError      string  `json:"lastError,omitempty"`
	OCRAvailable   bool    `json:"ocrAvailable"`
	OCRProvider    string  `json:"ocrProvider,omitempty"`
	Entries        []Entry `json:"entries,omitempty"`
}

type IndexRequest struct {
	Sources []string `json:"sources,omitempty"`
	Limit   int      `json:"limit,omitempty"`
	Force   bool     `json:"force"`
}

type BatchResult struct {
	OK         bool    `json:"ok"`
	StartedAt  int64   `json:"startedAt"`
	FinishedAt int64   `json:"finishedAt"`
	Indexed    int     `json:"indexed"`
	Skipped    int     `json:"skipped"`
	Failed     int     `json:"failed"`
	LastError  string  `json:"lastError,omitempty"`
	Entries    []Entry `json:"entries,omitempty"`
}

type RetentionResult struct {
	OK             bool  `json:"ok"`
	RetentionDays  int   `json:"retentionDays"`
	CutoffAt       int64 `json:"cutoffAt,omitempty"`
	Removed        int   `json:"removed"`
	RemovedStale   int   `json:"removedStale"`
	RemovedExpired int   `json:"removedExpired"`
	Kept           int   `json:"kept"`
	RemainingCount int   `json:"remainingCount"`
	AppliedAt      int64 `json:"appliedAt"`
}

type Service struct {
	mu        sync.RWMutex
	path      string
	entries   []Entry
	captures  CaptureProvider
	clipboard ClipboardProvider
	ocr       OCRProvider
	now       func() time.Time
	lastRunAt int64
	lastError string
}

func NewService(captures CaptureProvider, clipboard ClipboardProvider, ocrProvider OCRProvider) *Service {
	return NewServiceWithPath(defaultIndexPath(), captures, clipboard, ocrProvider)
}

func NewServiceWithPath(path string, captures CaptureProvider, clipboard ClipboardProvider, ocrProvider OCRProvider) *Service {
	service := &Service{
		path:      path,
		captures:  captures,
		clipboard: clipboard,
		ocr:       ocrProvider,
		now:       time.Now,
	}
	service.load()
	return service
}

func (s *Service) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.statusLocked(false)
}

func (s *Service) List(query string, limit int) []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.listLocked(query, limit)
}

func (s *Service) IndexRecent(request IndexRequest) BatchResult {
	startedAt := s.now().Unix()
	limit := request.Limit
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	sources := normalizeSources(request.Sources)
	result := BatchResult{StartedAt: startedAt, OK: true}

	for _, source := range sources {
		switch source {
		case SourceCaptureHistory:
			if s.captures == nil {
				result.Failed++
				result.LastError = "截图历史服务不可用"
				continue
			}
			for _, item := range s.captures.List("", limit) {
				entry, skipped := s.indexCaptureEntry(item, request.Force)
				if skipped {
					result.Skipped++
					continue
				}
				result.Entries = append(result.Entries, entry)
				if entry.OK {
					result.Indexed++
				} else {
					result.Failed++
					result.LastError = entry.Error
				}
			}
		case SourceClipboardHistory:
			if s.clipboard == nil {
				result.Failed++
				result.LastError = "剪贴板历史服务不可用"
				continue
			}
			for _, item := range s.clipboard.List("", limit) {
				if item.Type != clipboardhistory.EntryImage {
					continue
				}
				entry, skipped := s.indexClipboardEntry(item, request.Force)
				if skipped {
					result.Skipped++
					continue
				}
				result.Entries = append(result.Entries, entry)
				if entry.OK {
					result.Indexed++
				} else {
					result.Failed++
					result.LastError = entry.Error
				}
			}
		}
	}

	result.FinishedAt = s.now().Unix()
	result.OK = result.Failed == 0
	s.mu.Lock()
	s.lastRunAt = result.FinishedAt
	s.lastError = result.LastError
	s.mu.Unlock()
	return result
}

func (s *Service) IndexCapture(captureID string, force bool) Entry {
	if s.captures == nil {
		return s.recordEntry(Entry{ID: entryID(SourceCaptureHistory, captureID), Source: SourceCaptureHistory, SourceID: strings.TrimSpace(captureID), OK: false, Error: "截图历史服务不可用"})
	}
	item := s.captures.Entry(captureID)
	if item.ID == "" {
		return s.recordEntry(Entry{ID: entryID(SourceCaptureHistory, captureID), Source: SourceCaptureHistory, SourceID: strings.TrimSpace(captureID), OK: false, Error: "未找到截图记录"})
	}
	entry, _ := s.indexCaptureEntry(item, force)
	return entry
}

func (s *Service) IndexClipboardImage(clipboardID string, force bool) Entry {
	if s.clipboard == nil {
		return s.recordEntry(Entry{ID: entryID(SourceClipboardHistory, clipboardID), Source: SourceClipboardHistory, SourceID: strings.TrimSpace(clipboardID), OK: false, Error: "剪贴板历史服务不可用"})
	}
	item := s.clipboard.Entry(clipboardID)
	if item.ID == "" || item.Type != clipboardhistory.EntryImage {
		return s.recordEntry(Entry{ID: entryID(SourceClipboardHistory, clipboardID), Source: SourceClipboardHistory, SourceID: strings.TrimSpace(clipboardID), OK: false, Error: "未找到剪贴板图片"})
	}
	entry, _ := s.indexClipboardEntry(item, force)
	return entry
}

func (s *Service) ApplyRetentionPolicy(retentionDays int) RetentionResult {
	if retentionDays <= 0 {
		retentionDays = 30
	}
	now := s.now()
	cutoff := now.Add(-time.Duration(retentionDays) * 24 * time.Hour).Unix()
	result := RetentionResult{
		OK:            true,
		RetentionDays: retentionDays,
		CutoffAt:      cutoff,
		AppliedAt:     now.Unix(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.entries[:0]
	for _, entry := range s.entries {
		expired := entryAgeTime(entry) != 0 && entryAgeTime(entry) < cutoff
		stale := !s.sourceExists(entry)
		if expired || stale {
			result.Removed++
			if expired {
				result.RemovedExpired++
			}
			if stale {
				result.RemovedStale++
			}
			continue
		}
		result.Kept++
		next = append(next, entry)
	}
	s.entries = next
	s.lastRunAt = result.AppliedAt
	if result.Removed > 0 {
		s.saveLockedWithStatus()
	}
	result.RemainingCount = len(s.entries)
	return result
}

func (s *Service) Search(query string) []contracts.SearchResult {
	if isIndexCommand(query) {
		return []contracts.SearchResult{indexCommandResult()}
	}
	query = imageQuery(query)
	if len([]rune(query)) < 2 {
		return nil
	}
	entries := s.List(query, 20)
	results := make([]contracts.SearchResult, 0, len(entries))
	normalized := strings.ToLower(query)
	for _, entry := range entries {
		if entry.Sensitive || entry.Redacted || strings.TrimSpace(entry.Text) == "" {
			continue
		}
		score := scoreEntry(entry, normalized)
		if score <= 0 {
			continue
		}
		results = append(results, entryToResult(entry, score))
	}
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	return results
}

func (s *Service) sourceExists(entry Entry) bool {
	switch normalizeSource(entry.Source) {
	case SourceCaptureHistory:
		if s.captures == nil {
			return true
		}
		return s.captures.Entry(entry.SourceID).ID != ""
	case SourceClipboardHistory:
		if s.clipboard == nil {
			return true
		}
		item := s.clipboard.Entry(entry.SourceID)
		return item.ID != "" && item.Type == clipboardhistory.EntryImage
	default:
		return true
	}
}

func entryAgeTime(entry Entry) int64 {
	if entry.CreatedAt > 0 {
		return entry.CreatedAt
	}
	return entry.IndexedAt
}

func isIndexCommand(query string) bool {
	normalized := strings.ToLower(strings.TrimSpace(query))
	if normalized == "" {
		return false
	}
	switch normalized {
	case "img index", "image index", "images index", "ocr index", "index images", "图片索引", "索引图片", "图像索引":
		return true
	default:
		return strings.HasPrefix(normalized, "img index ") || strings.HasPrefix(normalized, "image index ") || strings.HasPrefix(normalized, "ocr index ")
	}
}

func indexCommandResult() contracts.SearchResult {
	return contracts.SearchResult{
		ID:       "image-index-recent",
		Type:     contracts.ResultPluginTrigger,
		Title:    "索引最近图片 OCR",
		Subtitle: "图片索引 · 截图历史 + 剪贴板图片",
		Detail:   "对最近截图和剪贴板图片执行本地 OCR，并把非敏感文字加入本地图片索引。",
		Icon:     "image",
		Score:    91,
		Tags:     []string{"图片索引", "OCR", "截图历史", "剪贴板图片"},
		Payload: map[string]interface{}{
			"command": "image_index_recent",
		},
		Preview: contracts.PreviewDescriptor{
			Kind:     contracts.PreviewText,
			Title:    "索引最近图片 OCR",
			Subtitle: "本地 RapidOCR，不外发图片",
			Text:     "扫描最近截图历史和剪贴板图片。疑似敏感 OCR 结果只记录屏蔽状态，不进入搜索正文。",
			Meta: []contracts.LabelValue{
				{Label: "范围", Value: "截图历史 + 剪贴板图片"},
				{Label: "隐私", Value: "敏感文字默认屏蔽"},
			},
		},
		Actions: []contracts.PreviewAction{
			{
				ID:       "image_index_recent",
				Label:    "开始索引",
				Icon:     "run",
				Kind:     contracts.ActionPlugin,
				Shortcut: "Enter",
				Payload: map[string]interface{}{
					"limit": 30,
				},
				Feedback: &contracts.ActionFeedback{SuccessLabel: "已索引图片", DurationMS: 1800},
			},
		},
	}
}

func (s *Service) indexCaptureEntry(item capturehistory.Entry, force bool) (Entry, bool) {
	item.ID = strings.TrimSpace(item.ID)
	if item.ID == "" || strings.TrimSpace(item.ImagePath) == "" {
		return Entry{}, true
	}
	if !force && s.hasEntry(SourceCaptureHistory, item.ID) {
		return Entry{}, true
	}
	if s.ocr == nil {
		return s.recordEntry(entryFromCapture(item, ocr.Result{OK: false, Error: "OCR 服务不可用"}, s.now())), false
	}
	return s.recordEntry(entryFromCapture(item, s.ocr.RecognizeCapture(item.ID), s.now())), false
}

func (s *Service) indexClipboardEntry(item clipboardhistory.Entry, force bool) (Entry, bool) {
	item.ID = strings.TrimSpace(item.ID)
	if item.ID == "" || item.Type != clipboardhistory.EntryImage || strings.TrimSpace(item.ImagePath) == "" {
		return Entry{}, true
	}
	if !force && s.hasEntry(SourceClipboardHistory, item.ID) {
		return Entry{}, true
	}
	if s.ocr == nil {
		return s.recordEntry(entryFromClipboard(item, ocr.Result{OK: false, Error: "OCR 服务不可用"}, s.now())), false
	}
	return s.recordEntry(entryFromClipboard(item, s.ocr.RecognizeClipboardImage(item.ID), s.now())), false
}

func (s *Service) hasEntry(source string, sourceID string) bool {
	id := entryID(source, sourceID)
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, entry := range s.entries {
		if entry.ID == id {
			return true
		}
	}
	return false
}

func (s *Service) recordEntry(entry Entry) Entry {
	entry = normalizeEntry(entry)
	s.mu.Lock()
	defer s.mu.Unlock()
	next := make([]Entry, 0, len(s.entries)+1)
	for _, existing := range s.entries {
		if existing.ID != entry.ID {
			next = append(next, existing)
		}
	}
	s.entries = append([]Entry{entry}, next...)
	s.lastRunAt = entry.IndexedAt
	if !entry.OK {
		s.lastError = entry.Error
	} else {
		s.lastError = ""
	}
	s.saveLockedWithStatus()
	return entry
}

func (s *Service) listLocked(query string, limit int) []Entry {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	normalized := strings.ToLower(strings.TrimSpace(query))
	items := make([]Entry, 0, len(s.entries))
	for _, entry := range s.entries {
		if normalized == "" || entryMatches(entry, normalized) {
			items = append(items, entry)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].IndexedAt > items[j].IndexedAt
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return append([]Entry{}, items...)
}

func (s *Service) statusLocked(includeEntries bool) Status {
	status := Status{Path: s.path, Count: len(s.entries), LastRunAt: s.lastRunAt, LastError: s.lastError}
	if s.ocr != nil {
		ocrStatus := s.ocr.Status()
		status.OCRAvailable = ocrStatus.Available
		status.OCRProvider = ocrStatus.Provider
	}
	for _, entry := range s.entries {
		if entry.OK {
			status.IndexedCount++
		} else {
			status.FailedCount++
		}
		if entry.Sensitive || entry.Redacted {
			status.SensitiveCount++
		}
		if entry.IndexedAt > status.LastIndexedAt {
			status.LastIndexedAt = entry.IndexedAt
		}
	}
	if includeEntries {
		status.Entries = s.listLocked("", 100)
	}
	return status
}

func (s *Service) load() {
	if s.path == "" {
		return
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var state struct {
		Version int     `json:"version"`
		Entries []Entry `json:"entries"`
	}
	if json.Unmarshal(raw, &state) != nil {
		return
	}
	for _, entry := range state.Entries {
		entry = normalizeEntry(entry)
		if entry.ID != "" {
			s.entries = append(s.entries, entry)
		}
	}
}

func (s *Service) saveLockedWithStatus() {
	if err := s.saveLocked(); err != nil {
		s.lastError = err.Error()
	}
}

func (s *Service) saveLocked() error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	state := struct {
		Version int     `json:"version"`
		Entries []Entry `json:"entries"`
	}{
		Version: 1,
		Entries: s.entries,
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o600)
}

func entryFromCapture(item capturehistory.Entry, result ocr.Result, now time.Time) Entry {
	return entryFromOCR(SourceCaptureHistory, item.ID, item.ImagePath, item.CreatedAt, item.Width, item.Height, result, now)
}

func entryFromClipboard(item clipboardhistory.Entry, result ocr.Result, now time.Time) Entry {
	return entryFromOCR(SourceClipboardHistory, item.ID, item.ImagePath, item.CreatedAt, item.Width, item.Height, result, now)
}

func entryFromOCR(source string, sourceID string, imagePath string, createdAt int64, width int, height int, result ocr.Result, now time.Time) Entry {
	text := strings.TrimSpace(result.Text)
	redacted := result.Sensitive
	errorText := strings.TrimSpace(result.Error)
	if redacted {
		text = ""
		if errorText == "" {
			errorText = "OCR 结果疑似敏感，未写入图片索引文本"
		}
	}
	return Entry{
		ID:           entryID(source, sourceID),
		Source:       source,
		SourceID:     strings.TrimSpace(sourceID),
		ImagePath:    strings.TrimSpace(imagePath),
		Text:         text,
		Lines:        result.Lines,
		Provider:     result.Provider,
		CreatedAt:    createdAt,
		IndexedAt:    now.Unix(),
		Width:        firstPositive(width, result.Width),
		Height:       firstPositive(height, result.Height),
		OK:           result.OK,
		Sensitive:    result.Sensitive,
		Redacted:     redacted,
		Error:        errorText,
		RecognizedAt: result.RecognizedAt,
	}
}

func normalizeEntry(entry Entry) Entry {
	entry.Source = normalizeSource(entry.Source)
	entry.SourceID = strings.TrimSpace(entry.SourceID)
	entry.ImagePath = strings.TrimSpace(entry.ImagePath)
	entry.Text = strings.TrimSpace(entry.Text)
	entry.Provider = strings.TrimSpace(entry.Provider)
	entry.Error = strings.TrimSpace(entry.Error)
	if entry.ID == "" {
		entry.ID = entryID(entry.Source, entry.SourceID)
	}
	if entry.IndexedAt == 0 {
		entry.IndexedAt = time.Now().Unix()
	}
	if entry.Sensitive {
		entry.Redacted = true
		entry.Text = ""
		if entry.Error == "" {
			entry.Error = "OCR 结果疑似敏感，未写入图片索引文本"
		}
	}
	if entry.Provider == "" {
		entry.Provider = "rapidocr_onnxruntime"
	}
	return entry
}

func entryToResult(entry Entry, score float64) contracts.SearchResult {
	title := "图片 OCR " + formatTime(entry.CreatedAt)
	if entry.Source == SourceCaptureHistory {
		title = "截图 OCR " + formatTime(entry.CreatedAt)
	}
	if entry.Source == SourceClipboardHistory {
		title = "剪贴板图片 OCR " + formatTime(entry.CreatedAt)
	}
	resultType := contracts.ResultCapture
	if entry.Source == SourceClipboardHistory {
		resultType = contracts.ResultClipboard
	}
	actions := []contracts.PreviewAction{
		contracts.CopyAction("copy_image_ocr_text", "复制 OCR 文本", entry.Text, "Enter"),
	}
	if entry.Source == SourceCaptureHistory {
		actions = append(actions,
			contracts.PreviewAction{
				ID:      "open_capture_ocr_image",
				Label:   "打开图片",
				Icon:    "open",
				Kind:    contracts.ActionOpen,
				Payload: map[string]interface{}{"path": entry.ImagePath},
			},
			contracts.PreviewAction{
				ID:      "open_capture_ocr_parent",
				Label:   "打开所在文件夹",
				Icon:    "folder",
				Kind:    contracts.ActionOpenParent,
				Payload: map[string]interface{}{"path": entry.ImagePath},
			},
		)
	} else {
		actions = append(actions,
			contracts.PreviewAction{
				ID:    "copy_clipboard_image",
				Label: "复制图片",
				Icon:  "copy",
				Kind:  contracts.ActionPlugin,
				Payload: map[string]interface{}{
					"clipboardId": entry.SourceID,
				},
				Feedback: &contracts.ActionFeedback{SuccessLabel: "图片已复制", DurationMS: 1400},
			},
			contracts.PreviewAction{
				ID:    "pin_clipboard_image",
				Label: "贴到屏幕",
				Icon:  "pin",
				Kind:  contracts.ActionPin,
				Payload: map[string]interface{}{
					"clipboardId": entry.SourceID,
				},
				Feedback: &contracts.ActionFeedback{SuccessLabel: "已创建贴图", DurationMS: 1400},
			},
		)
	}
	return contracts.SearchResult{
		ID:       "image-index-" + entry.ID,
		Type:     resultType,
		Title:    title,
		Subtitle: sourceLabel(entry.Source) + " · OCR 索引",
		Detail:   entry.Text,
		Icon:     "image",
		Score:    score,
		Tags:     []string{"图片索引", "OCR", sourceLabel(entry.Source), fmt.Sprintf("%dx%d", entry.Width, entry.Height)},
		Payload: map[string]interface{}{
			"source":    entry.Source,
			"sourceId":  entry.SourceID,
			"imagePath": entry.ImagePath,
		},
		Preview: contracts.PreviewDescriptor{
			Kind:      contracts.PreviewImage,
			Title:     title,
			Subtitle:  sourceLabel(entry.Source),
			Text:      previewText(entry.Text),
			ImageHint: fmt.Sprintf("%dx%d · %s", entry.Width, entry.Height, entry.Provider),
			Meta: []contracts.LabelValue{
				{Label: "来源", Value: sourceLabel(entry.Source)},
				{Label: "尺寸", Value: fmt.Sprintf("%dx%d", entry.Width, entry.Height)},
				{Label: "OCR", Value: entry.Provider},
			},
			Evidence: []contracts.LabelValue{
				{Label: "索引时间", Value: formatTime(entry.IndexedAt)},
				{Label: "图片路径", Value: entry.ImagePath},
			},
		},
		Actions: actions,
	}
}

func scoreEntry(entry Entry, query string) float64 {
	score := 0.0
	text := strings.ToLower(entry.Text)
	if strings.Contains(text, query) {
		score = 88
		if strings.HasPrefix(text, query) {
			score += 8
		}
	}
	if strings.Contains(strings.ToLower(entry.ImagePath), query) {
		score += 8
	}
	if strings.Contains(strings.ToLower(sourceLabel(entry.Source)), query) {
		score += 4
	}
	return score
}

func entryMatches(entry Entry, query string) bool {
	if query == "" {
		return true
	}
	if entry.Sensitive || entry.Redacted {
		return strings.Contains(strings.ToLower(sourceLabel(entry.Source)), query) || strings.Contains(strings.ToLower(entry.Error), query)
	}
	parts := []string{
		entry.Text,
		entry.ImagePath,
		entry.Provider,
		sourceLabel(entry.Source),
		fmt.Sprintf("%dx%d", entry.Width, entry.Height),
	}
	return strings.Contains(strings.ToLower(strings.Join(parts, " ")), query)
}

func imageQuery(query string) string {
	query = strings.TrimSpace(query)
	parts := strings.Fields(query)
	if len(parts) == 0 {
		return ""
	}
	switch strings.ToLower(parts[0]) {
	case "img", "image", "images", "ocr", "图片", "图像", "截图文字", "图片文字":
		return strings.TrimSpace(strings.TrimPrefix(query, parts[0]))
	default:
		return query
	}
}

func normalizeSources(sources []string) []string {
	if len(sources) == 0 {
		return []string{SourceCaptureHistory, SourceClipboardHistory}
	}
	result := make([]string, 0, len(sources))
	for _, source := range sources {
		source = normalizeSource(source)
		if source == "" {
			continue
		}
		if source != SourceCaptureHistory && source != SourceClipboardHistory {
			continue
		}
		if !contains(result, source) {
			result = append(result, source)
		}
	}
	if len(result) == 0 {
		return []string{SourceCaptureHistory, SourceClipboardHistory}
	}
	return result
}

func normalizeSource(source string) string {
	source = strings.ToLower(strings.TrimSpace(source))
	switch source {
	case "capture", "capture_history", "screenshot", "截图":
		return SourceCaptureHistory
	case "clipboard", "clipboard_history", "剪贴板":
		return SourceClipboardHistory
	default:
		return source
	}
}

func entryID(source string, sourceID string) string {
	source = normalizeSource(source)
	sourceID = strings.TrimSpace(sourceID)
	if source == "" || sourceID == "" {
		return ""
	}
	return source + "-" + sourceID
}

func defaultIndexPath() string {
	base := os.Getenv("APPDATA")
	if strings.TrimSpace(base) == "" {
		base = "."
	}
	return filepath.Join(base, "Ariadne", "image_index.json")
}

func previewText(text string) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= 600 {
		return string(runes)
	}
	return string(runes[:600]) + "..."
}

func formatTime(unix int64) string {
	if unix == 0 {
		return ""
	}
	return time.Unix(unix, 0).Format("2006-01-02 15:04")
}

func sourceLabel(source string) string {
	switch normalizeSource(source) {
	case SourceCaptureHistory:
		return "截图历史"
	case SourceClipboardHistory:
		return "剪贴板图片"
	default:
		return source
	}
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
