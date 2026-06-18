package capturehistory

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ariadne/internal/appdb"
	"ariadne/internal/contracts"
	"ariadne/internal/imagepreview"
)

type Entry struct {
	ID              string   `json:"id"`
	ImagePath       string   `json:"imagePath"`
	ThumbnailPath   string   `json:"thumbnailPath,omitempty"`
	ThumbnailWidth  int      `json:"thumbnailWidth,omitempty"`
	ThumbnailHeight int      `json:"thumbnailHeight,omitempty"`
	ThumbnailBytes  int64    `json:"thumbnailBytes,omitempty"`
	SavedPath       string   `json:"savedPath,omitempty"`
	CreatedAt       int64    `json:"createdAt"`
	Source          string   `json:"source"`
	Actions         []string `json:"actions,omitempty"`
	Pinned          bool     `json:"pinned"`
	Width           int      `json:"width"`
	Height          int      `json:"height"`
	Bytes           int64    `json:"bytes"`
	Signature       string   `json:"signature"`
	Tags            []string `json:"tags,omitempty"`
}

type Status struct {
	Path                  string  `json:"path"`
	ImageDir              string  `json:"imageDir"`
	ThumbnailDir          string  `json:"thumbnailDir,omitempty"`
	Count                 int     `json:"count"`
	PinnedCount           int     `json:"pinnedCount"`
	ThumbnailCount        int     `json:"thumbnailCount"`
	ThumbnailBytes        int64   `json:"thumbnailBytes"`
	LastEntryAt           int64   `json:"lastEntryAt,omitempty"`
	LastSaveError         string  `json:"lastSaveError,omitempty"`
	LastCaptureError      string  `json:"lastCaptureError,omitempty"`
	VirtualizedPath       string  `json:"virtualizedPath,omitempty"`
	VirtualizedExists     bool    `json:"virtualizedExists"`
	VirtualizedBytes      int64   `json:"virtualizedBytes"`
	VirtualizedImageDir   string  `json:"virtualizedImageDir,omitempty"`
	VirtualizedImageCount int     `json:"virtualizedImageCount"`
	VirtualizedImageBytes int64   `json:"virtualizedImageBytes"`
	Entries               []Entry `json:"entries,omitempty"`
}

type RetentionResult struct {
	OK                 bool  `json:"ok"`
	RetentionDays      int   `json:"retentionDays"`
	MaxStorageMB       int   `json:"maxStorageMb,omitempty"`
	StorageBudgetBytes int64 `json:"storageBudgetBytes,omitempty"`
	KeepPinned         bool  `json:"keepPinned"`
	CutoffAt           int64 `json:"cutoffAt,omitempty"`
	Removed            int   `json:"removed"`
	RemovedByStorage   int   `json:"removedByStorage,omitempty"`
	Kept               int   `json:"kept"`
	KeptPinned         int   `json:"keptPinned"`
	RemainingCount     int   `json:"remainingCount"`
	RemainingBytes     int64 `json:"remainingBytes,omitempty"`
	AppliedAt          int64 `json:"appliedAt"`
}

type CaptureOptions struct {
	CaptureScope string `json:"captureScope,omitempty"`
	MultiMonitor string `json:"multiMonitor,omitempty"`
}

type capturedScreen struct {
	Data    []byte
	Width   int
	Height  int
	Bounds  ScreenBounds
	Actions []string
	Tags    []string
}

var captureScreenArtifacts = captureScreenPNGs

type thumbnailBackfillResult struct {
	Created int
	Skipped int
	Failed  int
}

var historySaveDebounceDelay = 600 * time.Millisecond

const (
	defaultStatusEntryLimit = 500
	maxListEntryLimit       = 5000
	bytesPerMegabyte        = 1024 * 1024
)

type Service struct {
	mu               sync.RWMutex
	path             string
	imageDir         string
	thumbnailDir     string
	entries          []Entry
	maxEntries       int
	lastSaveError    string
	lastCaptureError string
	observers        []func(Entry)
	saveDirty        bool
	saveScheduled    bool
}

func NewService() *Service {
	return NewServiceWithPaths(defaultHistoryPath(), defaultImageDir())
}

func NewServiceWithPaths(path string, imageDir string) *Service {
	service := &Service{path: path, imageDir: imageDir, thumbnailDir: defaultThumbnailDir(imageDir)}
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
		if entry.Source == "legacy_x_tools" || stringListContainsFold(entry.Tags, "legacy_x_tools") || stringListContainsFold(entry.Actions, "legacy_x_tools") {
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

func (s *Service) Entry(id string) Entry {
	id = strings.TrimSpace(id)
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, entry := range s.entries {
		if entry.ID == id {
			return entry
		}
	}
	return Entry{}
}

func (s *Service) CaptureScreen(source string) Status {
	return s.CaptureScreenWithOptions(source, CaptureOptions{})
}

func (s *Service) CaptureScreenWithOptions(source string, options CaptureOptions) Status {
	captures, err := captureScreenArtifacts(options)
	if err != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.lastCaptureError = err.Error()
		return s.statusLocked(true)
	}
	if len(captures) == 0 {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.lastCaptureError = "没有可用截图区域"
		return s.statusLocked(true)
	}
	var status Status
	for _, capture := range captures {
		status = s.addPNGWithTags(capture.Data, capture.Width, capture.Height, source, "", append([]string{"screen"}, capture.Actions...), capture.Tags)
		if status.LastCaptureError != "" {
			return status
		}
	}
	return status
}

func (s *Service) CaptureRegion(x int, y int, width int, height int, source string, actions []string) Status {
	data, regionWidth, regionHeight, err := CaptureRegionPNG(x, y, width, height)
	if err != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.lastCaptureError = err.Error()
		return s.statusLocked(true)
	}
	return s.AddPNG(data, regionWidth, regionHeight, source, "", append([]string{"region"}, actions...))
}

func (s *Service) AddPNG(data []byte, width int, height int, source string, savedPath string, actions []string) Status {
	return s.addPNGWithTags(data, width, height, source, savedPath, actions, nil)
}

func (s *Service) ImportLegacyEntries(entries []Entry) Status {
	prepared := make([]struct {
		entry Entry
		data  []byte
	}, 0, len(entries))
	for _, legacy := range entries {
		raw, err := os.ReadFile(strings.TrimSpace(legacy.ImagePath))
		if err != nil || len(raw) == 0 {
			continue
		}
		signature := strings.TrimSpace(legacy.Signature)
		if signature == "" {
			signature = "png:" + sha1HexBytes(raw)
		}
		id := strings.TrimSpace(legacy.ID)
		if id == "" {
			id = stableID(signature + "|legacy")
		}
		source := strings.TrimSpace(legacy.Source)
		if source == "" {
			source = "legacy_x_tools"
		}
		filename := "legacy-" + stableID(signature+"|"+id) + ".png"
		imagePath := filepath.Join(s.imageDir, filename)
		entry := normalizeEntry(Entry{
			ID:            id,
			ImagePath:     imagePath,
			ThumbnailPath: s.thumbnailPathForEntry(Entry{ID: id, ImagePath: imagePath}),
			SavedPath:     legacy.SavedPath,
			CreatedAt:     legacy.CreatedAt,
			Source:        source,
			Actions:       append([]string{"legacy_x_tools"}, legacy.Actions...),
			Pinned:        legacy.Pinned,
			Width:         legacy.Width,
			Height:        legacy.Height,
			Bytes:         int64(len(raw)),
			Signature:     signature,
			Tags:          append([]string{"legacy_x_tools"}, legacy.Tags...),
		})
		prepared = append(prepared, struct {
			entry Entry
			data  []byte
		}{entry: entry, data: raw})
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range prepared {
		duplicate := false
		for i := range s.entries {
			if s.entries[i].Signature == item.entry.Signature {
				if item.entry.Pinned {
					s.entries[i].Pinned = true
				}
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		if err := s.writeImage(item.entry.ImagePath, item.data); err != nil {
			s.lastCaptureError = err.Error()
			continue
		}
		s.writeThumbnail(&item.entry, item.data)
		s.entries = append([]Entry{item.entry}, s.entries...)
	}
	s.trimLocked()
	s.saveLockedWithStatus()
	return s.statusLocked(true)
}

func (s *Service) addPNGWithTags(data []byte, width int, height int, source string, savedPath string, actions []string, tags []string) Status {
	entry := s.makeEntry(data, width, height, source, savedPath, actions, tags)
	s.mu.Lock()
	if entry.ID == "" || len(data) == 0 {
		status := s.statusLocked(false)
		s.mu.Unlock()
		return status
	}
	if err := s.writeImage(entry.ImagePath, data); err != nil {
		s.lastCaptureError = err.Error()
		status := s.statusLocked(true)
		s.mu.Unlock()
		return status
	}
	s.writeThumbnail(&entry, data)
	s.lastCaptureError = ""
	s.entries = append([]Entry{entry}, s.entries...)
	s.trimLocked()
	if shouldSaveCaptureHistoryAsync(source) {
		s.scheduleSaveLocked()
	} else {
		s.saveLockedWithStatus()
	}
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
		if entry.ID == id {
			s.removeStoredEntry(entry)
			continue
		}
		next = append(next, entry)
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
			continue
		}
		s.removeStoredEntry(entry)
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
		s.removeStoredEntry(entry)
	}
	s.entries = next
	if result.Removed > 0 {
		s.saveLockedWithStatus()
	}
	result.RemainingCount = len(s.entries)
	return result
}

func (s *Service) ApplyStoragePolicy(maxStorageMB int, keepPinned bool) RetentionResult {
	return s.applyStorageBudget(maxStorageMB, storageBudgetBytes(maxStorageMB), keepPinned)
}

func (s *Service) applyStorageBudget(maxStorageMB int, budgetBytes int64, keepPinned bool) RetentionResult {
	now := time.Now()
	result := RetentionResult{
		OK:                 true,
		MaxStorageMB:       maxStorageMB,
		StorageBudgetBytes: budgetBytes,
		KeepPinned:         keepPinned,
		AppliedAt:          now.Unix(),
	}
	if budgetBytes <= 0 {
		s.mu.RLock()
		result.Kept = len(s.entries)
		result.RemainingCount = len(s.entries)
		for _, entry := range s.entries {
			if entry.Pinned {
				result.KeptPinned++
			}
			result.RemainingBytes += entryStorageBytes(entry)
		}
		s.mu.RUnlock()
		return result
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	entryBytes := make(map[string]int64, len(s.entries))
	totalBytes := int64(0)
	for _, entry := range s.entries {
		bytes := entryStorageBytes(entry)
		entryBytes[entry.ID] = bytes
		totalBytes += bytes
	}
	if totalBytes <= budgetBytes {
		result.Kept = len(s.entries)
		result.RemainingCount = len(s.entries)
		result.RemainingBytes = totalBytes
		for _, entry := range s.entries {
			if entry.Pinned {
				result.KeptPinned++
			}
		}
		return result
	}

	removable := make([]Entry, 0, len(s.entries))
	for _, entry := range s.entries {
		if keepPinned && entry.Pinned {
			continue
		}
		removable = append(removable, entry)
	}
	sort.SliceStable(removable, func(i, j int) bool {
		if removable[i].CreatedAt == removable[j].CreatedAt {
			return removable[i].ID < removable[j].ID
		}
		return removable[i].CreatedAt < removable[j].CreatedAt
	})

	removeIDs := make(map[string]bool)
	for _, entry := range removable {
		if totalBytes <= budgetBytes {
			break
		}
		removeIDs[entry.ID] = true
		totalBytes -= entryBytes[entry.ID]
		if totalBytes < 0 {
			totalBytes = 0
		}
		result.Removed++
		result.RemovedByStorage++
		s.removeStoredEntry(entry)
	}
	if len(removeIDs) > 0 {
		next := make([]Entry, 0, len(s.entries)-len(removeIDs))
		for _, entry := range s.entries {
			if removeIDs[entry.ID] {
				continue
			}
			next = append(next, entry)
		}
		s.entries = next
		s.saveLockedWithStatus()
	}
	result.Kept = len(s.entries)
	result.RemainingCount = len(s.entries)
	result.RemainingBytes = totalBytes
	for _, entry := range s.entries {
		if entry.Pinned {
			result.KeptPinned++
		}
	}
	return result
}

func (s *Service) ImageDataURL(id string) string {
	return s.imageDataURL(id, false)
}

func (s *Service) ThumbnailDataURL(id string) string {
	return s.imageDataURL(id, true)
}

func (s *Service) imageDataURL(id string, preferThumbnail bool) string {
	id = strings.TrimSpace(id)
	s.mu.RLock()
	var imagePath string
	for _, entry := range s.entries {
		if entry.ID == id {
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
	if err != nil || len(raw) == 0 {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)
}

func (s *Service) Search(query string) []contracts.SearchResult {
	query = captureQuery(query)
	if len([]rune(query)) < 2 {
		return nil
	}
	entries := s.List(query, 24)
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

func captureQuery(query string) string {
	query = strings.TrimSpace(query)
	parts := strings.Fields(query)
	if len(parts) == 0 {
		return ""
	}
	switch strings.ToLower(parts[0]) {
	case "cap", "capture", "shot", "screenshot", "截图历史", "捕获历史", "截图":
		return strings.TrimSpace(strings.TrimPrefix(query, parts[0]))
	default:
		return query
	}
}

func (s *Service) listLocked(query string, limit int) []Entry {
	if limit <= 0 {
		limit = defaultStatusEntryLimit
	}
	if limit > maxListEntryLimit {
		limit = maxListEntryLimit
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
	localAppData := os.Getenv("LOCALAPPDATA")
	virtualizedPath, virtualizedExists, virtualizedBytes := findVirtualizedFile(s.path, os.Getenv("APPDATA"), localAppData)
	virtualizedImageDir, virtualizedImageCount, virtualizedImageBytes := findVirtualizedDir(s.imageDir, os.Getenv("APPDATA"), localAppData)
	thumbnailCount, thumbnailBytes := countFilesInDir(s.thumbnailDir)
	status := Status{
		Path:                  firstNonEmpty(appdb.DatabasePathForPath(s.path), s.path),
		ImageDir:              s.imageDir,
		ThumbnailDir:          s.thumbnailDir,
		Count:                 len(s.entries),
		ThumbnailCount:        thumbnailCount,
		ThumbnailBytes:        thumbnailBytes,
		LastSaveError:         s.lastSaveError,
		LastCaptureError:      s.lastCaptureError,
		VirtualizedPath:       virtualizedPath,
		VirtualizedExists:     virtualizedExists,
		VirtualizedBytes:      virtualizedBytes,
		VirtualizedImageDir:   virtualizedImageDir,
		VirtualizedImageCount: virtualizedImageCount,
		VirtualizedImageBytes: virtualizedImageBytes,
	}
	for _, entry := range s.entries {
		if entry.Pinned {
			status.PinnedCount++
		}
		if entry.CreatedAt > status.LastEntryAt {
			status.LastEntryAt = entry.CreatedAt
		}
	}
	if includeEntries {
		status.Entries = s.listLocked("", defaultStatusEntryLimit)
	}
	return status
}

func (s *Service) load() {
	if s.path == "" {
		return
	}
	entries, ok, err := loadEntriesFromSQLite(s.path)
	if err != nil || !ok {
		return
	}
	for _, entry := range entries {
		entry = normalizeEntry(entry)
		if entry.ID != "" && entry.ImagePath != "" {
			s.entries = append(s.entries, entry)
		}
	}
	sortEntries(s.entries)
	s.trimLocked()
}

func (s *Service) saveLockedWithStatus() {
	s.saveDirty = false
	s.lastSaveError = ""
	if err := s.saveLocked(); err != nil {
		s.lastSaveError = err.Error()
	}
}

func (s *Service) scheduleSaveLocked() {
	if s.path == "" {
		return
	}
	s.lastSaveError = ""
	s.saveDirty = true
	if s.saveScheduled {
		return
	}
	delay := historySaveDebounceDelay
	if delay <= 0 {
		s.saveLockedWithStatus()
		return
	}
	s.saveScheduled = true
	time.AfterFunc(delay, s.runScheduledSave)
}

func (s *Service) runScheduledSave() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveScheduled = false
	if !s.saveDirty {
		return
	}
	s.saveLockedWithStatus()
}

func (s *Service) saveLocked() error {
	if s.path == "" {
		return nil
	}
	return saveEntriesToSQLite(s.path, s.entries)
}

func shouldSaveCaptureHistoryAsync(source string) bool {
	source = strings.ToLower(strings.TrimSpace(source))
	return strings.Contains(source, "time_machine")
}

func (s *Service) trimLocked() {
	if s.maxEntries <= 0 {
		return
	}
	for len(s.entries) > s.maxEntries {
		remove := len(s.entries) - 1
		for i := len(s.entries) - 1; i >= 0; i-- {
			if !s.entries[i].Pinned {
				remove = i
				break
			}
		}
		s.removeStoredEntry(s.entries[remove])
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
		if entry.ThumbnailPath == "" {
			entry.ThumbnailPath = s.thumbnailPathForEntry(*entry)
		}
		s.writeThumbnail(entry, raw)
		if entry.ThumbnailPath != "" && fileExists(entry.ThumbnailPath) {
			result.Created++
			changed = true
			continue
		}
		if entry.Width > imagepreview.DefaultMaxSide || entry.Height > imagepreview.DefaultMaxSide {
			result.Failed++
		} else {
			result.Skipped++
			changed = true
		}
	}
	if changed {
		s.saveLockedWithStatus()
	}
	return result
}

func (s *Service) makeEntry(data []byte, width int, height int, source string, savedPath string, actions []string, tags []string) Entry {
	source = strings.TrimSpace(source)
	if source == "" {
		source = "manual"
	}
	signature := "png:" + sha1HexBytes(data)
	now := time.Now()
	createdAt := now.Unix()
	id := stableID(signature + "|" + now.UTC().Format(time.RFC3339Nano) + "|" + strconv.Itoa(len(data)))
	imagePath := filepath.Join(s.imageDir, "capture-"+now.Format("20060102-150405.000000000")+"-"+id+".png")
	thumbnailPath := s.thumbnailPathForEntry(Entry{ID: id, ImagePath: imagePath})
	return normalizeEntry(Entry{
		ID:            id,
		ImagePath:     imagePath,
		ThumbnailPath: thumbnailPath,
		SavedPath:     strings.TrimSpace(savedPath),
		CreatedAt:     createdAt,
		Source:        source,
		Actions:       cleanStrings(actions),
		Width:         width,
		Height:        height,
		Bytes:         int64(len(data)),
		Signature:     signature,
		Tags:          append([]string{"截图", "捕获历史", fmt.Sprintf("%dx%d", width, height)}, cleanStrings(tags)...),
	})
}

func (s *Service) writeImage(path string, data []byte) error {
	if path == "" {
		return fmt.Errorf("缺少截图保存路径")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (s *Service) writeThumbnail(entry *Entry, data []byte) {
	if entry == nil || entry.ThumbnailPath == "" {
		return
	}
	thumbnail, width, height, ok, err := imagepreview.CreatePNGThumbnail(data, imagepreview.DefaultMaxSide)
	if err != nil || !ok {
		clearThumbnailFields(entry)
		return
	}
	if err := s.writeImage(entry.ThumbnailPath, thumbnail); err != nil {
		clearThumbnailFields(entry)
		return
	}
	entry.ThumbnailWidth = width
	entry.ThumbnailHeight = height
	entry.ThumbnailBytes = int64(len(thumbnail))
}

func (s *Service) thumbnailPathForEntry(entry Entry) string {
	if s.thumbnailDir == "" {
		return ""
	}
	stem := strings.TrimSuffix(filepath.Base(entry.ImagePath), filepath.Ext(entry.ImagePath))
	if stem == "" {
		stem = entry.ID
	}
	if stem == "" {
		stem = stableID(entry.ImagePath)
	}
	return filepath.Join(s.thumbnailDir, stem+".thumb.png")
}

func (s *Service) removeStoredEntry(entry Entry) {
	removeFileInside(entry.ImagePath, s.imageDir)
	removeFileInside(entry.ThumbnailPath, s.thumbnailDir)
}

func clearThumbnailFields(entry *Entry) {
	entry.ThumbnailPath = ""
	entry.ThumbnailWidth = 0
	entry.ThumbnailHeight = 0
	entry.ThumbnailBytes = 0
}

func normalizeEntry(entry Entry) Entry {
	entry.ID = strings.TrimSpace(entry.ID)
	entry.ImagePath = strings.TrimSpace(entry.ImagePath)
	entry.ThumbnailPath = strings.TrimSpace(entry.ThumbnailPath)
	entry.SavedPath = strings.TrimSpace(entry.SavedPath)
	entry.Source = strings.TrimSpace(entry.Source)
	if entry.Source == "" {
		entry.Source = "import"
	}
	if entry.CreatedAt == 0 {
		entry.CreatedAt = time.Now().Unix()
	}
	if entry.Width < 0 {
		entry.Width = 0
	}
	if entry.Height < 0 {
		entry.Height = 0
	}
	if entry.ThumbnailWidth < 0 {
		entry.ThumbnailWidth = 0
	}
	if entry.ThumbnailHeight < 0 {
		entry.ThumbnailHeight = 0
	}
	if entry.ThumbnailBytes < 0 {
		entry.ThumbnailBytes = 0
	}
	entry.Actions = cleanStrings(entry.Actions)
	entry.Tags = cleanStrings(entry.Tags)
	if entry.Signature == "" {
		entry.Signature = "path:" + sha1Hex(entry.ImagePath)
	}
	if entry.ID == "" {
		entry.ID = stableID(entry.Signature)
	}
	dimension := fmt.Sprintf("%dx%d", entry.Width, entry.Height)
	entry.Tags = appendTag(entry.Tags, "截图")
	entry.Tags = appendTag(entry.Tags, "捕获历史")
	if entry.Width > 0 && entry.Height > 0 {
		entry.Tags = appendTag(entry.Tags, dimension)
	}
	return entry
}

func entryToResult(entry Entry, score float64) contracts.SearchResult {
	pinID := "capture_pin"
	pinLabel := "置顶"
	pinSuccess := "已置顶"
	if entry.Pinned {
		pinID = "capture_unpin"
		pinLabel = "取消置顶"
		pinSuccess = "已取消置顶"
	}
	title := fmt.Sprintf("截图 %s", time.Unix(entry.CreatedAt, 0).Format("01-02 15:04"))
	if entry.Width > 0 && entry.Height > 0 {
		title = fmt.Sprintf("%s · %dx%d", title, entry.Width, entry.Height)
	}
	pathForOpen := entry.ImagePath
	if entry.SavedPath != "" {
		pathForOpen = entry.SavedPath
	}
	return contracts.SearchResult{
		ID:       "capture-" + entry.ID,
		Type:     contracts.ResultCapture,
		Title:    title,
		Subtitle: "捕获历史 · " + entry.Source,
		Detail:   entry.ImagePath,
		Icon:     "capture",
		Score:    score,
		Tags:     append([]string{}, entry.Tags...),
		Payload: map[string]interface{}{
			"captureId":     entry.ID,
			"imagePath":     entry.ImagePath,
			"thumbnailPath": entry.ThumbnailPath,
			"savedPath":     entry.SavedPath,
			"pinned":        entry.Pinned,
		},
		Preview: contracts.PreviewDescriptor{
			Kind:      contracts.PreviewImage,
			Title:     title,
			Subtitle:  "本地截图历史",
			Text:      entry.ImagePath,
			ImageHint: fmt.Sprintf("%dx%d · %s", entry.Width, entry.Height, formatBytes(entry.Bytes)),
			Meta: []contracts.LabelValue{
				{Label: "来源", Value: entry.Source},
				{Label: "尺寸", Value: fmt.Sprintf("%dx%d", entry.Width, entry.Height)},
				{Label: "置顶", Value: boolLabel(entry.Pinned)},
			},
			Evidence: []contracts.LabelValue{
				{Label: "记录时间", Value: time.Unix(entry.CreatedAt, 0).Format("2006-01-02 15:04:05")},
				{Label: "路径", Value: entry.ImagePath},
				{Label: "预览图", Value: thumbnailLabel(entry)},
			},
		},
		Actions: []contracts.PreviewAction{
			{
				ID:       "open_capture",
				Label:    "打开",
				Icon:     "open",
				Kind:     contracts.ActionOpen,
				Shortcut: "Enter",
				Payload:  map[string]interface{}{"path": pathForOpen},
				Feedback: &contracts.ActionFeedback{SuccessLabel: "已打开", DurationMS: 1400},
			},
			{
				ID:      "open_capture_parent",
				Label:   "打开所在文件夹",
				Icon:    "folder",
				Kind:    contracts.ActionOpenParent,
				Payload: map[string]interface{}{"path": entry.ImagePath},
			},
			contracts.CopyAction("copy_capture_path", "复制路径", entry.ImagePath, ""),
			{
				ID:    "pin_capture_image",
				Label: "创建贴图",
				Icon:  "pin",
				Kind:  contracts.ActionPin,
				Payload: map[string]interface{}{
					"captureId": entry.ID,
				},
				Feedback: &contracts.ActionFeedback{SuccessLabel: "已创建贴图", DurationMS: 1400},
			},
			{
				ID:    "recognize_qr",
				Label: "识别二维码",
				Icon:  "plugin",
				Kind:  contracts.ActionPlugin,
				Payload: map[string]interface{}{
					"captureId": entry.ID,
				},
				Feedback: &contracts.ActionFeedback{SuccessLabel: "已识别二维码", DurationMS: 1600},
			},
			{
				ID:    pinID,
				Label: pinLabel,
				Icon:  "pin",
				Kind:  contracts.ActionPin,
				Payload: map[string]interface{}{
					"captureId": entry.ID,
				},
				Feedback: &contracts.ActionFeedback{SuccessLabel: pinSuccess, DurationMS: 1400},
			},
			contracts.RememberAction("remember_capture", "加入记忆", "capture-"+entry.ID),
		},
	}
}

func scoreEntry(entry Entry, query string) float64 {
	if query == "" {
		return 0
	}
	haystack := strings.ToLower(entryHaystack(entry))
	score := 0.0
	if strings.Contains(haystack, query) {
		score = 74
	}
	if strings.Contains(strings.ToLower(fmt.Sprintf("%dx%d", entry.Width, entry.Height)), query) {
		score = 94
	}
	if strings.Contains(strings.ToLower(entry.Source), query) {
		score += 8
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
	return strings.Contains(strings.ToLower(entryHaystack(entry)), query)
}

func entryHaystack(entry Entry) string {
	parts := []string{
		entry.ID,
		entry.ImagePath,
		entry.SavedPath,
		entry.Source,
		fmt.Sprintf("%dx%d", entry.Width, entry.Height),
		time.Unix(entry.CreatedAt, 0).Format("2006-01-02 15:04:05"),
	}
	parts = append(parts, entry.Actions...)
	parts = append(parts, entry.Tags...)
	return strings.Join(parts, " ")
}

func sortEntries(entries []Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Pinned != entries[j].Pinned {
			return entries[i].Pinned
		}
		return entries[i].CreatedAt > entries[j].CreatedAt
	})
}

func cleanStrings(items []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, item)
	}
	return result
}

func stringListContainsFold(items []string, value string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), value) {
			return true
		}
	}
	return false
}

func appendTag(tags []string, next string) []string {
	for _, tag := range tags {
		if tag == next {
			return tags
		}
	}
	return append(tags, next)
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
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

func thumbnailLabel(entry Entry) string {
	if entry.ThumbnailPath == "" {
		return "原图预览"
	}
	if entry.ThumbnailWidth > 0 && entry.ThumbnailHeight > 0 {
		return fmt.Sprintf("%dx%d · %s", entry.ThumbnailWidth, entry.ThumbnailHeight, formatBytes(entry.ThumbnailBytes))
	}
	return entry.ThumbnailPath
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

func storageBudgetBytes(maxStorageMB int) int64 {
	if maxStorageMB <= 0 {
		return 0
	}
	return int64(maxStorageMB) * bytesPerMegabyte
}

func entryStorageBytes(entry Entry) int64 {
	total := fileSize(entry.ImagePath) + fileSize(entry.ThumbnailPath)
	if total > 0 {
		return total
	}
	if entry.Bytes > 0 {
		total += entry.Bytes
	}
	if entry.ThumbnailBytes > 0 {
		total += entry.ThumbnailBytes
	}
	return total
}

func fileSize(path string) int64 {
	if path == "" {
		return 0
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return 0
	}
	return info.Size()
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func removeFileInside(path string, root string) {
	if path == "" || root == "" || !isPathInside(path, root) {
		return
	}
	_ = os.Remove(path)
}

func isPathInside(path string, root string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}

func findVirtualizedFile(path string, appData string, localAppData string) (string, bool, int64) {
	matches := virtualizedMatches(path, appData, localAppData)
	sort.Slice(matches, func(i, j int) bool {
		left, leftErr := os.Stat(matches[i])
		right, rightErr := os.Stat(matches[j])
		if leftErr != nil || rightErr != nil {
			return matches[i] < matches[j]
		}
		return left.ModTime().After(right.ModTime())
	})
	for _, match := range matches {
		info, statErr := os.Stat(match)
		if statErr == nil && !info.IsDir() {
			return match, true, info.Size()
		}
	}
	return "", false, 0
}

func findVirtualizedDir(path string, appData string, localAppData string) (string, int, int64) {
	matches := virtualizedMatches(path, appData, localAppData)
	for _, match := range matches {
		info, statErr := os.Stat(match)
		if statErr != nil || !info.IsDir() {
			continue
		}
		entries, readErr := os.ReadDir(match)
		if readErr != nil {
			return match, 0, 0
		}
		count := 0
		bytes := int64(0)
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			entryInfo, entryErr := entry.Info()
			if entryErr != nil {
				continue
			}
			count++
			bytes += entryInfo.Size()
		}
		return match, count, bytes
	}
	return "", 0, 0
}

func virtualizedMatches(path string, appData string, localAppData string) []string {
	if path == "" || appData == "" || localAppData == "" {
		return nil
	}
	relative, ok := pathRelativeTo(path, appData)
	if !ok {
		return nil
	}
	pattern := filepath.Join(localAppData, "Packages", "*", "LocalCache", "Roaming", relative)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}
	return matches
}

func pathRelativeTo(path string, base string) (string, bool) {
	cleanPath := filepath.Clean(path)
	cleanBase := filepath.Clean(base)
	relative, err := filepath.Rel(cleanBase, cleanPath)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return relative, true
}

func defaultHistoryPath() string {
	return filepath.Join(defaultBaseDir(), "capture_history.json")
}

func defaultImageDir() string {
	return filepath.Join(defaultBaseDir(), "capture_images")
}

func defaultThumbnailDir(imageDir string) string {
	if imageDir != "" {
		return filepath.Join(filepath.Dir(imageDir), "capture_thumbnails")
	}
	return filepath.Join(defaultBaseDir(), "capture_thumbnails")
}

func defaultBaseDir() string {
	base := os.Getenv("APPDATA")
	if base == "" {
		if dir, err := os.UserConfigDir(); err == nil {
			base = dir
		}
	}
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "Ariadne")
}
