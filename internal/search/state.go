package search

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"ariadne/internal/appdb"
)

type UsageRecord struct {
	ResultID   string `json:"resultId"`
	Favorite   bool   `json:"favorite"`
	UseCount   int    `json:"useCount"`
	LastUsedAt int64  `json:"lastUsedAt"`
}

type UsageStatus struct {
	Path    string        `json:"path"`
	Count   int           `json:"count"`
	Records []UsageRecord `json:"records"`
}

type ClearUsageResult struct {
	OK      bool        `json:"ok"`
	Message string      `json:"message"`
	Cleared int         `json:"cleared"`
	Status  UsageStatus `json:"status"`
}

type StateStore struct {
	mu      sync.RWMutex
	path    string
	records map[string]UsageRecord
}

func NewStateStore(path string) *StateStore {
	store := &StateStore{path: path, records: map[string]UsageRecord{}}
	store.load()
	return store
}

func defaultStatePath() string {
	base := os.Getenv("APPDATA")
	if base == "" {
		if dir, err := os.UserConfigDir(); err == nil {
			base = dir
		}
	}
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "Ariadne", "search_state.json")
}

func (s *StateStore) Get(resultID string) UsageRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.records[resultID]
}

func (s *StateStore) RecordUse(resultID string) UsageRecord {
	resultID = strings.TrimSpace(resultID)
	if resultID == "" {
		return UsageRecord{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record := s.records[resultID]
	record.ResultID = resultID
	record.UseCount++
	record.LastUsedAt = time.Now().UnixMilli()
	s.records[resultID] = record
	_ = s.saveLocked()
	return record
}

func (s *StateStore) SetFavorite(resultID string, favorite bool) UsageRecord {
	resultID = strings.TrimSpace(resultID)
	if resultID == "" {
		return UsageRecord{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record := s.records[resultID]
	record.ResultID = resultID
	record.Favorite = favorite
	if isEmptyUsageRecord(record) {
		delete(s.records, resultID)
	} else {
		s.records[resultID] = record
	}
	_ = s.saveLocked()
	return record
}

func (s *StateStore) Status() UsageStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.statusLocked()
}

func (s *StateStore) Clear() ClearUsageResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	cleared := len(s.records)
	previous := cloneUsageRecords(s.records)
	s.records = map[string]UsageRecord{}
	if err := s.saveLocked(); err != nil {
		s.records = previous
		return ClearUsageResult{
			OK:      false,
			Message: "搜索收藏和最近使用清理失败: " + err.Error(),
			Cleared: cleared,
			Status:  s.statusLocked(),
		}
	}
	return ClearUsageResult{
		OK:      true,
		Message: clearUsageMessage(cleared),
		Cleared: cleared,
		Status:  s.statusLocked(),
	}
}

func cloneUsageRecords(records map[string]UsageRecord) map[string]UsageRecord {
	next := make(map[string]UsageRecord, len(records))
	for key, record := range records {
		next[key] = record
	}
	return next
}

func isEmptyUsageRecord(record UsageRecord) bool {
	return !record.Favorite && record.UseCount <= 0 && record.LastUsedAt <= 0
}

func (s *StateStore) statusLocked() UsageStatus {
	records := make([]UsageRecord, 0, len(s.records))
	for _, record := range s.records {
		records = append(records, record)
	}
	sortUsageRecords(records)
	return UsageStatus{Path: firstNonEmpty(appdb.DatabasePathForPath(s.path), s.path), Count: len(records), Records: records}
}

func sortUsageRecords(records []UsageRecord) {
	sort.SliceStable(records, func(i, j int) bool {
		return usageRecordLess(records[i], records[j])
	})
}

func usageRecordLess(left UsageRecord, right UsageRecord) bool {
	if left.Favorite != right.Favorite {
		return left.Favorite
	}
	if left.LastUsedAt != right.LastUsedAt {
		return left.LastUsedAt > right.LastUsedAt
	}
	return left.ResultID < right.ResultID
}

func clearUsageMessage(cleared int) string {
	if cleared == 0 {
		return "没有可清理的搜索收藏或最近使用记录"
	}
	return "已清理搜索收藏和最近使用记录"
}

func (s *StateStore) load() {
	if s.path == "" {
		return
	}
	records, ok, err := loadUsageRecordsFromSQLite(s.path)
	if err != nil || !ok {
		return
	}
	for id, record := range records {
		record = normalizeUsageRecord(id, record)
		if isEmptyUsageRecord(record) {
			continue
		}
		s.records[record.ResultID] = record
	}
}

func (s *StateStore) saveLocked() error {
	if s.path == "" {
		return nil
	}
	return saveUsageRecordsToSQLite(s.path, s.records)
}

func normalizeUsageRecord(id string, record UsageRecord) UsageRecord {
	id = strings.TrimSpace(firstNonEmpty(id, record.ResultID))
	record.ResultID = id
	if id == "" {
		return UsageRecord{}
	}
	return record
}

func usageBoost(record UsageRecord, now time.Time) float64 {
	boost := 0.0
	if record.Favorite {
		boost += 1000
	}
	if record.UseCount > 0 {
		boost += minFloat(float64(record.UseCount)*4, 40)
	}
	if record.LastUsedAt > 0 {
		age := now.Sub(time.UnixMilli(record.LastUsedAt))
		switch {
		case age <= time.Hour:
			boost += 35
		case age <= 24*time.Hour:
			boost += 25
		case age <= 7*24*time.Hour:
			boost += 15
		case age <= 30*24*time.Hour:
			boost += 6
		}
	}
	return boost
}

func minFloat(left float64, right float64) float64 {
	if left < right {
		return left
	}
	return right
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
