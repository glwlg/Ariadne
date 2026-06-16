package skills

import (
	"archive/zip"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"ariadne/internal/workmemory"
)

var skillIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Skill struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	Evidence  []string `json:"evidence"`
	Source    string   `json:"source,omitempty"`
	CreatedAt int64    `json:"createdAt,omitempty"`
	UpdatedAt int64    `json:"updatedAt,omitempty"`
}

type Status struct {
	Path          string  `json:"path"`
	Count         int     `json:"count"`
	LastSaveError string  `json:"lastSaveError,omitempty"`
	Skills        []Skill `json:"skills"`
}

type DraftSaveRequest struct {
	Draft     workmemory.Draft `json:"draft"`
	Confirmed bool             `json:"confirmed"`
}

type DraftSaveResult struct {
	OK                   bool     `json:"ok"`
	Message              string   `json:"message"`
	Skill                Skill    `json:"skill,omitempty"`
	Status               Status   `json:"status"`
	RequiresConfirmation bool     `json:"requiresConfirmation,omitempty"`
	RiskReasons          []string `json:"riskReasons,omitempty"`
}

type ExportRequest struct {
	SkillID   string `json:"skillId"`
	Confirmed bool   `json:"confirmed"`
}

type ExportResult struct {
	OK                   bool     `json:"ok"`
	Message              string   `json:"message"`
	Skill                Skill    `json:"skill,omitempty"`
	Directory            string   `json:"directory,omitempty"`
	ZipPath              string   `json:"zipPath,omitempty"`
	Bytes                int64    `json:"bytes,omitempty"`
	ExportedAt           int64    `json:"exportedAt,omitempty"`
	RequiresConfirmation bool     `json:"requiresConfirmation,omitempty"`
	RiskReasons          []string `json:"riskReasons,omitempty"`
}

type InstallRequest struct {
	SkillID    string `json:"skillId"`
	TargetRoot string `json:"targetRoot,omitempty"`
	Confirmed  bool   `json:"confirmed"`
	Overwrite  bool   `json:"overwrite,omitempty"`
}

type InstallResult struct {
	OK                   bool     `json:"ok"`
	Message              string   `json:"message"`
	Skill                Skill    `json:"skill,omitempty"`
	TargetRoot           string   `json:"targetRoot,omitempty"`
	InstalledDir         string   `json:"installedDir,omitempty"`
	Files                []string `json:"files,omitempty"`
	InstalledAt          int64    `json:"installedAt,omitempty"`
	RefreshRequested     bool     `json:"refreshRequested,omitempty"`
	RefreshMarker        string   `json:"refreshMarker,omitempty"`
	RefreshManifest      string   `json:"refreshManifest,omitempty"`
	RequiresConfirmation bool     `json:"requiresConfirmation,omitempty"`
	RiskReasons          []string `json:"riskReasons,omitempty"`
}

type InstallDiagnosticsRequest struct {
	SkillID    string `json:"skillId,omitempty"`
	TargetRoot string `json:"targetRoot,omitempty"`
}

type CodexInstalledSkill struct {
	ID             string `json:"id"`
	Title          string `json:"title,omitempty"`
	Directory      string `json:"directory"`
	SkillPath      string `json:"skillPath"`
	Readable       bool   `json:"readable"`
	Bytes          int64  `json:"bytes,omitempty"`
	UpdatedAt      int64  `json:"updatedAt,omitempty"`
	AriadneManaged bool   `json:"ariadneManaged,omitempty"`
	Error          string `json:"error,omitempty"`
}

type RefreshDiagnostics struct {
	ManifestPath            string `json:"manifestPath"`
	MarkerPath              string `json:"markerPath"`
	ManifestExists          bool   `json:"manifestExists"`
	MarkerExists            bool   `json:"markerExists"`
	Valid                   bool   `json:"valid"`
	Source                  string `json:"source,omitempty"`
	Action                  string `json:"action,omitempty"`
	SkillID                 string `json:"skillId,omitempty"`
	SkillTitle              string `json:"skillTitle,omitempty"`
	InstalledDir            string `json:"installedDir,omitempty"`
	RequestedAt             int64  `json:"requestedAt,omitempty"`
	MarkerID                string `json:"markerId,omitempty"`
	MarkerText              string `json:"markerText,omitempty"`
	MarkerMatchesManifest   bool   `json:"markerMatchesManifest"`
	SkillMatchesRequest     bool   `json:"skillMatchesRequest"`
	InstalledDirExists      bool   `json:"installedDirExists"`
	InstalledSkillFileFound bool   `json:"installedSkillFileFound"`
	Error                   string `json:"error,omitempty"`
}

type InstallDiagnosticsResult struct {
	OK                  bool                  `json:"ok"`
	Message             string                `json:"message"`
	TargetRoot          string                `json:"targetRoot"`
	TargetRootExists    bool                  `json:"targetRootExists"`
	TargetRootReadable  bool                  `json:"targetRootReadable"`
	SkillID             string                `json:"skillId,omitempty"`
	InstalledDir        string                `json:"installedDir,omitempty"`
	Installed           bool                  `json:"installed"`
	SkillPath           string                `json:"skillPath,omitempty"`
	SkillFileExists     bool                  `json:"skillFileExists"`
	SkillFileBytes      int64                 `json:"skillFileBytes,omitempty"`
	SkillUpdatedAt      int64                 `json:"skillUpdatedAt,omitempty"`
	DiscoveredCount     int                   `json:"discoveredCount"`
	AriadneManagedCount int                   `json:"ariadneManagedCount"`
	Skills              []CodexInstalledSkill `json:"skills"`
	Refresh             RefreshDiagnostics    `json:"refresh"`
	LastError           string                `json:"lastError,omitempty"`
}

type codexRefreshManifest struct {
	Version      int    `json:"version"`
	Source       string `json:"source"`
	Action       string `json:"action"`
	SkillID      string `json:"skillId"`
	SkillTitle   string `json:"skillTitle"`
	TargetRoot   string `json:"targetRoot"`
	InstalledDir string `json:"installedDir"`
	RequestedAt  int64  `json:"requestedAt"`
	MarkerID     string `json:"markerId"`
}

type Service struct {
	mu            sync.RWMutex
	path          string
	skills        []Skill
	lastSaveError string
}

type stateFile struct {
	Version int     `json:"version"`
	Skills  []Skill `json:"skills"`
}

func NewService() *Service {
	return NewServiceWithPath(defaultPath())
}

func NewServiceWithPath(path string) *Service {
	service := &Service{path: path}
	service.load()
	return service
}

func (s *Service) List() []Skill {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneSkills(s.skills)
}

func (s *Service) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.statusLocked()
}

func (s *Service) Upsert(next Skill) Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalized, ok := normalizeSkill(next)
	if !ok {
		s.lastSaveError = "Skill ID、标题或正文无效"
		return s.statusLocked()
	}
	if normalized.CreatedAt == 0 {
		normalized.CreatedAt = time.Now().Unix()
	}
	normalized.UpdatedAt = time.Now().Unix()

	replaced := false
	for index := range s.skills {
		if s.skills[index].ID == normalized.ID {
			s.skills[index] = normalized
			replaced = true
			break
		}
	}
	if !replaced {
		s.skills = append(s.skills, normalized)
	}
	sortSkills(s.skills)
	s.lastSaveError = ""
	if err := s.saveLocked(); err != nil {
		s.lastSaveError = err.Error()
	}
	return s.statusLocked()
}

func (s *Service) SaveSkillDraft(request DraftSaveRequest) DraftSaveResult {
	skill, ok := skillFromDraft(request.Draft)
	if !ok {
		return DraftSaveResult{OK: false, Message: "Skill 草稿无效", Status: s.Status()}
	}
	riskReasons := draftRiskReasons(request.Draft, skill)
	if !request.Confirmed {
		if len(riskReasons) == 0 {
			riskReasons = []string{"Skill 草稿来自工作记忆，保存为正式资产前需要用户确认"}
		}
		return DraftSaveResult{
			OK:                   false,
			Message:              "保存 Skill 需要确认",
			Skill:                skill,
			Status:               s.Status(),
			RequiresConfirmation: true,
			RiskReasons:          riskReasons,
		}
	}
	status := s.Upsert(skill)
	if status.LastSaveError != "" {
		return DraftSaveResult{OK: false, Message: "Skill 保存失败: " + status.LastSaveError, Skill: skill, Status: status}
	}
	return DraftSaveResult{OK: true, Message: "Skill 已保存为正式资产", Skill: skill, Status: status, RiskReasons: riskReasons}
}

func (s *Service) ExportSkillPackage(request ExportRequest) ExportResult {
	skillID := strings.TrimSpace(strings.ToLower(request.SkillID))
	skill, ok := s.find(skillID)
	if !ok {
		return ExportResult{OK: false, Message: "未找到对应 Skill"}
	}
	riskReasons := exportRiskReasons(skill)
	if !request.Confirmed {
		return ExportResult{
			OK:                   false,
			Message:              "导出 Codex Skill 包需要确认",
			Skill:                skill,
			RequiresConfirmation: true,
			RiskReasons:          riskReasons,
		}
	}

	exportedAt := time.Now()
	baseDir := filepath.Join(filepath.Dir(s.path), "skill_exports")
	skillDir := filepath.Join(baseDir, skill.ID)
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return ExportResult{OK: false, Message: "创建 Skill 导出目录失败: " + err.Error(), Skill: skill}
	}
	if err := os.RemoveAll(skillDir); err != nil {
		return ExportResult{OK: false, Message: "清理旧 Skill 导出目录失败: " + err.Error(), Skill: skill}
	}
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return ExportResult{OK: false, Message: "创建 Skill 包目录失败: " + err.Error(), Skill: skill}
	}

	skillMarkdown := renderSkillMarkdown(skill)
	skillPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(skillMarkdown), 0o600); err != nil {
		return ExportResult{OK: false, Message: "写入 SKILL.md 失败: " + err.Error(), Skill: skill, Directory: skillDir}
	}

	zipPath := filepath.Join(baseDir, skill.ID+".zip")
	if err := writeSkillZip(zipPath, skill.ID, skillMarkdown); err != nil {
		return ExportResult{OK: false, Message: "生成 Skill zip 失败: " + err.Error(), Skill: skill, Directory: skillDir, ZipPath: zipPath}
	}
	bytes := int64(0)
	if info, err := os.Stat(zipPath); err == nil {
		bytes = info.Size()
	}
	return ExportResult{
		OK:          true,
		Message:     "Codex Skill 包已导出",
		Skill:       skill,
		Directory:   skillDir,
		ZipPath:     zipPath,
		Bytes:       bytes,
		ExportedAt:  exportedAt.Unix(),
		RiskReasons: riskReasons,
	}
}

func (s *Service) InstallSkillPackage(request InstallRequest) InstallResult {
	skillID := strings.TrimSpace(strings.ToLower(request.SkillID))
	skill, ok := s.find(skillID)
	if !ok {
		return InstallResult{OK: false, Message: "未找到对应 Skill"}
	}
	targetRoot := strings.TrimSpace(request.TargetRoot)
	if targetRoot == "" {
		targetRoot = defaultSkillInstallRoot()
	}
	riskReasons := installRiskReasons(skill, targetRoot, request.Overwrite)
	if !request.Confirmed {
		return InstallResult{
			OK:                   false,
			Message:              "安装到 Codex skills 目录需要确认",
			Skill:                skill,
			TargetRoot:           targetRoot,
			RequiresConfirmation: true,
			RiskReasons:          riskReasons,
		}
	}
	if targetRoot == "" {
		return InstallResult{OK: false, Message: "无法确定 Codex skills 目标目录", Skill: skill}
	}

	targetRootAbs, err := filepath.Abs(targetRoot)
	if err != nil {
		return InstallResult{OK: false, Message: "解析 Codex skills 目录失败: " + err.Error(), Skill: skill, TargetRoot: targetRoot}
	}
	installedDir := filepath.Join(targetRootAbs, skill.ID)
	installedDirAbs, err := filepath.Abs(installedDir)
	if err != nil {
		return InstallResult{OK: false, Message: "解析 Skill 安装目录失败: " + err.Error(), Skill: skill, TargetRoot: targetRootAbs}
	}
	if !pathInside(targetRootAbs, installedDirAbs) {
		return InstallResult{OK: false, Message: "拒绝写入 Codex skills 目录外的路径", Skill: skill, TargetRoot: targetRootAbs, InstalledDir: installedDirAbs}
	}

	if info, err := os.Stat(installedDirAbs); err == nil {
		if !info.IsDir() {
			return InstallResult{OK: false, Message: "目标 Skill 路径已存在且不是目录", Skill: skill, TargetRoot: targetRootAbs, InstalledDir: installedDirAbs}
		}
		if !request.Overwrite {
			return InstallResult{
				OK:                   false,
				Message:              "目标 Skill 已存在，需要确认覆盖",
				Skill:                skill,
				TargetRoot:           targetRootAbs,
				InstalledDir:         installedDirAbs,
				RequiresConfirmation: true,
				RiskReasons:          installRiskReasons(skill, targetRootAbs, true),
			}
		}
		if err := os.RemoveAll(installedDirAbs); err != nil {
			return InstallResult{OK: false, Message: "清理旧 Skill 安装目录失败: " + err.Error(), Skill: skill, TargetRoot: targetRootAbs, InstalledDir: installedDirAbs}
		}
	} else if !os.IsNotExist(err) {
		return InstallResult{OK: false, Message: "检查 Skill 安装目录失败: " + err.Error(), Skill: skill, TargetRoot: targetRootAbs, InstalledDir: installedDirAbs}
	}
	if err := os.MkdirAll(installedDirAbs, 0o755); err != nil {
		return InstallResult{OK: false, Message: "创建 Skill 安装目录失败: " + err.Error(), Skill: skill, TargetRoot: targetRootAbs, InstalledDir: installedDirAbs}
	}

	skillMarkdown := renderSkillMarkdown(skill)
	skillPath := filepath.Join(installedDirAbs, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(skillMarkdown), 0o600); err != nil {
		return InstallResult{OK: false, Message: "写入 Codex SKILL.md 失败: " + err.Error(), Skill: skill, TargetRoot: targetRootAbs, InstalledDir: installedDirAbs}
	}
	installedAt := time.Now()
	refreshManifest, refreshMarker, err := writeCodexRefreshMarker(targetRootAbs, installedDirAbs, skill, installedAt)
	if err != nil {
		return InstallResult{
			OK:           false,
			Message:      "Skill 已安装，但写入 Codex refresh marker 失败: " + err.Error(),
			Skill:        skill,
			TargetRoot:   targetRootAbs,
			InstalledDir: installedDirAbs,
			Files:        []string{"SKILL.md"},
			InstalledAt:  installedAt.Unix(),
			RiskReasons:  riskReasons,
		}
	}
	return InstallResult{
		OK:               true,
		Message:          "Skill 已安装到 Codex skills 目录，并已写入 Ariadne refresh marker",
		Skill:            skill,
		TargetRoot:       targetRootAbs,
		InstalledDir:     installedDirAbs,
		Files:            []string{"SKILL.md", ".ariadne-refresh.json", ".ariadne-refresh.touch"},
		InstalledAt:      installedAt.Unix(),
		RefreshRequested: true,
		RefreshMarker:    refreshMarker,
		RefreshManifest:  refreshManifest,
		RiskReasons:      riskReasons,
	}
}

func (s *Service) InstallDiagnostics(request InstallDiagnosticsRequest) InstallDiagnosticsResult {
	skillID := strings.TrimSpace(strings.ToLower(request.SkillID))
	if skillID != "" && !skillIDPattern.MatchString(skillID) {
		return InstallDiagnosticsResult{OK: false, Message: "Skill ID 无效", SkillID: skillID, LastError: "invalid skill id"}
	}
	targetRoot := strings.TrimSpace(request.TargetRoot)
	if targetRoot == "" {
		targetRoot = defaultSkillInstallRoot()
	}
	targetRootAbs, err := filepath.Abs(targetRoot)
	if err != nil {
		return InstallDiagnosticsResult{OK: false, Message: "解析 Codex skills 目录失败: " + err.Error(), SkillID: skillID, TargetRoot: targetRoot, LastError: err.Error()}
	}

	result := InstallDiagnosticsResult{
		TargetRoot: targetRootAbs,
		SkillID:    skillID,
		Refresh:    inspectCodexRefreshMarker(targetRootAbs, skillID),
	}
	info, err := os.Stat(targetRootAbs)
	if err != nil {
		if os.IsNotExist(err) {
			result.Message = "Codex skills 目录不存在"
		} else {
			result.Message = "读取 Codex skills 目录失败: " + err.Error()
			result.LastError = err.Error()
		}
		return result
	}
	if !info.IsDir() {
		result.Message = "Codex skills 目标路径不是目录"
		result.TargetRootExists = true
		result.LastError = "target root is not a directory"
		return result
	}

	skills, readErr := readCodexInstalledSkills(targetRootAbs)
	result.TargetRootExists = true
	result.TargetRootReadable = readErr == nil
	result.Skills = skills
	result.DiscoveredCount = len(skills)
	for _, installed := range skills {
		if installed.AriadneManaged {
			result.AriadneManagedCount++
		}
		if skillID != "" && installed.ID == skillID {
			result.Installed = installed.Readable
			result.InstalledDir = installed.Directory
			result.SkillPath = installed.SkillPath
			result.SkillFileExists = installed.Readable
			result.SkillFileBytes = installed.Bytes
			result.SkillUpdatedAt = installed.UpdatedAt
		}
	}
	if readErr != nil {
		result.Message = "读取 Codex skills 目录失败: " + readErr.Error()
		result.LastError = readErr.Error()
		return result
	}
	if skillID == "" {
		result.OK = true
		result.Message = fmt.Sprintf("Codex skills 目录可读，发现 %d 个 Skill", result.DiscoveredCount)
		return result
	}
	if !result.Installed {
		result.Message = "目标 Skill 未出现在 Codex skills 目录"
		return result
	}
	if !result.Refresh.Valid || !result.Refresh.SkillMatchesRequest {
		result.Message = "目标 Skill 已安装，但刷新握手未通过本地核验"
		return result
	}
	result.OK = true
	result.Message = "目标 Skill 已安装，Codex skills 发现目录和 Ariadne 刷新握手均已通过本地核验"
	return result
}

func (s *Service) statusLocked() Status {
	return Status{
		Path:          s.path,
		Count:         len(s.skills),
		LastSaveError: s.lastSaveError,
		Skills:        cloneSkills(s.skills),
	}
}

func (s *Service) find(id string) (Skill, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, skill := range s.skills {
		if skill.ID == id {
			return cloneSkill(skill), true
		}
	}
	return Skill{}, false
}

func (s *Service) load() {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var payload stateFile
	if err := json.Unmarshal(raw, &payload); err == nil {
		s.skills = normalizeSkills(payload.Skills)
		return
	}
	var legacy []Skill
	if err := json.Unmarshal(raw, &legacy); err == nil {
		s.skills = normalizeSkills(legacy)
	}
}

func (s *Service) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	payload := stateFile{
		Version: 1,
		Skills:  cloneSkills(s.skills),
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o600)
}

func skillFromDraft(draft workmemory.Draft) (Skill, bool) {
	title := strings.TrimSpace(draft.Title)
	if title == "" {
		title = "工作记忆 Skill"
	}
	skill := Skill{
		ID:        skillIDFromDraft(draft),
		Title:     title,
		Body:      strings.TrimSpace(draft.Body),
		Evidence:  cleanDraftStrings(draft.Evidence),
		Source:    "work_memory",
		CreatedAt: draft.CreatedAt,
		UpdatedAt: time.Now().Unix(),
	}
	return normalizeSkill(skill)
}

func skillIDFromDraft(draft workmemory.Draft) string {
	id := strings.ToLower(strings.TrimSpace(draft.ID))
	id = strings.TrimPrefix(id, "knowledge-")
	id = strings.TrimPrefix(id, "skill-draft-")
	id = strings.Trim(id, "-")
	if id != "" {
		candidate := "memory-skill-" + slugIDPart(id)
		if skillIDPattern.MatchString(candidate) {
			return candidate
		}
	}
	sum := sha1.Sum([]byte(strings.ToLower(strings.TrimSpace(strings.Join([]string{
		draft.Title,
		draft.Body,
		fmt.Sprint(draft.CreatedAt),
	}, "\n")))))
	return "memory-skill-" + hex.EncodeToString(sum[:])[:12]
}

func draftRiskReasons(draft workmemory.Draft, skill Skill) []string {
	reasons := []string{"草稿来自工作记忆知识沉淀，需要确认后才写入正式 Skill"}
	if len(skill.Evidence) == 0 {
		reasons = append(reasons, "缺少证据引用，保存后需要人工补充来源")
	}
	if len([]rune(strings.TrimSpace(draft.Body))) < 20 {
		reasons = append(reasons, "正文较短，建议保存前补充复用步骤或适用边界")
	}
	return uniqueStrings(reasons)
}

func exportRiskReasons(skill Skill) []string {
	reasons := []string{"导出包会生成可安装的 Codex skill 文件，安装前需要确认内容不包含敏感信息"}
	if len(skill.Evidence) > 0 {
		reasons = append(reasons, "导出的 SKILL.md 会保留 evidence ID 作为来源线索")
	}
	if strings.Contains(strings.ToLower(skill.Body), "password") || strings.Contains(skill.Body, "密码") || strings.Contains(strings.ToLower(skill.Body), "token") {
		reasons = append(reasons, "正文疑似包含敏感词，安装或分享前必须人工复核")
	}
	return uniqueStrings(reasons)
}

func installRiskReasons(skill Skill, targetRoot string, overwrite bool) []string {
	reasons := []string{
		"安装会写入 Codex skills 发现目录，Codex 重启或刷新后可能加载该 Skill",
		"安装成功后会在 skills 根目录写入 Ariadne refresh marker，供 Codex runtime 或后续工具检测 newly installed skill",
		"Skill 内容来自工作记忆，安装前需要确认不包含敏感信息",
	}
	if strings.TrimSpace(targetRoot) != "" {
		reasons = append(reasons, "目标目录: "+targetRoot)
	}
	if overwrite {
		reasons = append(reasons, "若同名 Skill 已存在，将覆盖旧的 SKILL.md")
	}
	if len(skill.Evidence) > 0 {
		reasons = append(reasons, "安装的 SKILL.md 会保留 evidence ID 作为来源线索")
	}
	if strings.Contains(strings.ToLower(skill.Body), "password") || strings.Contains(skill.Body, "密码") || strings.Contains(strings.ToLower(skill.Body), "token") {
		reasons = append(reasons, "正文疑似包含敏感词，安装前必须人工复核")
	}
	return uniqueStrings(reasons)
}

func writeCodexRefreshMarker(targetRoot string, installedDir string, skill Skill, requestedAt time.Time) (string, string, error) {
	if err := os.MkdirAll(targetRoot, 0o755); err != nil {
		return "", "", err
	}
	markerID := fmt.Sprintf("%s-%d", skill.ID, requestedAt.UnixNano())
	manifest := codexRefreshManifest{
		Version:      1,
		Source:       "ariadne",
		Action:       "skills.refresh",
		SkillID:      skill.ID,
		SkillTitle:   skill.Title,
		TargetRoot:   targetRoot,
		InstalledDir: installedDir,
		RequestedAt:  requestedAt.Unix(),
		MarkerID:     markerID,
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", "", err
	}
	manifestPath := filepath.Join(targetRoot, ".ariadne-refresh.json")
	if err := os.WriteFile(manifestPath, raw, 0o600); err != nil {
		return "", "", err
	}
	markerPath := filepath.Join(targetRoot, ".ariadne-refresh.touch")
	if err := os.WriteFile(markerPath, []byte(markerID+"\n"), 0o600); err != nil {
		return "", "", err
	}
	return manifestPath, markerPath, nil
}

func readCodexInstalledSkills(targetRoot string) ([]CodexInstalledSkill, error) {
	entries, err := os.ReadDir(targetRoot)
	if err != nil {
		return nil, err
	}
	skills := make([]CodexInstalledSkill, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		directory := filepath.Join(targetRoot, entry.Name())
		skillPath := filepath.Join(directory, "SKILL.md")
		installed := CodexInstalledSkill{
			ID:        strings.ToLower(entry.Name()),
			Directory: directory,
			SkillPath: skillPath,
		}
		info, statErr := os.Stat(skillPath)
		if statErr != nil {
			if !os.IsNotExist(statErr) {
				installed.Error = statErr.Error()
			} else {
				installed.Error = "SKILL.md not found"
			}
			skills = append(skills, installed)
			continue
		}
		if info.IsDir() {
			installed.Error = "SKILL.md is a directory"
			skills = append(skills, installed)
			continue
		}
		installed.Readable = true
		installed.Bytes = info.Size()
		installed.UpdatedAt = info.ModTime().Unix()
		raw, readErr := os.ReadFile(skillPath)
		if readErr != nil {
			installed.Readable = false
			installed.Error = readErr.Error()
			skills = append(skills, installed)
			continue
		}
		name, title, managed := parseSkillMarkdownSummary(string(raw))
		if name != "" {
			installed.ID = strings.ToLower(name)
		}
		installed.Title = title
		installed.AriadneManaged = managed
		skills = append(skills, installed)
	}
	sort.SliceStable(skills, func(i, j int) bool {
		if skills[i].AriadneManaged != skills[j].AriadneManaged {
			return skills[i].AriadneManaged
		}
		return skills[i].ID < skills[j].ID
	})
	return skills, nil
}

func inspectCodexRefreshMarker(targetRoot string, requestedSkillID string) RefreshDiagnostics {
	status := RefreshDiagnostics{
		ManifestPath:        filepath.Join(targetRoot, ".ariadne-refresh.json"),
		MarkerPath:          filepath.Join(targetRoot, ".ariadne-refresh.touch"),
		SkillMatchesRequest: requestedSkillID == "",
	}
	manifestRaw, manifestErr := os.ReadFile(status.ManifestPath)
	if manifestErr != nil {
		if !os.IsNotExist(manifestErr) {
			status.Error = manifestErr.Error()
		}
		return status
	}
	status.ManifestExists = true
	var manifest codexRefreshManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		status.Error = "decode refresh manifest: " + err.Error()
		return status
	}
	status.Source = manifest.Source
	status.Action = manifest.Action
	status.SkillID = manifest.SkillID
	status.SkillTitle = manifest.SkillTitle
	status.InstalledDir = manifest.InstalledDir
	status.RequestedAt = manifest.RequestedAt
	status.MarkerID = manifest.MarkerID
	status.SkillMatchesRequest = requestedSkillID == "" || requestedSkillID == manifest.SkillID

	markerRaw, markerErr := os.ReadFile(status.MarkerPath)
	if markerErr != nil {
		if !os.IsNotExist(markerErr) {
			status.Error = markerErr.Error()
		}
		return status
	}
	status.MarkerExists = true
	status.MarkerText = strings.TrimSpace(string(markerRaw))
	status.MarkerMatchesManifest = status.MarkerText != "" && status.MarkerText == manifest.MarkerID
	if info, err := os.Stat(manifest.InstalledDir); err == nil && info.IsDir() {
		status.InstalledDirExists = true
	}
	if info, err := os.Stat(filepath.Join(manifest.InstalledDir, "SKILL.md")); err == nil && !info.IsDir() {
		status.InstalledSkillFileFound = true
	}
	insideTarget := pathInside(targetRoot, manifest.InstalledDir)
	status.Valid = manifest.Source == "ariadne" &&
		manifest.Action == "skills.refresh" &&
		manifest.SkillID != "" &&
		manifest.InstalledDir != "" &&
		manifest.MarkerID != "" &&
		insideTarget &&
		status.MarkerMatchesManifest &&
		status.SkillMatchesRequest &&
		status.InstalledDirExists &&
		status.InstalledSkillFileFound
	return status
}

func parseSkillMarkdownSummary(raw string) (string, string, bool) {
	name := ""
	title := ""
	inFrontMatter := false
	for index, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if index == 0 && trimmed == "---" {
			inFrontMatter = true
			continue
		}
		if inFrontMatter && trimmed == "---" {
			inFrontMatter = false
			continue
		}
		if inFrontMatter && strings.HasPrefix(trimmed, "name:") {
			name = strings.ToLower(strings.TrimSpace(trimYAMLValue(strings.TrimSpace(strings.TrimPrefix(trimmed, "name:")))))
			continue
		}
		if title == "" && strings.HasPrefix(trimmed, "# ") {
			title = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}
	managed := strings.Contains(raw, "Ariadne work-memory guidance") ||
		strings.Contains(raw, "Generated from a user-confirmed local Skill asset")
	return name, title, managed
}

func trimYAMLValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if value[0] == '"' && value[len(value)-1] == '"' || value[0] == '\'' && value[len(value)-1] == '\'' {
			return strings.TrimSpace(value[1 : len(value)-1])
		}
	}
	return value
}

func normalizeSkill(skill Skill) (Skill, bool) {
	id := strings.ToLower(strings.TrimSpace(skill.ID))
	if !skillIDPattern.MatchString(id) {
		return Skill{}, false
	}
	title := strings.TrimSpace(skill.Title)
	body := strings.TrimSpace(skill.Body)
	if title == "" || body == "" {
		return Skill{}, false
	}
	createdAt := skill.CreatedAt
	if createdAt == 0 {
		createdAt = skill.UpdatedAt
	}
	if createdAt == 0 {
		createdAt = time.Now().Unix()
	}
	return Skill{
		ID:        id,
		Title:     title,
		Body:      body,
		Evidence:  cleanDraftStrings(skill.Evidence),
		Source:    strings.TrimSpace(skill.Source),
		CreatedAt: createdAt,
		UpdatedAt: skill.UpdatedAt,
	}, true
}

func normalizeSkills(skills []Skill) []Skill {
	result := make([]Skill, 0, len(skills))
	for _, skill := range skills {
		normalized, ok := normalizeSkill(skill)
		if ok {
			result = append(result, normalized)
		}
	}
	sortSkills(result)
	return result
}

func sortSkills(skills []Skill) {
	sort.SliceStable(skills, func(i, j int) bool {
		left := skills[i].UpdatedAt
		if left == 0 {
			left = skills[i].CreatedAt
		}
		right := skills[j].UpdatedAt
		if right == 0 {
			right = skills[j].CreatedAt
		}
		if left == right {
			return skills[i].ID < skills[j].ID
		}
		return left > right
	})
}

func cloneSkills(skills []Skill) []Skill {
	result := make([]Skill, len(skills))
	for index, skill := range skills {
		result[index] = cloneSkill(skill)
	}
	return result
}

func cloneSkill(skill Skill) Skill {
	result := skill
	result.Evidence = append([]string(nil), skill.Evidence...)
	return result
}

func renderSkillMarkdown(skill Skill) string {
	description := skillDescription(skill)
	var builder strings.Builder
	builder.WriteString("---\n")
	builder.WriteString("name: " + skill.ID + "\n")
	builder.WriteString("description: " + yamlDoubleQuote(description) + "\n")
	builder.WriteString("---\n\n")
	builder.WriteString("# " + strings.TrimSpace(skill.Title) + "\n\n")
	builder.WriteString("Use this skill when the task matches this user-confirmed Ariadne work-memory guidance.\n\n")
	builder.WriteString("## Guidance\n\n")
	builder.WriteString(strings.TrimSpace(skill.Body) + "\n\n")
	if len(skill.Evidence) > 0 {
		builder.WriteString("## Source Evidence\n\n")
		for _, evidence := range skill.Evidence {
			builder.WriteString("- " + evidence + "\n")
		}
		builder.WriteString("\n")
	}
	builder.WriteString("## Safety\n\n")
	builder.WriteString("- Review the guidance before applying it to a different repository or machine.\n")
	builder.WriteString("- Do not expose private work-memory content outside the local environment without user confirmation.\n")
	return builder.String()
}

func skillDescription(skill Skill) string {
	title := strings.TrimSpace(skill.Title)
	if title == "" {
		title = skill.ID
	}
	return "Use when Codex needs to apply the Ariadne work-memory guidance captured as " + title + ". Generated from a user-confirmed local Skill asset; review source evidence before use."
}

func yamlDoubleQuote(value string) string {
	escaped := strings.ReplaceAll(value, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
	escaped = strings.ReplaceAll(escaped, "\n", " ")
	return "\"" + escaped + "\""
}

func writeSkillZip(zipPath string, skillID string, skillMarkdown string) error {
	if err := os.MkdirAll(filepath.Dir(zipPath), 0o755); err != nil {
		return err
	}
	file, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	header := &zip.FileHeader{
		Name:   filepath.ToSlash(filepath.Join(skillID, "SKILL.md")),
		Method: zip.Deflate,
	}
	entry, err := writer.CreateHeader(header)
	if err != nil {
		_ = writer.Close()
		return err
	}
	if _, err := io.WriteString(entry, skillMarkdown); err != nil {
		_ = writer.Close()
		return err
	}
	return writer.Close()
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

func defaultSkillInstallRoot() string {
	if codexHome := strings.TrimSpace(os.Getenv("CODEX_HOME")); codexHome != "" {
		return filepath.Join(codexHome, "skills")
	}
	if userProfile := strings.TrimSpace(os.Getenv("USERPROFILE")); userProfile != "" {
		return filepath.Join(userProfile, ".codex", "skills")
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".codex", "skills")
	}
	return filepath.Join(".", ".codex", "skills")
}

func pathInside(parent string, child string) bool {
	parentAbs, err := filepath.Abs(parent)
	if err != nil {
		return false
	}
	childAbs, err := filepath.Abs(child)
	if err != nil {
		return false
	}
	parentAbs = filepath.Clean(parentAbs)
	childAbs = filepath.Clean(childAbs)
	if strings.EqualFold(parentAbs, childAbs) {
		return false
	}
	relative, err := filepath.Rel(parentAbs, childAbs)
	if err != nil {
		return false
	}
	return relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func defaultPath() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = os.Getenv("APPDATA")
	}
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "Ariadne", "skills.json")
}
