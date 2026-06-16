package ocr

import (
	"context"
	"crypto/sha1"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ariadne/internal/capturehistory"
	"ariadne/internal/clipboardhistory"
	"ariadne/internal/workmemory"
)

//go:embed rapidocr_bridge.py
var bridgeFS embed.FS

type CaptureProvider interface {
	Entry(id string) capturehistory.Entry
	CaptureScreen(source string) capturehistory.Status
}

type ClipboardProvider interface {
	Entry(id string) clipboardhistory.Entry
}

type WorkMemoryProvider interface {
	Entry(id string) workmemory.Entry
	ApplyOCRText(id string, text string, provider string) workmemory.Entry
}

type Rect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type Line struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	Rect       Rect    `json:"rect,omitempty"`
}

type Status struct {
	Available  bool   `json:"available"`
	Provider   string `json:"provider"`
	Mode       string `json:"mode"`
	PythonPath string `json:"pythonPath,omitempty"`
	BridgePath string `json:"bridgePath,omitempty"`
	LastError  string `json:"lastError,omitempty"`
	LastRunAt  int64  `json:"lastRunAt,omitempty"`
}

type Result struct {
	OK           bool              `json:"ok"`
	Text         string            `json:"text,omitempty"`
	Lines        []Line            `json:"lines,omitempty"`
	Source       string            `json:"source,omitempty"`
	CaptureID    string            `json:"captureId,omitempty"`
	ClipboardID  string            `json:"clipboardId,omitempty"`
	MemoryID     string            `json:"memoryId,omitempty"`
	ImagePath    string            `json:"imagePath,omitempty"`
	Width        int               `json:"width,omitempty"`
	Height       int               `json:"height,omitempty"`
	Provider     string            `json:"provider,omitempty"`
	ElapsedMs    int               `json:"elapsedMs,omitempty"`
	Sensitive    bool              `json:"sensitive"`
	Error        string            `json:"error,omitempty"`
	RecognizedAt int64             `json:"recognizedAt,omitempty"`
	WorkMemory   *workmemory.Entry `json:"workMemory,omitempty"`
}

type bridgeOutput struct {
	OK        bool   `json:"ok"`
	Provider  string `json:"provider"`
	Text      string `json:"text"`
	Lines     []Line `json:"lines"`
	ElapsedMs int    `json:"elapsedMs"`
	Error     string `json:"error"`
}

type bridgeRunner func(context.Context, string) (bridgeOutput, Status)

type AIOCRPolicy struct {
	Enabled  bool
	Provider string
	BaseURL  string
	Model    string
}

type AIOCRJob struct {
	Provider  string
	BaseURL   string
	Model     string
	ImagePath string
}

type AIResult struct {
	Provider  string
	Text      string
	Lines     []Line
	ElapsedMs int
}

type AIClient interface {
	RecognizeImageOCR(context.Context, AIOCRJob) (AIResult, error)
}

type Service struct {
	mu        sync.RWMutex
	captures  CaptureProvider
	clipboard ClipboardProvider
	memory    WorkMemoryProvider
	runner    bridgeRunner
	aiClient  AIClient
	aiPolicy  AIOCRPolicy
	last      Result
	status    Status
	now       func() time.Time
}

func NewService(captures CaptureProvider, clipboard ClipboardProvider, memory WorkMemoryProvider) *Service {
	service := &Service{
		captures:  captures,
		clipboard: clipboard,
		memory:    memory,
		now:       time.Now,
	}
	service.runner = service.runRapidOCRBridge
	return service
}

func RegisterAIClient(service *Service, client AIClient) {
	if service == nil {
		return
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	service.aiClient = client
}

func (s *Service) ApplyAIOCRPolicy(policy AIOCRPolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.aiPolicy = normalizeAIOCRPolicy(policy)
}

func (s *Service) Status() Status {
	status := s.detectStatus()
	s.mu.RLock()
	if s.aiOCRConfiguredLocked() {
		status.Available = true
		status.Provider = firstNonEmpty(s.aiPolicy.Provider, "openai-compatible") + ":" + s.aiPolicy.Model
		status.Mode = "vision_model+rapidocr_fallback"
	}
	if s.status.LastError != "" {
		status.LastError = s.status.LastError
	}
	if s.status.LastRunAt != 0 {
		status.LastRunAt = s.status.LastRunAt
	}
	s.mu.RUnlock()
	return status
}

func (s *Service) LastResult() Result {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.last
}

func (s *Service) RecognizeCapture(captureID string) Result {
	captureID = strings.TrimSpace(captureID)
	if captureID == "" {
		return s.record(Result{OK: false, Source: "capture_history", Error: "缺少截图记录 ID"})
	}
	if s.captures == nil {
		return s.record(Result{OK: false, Source: "capture_history", CaptureID: captureID, Error: "截图历史服务不可用"})
	}
	entry := s.captures.Entry(captureID)
	if entry.ID == "" || strings.TrimSpace(entry.ImagePath) == "" {
		return s.record(Result{OK: false, Source: "capture_history", CaptureID: captureID, Error: "未找到截图记录"})
	}
	result := s.recognizeImage(entry.ImagePath, "capture_history")
	result.CaptureID = entry.ID
	result.Width = entry.Width
	result.Height = entry.Height
	return s.record(result)
}

func (s *Service) RecognizeCurrentScreen() Result {
	if s.captures == nil {
		return s.record(Result{OK: false, Source: "current_screen", Error: "截图历史服务不可用"})
	}
	status := s.captures.CaptureScreen("ocr_current_screen")
	if status.LastCaptureError != "" {
		return s.record(Result{OK: false, Source: "current_screen", Error: status.LastCaptureError})
	}
	capture, ok := latestCapture(status.Entries, "ocr_current_screen")
	if !ok {
		return s.record(Result{OK: false, Source: "current_screen", Error: "截图服务未返回 OCR 记录"})
	}
	result := s.recognizeImage(capture.ImagePath, "current_screen")
	result.CaptureID = capture.ID
	result.Width = capture.Width
	result.Height = capture.Height
	return s.record(result)
}

func (s *Service) RecognizeClipboardImage(clipboardID string) Result {
	clipboardID = strings.TrimSpace(clipboardID)
	if clipboardID == "" {
		return s.record(Result{OK: false, Source: "clipboard_history", Error: "缺少剪贴板记录 ID"})
	}
	if s.clipboard == nil {
		return s.record(Result{OK: false, Source: "clipboard_history", ClipboardID: clipboardID, Error: "剪贴板历史服务不可用"})
	}
	entry := s.clipboard.Entry(clipboardID)
	if entry.ID == "" || entry.Type != clipboardhistory.EntryImage || strings.TrimSpace(entry.ImagePath) == "" {
		return s.record(Result{OK: false, Source: "clipboard_history", ClipboardID: clipboardID, Error: "未找到剪贴板图片"})
	}
	result := s.recognizeImage(entry.ImagePath, "clipboard_history")
	result.ClipboardID = entry.ID
	result.Width = entry.Width
	result.Height = entry.Height
	return s.record(result)
}

func (s *Service) RecognizeWorkMemory(memoryID string) Result {
	memoryID = strings.TrimSpace(memoryID)
	if memoryID == "" {
		return s.record(Result{OK: false, Source: "work_memory", Error: "缺少工作记忆 ID"})
	}
	if s.memory == nil {
		return s.record(Result{OK: false, Source: "work_memory", MemoryID: memoryID, Error: "工作记忆服务不可用"})
	}
	entry := s.memory.Entry(memoryID)
	if entry.ID == "" || strings.TrimSpace(entry.ImagePath) == "" {
		return s.record(Result{OK: false, Source: "work_memory", MemoryID: memoryID, Error: "该工作记忆没有图片证据"})
	}
	if entry.Sensitive {
		updated := s.memory.ApplyOCRText(memoryID, "", "blocked_sensitive")
		return s.record(Result{OK: false, Source: "work_memory", MemoryID: memoryID, ImagePath: entry.ImagePath, Error: "敏感条目默认不执行 OCR", WorkMemory: &updated})
	}
	result := s.recognizeImage(entry.ImagePath, "work_memory")
	result.MemoryID = entry.ID
	result.Width = entry.Width
	result.Height = entry.Height
	if result.OK {
		updated := s.memory.ApplyOCRText(entry.ID, result.Text, result.Provider)
		result.WorkMemory = &updated
		result.Sensitive = updated.Sensitive
	} else {
		updated := s.memory.ApplyOCRText(entry.ID, "", "failed: "+result.Error)
		result.WorkMemory = &updated
	}
	return s.record(result)
}

func (s *Service) RecognizeImagePath(path string) Result {
	return s.record(s.recognizeImage(path, "image_path"))
}

func (s *Service) recognizeImage(path string, source string) Result {
	path = strings.TrimSpace(path)
	if path == "" {
		return Result{OK: false, Source: source, Error: "缺少图片路径"}
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return Result{OK: false, Source: source, ImagePath: path, Error: "图片文件不存在"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	output, status := s.runConfiguredOCR(ctx, path)
	result := Result{
		OK:           output.OK,
		Text:         strings.TrimSpace(output.Text),
		Lines:        cleanLines(output.Lines),
		Source:       source,
		ImagePath:    path,
		Provider:     firstNonEmpty(output.Provider, status.Provider, "rapidocr_onnxruntime"),
		ElapsedMs:    output.ElapsedMs,
		Error:        strings.TrimSpace(output.Error),
		RecognizedAt: s.now().Unix(),
	}
	if result.OK {
		result.Sensitive = looksSensitive(result.Text)
		if result.Text == "" {
			result.Error = ""
		}
	} else if result.Error == "" {
		result.Error = firstNonEmpty(status.LastError, "OCR 不可用")
	}
	s.mu.Lock()
	s.status = status
	s.status.LastRunAt = result.RecognizedAt
	if !result.OK {
		s.status.LastError = result.Error
	} else {
		s.status.LastError = ""
	}
	s.mu.Unlock()
	return result
}

func (s *Service) runConfiguredOCR(ctx context.Context, imagePath string) (bridgeOutput, Status) {
	s.mu.RLock()
	client := s.aiClient
	policy := s.aiPolicy
	runner := s.runner
	s.mu.RUnlock()
	if runner == nil {
		runner = s.runRapidOCRBridge
	}
	if !aiOCRPolicyReady(policy) || client == nil {
		return runner(ctx, imagePath)
	}

	started := time.Now()
	result, err := client.RecognizeImageOCR(ctx, AIOCRJob{
		Provider:  policy.Provider,
		BaseURL:   policy.BaseURL,
		Model:     policy.Model,
		ImagePath: imagePath,
	})
	if err == nil && strings.TrimSpace(result.Text) != "" {
		elapsed := result.ElapsedMs
		if elapsed <= 0 {
			elapsed = int(time.Since(started) / time.Millisecond)
		}
		return bridgeOutput{
				OK:        true,
				Provider:  firstNonEmpty(result.Provider, policy.Provider+":"+policy.Model),
				Text:      strings.TrimSpace(result.Text),
				Lines:     cleanLines(result.Lines),
				ElapsedMs: elapsed,
			}, Status{
				Available: true,
				Provider:  firstNonEmpty(result.Provider, policy.Provider+":"+policy.Model),
				Mode:      "vision_model",
			}
	}

	aiError := "大模型 OCR 返回空内容"
	if err != nil {
		aiError = err.Error()
	}
	output, status := runner(ctx, imagePath)
	if !output.OK {
		output.Error = firstNonEmpty(output.Error, status.LastError, "本地 OCR 不可用")
		output.Error = fmt.Sprintf("大模型 OCR 失败: %s；已尝试本地 OCR: %s", aiError, output.Error)
		status.LastError = output.Error
	}
	return output, status
}

func (s *Service) runRapidOCRBridge(ctx context.Context, imagePath string) (bridgeOutput, Status) {
	status := s.detectStatus()
	if !status.Available {
		return bridgeOutput{OK: false, Provider: status.Provider, Error: status.LastError}, status
	}
	bridgePath, err := writeBridgeFile()
	if err != nil {
		status.Available = false
		status.LastError = err.Error()
		return bridgeOutput{OK: false, Provider: status.Provider, Error: status.LastError}, status
	}
	status.BridgePath = bridgePath
	cmdPath := status.PythonPath
	args := []string{bridgePath, imagePath}
	if status.Mode == "uv" {
		cmdPath = status.PythonPath
		args = []string{"run", "python", bridgePath, imagePath}
	}
	cmd := exec.CommandContext(ctx, cmdPath, args...)
	cmd.Env = ocrCommandEnv(os.Environ())
	configureOCRCommand(cmd)
	if root := repoRoot(); root != "" && status.Mode == "uv" {
		cmd.Dir = root
	}
	raw, err := cmd.Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) && len(exitError.Stderr) > 0 {
			status.LastError = strings.TrimSpace(string(exitError.Stderr))
		} else if ctx.Err() != nil {
			status.LastError = "OCR 执行超时"
		} else {
			status.LastError = err.Error()
		}
		return bridgeOutput{OK: false, Provider: status.Provider, Error: status.LastError}, status
	}
	var output bridgeOutput
	if err := json.Unmarshal(raw, &output); err != nil {
		status.LastError = "OCR 输出解析失败: " + err.Error()
		return bridgeOutput{OK: false, Provider: status.Provider, Error: status.LastError}, status
	}
	if output.Provider == "" {
		output.Provider = status.Provider
	}
	if !output.OK && output.Error != "" {
		status.LastError = output.Error
	}
	return output, status
}

func (s *Service) aiOCRConfiguredLocked() bool {
	return aiOCRPolicyReady(s.aiPolicy) && s.aiClient != nil
}

func normalizeAIOCRPolicy(policy AIOCRPolicy) AIOCRPolicy {
	policy.Provider = strings.TrimSpace(policy.Provider)
	if policy.Provider == "" {
		policy.Provider = "openai-compatible"
	}
	policy.BaseURL = strings.TrimSpace(policy.BaseURL)
	policy.Model = strings.TrimSpace(policy.Model)
	return policy
}

func aiOCRPolicyReady(policy AIOCRPolicy) bool {
	if !policy.Enabled {
		return false
	}
	provider := strings.ToLower(strings.TrimSpace(policy.Provider))
	return strings.TrimSpace(policy.Model) != "" && provider != "" && provider != "disabled" && provider != "none" && provider != "off"
}

func (s *Service) detectStatus() Status {
	status := Status{Provider: "rapidocr_onnxruntime", Mode: "python"}
	if env := strings.TrimSpace(os.Getenv("ARIADNE_OCR_PYTHON")); env != "" {
		status.PythonPath = env
		status.Available = fileExists(env)
		if !status.Available {
			status.LastError = "ARIADNE_OCR_PYTHON 指向的 Python 不存在"
		}
		return status
	}
	if path := findAriadneOCRPython(); path != "" {
		status.PythonPath = path
		status.Available = true
		return status
	}
	if pyPath, err := exec.LookPath("python"); err == nil && pyPath != "" {
		status.PythonPath = pyPath
		status.Available = true
		return status
	}
	if allowRepoOCRPython() {
		if path := findRepoVenvPython(); path != "" {
			status.PythonPath = path
			status.Available = true
			return status
		}
		if uvPath, err := exec.LookPath("uv"); err == nil && uvPath != "" && repoRoot() != "" {
			status.Mode = "uv"
			status.PythonPath = uvPath
			status.Available = true
			return status
		}
	}
	status.LastError = "未找到 Ariadne OCR Python。请配置 ARIADNE_OCR_PYTHON，或安装 Ariadne 专用 OCR runtime 到 %LOCALAPPDATA%\\Ariadne\\ocr-python；不会默认调用旧 x-tools .venv。"
	return status
}

func (s *Service) record(result Result) Result {
	if result.RecognizedAt == 0 {
		result.RecognizedAt = s.now().Unix()
	}
	if result.Provider == "" {
		result.Provider = "rapidocr_onnxruntime"
	}
	if result.OK && result.Text == "" && result.Error == "" {
		result.Error = "未识别到文字"
	}
	if !result.OK && result.Error == "" {
		result.Error = "OCR 不可用"
	}
	s.mu.Lock()
	s.last = result
	s.mu.Unlock()
	return result
}

func writeBridgeFile() (string, error) {
	raw, err := bridgeFS.ReadFile("rapidocr_bridge.py")
	if err != nil {
		return "", err
	}
	sum := sha1.Sum(raw)
	name := "ariadne-rapidocr-" + hex.EncodeToString(sum[:6]) + ".py"
	path := filepath.Join(os.TempDir(), name)
	if existing, err := os.ReadFile(path); err == nil && string(existing) == string(raw) {
		return path, nil
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func ocrCommandEnv(base []string) []string {
	env := make([]string, 0, len(base)+3)
	skip := map[string]bool{
		"PYTHONIOENCODING": true,
		"PYTHONUTF8":       true,
	}
	for _, item := range base {
		key, _, ok := strings.Cut(item, "=")
		if !ok || skip[strings.ToUpper(key)] {
			continue
		}
		env = append(env, item)
	}
	env = append(env,
		"PYTHONIOENCODING=utf-8",
		"PYTHONUTF8=1",
		"ARIADNE_OCR_BRIDGE=1",
	)
	return env
}

func findAriadneOCRPython() string {
	candidates := []string{}
	if env := strings.TrimSpace(os.Getenv("ARIADNE_OCR_HOME")); env != "" {
		candidates = append(candidates, pythonCandidatesIn(env)...)
	}
	if exe, err := os.Executable(); err == nil && exe != "" {
		exeDir := filepath.Dir(exe)
		for _, rel := range []string{
			filepath.Join("ocr-python"),
			filepath.Join("runtime", "ocr-python"),
			filepath.Join("runtime", "python"),
			filepath.Join("python"),
		} {
			candidates = append(candidates, pythonCandidatesIn(filepath.Join(exeDir, rel))...)
		}
	}
	if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
		candidates = append(candidates, pythonCandidatesIn(filepath.Join(local, "Ariadne", "ocr-python"))...)
		candidates = append(candidates, pythonCandidatesIn(filepath.Join(local, "Ariadne", "runtime", "ocr-python"))...)
	}
	if appdata := strings.TrimSpace(os.Getenv("APPDATA")); appdata != "" {
		candidates = append(candidates, pythonCandidatesIn(filepath.Join(appdata, "Ariadne", "ocr-python"))...)
	}
	for _, candidate := range candidates {
		if fileExists(candidate) {
			return candidate
		}
	}
	return ""
}

func pythonCandidatesIn(root string) []string {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	return []string{
		filepath.Join(root, "python.exe"),
		filepath.Join(root, "Scripts", "python.exe"),
		filepath.Join(root, "bin", "python"),
	}
}

func allowRepoOCRPython() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv("ARIADNE_OCR_ALLOW_REPO_PYTHON")))
	return value == "1" || value == "true" || value == "yes"
}

func findRepoVenvPython() string {
	root := repoRoot()
	if root == "" {
		root, _ = os.Getwd()
	}
	if root == "" {
		return ""
	}
	candidates := []string{
		filepath.Join(root, ".venv", "Scripts", "python.exe"),
		filepath.Join(root, ".venv", "bin", "python"),
	}
	for _, candidate := range candidates {
		if fileExists(candidate) {
			return candidate
		}
	}
	return ""
}

func repoRoot() string {
	seen := map[string]bool{}
	starts := []string{}
	if wd, err := os.Getwd(); err == nil {
		starts = append(starts, wd)
	}
	if exe, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(exe))
	}
	for _, start := range starts {
		dir, _ := filepath.Abs(start)
		for dir != "" && !seen[dir] {
			seen[dir] = true
			if fileExists(filepath.Join(dir, "pyproject.toml")) && fileExists(filepath.Join(dir, "uv.lock")) {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return ""
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func latestCapture(entries []capturehistory.Entry, source string) (capturehistory.Entry, bool) {
	var newest capturehistory.Entry
	for _, entry := range entries {
		if source != "" && entry.Source != source {
			continue
		}
		if newest.ID == "" || entry.CreatedAt > newest.CreatedAt {
			newest = entry
		}
	}
	return newest, newest.ID != ""
}

func cleanLines(lines []Line) []Line {
	result := make([]Line, 0, len(lines))
	for _, line := range lines {
		line.Text = strings.TrimSpace(line.Text)
		if line.Text == "" {
			continue
		}
		if line.Confidence < 0 {
			line.Confidence = 0
		}
		result = append(result, line)
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func looksSensitive(text string) bool {
	lower := strings.ToLower(text)
	patterns := []string{
		"password", "passwd", "pwd=", "token", "api_key", "apikey", "secret",
		"authorization:", "bearer ", "cookie:", "private key", "ssh-rsa", "ssh-ed25519",
		"数据库密码", "密码", "密钥", "令牌", "验证码",
	}
	for _, pattern := range patterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

func RuntimeAvailable() bool {
	service := NewService(nil, nil, nil)
	return service.detectStatus().Available
}

func RuntimeNote() string {
	service := NewService(nil, nil, nil)
	status := service.detectStatus()
	if status.Available {
		return fmt.Sprintf("使用 Ariadne 本地 RapidOCR bridge，Python=%s；图片不外发。", status.PythonPath)
	}
	return status.LastError
}
