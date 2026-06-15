package legacybridge

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"

	"ariadne/internal/contracts"
)

const defaultTimeout = 5 * time.Second

//go:embed runner.py
var embeddedRunner string

type Options struct {
	Enabled       bool
	PythonPath    string
	WorkspaceRoot string
	PluginDir     string
	RunnerPath    string
	Timeout       time.Duration
}

type Service struct {
	options Options
}

type Manifest struct {
	ID                   string                 `json:"id"`
	Name                 string                 `json:"name"`
	Description          string                 `json:"description"`
	Keywords             []string               `json:"keywords"`
	SupportedPlatforms   []string               `json:"supportedPlatforms"`
	RequiredCapabilities []string               `json:"requiredCapabilities"`
	CommandSchema        map[string]interface{} `json:"commandSchema,omitempty"`
	DirectAction         bool                   `json:"directAction,omitempty"`
	LoadError            string                 `json:"loadError,omitempty"`
}

type runnerEnvelope struct {
	OK        bool                     `json:"ok"`
	Error     string                   `json:"error,omitempty"`
	Manifests []Manifest               `json:"manifests,omitempty"`
	Results   []map[string]interface{} `json:"results,omitempty"`
}

func DefaultOptions() Options {
	return Options{
		Enabled:    true,
		PythonPath: "python",
		Timeout:    defaultTimeout,
	}
}

func New(options Options) *Service {
	if options.PythonPath == "" {
		options.PythonPath = "python"
	}
	if options.Timeout <= 0 {
		options.Timeout = defaultTimeout
	}
	if options.WorkspaceRoot == "" {
		options.WorkspaceRoot = FindWorkspaceRoot("")
	}
	if options.PluginDir == "" && options.WorkspaceRoot != "" {
		options.PluginDir = filepath.Join(options.WorkspaceRoot, "src", "plugins")
	}
	if options.RunnerPath == "" {
		options.RunnerPath = defaultRunnerPath()
	}
	return &Service{options: options}
}

func FindWorkspaceRoot(start string) string {
	if start == "" {
		if cwd, err := os.Getwd(); err == nil {
			start = cwd
		}
	}
	if start == "" {
		return ""
	}
	current, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		marker := filepath.Join(current, "src", "core", "plugin_base.py")
		if _, err := os.Stat(marker); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

func (s *Service) List() ([]Manifest, error) {
	envelope, err := s.run("list")
	if err != nil {
		return nil, err
	}
	return envelope.Manifests, nil
}

func (s *Service) Execute(keyword string, query string) []contracts.SearchResult {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return []contracts.SearchResult{diagnosticResult("legacy-python-missing-keyword", "缺少旧插件关键词", "用法: legacy <plugin-keyword> [query]")}
	}
	envelope, err := s.run("execute", "--keyword", keyword, "--query", query)
	if err != nil {
		return []contracts.SearchResult{diagnosticResult("legacy-python-error", "Python 旧插件桥不可用", err.Error())}
	}
	if len(envelope.Results) == 0 {
		return []contracts.SearchResult{diagnosticResult("legacy-python-empty", "旧插件无结果", "插件 "+keyword+" 没有返回可展示结果。")}
	}
	results := make([]contracts.SearchResult, 0, len(envelope.Results))
	for index, raw := range envelope.Results {
		results = append(results, rawResultToSearchResult(keyword, index, raw))
	}
	if err := contracts.ValidateActionSurfaces(results); err != nil {
		return []contracts.SearchResult{diagnosticResult("legacy-python-invalid-action", "旧插件结果动作无效", err.Error())}
	}
	return results
}

func (s *Service) run(command string, extraArgs ...string) (runnerEnvelope, error) {
	options, err := s.resolvedOptions()
	if err != nil {
		return runnerEnvelope{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), options.Timeout)
	defer cancel()

	args := []string{
		options.RunnerPath,
		"--workspace-root", options.WorkspaceRoot,
		"--plugin-dir", options.PluginDir,
		command,
	}
	args = append(args, extraArgs...)

	cmd := exec.CommandContext(ctx, options.PythonPath, args...)
	cmd.Dir = options.WorkspaceRoot
	cmd.Env = legacyEnv(options.WorkspaceRoot)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return runnerEnvelope{}, fmt.Errorf("legacy bridge timed out after %s", options.Timeout)
		}
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return runnerEnvelope{}, fmt.Errorf("%w: %s", err, detail)
		}
		return runnerEnvelope{}, err
	}

	raw := bytes.TrimSpace(stdout.Bytes())
	if len(raw) == 0 {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return runnerEnvelope{}, fmt.Errorf("legacy bridge returned empty output: %s", detail)
		}
		return runnerEnvelope{}, fmt.Errorf("legacy bridge returned empty output")
	}
	var envelope runnerEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return runnerEnvelope{}, fmt.Errorf("decode legacy bridge output: %w", err)
	}
	if !envelope.OK {
		if envelope.Error == "" {
			envelope.Error = "legacy bridge failed"
		}
		return envelope, errors.New(envelope.Error)
	}
	return envelope, nil
}

func (s *Service) resolvedOptions() (Options, error) {
	options := s.options
	if !options.Enabled {
		return options, fmt.Errorf("Python legacy bridge is disabled")
	}
	if strings.TrimSpace(options.WorkspaceRoot) == "" {
		return options, fmt.Errorf("cannot locate legacy Python workspace root")
	}
	if strings.TrimSpace(options.PluginDir) == "" {
		return options, fmt.Errorf("cannot locate legacy Python plugin directory")
	}
	if strings.TrimSpace(options.RunnerPath) == "" {
		return options, fmt.Errorf("cannot locate legacy bridge runner")
	}
	runnerPath, err := ensureRunnerPath(options.RunnerPath)
	if err != nil {
		return options, fmt.Errorf("legacy bridge runner unavailable: %w", err)
	}
	options.RunnerPath = runnerPath
	if _, err := os.Stat(options.PluginDir); err != nil {
		return options, fmt.Errorf("legacy plugin directory unavailable: %w", err)
	}
	return options, nil
}

func defaultRunnerPath() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	return filepath.Join(filepath.Dir(file), "runner.py")
}

func ensureRunnerPath(path string) (string, error) {
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	if embeddedRunner == "" {
		return "", fmt.Errorf("embedded runner is empty")
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil || cacheDir == "" {
		cacheDir = os.TempDir()
	}
	dir := filepath.Join(cacheDir, "Ariadne", "legacybridge")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	target := filepath.Join(dir, "runner.py")
	if current, err := os.ReadFile(target); err == nil && string(current) == embeddedRunner {
		return target, nil
	}
	if err := os.WriteFile(target, []byte(embeddedRunner), 0644); err != nil {
		return "", err
	}
	return target, nil
}

func legacyEnv(workspaceRoot string) []string {
	env := os.Environ()
	env = append(env, "PYTHONIOENCODING=utf-8", "ARIADNE_LEGACY_BRIDGE=1")
	if workspaceRoot == "" {
		return env
	}
	existing := os.Getenv("PYTHONPATH")
	if existing == "" {
		env = append(env, "PYTHONPATH="+workspaceRoot)
		return env
	}
	env = append(env, "PYTHONPATH="+workspaceRoot+string(os.PathListSeparator)+existing)
	return env
}

func rawResultToSearchResult(keyword string, index int, raw map[string]interface{}) contracts.SearchResult {
	legacyType := stringField(raw, "type")
	title := firstStringField(raw, "name", "title", "label")
	if title == "" {
		title = "Python legacy result"
	}
	text := firstStringField(raw, "path", "text", "value", "detail", "error")
	if text == "" {
		text = title
	}
	detail := firstStringField(raw, "detail", "path", "text", "value", "error")
	if detail == "" {
		detail = compactJSON(raw)
	}
	id := fmt.Sprintf("legacy-python-%s-%d", sanitizeID(keyword), index+1)
	tags := []string{"legacy", "python"}
	if legacyType != "" {
		tags = append(tags, legacyType)
	}
	label := "复制结果"
	if isErrorType(legacyType) || firstStringField(raw, "error") != "" {
		label = "复制诊断"
	}
	return contracts.SearchResult{
		ID:       id,
		Type:     contracts.ResultPluginResult,
		Title:    title,
		Subtitle: "Python 旧插件 · " + keyword,
		Detail:   detail,
		Icon:     "plugin",
		Score:    96,
		Tags:     tags,
		Payload: map[string]interface{}{
			"legacy":        true,
			"legacyKeyword": keyword,
			"legacyType":    legacyType,
			"raw":           raw,
		},
		Preview: contracts.PreviewDescriptor{
			Kind:     contracts.PreviewText,
			Title:    title,
			Subtitle: "Python 旧插件返回",
			Text:     text,
			Meta: []contracts.LabelValue{
				{Label: "旧插件", Value: keyword},
				{Label: "结果类型", Value: legacyType},
			},
		},
		Actions: []contracts.PreviewAction{
			contracts.CopyAction("copy_legacy_result", label, text, "Enter"),
			contracts.RememberAction("remember_legacy_result", "加入记忆", id),
		},
	}
}

func diagnosticResult(id string, title string, detail string) contracts.SearchResult {
	if detail == "" {
		detail = title
	}
	return contracts.SearchResult{
		ID:       id,
		Type:     contracts.ResultPluginResult,
		Title:    title,
		Subtitle: "Python 旧插件桥",
		Detail:   detail,
		Icon:     "error",
		Score:    92,
		Tags:     []string{"legacy", "python", "diagnostic"},
		Preview:  contracts.PreviewDescriptor{Kind: contracts.PreviewText, Title: title, Subtitle: "Python 旧插件桥", Text: detail},
		Actions:  []contracts.PreviewAction{contracts.CopyAction("copy_diagnostic", "复制诊断", detail, "")},
	}
}

func firstStringField(raw map[string]interface{}, names ...string) string {
	for _, name := range names {
		value := stringField(raw, name)
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func stringField(raw map[string]interface{}, name string) string {
	value, ok := raw[name]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func compactJSON(value interface{}) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(raw)
}

func sanitizeID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "unknown"
	}
	return result
}

func isErrorType(value string) bool {
	value = strings.ToLower(value)
	return strings.Contains(value, "error") || strings.Contains(value, "fail")
}
