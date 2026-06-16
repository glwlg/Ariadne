package workflows

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ariadne/internal/contracts"
	"ariadne/internal/workmemory"
)

var workflowIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var placeholderPattern = regexp.MustCompile(`\{([^{}]+)\}`)

var workflowKeywords = map[string]bool{
	"wf":       true,
	"workflow": true,
	"flow":     true,
	"macro":    true,
	"工作流":      true,
	"宏":        true,
}

var allowedVariables = map[string]bool{
	"clipboard": true,
	"input":     true,
	"prev":      true,
}

type CommandExecutor interface {
	Execute(keyword string, query string) []contracts.SearchResult
}

type Step struct {
	Command string `json:"command"`
	Pick    string `json:"pick,omitempty"`
}

type Workflow struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Steps       []Step `json:"steps"`
	UpdatedAt   int64  `json:"updatedAt,omitempty"`
}

type Status struct {
	Path           string     `json:"path"`
	LegacyPath     string     `json:"legacyPath"`
	Count          int        `json:"count"`
	LastSaveError  string     `json:"lastSaveError,omitempty"`
	LegacyImported bool       `json:"legacyImported"`
	Workflows      []Workflow `json:"workflows"`
}

type RunRequest struct {
	WorkflowID    string `json:"workflowId"`
	Input         string `json:"input,omitempty"`
	ClipboardText string `json:"clipboardText,omitempty"`
	Confirmed     bool   `json:"confirmed,omitempty"`
}

type StepRun struct {
	Index           int    `json:"index"`
	Command         string `json:"command"`
	RenderedCommand string `json:"renderedCommand"`
	Pick            string `json:"pick,omitempty"`
	PickedTitle     string `json:"pickedTitle,omitempty"`
	Output          string `json:"output,omitempty"`
	OK              bool   `json:"ok"`
	Message         string `json:"message,omitempty"`
}

type RunResult struct {
	OK                   bool      `json:"ok"`
	Message              string    `json:"message"`
	WorkflowID           string    `json:"workflowId"`
	WorkflowName         string    `json:"workflowName"`
	Output               string    `json:"output,omitempty"`
	Steps                []StepRun `json:"steps"`
	RequiresConfirmation bool      `json:"requiresConfirmation,omitempty"`
	RiskReasons          []string  `json:"riskReasons,omitempty"`
}

type ExportResult struct {
	OK         bool   `json:"ok"`
	Message    string `json:"message"`
	Path       string `json:"path,omitempty"`
	JSON       string `json:"json,omitempty"`
	Count      int    `json:"count"`
	Bytes      int64  `json:"bytes,omitempty"`
	ExportedAt int64  `json:"exportedAt,omitempty"`
}

type ImportResult struct {
	OK            bool   `json:"ok"`
	Message       string `json:"message"`
	ImportedCount int    `json:"importedCount"`
	Status        Status `json:"status"`
}

type DraftSaveRequest struct {
	Draft     workmemory.WorkflowDraft `json:"draft"`
	Confirmed bool                     `json:"confirmed"`
}

type DraftSaveResult struct {
	OK                   bool     `json:"ok"`
	Message              string   `json:"message"`
	Workflow             Workflow `json:"workflow,omitempty"`
	Status               Status   `json:"status"`
	RequiresConfirmation bool     `json:"requiresConfirmation,omitempty"`
	RiskReasons          []string `json:"riskReasons,omitempty"`
}

type Service struct {
	mu             sync.RWMutex
	path           string
	legacyPath     string
	workflows      []Workflow
	removed        map[string]bool
	executor       CommandExecutor
	lastSaveError  string
	legacyImported bool
}

type workflowExportPayload struct {
	Version    int        `json:"version"`
	ExportedBy string     `json:"exportedBy,omitempty"`
	ExportedAt int64      `json:"exportedAt,omitempty"`
	Workflows  []Workflow `json:"workflows"`
}

func NewService(executor CommandExecutor) *Service {
	return NewServiceWithPaths(defaultPath(), defaultLegacyPath(), executor)
}

func NewServiceWithPaths(path string, legacyPath string, executor CommandExecutor) *Service {
	service := &Service{
		path:       path,
		legacyPath: legacyPath,
		workflows:  defaultWorkflows(),
		removed:    map[string]bool{},
		executor:   executor,
	}
	service.load()
	return service
}

func (s *Service) Search(query string) []contracts.SearchResult {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}

	needle, forced := workflowQuery(query)
	if !forced && len([]rune(needle)) < 2 {
		return nil
	}

	workflows := s.List()
	results := make([]contracts.SearchResult, 0, len(workflows))
	first, input := splitFirst(needle)
	for _, workflow := range workflows {
		if len(workflow.Steps) == 0 {
			continue
		}
		actionInput := ""
		score := workflowScore(workflow, needle)
		if forced && first != "" && first == strings.ToLower(workflow.ID) {
			score = 96
			actionInput = input
		}
		if forced && needle == "" {
			score = 82
		}
		if score <= 0 {
			continue
		}
		results = append(results, workflowToResult(workflow, actionInput, score))
	}
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	return results
}

func (s *Service) List() []Workflow {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneWorkflows(s.workflows)
}

func (s *Service) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.statusLocked()
}

func (s *Service) NewWorkflow() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := uniqueID(s.workflows, "new-workflow")
	s.workflows = append(s.workflows, Workflow{
		ID:          id,
		Name:        "新工作流",
		Description: "描述这个命令链的用途",
		Steps:       []Step{{Command: "hash {input}", Pick: "MD5"}},
		UpdatedAt:   time.Now().Unix(),
	})
	sortWorkflows(s.workflows)
	s.lastSaveError = ""
	if err := s.saveLocked(); err != nil {
		s.lastSaveError = err.Error()
	}
	return s.statusLocked()
}

func (s *Service) Upsert(next Workflow) Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalized, ok := normalizeWorkflow(next)
	if !ok {
		s.lastSaveError = "工作流 ID、名称或步骤无效"
		return s.statusLocked()
	}
	normalized.UpdatedAt = time.Now().Unix()

	replaced := false
	for i := range s.workflows {
		if s.workflows[i].ID == normalized.ID {
			s.workflows[i] = normalized
			replaced = true
			break
		}
	}
	if !replaced {
		s.workflows = append(s.workflows, normalized)
	}
	delete(s.removed, normalized.ID)
	sortWorkflows(s.workflows)
	s.lastSaveError = ""
	if err := s.saveLocked(); err != nil {
		s.lastSaveError = err.Error()
	}
	return s.statusLocked()
}

func (s *Service) Remove(id string) Status {
	id = strings.TrimSpace(strings.ToLower(id))
	s.mu.Lock()
	defer s.mu.Unlock()
	next := make([]Workflow, 0, len(s.workflows))
	for _, workflow := range s.workflows {
		if workflow.ID != id {
			next = append(next, workflow)
		}
	}
	s.workflows = next
	if id != "" {
		s.removed[id] = true
	}
	s.lastSaveError = ""
	if err := s.saveLocked(); err != nil {
		s.lastSaveError = err.Error()
	}
	return s.statusLocked()
}

func (s *Service) ExportData() ExportResult {
	workflows := s.List()
	exportedAt := time.Now()
	payload := workflowExportPayload{
		Version:    1,
		ExportedBy: "Ariadne",
		ExportedAt: exportedAt.Unix(),
		Workflows:  workflows,
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return ExportResult{OK: false, Message: err.Error(), Count: len(workflows), ExportedAt: exportedAt.Unix()}
	}
	path := workflowExportPath(s.path, exportedAt)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return ExportResult{OK: false, Message: err.Error(), Path: path, JSON: string(raw), Count: len(workflows), ExportedAt: exportedAt.Unix()}
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return ExportResult{OK: false, Message: err.Error(), Path: path, JSON: string(raw), Count: len(workflows), ExportedAt: exportedAt.Unix()}
	}
	bytes := int64(len(raw))
	if info, err := os.Stat(path); err == nil {
		bytes = info.Size()
	}
	return ExportResult{
		OK:         true,
		Message:    "工作流已导出",
		Path:       path,
		JSON:       string(raw),
		Count:      len(workflows),
		Bytes:      bytes,
		ExportedAt: exportedAt.Unix(),
	}
}

func (s *Service) ImportData(raw string) ImportResult {
	workflows, err := parseWorkflowImport(raw)
	if err != nil {
		s.mu.RLock()
		status := s.statusLocked()
		s.mu.RUnlock()
		return ImportResult{OK: false, Message: err.Error(), Status: status}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	imported := 0
	for _, workflow := range workflows {
		normalized, ok := normalizeWorkflow(workflow)
		if !ok {
			continue
		}
		replaced := false
		for index := range s.workflows {
			if s.workflows[index].ID == normalized.ID {
				s.workflows[index] = normalized
				replaced = true
				break
			}
		}
		if !replaced {
			s.workflows = append(s.workflows, normalized)
		}
		delete(s.removed, normalized.ID)
		imported++
	}
	if imported == 0 {
		s.lastSaveError = "导入内容没有合法工作流"
		return ImportResult{OK: false, Message: s.lastSaveError, Status: s.statusLocked()}
	}
	sortWorkflows(s.workflows)
	s.lastSaveError = ""
	if err := s.saveLocked(); err != nil {
		s.lastSaveError = err.Error()
		return ImportResult{OK: false, Message: "导入后保存失败: " + err.Error(), ImportedCount: imported, Status: s.statusLocked()}
	}
	return ImportResult{OK: true, Message: fmt.Sprintf("已导入 %d 个工作流", imported), ImportedCount: imported, Status: s.statusLocked()}
}

func (s *Service) SaveWorkflowDraft(request DraftSaveRequest) DraftSaveResult {
	workflow, ok := workflowFromDraft(request.Draft)
	if !ok {
		return DraftSaveResult{OK: false, Message: "候选工作流草稿无效", Status: s.Status()}
	}
	riskReasons := draftRiskReasons(request.Draft, workflow)
	if !request.Confirmed {
		if len(riskReasons) == 0 {
			riskReasons = []string{"候选工作流来自工作记忆经验发现，保存为正式工作流前需要用户确认"}
		}
		return DraftSaveResult{
			OK:                   false,
			Message:              "保存候选工作流需要确认",
			Workflow:             workflow,
			Status:               s.Status(),
			RequiresConfirmation: true,
			RiskReasons:          riskReasons,
		}
	}
	status := s.Upsert(workflow)
	if status.LastSaveError != "" {
		return DraftSaveResult{OK: false, Message: "候选工作流保存失败: " + status.LastSaveError, Workflow: workflow, Status: status}
	}
	return DraftSaveResult{OK: true, Message: "候选工作流已保存为正式工作流", Workflow: workflow, Status: status, RiskReasons: riskReasons}
}

func (s *Service) Run(request RunRequest) RunResult {
	workflowID := strings.TrimSpace(strings.ToLower(request.WorkflowID))
	if workflowID == "" {
		return RunResult{OK: false, Message: "缺少工作流 ID"}
	}
	workflow, ok := s.find(workflowID)
	if !ok {
		return RunResult{OK: false, Message: "未找到对应工作流", WorkflowID: workflowID}
	}
	if s.executor == nil {
		return RunResult{OK: false, Message: "工作流执行器未接入", WorkflowID: workflow.ID, WorkflowName: workflow.Name}
	}

	context := map[string]string{
		"clipboard": strings.TrimSpace(request.ClipboardText),
		"input":     strings.TrimSpace(request.Input),
		"prev":      "",
	}
	if risks := workflowRiskReasons(workflow, context); len(risks) > 0 && !request.Confirmed {
		return RunResult{
			OK:                   false,
			Message:              "工作流包含高风险步骤，需要再次确认",
			WorkflowID:           workflow.ID,
			WorkflowName:         workflow.Name,
			RequiresConfirmation: true,
			RiskReasons:          risks,
		}
	}
	steps := make([]StepRun, 0, len(workflow.Steps))
	for index, step := range workflow.Steps {
		run := StepRun{Index: index + 1, Command: step.Command, Pick: step.Pick}
		unknown := unknownPlaceholders(step.Command + " " + step.Pick)
		if len(unknown) > 0 {
			run.Message = "未知变量: " + strings.Join(unknown, ", ")
			steps = append(steps, run)
			return failedRun(workflow, "工作流失败: 第"+strconv.Itoa(index+1)+"步包含未知变量", steps)
		}

		rendered := renderTemplate(step.Command, context)
		run.RenderedCommand = rendered
		keyword, args := splitFirst(rendered)
		if keyword == "" {
			run.Message = "命令为空"
			steps = append(steps, run)
			return failedRun(workflow, "工作流失败: 第"+strconv.Itoa(index+1)+"步命令为空", steps)
		}
		if workflowKeywords[keyword] {
			run.Message = "不允许递归运行工作流"
			steps = append(steps, run)
			return failedRun(workflow, "工作流失败: 第"+strconv.Itoa(index+1)+"步禁止递归工作流", steps)
		}

		results := s.executor.Execute(keyword, args)
		chosen := pickResult(results, renderTemplate(step.Pick, context))
		if chosen == nil {
			run.Message = "未匹配到结果"
			steps = append(steps, run)
			return failedRun(workflow, "工作流失败: 第"+strconv.Itoa(index+1)+"步未匹配到结果", steps)
		}
		if risks := resultRiskReasons(*chosen); len(risks) > 0 && !request.Confirmed {
			run.Message = "命中结果需要确认: " + strings.Join(risks, "；")
			steps = append(steps, run)
			return RunResult{
				OK:                   false,
				Message:              "工作流命中高风险结果，需要再次确认",
				WorkflowID:           workflow.ID,
				WorkflowName:         workflow.Name,
				Steps:                steps,
				RequiresConfirmation: true,
				RiskReasons:          risks,
			}
		}
		output := resultOutput(*chosen)
		if output == "" {
			run.Message = "结果为空"
			steps = append(steps, run)
			return failedRun(workflow, "工作流失败: 第"+strconv.Itoa(index+1)+"步结果为空", steps)
		}

		run.OK = true
		run.PickedTitle = chosen.Title
		run.Output = output
		steps = append(steps, run)
		context["prev"] = output
	}

	if context["prev"] == "" {
		return failedRun(workflow, "工作流失败: 没有可复制结果", steps)
	}
	return RunResult{
		OK:           true,
		Message:      fmt.Sprintf("工作流完成：%s（共 %d 步）", workflow.Name, len(workflow.Steps)),
		WorkflowID:   workflow.ID,
		WorkflowName: workflow.Name,
		Output:       context["prev"],
		Steps:        steps,
	}
}

func (s *Service) find(id string) (Workflow, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, workflow := range s.workflows {
		if workflow.ID == id {
			return workflow, true
		}
	}
	return Workflow{}, false
}

func (s *Service) statusLocked() Status {
	return Status{
		Path:           s.path,
		LegacyPath:     s.legacyPath,
		Count:          len(s.workflows),
		LastSaveError:  s.lastSaveError,
		LegacyImported: s.legacyImported,
		Workflows:      cloneWorkflows(s.workflows),
	}
}

func (s *Service) load() {
	if s.path == "" {
		sortWorkflows(s.workflows)
		return
	}

	raw, err := os.ReadFile(s.path)
	if err == nil {
		var state struct {
			Version    int        `json:"version"`
			Workflows  []Workflow `json:"workflows"`
			RemovedIDs []string   `json:"removedIds,omitempty"`
		}
		if json.Unmarshal(raw, &state) == nil {
			s.applyState(state.Workflows, state.RemovedIDs)
			return
		}
	}

	if legacy, imported := importLegacyWorkflows(s.legacyPath); imported {
		s.workflows = legacy
		s.legacyImported = true
		s.lastSaveError = ""
		if err := s.saveLocked(); err != nil {
			s.lastSaveError = err.Error()
		}
		sortWorkflows(s.workflows)
		return
	}

	sortWorkflows(s.workflows)
}

func (s *Service) applyState(workflows []Workflow, removedIDs []string) {
	for _, id := range removedIDs {
		id = strings.TrimSpace(strings.ToLower(id))
		if id != "" {
			s.removed[id] = true
		}
	}
	merged := map[string]Workflow{}
	for _, workflow := range s.workflows {
		workflow, ok := normalizeWorkflow(workflow)
		if ok && !s.removed[workflow.ID] {
			merged[workflow.ID] = workflow
		}
	}
	for _, workflow := range workflows {
		workflow, ok := normalizeWorkflow(workflow)
		if ok {
			merged[workflow.ID] = workflow
		}
	}
	s.workflows = make([]Workflow, 0, len(merged))
	for _, workflow := range merged {
		s.workflows = append(s.workflows, workflow)
	}
	sortWorkflows(s.workflows)
}

func (s *Service) saveLocked() error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	removedIDs := make([]string, 0, len(s.removed))
	for id := range s.removed {
		removedIDs = append(removedIDs, id)
	}
	sort.Strings(removedIDs)
	state := struct {
		Version    int        `json:"version"`
		Workflows  []Workflow `json:"workflows"`
		RemovedIDs []string   `json:"removedIds,omitempty"`
	}{
		Version:    1,
		Workflows:  cloneWorkflows(s.workflows),
		RemovedIDs: removedIDs,
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o600)
}

func defaultWorkflows() []Workflow {
	return []Workflow{
		{
			ID:          "clip-md5",
			Name:        "剪贴板文本 -> MD5",
			Description: "读取剪贴板文本并复制其 MD5 值",
			Steps:       []Step{{Command: "hash {clipboard}", Pick: "MD5"}},
		},
		{
			ID:          "clip-url-encode",
			Name:        "剪贴板文本 -> URL 编码",
			Description: "读取剪贴板文本并复制 URL 编码结果",
			Steps:       []Step{{Command: "url {clipboard}", Pick: "编码结果"}},
		},
		{
			ID:          "clip-base64-encode",
			Name:        "剪贴板文本 -> Base64 编码",
			Description: "读取剪贴板文本并复制 Base64 编码结果",
			Steps:       []Step{{Command: "base64 {clipboard}", Pick: "编码结果"}},
		},
		{
			ID:          "now-timestamp",
			Name:        "当前时间 -> 时间戳",
			Description: "生成当前 Unix 时间戳并复制到剪贴板",
			Steps:       []Step{{Command: "timestamp now", Pick: "当前时间戳"}},
		},
	}
}

func normalizeWorkflow(workflow Workflow) (Workflow, bool) {
	id := strings.TrimSpace(strings.ToLower(workflow.ID))
	name := strings.TrimSpace(workflow.Name)
	if !workflowIDPattern.MatchString(id) || name == "" {
		return Workflow{}, false
	}
	steps := make([]Step, 0, len(workflow.Steps))
	for _, step := range workflow.Steps {
		command := strings.TrimSpace(step.Command)
		pick := strings.TrimSpace(step.Pick)
		if command == "" {
			continue
		}
		steps = append(steps, Step{Command: command, Pick: pick})
	}
	if len(steps) == 0 {
		return Workflow{}, false
	}
	return Workflow{
		ID:          id,
		Name:        name,
		Description: strings.TrimSpace(workflow.Description),
		Steps:       steps,
		UpdatedAt:   workflow.UpdatedAt,
	}, true
}

func workflowFromDraft(draft workmemory.WorkflowDraft) (Workflow, bool) {
	id := workflowIDFromDraft(draft)
	name := strings.TrimSpace(draft.Title)
	if name == "" {
		name = "工作记忆候选工作流"
	}
	steps := make([]Step, 0, len(draft.Steps))
	for _, step := range draft.Steps {
		command := strings.TrimSpace(step.Command)
		if command == "" {
			continue
		}
		steps = append(steps, Step{Command: command})
	}
	descriptionParts := []string{
		"由工作记忆经验发现生成，保存前需要用户审阅。",
	}
	if trigger := strings.TrimSpace(draft.Trigger); trigger != "" {
		descriptionParts = append(descriptionParts, "触发: "+trigger)
	}
	if input := strings.TrimSpace(draft.Input); input != "" {
		descriptionParts = append(descriptionParts, "输入: "+input)
	}
	if output := strings.TrimSpace(draft.Output); output != "" {
		descriptionParts = append(descriptionParts, "输出: "+output)
	}
	if risk := strings.TrimSpace(draft.RiskLevel); risk != "" {
		descriptionParts = append(descriptionParts, "风险: "+risk)
	}
	if len(draft.Evidence) > 0 {
		descriptionParts = append(descriptionParts, "证据: "+strings.Join(cleanDraftStrings(draft.Evidence), ", "))
	}
	return normalizeWorkflow(Workflow{
		ID:          id,
		Name:        name,
		Description: strings.Join(descriptionParts, "\n"),
		Steps:       steps,
		UpdatedAt:   time.Now().Unix(),
	})
}

func workflowIDFromDraft(draft workmemory.WorkflowDraft) string {
	id := strings.ToLower(strings.TrimSpace(draft.ID))
	id = strings.TrimPrefix(id, "workflow-draft-")
	id = strings.Trim(id, "-")
	if id != "" {
		candidate := "memory-" + slugIDPart(id)
		if workflowIDPattern.MatchString(candidate) {
			return candidate
		}
	}
	base := "memory-" + stableID(strings.Join([]string{draft.Title, draft.Trigger, strconv.FormatInt(draft.CreatedAt, 10)}, "\n"))
	if workflowIDPattern.MatchString(base) {
		return base
	}
	return "memory-workflow"
}

func draftRiskReasons(draft workmemory.WorkflowDraft, workflow Workflow) []string {
	reasons := []string{}
	if risk := strings.ToLower(strings.TrimSpace(draft.RiskLevel)); risk != "" && risk != "low" {
		reasons = append(reasons, "草稿风险等级: "+risk)
	}
	for _, step := range draft.Steps {
		if step.RequiresConfirm {
			label := strings.TrimSpace(step.Label)
			if label == "" {
				label = strings.TrimSpace(step.Command)
			}
			if label != "" {
				reasons = append(reasons, "步骤需要确认: "+label)
			}
		}
	}
	reasons = append(reasons, workflowRiskReasons(workflow, map[string]string{"clipboard": "", "input": "", "prev": ""})...)
	return uniqueStrings(reasons)
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
	cleaned := strings.Trim(builder.String(), "-")
	if cleaned == "" {
		return stableID(value)
	}
	return cleaned
}

func cleanDraftStrings(items []string) []string {
	cleaned := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			cleaned = append(cleaned, item)
		}
	}
	return cleaned
}

func uniqueStrings(items []string) []string {
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

func workflowQuery(query string) (string, bool) {
	keyword, rest := splitFirst(query)
	if workflowKeywords[keyword] {
		if restKeyword, restRest := splitFirst(rest); restKeyword == "run" {
			return strings.TrimSpace(restRest), true
		}
		return strings.TrimSpace(rest), true
	}
	return strings.TrimSpace(strings.ToLower(query)), false
}

func workflowScore(workflow Workflow, needle string) float64 {
	needle = strings.ToLower(strings.TrimSpace(needle))
	if needle == "" {
		return 0
	}
	if needle == strings.ToLower(workflow.ID) {
		return 92
	}
	if strings.HasPrefix(strings.ToLower(workflow.ID), needle) {
		return 84
	}
	haystack := strings.ToLower(strings.Join([]string{workflow.ID, workflow.Name, workflow.Description, stepsText(workflow.Steps)}, " "))
	if strings.Contains(haystack, needle) {
		return 70
	}
	return 0
}

func workflowToResult(workflow Workflow, input string, score float64) contracts.SearchResult {
	command := workflow.ID
	if strings.TrimSpace(input) != "" {
		command += " " + strings.TrimSpace(input)
	}
	stepLines := make([]string, 0, len(workflow.Steps))
	for i, step := range workflow.Steps {
		line := strconv.Itoa(i+1) + ". " + step.Command
		if step.Pick != "" {
			line += " | " + step.Pick
		}
		stepLines = append(stepLines, line)
	}
	risks := workflowRiskReasons(workflow, map[string]string{"clipboard": "", "input": strings.TrimSpace(input), "prev": ""})
	runKind := contracts.ActionRun
	runLabel := "运行工作流"
	if len(risks) > 0 {
		runKind = contracts.ActionDanger
		runLabel = "确认运行工作流"
	}
	runPayload := map[string]interface{}{
		"workflowId": workflow.ID,
		"input":      strings.TrimSpace(input),
		"command":    "wf " + command,
	}
	if len(risks) > 0 {
		runPayload["requiresConfirmation"] = true
		runPayload["riskReasons"] = risks
	}
	return contracts.SearchResult{
		ID:       "workflow-" + stableID(command),
		Type:     contracts.ResultWorkflow,
		Title:    "执行工作流: " + workflow.Name,
		Subtitle: "工作流 · " + workflow.ID,
		Detail:   workflow.Description,
		Icon:     "workflow",
		Score:    score,
		Tags:     []string{"工作流", workflow.ID},
		Payload: map[string]interface{}{
			"workflowId": workflow.ID,
			"input":      strings.TrimSpace(input),
		},
		Preview: contracts.PreviewDescriptor{
			Kind:     contracts.PreviewWorkflow,
			Title:    workflow.Name,
			Subtitle: workflow.ID,
			Text:     strings.Join(stepLines, "\n"),
			Meta: []contracts.LabelValue{
				{Label: "步骤", Value: strconv.Itoa(len(workflow.Steps))},
				{Label: "变量", Value: "{clipboard}, {input}, {prev}"},
				{Label: "说明", Value: workflow.Description},
			},
		},
		Actions: []contracts.PreviewAction{
			{
				ID:       "run_workflow",
				Label:    runLabel,
				Icon:     "run",
				Kind:     runKind,
				Shortcut: "Enter",
				Payload:  runPayload,
				Feedback: &contracts.ActionFeedback{SuccessLabel: "已运行", DurationMS: 1800},
			},
			contracts.PluginAction("open_tool", "编辑步骤", "open_workflow_center"),
			contracts.CopyAction("copy_workflow_command", "复制命令", "wf "+command, ""),
		},
	}
}

func splitFirst(value string) (string, string) {
	parts := strings.Fields(strings.TrimSpace(value))
	if len(parts) == 0 {
		return "", ""
	}
	keyword := strings.ToLower(parts[0])
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(value), parts[0]))
	return keyword, rest
}

func renderTemplate(template string, context map[string]string) string {
	return strings.TrimSpace(placeholderPattern.ReplaceAllStringFunc(template, func(match string) string {
		token := strings.Trim(match, "{}")
		return context[strings.TrimSpace(token)]
	}))
}

func unknownPlaceholders(text string) []string {
	matches := placeholderPattern.FindAllStringSubmatch(text, -1)
	unknown := map[string]bool{}
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		token := strings.TrimSpace(match[1])
		if token != "" && !allowedVariables[token] {
			unknown[token] = true
		}
	}
	items := make([]string, 0, len(unknown))
	for token := range unknown {
		items = append(items, token)
	}
	sort.Strings(items)
	return items
}

func pickResult(results []contracts.SearchResult, pick string) *contracts.SearchResult {
	if len(results) == 0 {
		return nil
	}
	pick = strings.ToLower(strings.TrimSpace(pick))
	if pick != "" {
		for i := range results {
			title := strings.ToLower(strings.TrimSpace(results[i].Title))
			previewTitle := strings.ToLower(strings.TrimSpace(results[i].Preview.Title))
			if strings.HasPrefix(title, pick) || strings.HasPrefix(previewTitle, pick) || strings.Contains(title, pick) {
				return &results[i]
			}
		}
	}
	return &results[0]
}

func resultOutput(result contracts.SearchResult) string {
	for _, action := range result.Actions {
		if action.Kind == contracts.ActionCopy {
			if text, ok := action.Payload["text"].(string); ok && strings.TrimSpace(text) != "" {
				return strings.TrimSpace(text)
			}
		}
	}
	for _, value := range []string{result.Detail, result.Preview.Text, result.Title} {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func failedRun(workflow Workflow, message string, steps []StepRun) RunResult {
	return RunResult{
		OK:           false,
		Message:      message,
		WorkflowID:   workflow.ID,
		WorkflowName: workflow.Name,
		Steps:        steps,
	}
}

func importLegacyWorkflows(path string) ([]Workflow, bool) {
	if path == "" {
		return nil, false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var config struct {
		Workflows []Workflow `json:"workflows"`
	}
	if json.Unmarshal(raw, &config) != nil {
		return nil, false
	}
	normalized := normalizeWorkflows(config.Workflows)
	if len(normalized) == 0 {
		return nil, false
	}
	return normalized, true
}

func parseWorkflowImport(raw string) ([]Workflow, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("导入内容为空")
	}
	var payload workflowExportPayload
	if err := json.Unmarshal([]byte(raw), &payload); err == nil && len(payload.Workflows) > 0 {
		normalized := normalizeWorkflows(payload.Workflows)
		if len(normalized) == 0 {
			return nil, fmt.Errorf("导入内容没有合法工作流")
		}
		return normalized, nil
	}
	var workflows []Workflow
	if err := json.Unmarshal([]byte(raw), &workflows); err == nil && len(workflows) > 0 {
		normalized := normalizeWorkflows(workflows)
		if len(normalized) == 0 {
			return nil, fmt.Errorf("导入内容没有合法工作流")
		}
		return normalized, nil
	}
	return nil, fmt.Errorf("导入内容不是 Ariadne 工作流 JSON")
}

func workflowExportPath(configPath string, exportedAt time.Time) string {
	base := filepath.Dir(configPath)
	if strings.TrimSpace(base) == "" || base == "." {
		base = defaultExportBase()
	}
	return filepath.Join(base, "exports", "ariadne-workflows-"+exportedAt.Format("20060102-150405")+".json")
}

func defaultExportBase() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = os.Getenv("APPDATA")
	}
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "Ariadne")
}

func workflowRiskReasons(workflow Workflow, context map[string]string) []string {
	reasons := []string{}
	for index, step := range workflow.Steps {
		rendered := renderTemplate(step.Command, context)
		keyword, args := splitFirst(rendered)
		if reason := commandRiskReason(keyword, args); reason != "" {
			reasons = append(reasons, "第 "+strconv.Itoa(index+1)+" 步: "+reason)
		}
	}
	return reasons
}

func commandRiskReason(keyword string, args string) string {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	args = strings.ToLower(strings.TrimSpace(args))
	switch keyword {
	case "sys", "system":
		command, _ := splitFirst(args)
		if command == "" {
			return "系统命令需要确认"
		}
		return "系统命令 `" + command + "` 需要确认"
	case "clip", "clipboard":
		if args == "clear" || args == "清空" {
			return "清理剪贴板历史需要确认"
		}
	case "cap", "capture", "shot", "截图历史", "捕获历史":
		if args == "clear" || args == "清空" {
			return "清理截图历史需要确认"
		}
	case "hosts", "host":
		if strings.Contains(args, "apply") || strings.Contains(args, "write") || strings.Contains(args, "启用") || strings.Contains(args, "写入") {
			return "写入 Hosts 需要确认"
		}
	}
	return ""
}

func resultRiskReasons(result contracts.SearchResult) []string {
	reasons := []string{}
	for _, action := range result.Actions {
		if action.Kind == contracts.ActionDanger {
			reasons = append(reasons, result.Title+" -> "+action.Label)
			continue
		}
		if action.Payload != nil {
			if value, ok := action.Payload["requiresConfirmation"].(bool); ok && value {
				reasons = append(reasons, result.Title+" -> "+action.Label)
			}
		}
	}
	return reasons
}

func normalizeWorkflows(workflows []Workflow) []Workflow {
	seen := map[string]bool{}
	result := make([]Workflow, 0, len(workflows))
	for _, workflow := range workflows {
		workflow, ok := normalizeWorkflow(workflow)
		if !ok || seen[workflow.ID] {
			continue
		}
		seen[workflow.ID] = true
		result = append(result, workflow)
	}
	sortWorkflows(result)
	return result
}

func sortWorkflows(workflows []Workflow) {
	sort.SliceStable(workflows, func(i, j int) bool {
		return workflows[i].ID < workflows[j].ID
	})
}

func cloneWorkflows(workflows []Workflow) []Workflow {
	result := make([]Workflow, len(workflows))
	for i, workflow := range workflows {
		result[i] = workflow
		result[i].Steps = append([]Step{}, workflow.Steps...)
	}
	return result
}

func stepsText(steps []Step) string {
	lines := make([]string, 0, len(steps))
	for _, step := range steps {
		lines = append(lines, step.Command, step.Pick)
	}
	return strings.Join(lines, " ")
}

func uniqueID(workflows []Workflow, base string) string {
	used := map[string]bool{}
	for _, workflow := range workflows {
		used[workflow.ID] = true
	}
	if !used[base] {
		return base
	}
	for i := 2; i < 1000; i++ {
		id := base + "-" + strconv.Itoa(i)
		if !used[id] {
			return id
		}
	}
	return base + "-" + stableID(strconv.FormatInt(time.Now().UnixNano(), 10))[:8]
}

func stableID(value string) string {
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
	return filepath.Join(base, "Ariadne", "workflows.json")
}

func defaultLegacyPath() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return ""
	}
	return filepath.Join(appData, "x-tools", "config.json")
}
