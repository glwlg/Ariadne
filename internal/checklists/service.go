package checklists

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"ariadne/internal/workmemory"
)

var checklistIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Checklist struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Context   string   `json:"context"`
	Items     []string `json:"items"`
	Evidence  []string `json:"evidence"`
	Source    string   `json:"source,omitempty"`
	CreatedAt int64    `json:"createdAt,omitempty"`
	UpdatedAt int64    `json:"updatedAt,omitempty"`
}

type Status struct {
	Path          string      `json:"path"`
	Count         int         `json:"count"`
	LastSaveError string      `json:"lastSaveError,omitempty"`
	Checklists    []Checklist `json:"checklists"`
}

type DraftSaveRequest struct {
	Draft     workmemory.ChecklistDraft `json:"draft"`
	Confirmed bool                      `json:"confirmed"`
}

type DraftSaveResult struct {
	OK                   bool      `json:"ok"`
	Message              string    `json:"message"`
	Checklist            Checklist `json:"checklist,omitempty"`
	Status               Status    `json:"status"`
	RequiresConfirmation bool      `json:"requiresConfirmation,omitempty"`
	RiskReasons          []string  `json:"riskReasons,omitempty"`
}

type Service struct {
	mu            sync.RWMutex
	path          string
	checklists    []Checklist
	lastSaveError string
}

type stateFile struct {
	Version    int         `json:"version"`
	Checklists []Checklist `json:"checklists"`
}

func NewService() *Service {
	return NewServiceWithPath(defaultPath())
}

func NewServiceWithPath(path string) *Service {
	service := &Service{path: path}
	service.load()
	return service
}

func (s *Service) List() []Checklist {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneChecklists(s.checklists)
}

func (s *Service) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.statusLocked()
}

func (s *Service) Upsert(next Checklist) Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalized, ok := normalizeChecklist(next)
	if !ok {
		s.lastSaveError = "检查清单 ID、标题或条目无效"
		return s.statusLocked()
	}
	if normalized.CreatedAt == 0 {
		normalized.CreatedAt = time.Now().Unix()
	}
	normalized.UpdatedAt = time.Now().Unix()

	replaced := false
	for index := range s.checklists {
		if s.checklists[index].ID == normalized.ID {
			s.checklists[index] = normalized
			replaced = true
			break
		}
	}
	if !replaced {
		s.checklists = append(s.checklists, normalized)
	}
	sortChecklists(s.checklists)
	s.lastSaveError = ""
	if err := s.saveLocked(); err != nil {
		s.lastSaveError = err.Error()
	}
	return s.statusLocked()
}

func (s *Service) SaveChecklistDraft(request DraftSaveRequest) DraftSaveResult {
	checklist, ok := checklistFromDraft(request.Draft)
	if !ok {
		return DraftSaveResult{OK: false, Message: "检查清单草稿无效", Status: s.Status()}
	}
	riskReasons := draftRiskReasons(request.Draft, checklist)
	if !request.Confirmed {
		if len(riskReasons) == 0 {
			riskReasons = []string{"检查清单来自工作记忆经验发现，保存为正式资产前需要用户确认"}
		}
		return DraftSaveResult{
			OK:                   false,
			Message:              "保存检查清单需要确认",
			Checklist:            checklist,
			Status:               s.Status(),
			RequiresConfirmation: true,
			RiskReasons:          riskReasons,
		}
	}
	status := s.Upsert(checklist)
	if status.LastSaveError != "" {
		return DraftSaveResult{OK: false, Message: "检查清单保存失败: " + status.LastSaveError, Checklist: checklist, Status: status}
	}
	return DraftSaveResult{OK: true, Message: "检查清单已保存为正式资产", Checklist: checklist, Status: status, RiskReasons: riskReasons}
}

func (s *Service) statusLocked() Status {
	return Status{
		Path:          s.path,
		Count:         len(s.checklists),
		LastSaveError: s.lastSaveError,
		Checklists:    cloneChecklists(s.checklists),
	}
}

func (s *Service) load() {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var payload stateFile
	if err := json.Unmarshal(raw, &payload); err == nil {
		s.checklists = normalizeChecklists(payload.Checklists)
		return
	}
	var legacy []Checklist
	if err := json.Unmarshal(raw, &legacy); err == nil {
		s.checklists = normalizeChecklists(legacy)
	}
}

func (s *Service) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	payload := stateFile{
		Version:    1,
		Checklists: cloneChecklists(s.checklists),
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o600)
}

func checklistFromDraft(draft workmemory.ChecklistDraft) (Checklist, bool) {
	title := strings.TrimSpace(draft.Title)
	if title == "" {
		title = "工作记忆检查清单"
	}
	checklist := Checklist{
		ID:        checklistIDFromDraft(draft),
		Title:     title,
		Context:   strings.TrimSpace(draft.Context),
		Items:     cleanDraftStrings(draft.Items),
		Evidence:  cleanDraftStrings(draft.Evidence),
		Source:    "work_memory",
		CreatedAt: draft.CreatedAt,
		UpdatedAt: time.Now().Unix(),
	}
	if checklist.Context == "" {
		checklist.Context = "由工作记忆经验发现生成，保存前需要用户审阅。"
	}
	return normalizeChecklist(checklist)
}

func checklistIDFromDraft(draft workmemory.ChecklistDraft) string {
	id := strings.ToLower(strings.TrimSpace(draft.ID))
	id = strings.TrimPrefix(id, "checklist-draft-")
	id = strings.Trim(id, "-")
	if id != "" {
		candidate := "memory-checklist-" + slugIDPart(id)
		if checklistIDPattern.MatchString(candidate) {
			return candidate
		}
	}
	sum := sha1.Sum([]byte(strings.ToLower(strings.TrimSpace(strings.Join([]string{
		draft.Title,
		draft.Context,
		fmt.Sprint(draft.CreatedAt),
	}, "\n")))))
	return "memory-checklist-" + hex.EncodeToString(sum[:])[:12]
}

func draftRiskReasons(draft workmemory.ChecklistDraft, checklist Checklist) []string {
	reasons := []string{}
	if draft.RequiresReview {
		reasons = append(reasons, "草稿来自工作记忆经验发现，需要确认后才写入正式检查清单")
	}
	if len(checklist.Evidence) == 0 {
		reasons = append(reasons, "缺少证据引用，后续复盘时需要人工补证据")
	}
	if len(checklist.Items) > 8 {
		reasons = append(reasons, "清单条目较多，建议保存前快速审阅")
	}
	return uniqueStrings(reasons)
}

func normalizeChecklist(checklist Checklist) (Checklist, bool) {
	id := strings.ToLower(strings.TrimSpace(checklist.ID))
	if !checklistIDPattern.MatchString(id) {
		return Checklist{}, false
	}
	title := strings.TrimSpace(checklist.Title)
	if title == "" {
		return Checklist{}, false
	}
	items := cleanDraftStrings(checklist.Items)
	if len(items) == 0 {
		return Checklist{}, false
	}
	createdAt := checklist.CreatedAt
	if createdAt == 0 {
		createdAt = checklist.UpdatedAt
	}
	if createdAt == 0 {
		createdAt = time.Now().Unix()
	}
	return Checklist{
		ID:        id,
		Title:     title,
		Context:   strings.TrimSpace(checklist.Context),
		Items:     items,
		Evidence:  cleanDraftStrings(checklist.Evidence),
		Source:    strings.TrimSpace(checklist.Source),
		CreatedAt: createdAt,
		UpdatedAt: checklist.UpdatedAt,
	}, true
}

func normalizeChecklists(checklists []Checklist) []Checklist {
	result := make([]Checklist, 0, len(checklists))
	for _, checklist := range checklists {
		normalized, ok := normalizeChecklist(checklist)
		if ok {
			result = append(result, normalized)
		}
	}
	sortChecklists(result)
	return result
}

func sortChecklists(checklists []Checklist) {
	sort.SliceStable(checklists, func(i, j int) bool {
		left := checklists[i].UpdatedAt
		if left == 0 {
			left = checklists[i].CreatedAt
		}
		right := checklists[j].UpdatedAt
		if right == 0 {
			right = checklists[j].CreatedAt
		}
		if left == right {
			return checklists[i].ID < checklists[j].ID
		}
		return left > right
	})
}

func cloneChecklists(checklists []Checklist) []Checklist {
	result := make([]Checklist, len(checklists))
	for index, checklist := range checklists {
		result[index] = checklist
		result[index].Items = append([]string(nil), checklist.Items...)
		result[index].Evidence = append([]string(nil), checklist.Evidence...)
	}
	return result
}

func cleanDraftStrings(values []string) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func uniqueStrings(values []string) []string {
	return cleanDraftStrings(values)
}

func slugIDPart(value string) string {
	var builder strings.Builder
	lastHyphen := false
	for _, char := range strings.ToLower(strings.TrimSpace(value)) {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' {
			builder.WriteRune(char)
			lastHyphen = false
			continue
		}
		if !lastHyphen {
			builder.WriteRune('-')
			lastHyphen = true
		}
	}
	slug := strings.Trim(builder.String(), "-")
	if slug != "" && len(slug) <= 48 {
		return slug
	}
	sum := sha1.Sum([]byte(strings.ToLower(strings.TrimSpace(value))))
	return hex.EncodeToString(sum[:])[:12]
}

func defaultPath() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = os.Getenv("APPDATA")
	}
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "Ariadne", "checklists.json")
}
