package clipboardhistory

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"ariadne/internal/capturehistory"
	"ariadne/internal/contracts"
	"ariadne/internal/imagepreview"
	"ariadne/internal/qrscan"
)

type EntryType string

const (
	EntryText  EntryType = "text"
	EntryImage EntryType = "image"
)

var errClipboardUnsupported = errors.New("system clipboard is only supported on windows")

func WriteImageToSystemClipboard(path string) error {
	return writeImageToSystemClipboard(path)
}

type Entry struct {
	ID              string    `json:"id"`
	Type            EntryType `json:"type"`
	Text            string    `json:"text"`
	ImagePath       string    `json:"imagePath,omitempty"`
	ThumbnailPath   string    `json:"thumbnailPath,omitempty"`
	ThumbnailWidth  int       `json:"thumbnailWidth,omitempty"`
	ThumbnailHeight int       `json:"thumbnailHeight,omitempty"`
	ThumbnailBytes  int64     `json:"thumbnailBytes,omitempty"`
	CreatedAt       int64     `json:"createdAt"`
	Pinned          bool      `json:"pinned"`
	Signature       string    `json:"signature"`
	ContentType     string    `json:"contentType"`
	Source          string    `json:"source"`
	Summary         string    `json:"summary"`
	Width           int       `json:"width,omitempty"`
	Height          int       `json:"height,omitempty"`
	Bytes           int64     `json:"bytes,omitempty"`
	Tags            []string  `json:"tags,omitempty"`
}

type Status struct {
	Path             string  `json:"path"`
	ImageDir         string  `json:"imageDir"`
	ThumbnailDir     string  `json:"thumbnailDir,omitempty"`
	Count            int     `json:"count"`
	PinnedCount      int     `json:"pinnedCount"`
	ImageCount       int     `json:"imageCount"`
	ThumbnailCount   int     `json:"thumbnailCount"`
	ThumbnailBytes   int64   `json:"thumbnailBytes"`
	LastEntryAt      int64   `json:"lastEntryAt,omitempty"`
	LastSaveError    string  `json:"lastSaveError,omitempty"`
	WatcherEnabled   bool    `json:"watcherEnabled"`
	WatcherRunning   bool    `json:"watcherRunning"`
	LastWatcherAt    int64   `json:"lastWatcherAt,omitempty"`
	LastWatcherError string  `json:"lastWatcherError,omitempty"`
	Entries          []Entry `json:"entries,omitempty"`
}

type CollectCurrentResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Entry   Entry  `json:"entry,omitempty"`
	Status  Status `json:"status"`
}

type RetentionResult struct {
	OK             bool  `json:"ok"`
	RetentionDays  int   `json:"retentionDays"`
	KeepPinned     bool  `json:"keepPinned"`
	CutoffAt       int64 `json:"cutoffAt,omitempty"`
	Removed        int   `json:"removed"`
	RemovedImages  int   `json:"removedImages"`
	Kept           int   `json:"kept"`
	KeptPinned     int   `json:"keptPinned"`
	RemainingCount int   `json:"remainingCount"`
	AppliedAt      int64 `json:"appliedAt"`
}

type thumbnailBackfillResult struct {
	Created int
	Skipped int
	Failed  int
}

type Service struct {
	mu                    sync.RWMutex
	path                  string
	imageDir              string
	thumbnailDir          string
	entries               []Entry
	maxEntries            int
	lastError             string
	watcherEnabled        bool
	watcherRunning        bool
	watcherPrimed         bool
	lastWatcherAt         int64
	lastWatcherError      string
	lastWatcherSignature  string
	watcherStop           chan struct{}
	watcherInterval       time.Duration
	clipboardReader       func(string, string) (Entry, error)
	watcherSourceOverride string
	captureSink           CaptureSink
	observers             []func(Entry)
}

type CaptureSink interface {
	AddPNG(data []byte, width int, height int, source string, savedPath string, actions []string) capturehistory.Status
}

func NewService(captureSinks ...CaptureSink) *Service {
	service := NewServiceWithPath(defaultHistoryPath())
	if len(captureSinks) > 0 {
		service.captureSink = captureSinks[0]
	}
	return service
}

func NewServiceWithPath(path string) *Service {
	return NewServiceWithPaths(path, defaultImageDir(path))
}

func NewServiceWithPaths(path string, imageDir string) *Service {
	service := &Service{
		path:            path,
		imageDir:        imageDir,
		thumbnailDir:    defaultThumbnailDir(imageDir),
		maxEntries:      200,
		watcherInterval: time.Second,
		clipboardReader: readSystemClipboardEntry,
	}
	service.load()
	service.ensureThumbnails()
	return service
}

func (s *Service) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.statusLocked(false)
}

func (s *Service) LegacyEntryCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, entry := range s.entries {
		if entry.Source == "legacy_x_tools" || stringListContainsFold(entry.Tags, "legacy_x_tools") {
			count++
		}
	}
	return count
}

func (s *Service) List(query string, limit int) []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.listLocked(query, limit)
}

func RegisterEntryObserver(service *Service, observer func(Entry)) {
	if service == nil {
		return
	}
	service.registerEntryObserver(observer)
}

func (s *Service) registerEntryObserver(observer func(Entry)) {
	if observer == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observers = append(s.observers, observer)
}

func (s *Service) AddText(text string, source string) Status {
	entry := makeTextEntry(text, source)
	s.mu.Lock()
	entry = s.addEntryLocked(entry)
	status := s.statusLocked(true)
	s.mu.Unlock()
	s.notifyEntryObservers(entry)
	return status
}

func (s *Service) AddPNG(pngBytes []byte, source string) Status {
	entry, err := s.makeImageEntryFromPNG(pngBytes, source)
	s.mu.Lock()
	if err != nil {
		s.lastError = err.Error()
		status := s.statusLocked(true)
		s.mu.Unlock()
		return status
	}
	entry = s.addEntryLocked(entry)
	status := s.statusLocked(true)
	s.mu.Unlock()
	s.notifyEntryObservers(entry)
	return status
}

func (s *Service) ImportLegacyEntries(entries []Entry) Status {
	imported := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		entry.Source = strings.TrimSpace(entry.Source)
		if entry.Source == "" {
			entry.Source = "legacy_x_tools"
		}
		if entry.Type == EntryImage {
			raw, err := os.ReadFile(strings.TrimSpace(entry.ImagePath))
			if err != nil || len(raw) == 0 {
				continue
			}
			imageEntry, err := s.makeImageEntryFromPNG(raw, entry.Source)
			if err != nil {
				continue
			}
			imageEntry.ID = strings.TrimSpace(entry.ID)
			imageEntry.CreatedAt = entry.CreatedAt
			imageEntry.Pinned = entry.Pinned
			imageEntry.Signature = strings.TrimSpace(entry.Signature)
			if imageEntry.Signature == "" {
				imageEntry.Signature = "img:" + sha1HexBytes(raw)
			}
			imageEntry.Tags = append(imageEntry.Tags, cleanStrings(entry.Tags)...)
			imported = append(imported, imageEntry)
			continue
		}
		imported = append(imported, entry)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, entry := range imported {
		s.addEntryLocked(entry)
	}
	return s.statusLocked(true)
}

func (s *Service) CollectCurrent(source string) Status {
	return s.CollectCurrentEntry(source).Status
}

func (s *Service) CollectCurrentEntry(source string) CollectCurrentResult {
	reader := s.clipboardReader
	if reader == nil {
		reader = readSystemClipboardEntry
	}
	entry, err := reader(s.imageDir, source)
	s.mu.Lock()
	if err != nil {
		if errors.Is(err, errClipboardUnsupported) {
			s.lastError = "当前平台不支持原生剪贴板读取"
		} else {
			s.lastError = err.Error()
		}
		status := s.statusLocked(true)
		message := s.lastError
		s.mu.Unlock()
		return CollectCurrentResult{OK: false, Message: message, Status: status}
	}
	entry = normalizeEntry(entry)
	if !entryIsValid(entry) {
		s.removeEntryFile(entry)
		s.lastError = "当前剪贴板没有可收集的文本或图片"
		status := s.statusLocked(true)
		message := s.lastError
		s.mu.Unlock()
		return CollectCurrentResult{OK: false, Message: message, Status: status}
	}
	s.lastError = ""
	entry = s.addEntryLocked(entry)
	status := s.statusLocked(true)
	s.mu.Unlock()
	s.notifyEntryObservers(entry)
	return CollectCurrentResult{OK: entry.ID != "", Message: "已读取当前剪贴板", Entry: entry, Status: status}
}

func (s *Service) CopyImage(id string) contracts.ActionResult {
	id = strings.TrimSpace(id)
	s.mu.RLock()
	var entry Entry
	for _, candidate := range s.entries {
		if candidate.ID == id {
			entry = candidate
			break
		}
	}
	s.mu.RUnlock()
	if entry.ID == "" {
		return contracts.ActionResult{OK: false, Message: "未找到剪贴板图片"}
	}
	if entry.Type != EntryImage {
		return contracts.ActionResult{OK: false, Message: "该记录不是图片"}
	}
	if err := writeImageToSystemClipboard(entry.ImagePath); err != nil {
		return contracts.ActionResult{OK: false, Message: err.Error()}
	}
	s.mu.Lock()
	s.lastWatcherSignature = entry.Signature
	s.mu.Unlock()
	return contracts.ActionResult{OK: true, Message: "图片已复制"}
}

func (s *Service) ImageDataURL(id string) string {
	return s.imageDataURL(id, false)
}

func (s *Service) ThumbnailDataURL(id string) string {
	return s.imageDataURL(id, true)
}

func (s *Service) imageDataURL(id string, preferThumbnail bool) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	s.mu.RLock()
	var imagePath string
	for _, entry := range s.entries {
		if entry.ID == id && entry.Type == EntryImage {
			imagePath = entry.ImagePath
			if preferThumbnail && entry.ThumbnailPath != "" && fileExists(entry.ThumbnailPath) {
				imagePath = entry.ThumbnailPath
			}
			break
		}
	}
	s.mu.RUnlock()
	if imagePath == "" {
		return ""
	}
	raw, err := os.ReadFile(imagePath)
	if err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)
}

func (s *Service) Entry(id string) Entry {
	id = strings.TrimSpace(id)
	if id == "" {
		return Entry{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, entry := range s.entries {
		if entry.ID == id {
			return entry
		}
	}
	return Entry{}
}

func (s *Service) AddImageToCapture(id string) capturehistory.Status {
	entry := s.imageEntry(id)
	if entry.ID == "" {
		return capturehistory.Status{LastCaptureError: "未找到剪贴板图片"}
	}
	if s.captureSink == nil {
		return capturehistory.Status{LastCaptureError: "截图历史服务不可用"}
	}
	raw, err := os.ReadFile(entry.ImagePath)
	if err != nil {
		return capturehistory.Status{LastCaptureError: err.Error()}
	}
	status := s.captureSink.AddPNG(raw, entry.Width, entry.Height, "clipboard_image", "", []string{"clipboard", "image"})
	return status
}

func (s *Service) DecodeImageQRCode(id string) qrscan.Result {
	entry := s.imageEntry(id)
	if entry.ID == "" {
		return qrscan.Result{OK: false, Source: "clipboard_history", Error: "未找到剪贴板图片", DecodedAt: time.Now().Unix()}
	}
	result := qrscan.DecodeImagePath(entry.ImagePath)
	result.Source = "clipboard_history"
	result.ImagePath = entry.ImagePath
	result.Width = entry.Width
	result.Height = entry.Height
	return result
}

func (s *Service) imageEntry(id string) Entry {
	id = strings.TrimSpace(id)
	if id == "" {
		return Entry{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, entry := range s.entries {
		if entry.ID == id && entry.Type == EntryImage {
			return entry
		}
	}
	return Entry{}
}

func (s *Service) ApplyWatcherSettings(privacyMode bool, sourceClipboard bool) Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	enabled := sourceClipboard && !privacyMode
	s.watcherEnabled = enabled
	if enabled {
		s.startWatcherLocked()
	} else {
		if privacyMode {
			s.lastWatcherError = "隐私模式已开启，剪贴板监听已暂停"
		} else {
			s.lastWatcherError = ""
		}
		s.stopWatcherLocked()
	}
	return s.statusLocked(false)
}

func (s *Service) StopWatcher() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.watcherEnabled = false
	s.stopWatcherLocked()
	return s.statusLocked(false)
}

func (s *Service) TogglePin(id string) Status {
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.entries {
		if s.entries[i].ID == id {
			s.entries[i].Pinned = !s.entries[i].Pinned
			break
		}
	}
	s.saveLockedWithStatus()
	return s.statusLocked(true)
}

func (s *Service) Delete(id string) Status {
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	next := make([]Entry, 0, len(s.entries))
	for _, entry := range s.entries {
		if entry.ID != id {
			next = append(next, entry)
		} else {
			s.removeEntryFile(entry)
		}
	}
	s.entries = next
	s.saveLockedWithStatus()
	return s.statusLocked(true)
}

func (s *Service) ClearUnpinned() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := make([]Entry, 0, len(s.entries))
	for _, entry := range s.entries {
		if entry.Pinned {
			next = append(next, entry)
		} else {
			s.removeEntryFile(entry)
		}
	}
	s.entries = next
	s.saveLockedWithStatus()
	return s.statusLocked(true)
}

func (s *Service) ApplyRetentionPolicy(retentionDays int, keepPinned bool) RetentionResult {
	if retentionDays <= 0 {
		retentionDays = 30
	}
	now := time.Now()
	cutoff := now.Add(-time.Duration(retentionDays) * 24 * time.Hour).Unix()
	result := RetentionResult{
		OK:            true,
		RetentionDays: retentionDays,
		KeepPinned:    keepPinned,
		CutoffAt:      cutoff,
		AppliedAt:     now.Unix(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	next := make([]Entry, 0, len(s.entries))
	for _, entry := range s.entries {
		if keepPinned && entry.Pinned {
			result.Kept++
			result.KeptPinned++
			next = append(next, entry)
			continue
		}
		if entry.CreatedAt == 0 || entry.CreatedAt >= cutoff {
			result.Kept++
			next = append(next, entry)
			continue
		}
		result.Removed++
		if entry.Type == EntryImage {
			result.RemovedImages++
		}
		s.removeEntryFile(entry)
	}
	s.entries = next
	if result.Removed > 0 {
		s.saveLockedWithStatus()
	}
	result.RemainingCount = len(s.entries)
	return result
}

func (s *Service) Search(query string) []contracts.SearchResult {
	query = clipboardQuery(query)
	if len([]rune(query)) < 2 {
		return nil
	}
	entries := s.List(query, 20)
	results := make([]contracts.SearchResult, 0, len(entries))
	normalized := strings.ToLower(query)
	for _, entry := range entries {
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

func clipboardQuery(query string) string {
	query = strings.TrimSpace(query)
	parts := strings.Fields(query)
	if len(parts) == 0 {
		return ""
	}
	switch strings.ToLower(parts[0]) {
	case "clip", "clipboard", "剪贴板":
		return strings.TrimSpace(strings.TrimPrefix(query, parts[0]))
	default:
		return query
	}
}

func (s *Service) listLocked(query string, limit int) []Entry {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	normalized := strings.ToLower(strings.TrimSpace(query))
	items := make([]Entry, 0, len(s.entries))
	for _, entry := range s.entries {
		if normalized == "" || entryMatches(entry, normalized) {
			items = append(items, entry)
		}
	}
	sortEntries(items)
	if len(items) > limit {
		items = items[:limit]
	}
	return append([]Entry{}, items...)
}

func (s *Service) statusLocked(includeEntries bool) Status {
	thumbnailCount, thumbnailBytes := countFilesInDir(s.thumbnailDir)
	status := Status{
		Path:             s.path,
		ImageDir:         s.imageDir,
		ThumbnailDir:     s.thumbnailDir,
		Count:            len(s.entries),
		ThumbnailCount:   thumbnailCount,
		ThumbnailBytes:   thumbnailBytes,
		LastSaveError:    s.lastError,
		WatcherEnabled:   s.watcherEnabled,
		WatcherRunning:   s.watcherRunning,
		LastWatcherAt:    s.lastWatcherAt,
		LastWatcherError: s.lastWatcherError,
	}
	for _, entry := range s.entries {
		if entry.Pinned {
			status.PinnedCount++
		}
		if entry.Type == EntryImage {
			status.ImageCount++
		}
		if entry.CreatedAt > status.LastEntryAt {
			status.LastEntryAt = entry.CreatedAt
		}
	}
	if includeEntries {
		status.Entries = s.listLocked("", s.maxEntries)
	}
	return status
}

func (s *Service) addEntryLocked(entry Entry) Entry {
	entry = normalizeEntry(entry)
	if !entryIsValid(entry) {
		s.removeEntryFile(entry)
		return Entry{}
	}

	next := make([]Entry, 0, len(s.entries)+1)
	for _, existing := range s.entries {
		if existing.Signature == entry.Signature {
			entry.ID = existing.ID
			entry.Pinned = existing.Pinned
			if existing.ImagePath != "" && existing.ImagePath != entry.ImagePath {
				s.removeEntryFile(existing)
			}
			continue
		}
		next = append(next, existing)
	}
	if entry.ID == "" {
		entry.ID = stableID(entry.Signature + "|" + time.Unix(entry.CreatedAt, 0).UTC().Format(time.RFC3339Nano))
	}
	s.entries = append([]Entry{entry}, next...)
	s.trimLocked()
	s.saveLockedWithStatus()
	return entry
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
		if entryIsValid(entry) {
			s.entries = append(s.entries, entry)
		}
	}
	sortEntries(s.entries)
	s.trimLocked()
}

func (s *Service) saveLockedWithStatus() {
	s.lastError = ""
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

func (s *Service) trimLocked() {
	for len(s.entries) > s.maxEntries {
		remove := len(s.entries) - 1
		for i := len(s.entries) - 1; i >= 0; i-- {
			if !s.entries[i].Pinned {
				remove = i
				break
			}
		}
		s.removeEntryFile(s.entries[remove])
		s.entries = append(s.entries[:remove], s.entries[remove+1:]...)
	}
}

func (s *Service) ensureThumbnails() thumbnailBackfillResult {
	result := thumbnailBackfillResult{}
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for i := range s.entries {
		entry := &s.entries[i]
		if entry.Type != EntryImage {
			result.Skipped++
			continue
		}
		if entry.ThumbnailPath != "" && fileExists(entry.ThumbnailPath) {
			result.Skipped++
			continue
		}
		if entry.Width > 0 && entry.Height > 0 && entry.Width <= imagepreview.DefaultMaxSide && entry.Height <= imagepreview.DefaultMaxSide {
			if entry.ThumbnailPath != "" {
				clearThumbnailFields(entry)
				changed = true
			}
			result.Skipped++
			continue
		}
		raw, err := os.ReadFile(entry.ImagePath)
		if err != nil || len(raw) == 0 {
			result.Failed++
			continue
		}
		filename := filepath.Base(entry.ImagePath)
		thumbnailPath, thumbnailWidth, thumbnailHeight, thumbnailBytes := writeThumbnail(raw, s.thumbnailDir, filename)
		if thumbnailPath == "" {
			if entry.Width > imagepreview.DefaultMaxSide || entry.Height > imagepreview.DefaultMaxSide {
				result.Failed++
			} else {
				result.Skipped++
				clearThumbnailFields(entry)
				changed = true
			}
			continue
		}
		entry.ThumbnailPath = thumbnailPath
		entry.ThumbnailWidth = thumbnailWidth
		entry.ThumbnailHeight = thumbnailHeight
		entry.ThumbnailBytes = thumbnailBytes
		result.Created++
		changed = true
	}
	if changed {
		s.saveLockedWithStatus()
	}
	return result
}

func (s *Service) startWatcherLocked() {
	if s.watcherStop != nil {
		s.watcherRunning = true
		return
	}
	s.watcherPrimed = false
	s.lastWatcherSignature = ""
	s.lastWatcherError = ""
	stop := make(chan struct{})
	interval := s.watcherInterval
	if interval <= 0 {
		interval = time.Second
	}
	s.watcherStop = stop
	s.watcherRunning = true
	go s.runWatcher(stop, interval)
}

func (s *Service) stopWatcherLocked() {
	if s.watcherStop != nil {
		close(s.watcherStop)
		s.watcherStop = nil
	}
	s.watcherRunning = false
	s.watcherPrimed = false
	s.lastWatcherSignature = ""
}

func (s *Service) runWatcher(stop <-chan struct{}, interval time.Duration) {
	s.pollClipboardOnce(false)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			s.pollClipboardOnce(true)
		}
	}
}

func (s *Service) pollClipboardOnce(recordChange bool) Status {
	reader := s.clipboardReader
	if reader == nil {
		reader = readSystemClipboardEntry
	}
	entry, err := reader(s.imageDir, s.watcherSource())
	now := time.Now().Unix()

	s.mu.Lock()
	s.lastWatcherAt = now
	if err != nil {
		if errors.Is(err, errClipboardUnsupported) {
			s.lastWatcherError = "当前平台不支持原生剪贴板监听"
		} else {
			s.lastWatcherError = err.Error()
		}
		status := s.statusLocked(false)
		s.mu.Unlock()
		return status
	}
	s.lastWatcherError = ""

	entry = normalizeEntry(entry)
	signature := entry.Signature
	if !entryIsValid(entry) {
		signature = ""
		s.removeEntryFile(entry)
	}
	if !s.watcherPrimed {
		s.watcherPrimed = true
		s.lastWatcherSignature = signature
		if !recordChange {
			s.removeEntryFile(entry)
			status := s.statusLocked(false)
			s.mu.Unlock()
			return status
		}
	}
	if signature == "" || signature == s.lastWatcherSignature {
		s.removeEntryFile(entry)
		status := s.statusLocked(false)
		s.mu.Unlock()
		return status
	}
	s.lastWatcherSignature = signature
	entry = s.addEntryLocked(entry)
	status := s.statusLocked(true)
	s.mu.Unlock()
	s.notifyEntryObservers(entry)
	return status
}

func (s *Service) notifyEntryObservers(entry Entry) {
	if entry.ID == "" {
		return
	}
	s.mu.RLock()
	observers := append([]func(Entry){}, s.observers...)
	s.mu.RUnlock()
	for _, observer := range observers {
		observer(entry)
	}
}

func (s *Service) watcherSource() string {
	if s.watcherSourceOverride != "" {
		return s.watcherSourceOverride
	}
	return "clipboard_watcher"
}

func makeTextEntry(text string, source string) Entry {
	text = strings.TrimSpace(text)
	source = strings.TrimSpace(source)
	if source == "" {
		source = "manual"
	}
	signature := "txt:" + sha1Hex(text)
	contentType, tags := classifyText(text)
	return Entry{
		Type:        EntryText,
		Text:        text,
		CreatedAt:   time.Now().Unix(),
		Signature:   signature,
		ContentType: contentType,
		Source:      source,
		Summary:     summarize(text, 120),
		Tags:        tags,
	}
}

func (s *Service) makeImageEntryFromPNG(pngBytes []byte, source string) (Entry, error) {
	return makeImageEntryFromPNG(pngBytes, s.imageDir, source, s.thumbnailDir)
}

func makeImageEntryFromPNG(pngBytes []byte, imageDir string, source string, thumbnailDirs ...string) (Entry, error) {
	pngBytes = append([]byte{}, pngBytes...)
	if len(pngBytes) == 0 {
		return Entry{}, fmt.Errorf("剪贴板图片为空")
	}
	config, err := decodePNGConfig(pngBytes)
	if err != nil {
		return Entry{}, err
	}
	if imageDir == "" {
		return Entry{}, fmt.Errorf("缺少剪贴板图片目录")
	}
	signature := "img:" + sha1HexBytes(pngBytes)
	filename := stableID(signature+"|"+time.Now().UTC().Format(time.RFC3339Nano)) + ".png"
	path := filepath.Join(imageDir, filename)
	thumbnailDir := defaultThumbnailDir(imageDir)
	if len(thumbnailDirs) > 0 && strings.TrimSpace(thumbnailDirs[0]) != "" {
		thumbnailDir = strings.TrimSpace(thumbnailDirs[0])
	}
	if err := os.MkdirAll(imageDir, 0o755); err != nil {
		return Entry{}, err
	}
	if err := os.WriteFile(path, pngBytes, 0o600); err != nil {
		return Entry{}, err
	}
	thumbnailPath, thumbnailWidth, thumbnailHeight, thumbnailBytes := writeThumbnail(pngBytes, thumbnailDir, filename)
	source = strings.TrimSpace(source)
	if source == "" {
		source = "manual"
	}
	return Entry{
		Type:            EntryImage,
		ImagePath:       path,
		ThumbnailPath:   thumbnailPath,
		ThumbnailWidth:  thumbnailWidth,
		ThumbnailHeight: thumbnailHeight,
		ThumbnailBytes:  thumbnailBytes,
		CreatedAt:       time.Now().Unix(),
		Signature:       signature,
		ContentType:     "image",
		Source:          source,
		Summary:         fmt.Sprintf("剪贴板图片 %dx%d", config.Width, config.Height),
		Width:           config.Width,
		Height:          config.Height,
		Bytes:           int64(len(pngBytes)),
		Tags:            []string{"剪贴板", "图片", fmt.Sprintf("%dx%d", config.Width, config.Height)},
	}, nil
}

func normalizeEntry(entry Entry) Entry {
	entry.ID = strings.TrimSpace(entry.ID)
	if entry.Type != EntryImage {
		entry.Type = EntryText
	}
	entry.Text = strings.TrimSpace(entry.Text)
	entry.ImagePath = strings.TrimSpace(entry.ImagePath)
	entry.ThumbnailPath = strings.TrimSpace(entry.ThumbnailPath)
	entry.Signature = strings.TrimSpace(entry.Signature)
	if entry.Signature == "" && entry.Type == EntryText {
		entry.Signature = "txt:" + sha1Hex(entry.Text)
	}
	if entry.Signature == "" && entry.Type == EntryImage && entry.ImagePath != "" {
		entry.Signature = "img:" + sha1Hex(entry.ImagePath)
	}
	if entry.ID == "" {
		entry.ID = stableID(entry.Signature)
	}
	if entry.CreatedAt == 0 {
		entry.CreatedAt = time.Now().Unix()
	}
	entry.Source = strings.TrimSpace(entry.Source)
	if entry.Source == "" {
		entry.Source = "import"
	}
	if entry.Type == EntryImage {
		entry.ContentType = "image"
		if entry.ThumbnailWidth < 0 {
			entry.ThumbnailWidth = 0
		}
		if entry.ThumbnailHeight < 0 {
			entry.ThumbnailHeight = 0
		}
		if entry.ThumbnailBytes < 0 {
			entry.ThumbnailBytes = 0
		}
		if entry.Summary == "" {
			entry.Summary = fmt.Sprintf("剪贴板图片 %dx%d", entry.Width, entry.Height)
		}
		if entry.Width > 0 && entry.Height > 0 {
			entry.Tags = mergeTags([]string{"剪贴板", "图片", fmt.Sprintf("%dx%d", entry.Width, entry.Height)}, entry.Tags)
		} else {
			entry.Tags = mergeTags([]string{"剪贴板", "图片"}, entry.Tags)
		}
		return entry
	}
	var classifiedTags []string
	entry.ContentType, classifiedTags = classifyText(entry.Text)
	entry.Tags = mergeTags(classifiedTags, entry.Tags)
	entry.Summary = summarize(entry.Text, 120)
	return entry
}

func cleanStrings(items []string) []string {
	return mergeTags(nil, items)
}

func stringListContainsFold(items []string, value string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), value) {
			return true
		}
	}
	return false
}

func mergeTags(base []string, extra []string) []string {
	seen := map[string]bool{}
	merged := []string{}
	for _, group := range [][]string{base, extra} {
		for _, item := range group {
			item = strings.TrimSpace(item)
			if item == "" || seen[item] {
				continue
			}
			seen[item] = true
			merged = append(merged, item)
		}
	}
	return merged
}

func classifyText(text string) (string, []string) {
	value := strings.TrimSpace(text)
	lower := strings.ToLower(value)
	if json.Valid([]byte(value)) {
		return "json", []string{"剪贴板", "JSON"}
	}
	if parsed, err := url.ParseRequestURI(value); err == nil && parsed.Scheme != "" {
		return "url", []string{"剪贴板", "URL"}
	}
	if strings.Contains(lower, "select ") || strings.Contains(lower, " from ") || strings.Contains(lower, " where ") {
		return "sql", []string{"剪贴板", "SQL"}
	}
	if strings.ContainsAny(value, "\n{}();") && (strings.Contains(lower, "func ") || strings.Contains(lower, "const ") || strings.Contains(lower, "import ") || strings.Contains(lower, "def ")) {
		return "code", []string{"剪贴板", "代码"}
	}
	if looksLikePath(value) {
		return "path", []string{"剪贴板", "路径"}
	}
	if strings.HasPrefix(lower, "git ") || strings.HasPrefix(lower, "go ") || strings.HasPrefix(lower, "pnpm ") || strings.HasPrefix(lower, "npm ") || strings.HasPrefix(lower, "wails3 ") {
		return "command", []string{"剪贴板", "命令"}
	}
	return "text", []string{"剪贴板", "文本"}
}

func looksLikePath(value string) bool {
	if strings.Contains(value, ":\\") || strings.HasPrefix(value, "\\\\") {
		return true
	}
	return strings.HasPrefix(value, "/") && strings.Count(value, "/") >= 2
}

func entryToResult(entry Entry, score float64) contracts.SearchResult {
	if entry.Type == EntryImage {
		return imageEntryToResult(entry, score)
	}
	pinID := "clipboard_pin"
	pinLabel := "置顶"
	pinSuccess := "已置顶"
	if entry.Pinned {
		pinID = "clipboard_unpin"
		pinLabel = "取消置顶"
		pinSuccess = "已取消置顶"
	}
	return contracts.SearchResult{
		ID:       "clipboard-" + entry.ID,
		Type:     contracts.ResultClipboard,
		Title:    summarize(entry.Text, 72),
		Subtitle: "剪贴板历史 · " + entry.ContentType,
		Detail:   entry.Text,
		Icon:     "clipboard",
		Score:    score,
		Tags:     append([]string{}, entry.Tags...),
		Payload: map[string]interface{}{
			"clipboardId": entry.ID,
			"contentType": entry.ContentType,
			"pinned":      entry.Pinned,
		},
		Preview: contracts.PreviewDescriptor{
			Kind:     contracts.PreviewText,
			Title:    "剪贴板历史",
			Subtitle: entry.ContentType,
			Text:     entry.Text,
			Meta: []contracts.LabelValue{
				{Label: "来源", Value: entry.Source},
				{Label: "类型", Value: entry.ContentType},
				{Label: "置顶", Value: boolLabel(entry.Pinned)},
			},
			Evidence: []contracts.LabelValue{
				{Label: "记录时间", Value: time.Unix(entry.CreatedAt, 0).Format("2006-01-02 15:04:05")},
			},
		},
		Actions: []contracts.PreviewAction{
			contracts.CopyAction("copy_clipboard_text", "复制内容", entry.Text, "Enter"),
			{
				ID:    pinID,
				Label: pinLabel,
				Icon:  "pin",
				Kind:  contracts.ActionPin,
				Payload: map[string]interface{}{
					"clipboardId": entry.ID,
				},
				Feedback: &contracts.ActionFeedback{SuccessLabel: pinSuccess, DurationMS: 1400},
			},
			contracts.RememberAction("remember_clipboard", "加入记忆", "clipboard-"+entry.ID),
			{
				ID:    "clipboard_delete",
				Label: "删除",
				Icon:  "run",
				Kind:  contracts.ActionDanger,
				Payload: map[string]interface{}{
					"clipboardId":          entry.ID,
					"requiresConfirmation": true,
				},
				Feedback: &contracts.ActionFeedback{SuccessLabel: "已删除", DurationMS: 1400},
			},
		},
	}
}

func imageEntryToResult(entry Entry, score float64) contracts.SearchResult {
	pinID := "clipboard_pin"
	pinLabel := "置顶"
	pinSuccess := "已置顶"
	if entry.Pinned {
		pinID = "clipboard_unpin"
		pinLabel = "取消置顶"
		pinSuccess = "已取消置顶"
	}
	dimension := fmt.Sprintf("%dx%d", entry.Width, entry.Height)
	return contracts.SearchResult{
		ID:       "clipboard-" + entry.ID,
		Type:     contracts.ResultClipboard,
		Title:    "剪贴板图片 " + dimension,
		Subtitle: "剪贴板历史 · 图片",
		Detail:   entry.ImagePath,
		Icon:     "image",
		Score:    score,
		Tags:     append([]string{}, entry.Tags...),
		Payload: map[string]interface{}{
			"clipboardId":   entry.ID,
			"contentType":   "image",
			"pinned":        entry.Pinned,
			"imagePath":     entry.ImagePath,
			"thumbnailPath": entry.ThumbnailPath,
		},
		Preview: contracts.PreviewDescriptor{
			Kind:      contracts.PreviewImage,
			Title:     "剪贴板图片",
			Subtitle:  dimension,
			ImageHint: entry.ImagePath,
			Meta: []contracts.LabelValue{
				{Label: "来源", Value: entry.Source},
				{Label: "尺寸", Value: dimension},
				{Label: "置顶", Value: boolLabel(entry.Pinned)},
			},
			Evidence: []contracts.LabelValue{
				{Label: "记录时间", Value: time.Unix(entry.CreatedAt, 0).Format("2006-01-02 15:04:05")},
				{Label: "图片路径", Value: entry.ImagePath},
				{Label: "预览图", Value: thumbnailLabel(entry)},
			},
		},
		Actions: []contracts.PreviewAction{
			{
				ID:       "copy_clipboard_image",
				Label:    "复制图片",
				Icon:     "copy",
				Kind:     contracts.ActionPlugin,
				Shortcut: "Enter",
				Payload: map[string]interface{}{
					"clipboardId": entry.ID,
				},
				Feedback: &contracts.ActionFeedback{SuccessLabel: "图片已复制", DurationMS: 1400},
			},
			{
				ID:    "pin_clipboard_image",
				Label: "贴到屏幕",
				Icon:  "pin",
				Kind:  contracts.ActionPin,
				Payload: map[string]interface{}{
					"clipboardId": entry.ID,
				},
				Feedback: &contracts.ActionFeedback{SuccessLabel: "已创建贴图", DurationMS: 1400},
			},
			{
				ID:    "recognize_clipboard_qr",
				Label: "识别二维码",
				Icon:  "plugin",
				Kind:  contracts.ActionPlugin,
				Payload: map[string]interface{}{
					"clipboardId": entry.ID,
				},
				Feedback: &contracts.ActionFeedback{SuccessLabel: "已识别二维码", DurationMS: 1600},
			},
			{
				ID:    "clipboard_image_to_capture",
				Label: "加入截图历史",
				Icon:  "capture",
				Kind:  contracts.ActionPlugin,
				Payload: map[string]interface{}{
					"clipboardId": entry.ID,
				},
				Feedback: &contracts.ActionFeedback{SuccessLabel: "已加入截图历史", DurationMS: 1400},
			},
			{
				ID:    pinID,
				Label: pinLabel,
				Icon:  "pin",
				Kind:  contracts.ActionPin,
				Payload: map[string]interface{}{
					"clipboardId": entry.ID,
				},
				Feedback: &contracts.ActionFeedback{SuccessLabel: pinSuccess, DurationMS: 1400},
			},
			contracts.RememberAction("remember_clipboard", "加入记忆", "clipboard-"+entry.ID),
			{
				ID:    "clipboard_delete",
				Label: "删除",
				Icon:  "run",
				Kind:  contracts.ActionDanger,
				Payload: map[string]interface{}{
					"clipboardId":          entry.ID,
					"requiresConfirmation": true,
				},
				Feedback: &contracts.ActionFeedback{SuccessLabel: "已删除", DurationMS: 1400},
			},
		},
	}
}

func scoreEntry(entry Entry, query string) float64 {
	if query == "" {
		return 0
	}
	score := 0.0
	if entry.Type == EntryImage {
		haystack := strings.ToLower(strings.Join([]string{
			entry.Summary,
			entry.ContentType,
			entry.Source,
			fmt.Sprintf("%dx%d", entry.Width, entry.Height),
			"image 图片 剪贴板",
		}, " "))
		if strings.Contains(haystack, query) {
			score = 76
		}
	}
	text := strings.ToLower(entry.Text)
	if strings.Contains(text, query) {
		score = 78
		if strings.HasPrefix(text, query) {
			score = 90
		}
	}
	if strings.Contains(strings.ToLower(entry.ContentType), query) {
		score += 8
	}
	for _, tag := range entry.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			score += 8
		}
	}
	if entry.Pinned {
		score += 10
	}
	return score
}

func entryMatches(entry Entry, query string) bool {
	if query == "" {
		return true
	}
	parts := []string{
		entry.Text,
		entry.Summary,
		entry.ContentType,
		entry.Source,
		entry.ImagePath,
		fmt.Sprintf("%dx%d", entry.Width, entry.Height),
	}
	parts = append(parts, entry.Tags...)
	return strings.Contains(strings.ToLower(strings.Join(parts, " ")), query)
}

func entryIsValid(entry Entry) bool {
	if entry.ID == "" && entry.Signature == "" {
		return false
	}
	if entry.Type == EntryImage {
		return entry.ImagePath != "" && fileExists(entry.ImagePath)
	}
	return entry.Text != ""
}

func (s *Service) removeEntryFile(entry Entry) {
	if entry.Type == EntryImage {
		removeFileInside(entry.ImagePath, s.imageDir)
		removeFileInside(entry.ThumbnailPath, s.thumbnailDir)
	}
}

func writeThumbnail(pngBytes []byte, thumbnailDir string, filename string) (string, int, int, int64) {
	thumbnail, width, height, ok, err := imagepreview.CreatePNGThumbnail(pngBytes, imagepreview.DefaultMaxSide)
	if err != nil || !ok || thumbnailDir == "" {
		return "", 0, 0, 0
	}
	if err := os.MkdirAll(thumbnailDir, 0o755); err != nil {
		return "", 0, 0, 0
	}
	stem := strings.TrimSuffix(filename, filepath.Ext(filename))
	path := filepath.Join(thumbnailDir, stem+".thumb.png")
	if err := os.WriteFile(path, thumbnail, 0o600); err != nil {
		return "", 0, 0, 0
	}
	return path, width, height, int64(len(thumbnail))
}

func clearThumbnailFields(entry *Entry) {
	entry.ThumbnailPath = ""
	entry.ThumbnailWidth = 0
	entry.ThumbnailHeight = 0
	entry.ThumbnailBytes = 0
}

func decodePNGConfig(pngBytes []byte) (image.Config, error) {
	config, err := png.DecodeConfig(bytes.NewReader(pngBytes))
	if err != nil {
		return image.Config{}, fmt.Errorf("无法读取剪贴板图片: %w", err)
	}
	return config, nil
}

func sortEntries(entries []Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Pinned != entries[j].Pinned {
			return entries[i].Pinned
		}
		return entries[i].CreatedAt > entries[j].CreatedAt
	})
}

func summarize(value string, limit int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "..."
}

func stableID(value string) string {
	return sha1Hex(strings.ToLower(value))[:12]
}

func sha1Hex(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

func sha1HexBytes(value []byte) string {
	sum := sha1.Sum(value)
	return hex.EncodeToString(sum[:])
}

func boolLabel(value bool) string {
	if value {
		return "是"
	}
	return "否"
}

func thumbnailLabel(entry Entry) string {
	if entry.ThumbnailPath == "" {
		return "原图预览"
	}
	if entry.ThumbnailWidth > 0 && entry.ThumbnailHeight > 0 {
		return fmt.Sprintf("%dx%d · %s", entry.ThumbnailWidth, entry.ThumbnailHeight, formatBytes(entry.ThumbnailBytes))
	}
	return entry.ThumbnailPath
}

func formatBytes(value int64) string {
	if value <= 0 {
		return "0 B"
	}
	if value < 1024 {
		return fmt.Sprintf("%d B", value)
	}
	if value < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(value)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(value)/(1024*1024))
}

func countFilesInDir(root string) (int, int64) {
	if root == "" {
		return 0, 0
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0, 0
	}
	count := 0
	bytes := int64(0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		count++
		bytes += info.Size()
	}
	return count, bytes
}

func defaultHistoryPath() string {
	base := os.Getenv("APPDATA")
	if base == "" {
		if dir, err := os.UserConfigDir(); err == nil {
			base = dir
		}
	}
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "Ariadne", "clipboard_history.json")
}

func defaultImageDir(historyPath string) string {
	if historyPath != "" {
		return filepath.Join(filepath.Dir(historyPath), "clipboard_images")
	}
	base := os.Getenv("APPDATA")
	if base == "" {
		if dir, err := os.UserConfigDir(); err == nil {
			base = dir
		}
	}
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "Ariadne", "clipboard_images")
}

func defaultThumbnailDir(imageDir string) string {
	if imageDir != "" {
		return filepath.Join(filepath.Dir(imageDir), "clipboard_thumbnails")
	}
	return filepath.Join(filepath.Dir(defaultHistoryPath()), "clipboard_thumbnails")
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func removeFileInside(path string, root string) {
	if path == "" || root == "" {
		return
	}
	resolvedPath, err := filepath.Abs(path)
	if err != nil {
		return
	}
	resolvedRoot, err := filepath.Abs(root)
	if err != nil {
		return
	}
	rel, err := filepath.Rel(resolvedRoot, resolvedPath)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return
	}
	_ = os.Remove(resolvedPath)
}
