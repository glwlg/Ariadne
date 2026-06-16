package apps

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"ariadne/internal/contracts"
)

type Shortcut struct {
	Name   string
	Path   string
	Source string
}

type Service struct {
	mu        sync.RWMutex
	roots     []string
	shortcuts []Shortcut
	scanned   bool
}

func NewService() *Service {
	return NewServiceWithRoots(defaultShortcutRoots()...)
}

func NewServiceWithRoots(roots ...string) *Service {
	return &Service{roots: cleanRoots(roots)}
}

func (s *Service) Search(query string) []contracts.SearchResult {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	shortcuts := s.Shortcuts()
	normalized := strings.ToLower(query)
	results := make([]contracts.SearchResult, 0, 12)
	for _, shortcut := range shortcuts {
		score := appScore(shortcut, normalized)
		if score <= 0 {
			continue
		}
		results = append(results, shortcutToResult(shortcut, score))
	}
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if len(results) > 12 {
		results = results[:12]
	}
	return results
}

func (s *Service) Shortcuts() []Shortcut {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.scanned {
		s.shortcuts = scanShortcuts(s.roots)
		s.scanned = true
	}
	return append([]Shortcut{}, s.shortcuts...)
}

func (s *Service) Refresh() []Shortcut {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shortcuts = scanShortcuts(s.roots)
	s.scanned = true
	return append([]Shortcut{}, s.shortcuts...)
}

func scanShortcuts(roots []string) []Shortcut {
	seen := map[string]bool{}
	shortcuts := []Shortcut{}
	for _, root := range roots {
		if root == "" {
			continue
		}
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".lnk") {
				return nil
			}
			name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			name = strings.TrimSpace(name)
			if name == "" {
				return nil
			}
			key := strings.ToLower(path)
			if seen[key] {
				return nil
			}
			seen[key] = true
			shortcuts = append(shortcuts, Shortcut{
				Name:   name,
				Path:   path,
				Source: sourceLabel(root),
			})
			return nil
		})
	}
	sort.SliceStable(shortcuts, func(i, j int) bool {
		return strings.ToLower(shortcuts[i].Name) < strings.ToLower(shortcuts[j].Name)
	})
	return shortcuts
}

func shortcutToResult(shortcut Shortcut, score float64) contracts.SearchResult {
	return contracts.SearchResult{
		ID:       "app-" + stableID(shortcut.Path),
		Type:     contracts.ResultApp,
		Title:    shortcut.Name,
		Subtitle: shortcut.Source + " · Start Menu",
		Detail:   shortcut.Path,
		Icon:     "app",
		Score:    score,
		Tags:     []string{"应用", "Start Menu", shortcut.Source},
		Payload: map[string]interface{}{
			"path":   shortcut.Path,
			"source": shortcut.Source,
		},
		Preview: contracts.PreviewDescriptor{
			Kind:     contracts.PreviewText,
			Title:    shortcut.Name,
			Subtitle: "Windows 应用快捷方式",
			Text:     "从开始菜单扫描到的应用快捷方式。Ariadne 会把快捷方式交给 Windows Shell 启动，不解析或改写目标路径。",
			Meta: []contracts.LabelValue{
				{Label: "快捷方式", Value: shortcut.Path},
				{Label: "来源", Value: shortcut.Source},
			},
		},
		Actions: []contracts.PreviewAction{
			{
				ID:       "open_app",
				Label:    "打开应用",
				Icon:     "open",
				Kind:     contracts.ActionOpen,
				Shortcut: "Enter",
				Payload:  map[string]interface{}{"path": shortcut.Path},
				Feedback: &contracts.ActionFeedback{SuccessLabel: "已启动", DurationMS: 1400},
			},
			contracts.CopyAction("copy_shortcut", "复制快捷方式路径", shortcut.Path, ""),
			contracts.RememberAction("remember_app", "加入记忆", "app-"+stableID(shortcut.Path)),
		},
	}
}

func appScore(shortcut Shortcut, query string) float64 {
	name := strings.ToLower(shortcut.Name)
	path := strings.ToLower(shortcut.Path)
	if name == query {
		return 100
	}
	if strings.HasPrefix(name, query) {
		return 90
	}
	if wordPrefix(name, query) {
		return 80
	}
	if strings.Contains(name, query) {
		return 70
	}
	if strings.Contains(path, query) {
		return 45
	}
	return 0
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

func defaultShortcutRoots() []string {
	roots := []string{}
	if appData := os.Getenv("APPDATA"); appData != "" {
		roots = append(roots, filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs"))
	}
	if programData := os.Getenv("PROGRAMDATA"); programData != "" {
		roots = append(roots, filepath.Join(programData, "Microsoft", "Windows", "Start Menu", "Programs"))
	}
	return roots
}

func cleanRoots(roots []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		key := strings.ToLower(filepath.Clean(root))
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, root)
	}
	return result
}

func sourceLabel(root string) string {
	normalized := strings.ToLower(root)
	if strings.Contains(normalized, strings.ToLower(os.Getenv("APPDATA"))) && os.Getenv("APPDATA") != "" {
		return "用户应用"
	}
	if strings.Contains(normalized, strings.ToLower(os.Getenv("PROGRAMDATA"))) && os.Getenv("PROGRAMDATA") != "" {
		return "系统应用"
	}
	return "应用"
}
