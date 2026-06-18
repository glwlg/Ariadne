package launchers

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"ariadne/internal/appdb"
	"ariadne/internal/contracts"
)

type LauncherKind string

const (
	LauncherApp     LauncherKind = "app"
	LauncherFile    LauncherKind = "file"
	LauncherFolder  LauncherKind = "folder"
	LauncherURL     LauncherKind = "url"
	LauncherCommand LauncherKind = "command"
)

type Launcher struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Kind       LauncherKind `json:"kind"`
	Target     string       `json:"target"`
	Arguments  string       `json:"arguments,omitempty"`
	WorkingDir string       `json:"workingDir,omitempty"`
	Keywords   []string     `json:"keywords,omitempty"`
	Tags       []string     `json:"tags,omitempty"`
	Enabled    bool         `json:"enabled"`
}

type Status struct {
	Path          string     `json:"path"`
	Count         int        `json:"count"`
	Items         []Launcher `json:"items"`
	LastSaveError string     `json:"lastSaveError,omitempty"`
}

type Service struct {
	mu        sync.RWMutex
	path      string
	launchers []Launcher
	removed   map[string]bool
	lastError string
}

func NewService() *Service {
	return NewServiceWithPath(defaultConfigPath())
}

func NewServiceWithPath(path string) *Service {
	service := &Service{path: path, launchers: defaultLaunchers(), removed: map[string]bool{}}
	service.load()
	return service
}

func (s *Service) Search(query string) []contracts.SearchResult {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	normalized := strings.ToLower(query)
	launchers := s.List()
	results := make([]contracts.SearchResult, 0, len(launchers))
	for _, launcher := range launchers {
		if !launcher.Enabled {
			continue
		}
		score := launcherScore(launcher, normalized)
		if score <= 0 {
			continue
		}
		results = append(results, launcherToResult(launcher, score))
	}
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	return results
}

func (s *Service) List() []Launcher {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Launcher{}, s.launchers...)
}

func (s *Service) Upsert(next Launcher) Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	next = normalizeLauncher(next)
	if next.ID == "" || next.Name == "" || next.Target == "" {
		return s.statusLocked()
	}
	replaced := false
	for i := range s.launchers {
		if s.launchers[i].ID == next.ID {
			s.launchers[i] = next
			replaced = true
			break
		}
	}
	if !replaced {
		s.launchers = append(s.launchers, next)
	}
	delete(s.removed, next.ID)
	sortLaunchers(s.launchers)
	s.lastError = ""
	if err := s.saveLocked(); err != nil {
		s.lastError = err.Error()
	}
	return s.statusLocked()
}

func (s *Service) Remove(id string) Status {
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	next := make([]Launcher, 0, len(s.launchers))
	for _, launcher := range s.launchers {
		if launcher.ID != id {
			next = append(next, launcher)
		}
	}
	s.launchers = next
	if id != "" {
		s.removed[id] = true
	}
	s.lastError = ""
	if err := s.saveLocked(); err != nil {
		s.lastError = err.Error()
	}
	return s.statusLocked()
}

func (s *Service) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.statusLocked()
}

func (s *Service) statusLocked() Status {
	return Status{Path: firstNonEmpty(appdb.DatabasePathForPath(s.path), s.path), Count: len(s.launchers), Items: append([]Launcher{}, s.launchers...), LastSaveError: s.lastError}
}

func (s *Service) load() {
	if s.path == "" {
		sortLaunchers(s.launchers)
		return
	}
	state, ok, err := loadLauncherStateFromSQLite(s.path)
	if err != nil || !ok {
		sortLaunchers(s.launchers)
		return
	}
	for id := range state.Removed {
		id = strings.TrimSpace(id)
		if id != "" {
			s.removed[id] = true
		}
	}
	merged := map[string]Launcher{}
	for _, launcher := range s.launchers {
		launcher = normalizeLauncher(launcher)
		if launcher.ID != "" && !s.removed[launcher.ID] {
			merged[launcher.ID] = launcher
		}
	}
	for _, launcher := range state.Launchers {
		launcher = normalizeLauncher(launcher)
		if launcher.ID != "" && launcher.Name != "" && launcher.Target != "" {
			merged[launcher.ID] = launcher
		}
	}
	s.launchers = make([]Launcher, 0, len(merged))
	for _, launcher := range merged {
		s.launchers = append(s.launchers, launcher)
	}
	sortLaunchers(s.launchers)
}

func (s *Service) saveLocked() error {
	if s.path == "" {
		return nil
	}
	return saveLauncherStateToSQLite(s.path, launcherState{Launchers: s.launchers, Removed: s.removed})
}

func launcherToResult(launcher Launcher, score float64) contracts.SearchResult {
	subtitle := launcherSubtitle(launcher)
	action := launcherAction(launcher)
	return contracts.SearchResult{
		ID:       "launcher-" + launcher.ID,
		Type:     contracts.ResultCommand,
		Title:    launcher.Name,
		Subtitle: subtitle,
		Detail:   launcher.Target,
		Icon:     launcherIcon(launcher.Kind),
		Score:    score,
		Tags:     append([]string{"启动项"}, launcher.Tags...),
		Payload: map[string]interface{}{
			"launcherId": launcher.ID,
			"kind":       string(launcher.Kind),
			"target":     launcher.Target,
			"arguments":  launcher.Arguments,
			"workingDir": launcher.WorkingDir,
		},
		Preview: contracts.PreviewDescriptor{
			Kind:     contracts.PreviewText,
			Title:    launcher.Name,
			Subtitle: subtitle,
			Text:     launcherPreviewText(launcher),
			Meta: []contracts.LabelValue{
				{Label: "类型", Value: string(launcher.Kind)},
				{Label: "目标", Value: launcher.Target},
				{Label: "关键词", Value: strings.Join(launcher.Keywords, ", ")},
			},
		},
		Actions: []contracts.PreviewAction{
			action,
			contracts.CopyAction("copy_target", "复制目标", launcher.Target, ""),
			contracts.RememberAction("remember_launcher", "加入记忆", "launcher-"+launcher.ID),
		},
	}
}

func launcherAction(launcher Launcher) contracts.PreviewAction {
	if launcher.Kind == LauncherCommand {
		return contracts.PreviewAction{
			ID:       "run_launcher",
			Label:    "确认运行",
			Icon:     "run",
			Kind:     contracts.ActionDanger,
			Shortcut: "Enter",
			Payload: map[string]interface{}{
				"command":              launcher.Target,
				"arguments":            launcher.Arguments,
				"workingDir":           launcher.WorkingDir,
				"requiresConfirmation": true,
			},
			Feedback: &contracts.ActionFeedback{SuccessLabel: "已启动", DurationMS: 1800},
		}
	}
	return contracts.PreviewAction{
		ID:       "open_launcher",
		Label:    "打开",
		Icon:     "open",
		Kind:     contracts.ActionOpen,
		Shortcut: "Enter",
		Payload:  map[string]interface{}{"path": launcher.Target},
		Feedback: &contracts.ActionFeedback{SuccessLabel: "已打开", DurationMS: 1400},
	}
}

func launcherScore(launcher Launcher, query string) float64 {
	name := strings.ToLower(launcher.Name)
	target := strings.ToLower(launcher.Target)
	keywords := strings.ToLower(strings.Join(launcher.Keywords, " "))
	if name == query {
		return 110
	}
	if strings.HasPrefix(name, query) {
		return 98
	}
	if wordPrefix(name, query) {
		return 88
	}
	if strings.Contains(keywords, query) {
		return 82
	}
	if strings.Contains(name, query) {
		return 76
	}
	if strings.Contains(target, query) {
		return 52
	}
	return 0
}

func launcherSubtitle(launcher Launcher) string {
	switch launcher.Kind {
	case LauncherApp:
		return "自定义启动项 · 应用"
	case LauncherFile:
		return "自定义启动项 · 文件"
	case LauncherFolder:
		return "自定义启动项 · 文件夹"
	case LauncherURL:
		return "自定义启动项 · URL"
	case LauncherCommand:
		return "自定义启动项 · 命令"
	default:
		return "自定义启动项"
	}
}

func launcherPreviewText(launcher Launcher) string {
	if launcher.Kind == LauncherCommand {
		return "命令类启动项属于中高风险动作，Ariadne 只展示确认动作，不会把命令伪装成低风险打开。"
	}
	return "由 Ariadne 自定义启动项配置提供。动作由结果显式声明，不根据 path 字段推断。"
}

func launcherIcon(kind LauncherKind) string {
	switch kind {
	case LauncherApp:
		return "app"
	case LauncherFile:
		return "file"
	case LauncherFolder:
		return "folder"
	case LauncherURL:
		return "open"
	case LauncherCommand:
		return "command"
	default:
		return "command"
	}
}

func normalizeLauncher(value Launcher) Launcher {
	value.ID = strings.TrimSpace(value.ID)
	value.Name = strings.TrimSpace(value.Name)
	value.Target = strings.TrimSpace(value.Target)
	value.Arguments = strings.TrimSpace(value.Arguments)
	value.WorkingDir = strings.TrimSpace(value.WorkingDir)
	value.Kind = normalizeKind(value.Kind)
	value.Keywords = cleanList(value.Keywords)
	value.Tags = cleanList(value.Tags)
	if value.ID == "" && (value.Name != "" || value.Target != "") {
		value.ID = stableID(value.Name + "|" + value.Target)
	}
	return value
}

func normalizeKind(kind LauncherKind) LauncherKind {
	switch LauncherKind(strings.ToLower(strings.TrimSpace(string(kind)))) {
	case LauncherApp:
		return LauncherApp
	case LauncherFile:
		return LauncherFile
	case LauncherFolder:
		return LauncherFolder
	case LauncherURL:
		return LauncherURL
	case LauncherCommand:
		return LauncherCommand
	default:
		return LauncherApp
	}
}

func defaultLaunchers() []Launcher {
	launchers := []Launcher{}
	if appData := os.Getenv("APPDATA"); appData != "" {
		launchers = append(launchers, Launcher{
			ID:       "ariadne-config-dir",
			Name:     "Ariadne 配置目录",
			Kind:     LauncherFolder,
			Target:   filepath.Join(appData, "Ariadne"),
			Keywords: []string{"ariadne", "config", "配置"},
			Tags:     []string{"配置"},
			Enabled:  true,
		})
	}
	if everything := findExisting(
		`C:\Program Files\Everything\Everything.exe`,
		`P:\Program Files\Everything\Everything.exe`,
	); everything != "" {
		launchers = append(launchers, Launcher{
			ID:       "everything",
			Name:     "Everything",
			Kind:     LauncherApp,
			Target:   everything,
			Keywords: []string{"everything", "file search", "文件搜索"},
			Tags:     []string{"搜索"},
			Enabled:  true,
		})
	}
	sortLaunchers(launchers)
	return launchers
}

func defaultConfigPath() string {
	base := os.Getenv("APPDATA")
	if base == "" {
		if dir, err := os.UserConfigDir(); err == nil {
			base = dir
		}
	}
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "Ariadne", "launchers.json")
}

func findExisting(paths ...string) string {
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

func sortLaunchers(launchers []Launcher) {
	sort.SliceStable(launchers, func(i, j int) bool {
		return strings.ToLower(launchers[i].Name) < strings.ToLower(launchers[j].Name)
	})
}

func cleanList(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, value)
	}
	return result
}

func wordPrefix(value string, query string) bool {
	for _, part := range strings.FieldsFunc(value, func(r rune) bool {
		return r == ' ' || r == '-' || r == '_' || r == '.'
	}) {
		if strings.HasPrefix(part, query) {
			return true
		}
	}
	return false
}

func stableID(value string) string {
	sum := sha1.Sum([]byte(strings.ToLower(value)))
	return hex.EncodeToString(sum[:])[:12]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
