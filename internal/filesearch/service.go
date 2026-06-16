package filesearch

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ariadne/internal/contracts"
)

const defaultMaxResults uint32 = 24

type rawResult struct {
	Name string
	Path string
}

type fileMetadata struct {
	Kind        string
	SizeLabel   string
	SizeBytes   int64
	Modified    time.Time
	ModifiedISO string
	Available   bool
	IsDirectory bool
	Error       string
}

type everythingClient interface {
	Search(query string, maxResults uint32) ([]rawResult, error)
}

type contextEverythingClient interface {
	SearchContext(ctx context.Context, query string, maxResults uint32) ([]rawResult, error)
}

type Service struct {
	mu              sync.Mutex
	dllPath         string
	client          everythingClient
	lastErr         string
	lastQuery       string
	lastElapsedMs   int64
	lastResultCount int
	lastUpdatedAt   int64
}

type EverythingStatus struct {
	DLLPath         string `json:"dllPath,omitempty"`
	DLLFound        bool   `json:"dllFound"`
	Ready           bool   `json:"ready"`
	LastError       string `json:"lastError,omitempty"`
	LastQuery       string `json:"lastQuery,omitempty"`
	LastElapsedMs   int64  `json:"lastElapsedMs"`
	LastResultCount int    `json:"lastResultCount"`
	LastUpdatedAt   int64  `json:"lastUpdatedAt,omitempty"`
	CoverageHint    string `json:"coverageHint,omitempty"`
}

func NewService() *Service {
	return NewServiceWithDLLPath(findDefaultEverythingDLL())
}

func NewServiceWithDLLPath(dllPath string) *Service {
	return &Service{dllPath: dllPath}
}

func NewServiceWithClient(client everythingClient) *Service {
	return &Service{client: client}
}

func (s *Service) Search(query string) []contracts.SearchResult {
	return s.SearchContext(context.Background(), query)
}

func (s *Service) SearchContext(ctx context.Context, query string) []contracts.SearchResult {
	if ctx == nil {
		ctx = context.Background()
	}
	query = strings.TrimSpace(query)
	if len([]rune(query)) < 2 {
		return nil
	}
	if ctx.Err() != nil {
		return nil
	}
	started := time.Now()
	client, err := s.ensureClient()
	if err != nil {
		s.setLastError(err)
		s.recordQueryStatus(query, time.Since(started).Milliseconds(), 0)
		if shouldShowCoverageHint(query) {
			return []contracts.SearchResult{coverageHintResult(query, s.Status())}
		}
		return nil
	}
	if ctx.Err() != nil {
		return nil
	}
	rawResults, err := searchEverything(ctx, client, query, defaultMaxResults)
	if ctx.Err() != nil {
		return nil
	}
	if err != nil {
		s.setLastError(err)
		s.recordQueryStatus(query, time.Since(started).Milliseconds(), 0)
		if shouldShowCoverageHint(query) {
			return []contracts.SearchResult{coverageHintResult(query, s.Status())}
		}
		return nil
	}
	s.setLastError(nil)
	results := make([]contracts.SearchResult, 0, len(rawResults))
	seen := map[string]bool{}
	for _, raw := range rawResults {
		if ctx.Err() != nil {
			return nil
		}
		fullPath := strings.TrimSpace(raw.Path)
		if fullPath == "" {
			continue
		}
		key := strings.ToLower(filepath.Clean(fullPath))
		if seen[key] {
			continue
		}
		seen[key] = true
		results = append(results, fileToResult(raw, fileScore(raw, query)))
	}
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	s.recordQueryStatus(query, time.Since(started).Milliseconds(), len(results))
	if len(results) == 0 && shouldShowCoverageHint(query) {
		return []contracts.SearchResult{coverageHintResult(query, s.Status())}
	}
	return results
}

func searchEverything(ctx context.Context, client everythingClient, query string, maxResults uint32) ([]rawResult, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if contextualClient, ok := client.(contextEverythingClient); ok {
		return contextualClient.SearchContext(ctx, query, maxResults)
	}
	return client.Search(query, maxResults)
}

func (s *Service) LastError() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastErr
}

func (s *Service) Status() EverythingStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	dllFound := s.dllPath != "" || s.client != nil
	return EverythingStatus{
		DLLPath:         s.dllPath,
		DLLFound:        dllFound,
		Ready:           dllFound && s.lastErr == "",
		LastError:       s.lastErr,
		LastQuery:       s.lastQuery,
		LastElapsedMs:   s.lastElapsedMs,
		LastResultCount: s.lastResultCount,
		LastUpdatedAt:   s.lastUpdatedAt,
		CoverageHint:    everythingCoverageHint(dllFound, s.lastErr, s.lastQuery, s.lastResultCount),
	}
}

func (s *Service) ensureClient() (everythingClient, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil {
		return s.client, nil
	}
	client, err := newEverythingClient(s.dllPath)
	if err != nil {
		return nil, err
	}
	s.client = client
	return s.client, nil
}

func (s *Service) setLastError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil {
		s.lastErr = ""
		return
	}
	s.lastErr = err.Error()
}

func (s *Service) recordQueryStatus(query string, elapsedMs int64, resultCount int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastQuery = query
	s.lastElapsedMs = elapsedMs
	s.lastResultCount = resultCount
	s.lastUpdatedAt = time.Now().Unix()
}

func fileToResult(raw rawResult, score float64) contracts.SearchResult {
	fullPath := filepath.Clean(raw.Path)
	name := strings.TrimSpace(raw.Name)
	if name == "" {
		name = filepath.Base(fullPath)
	}
	dir := filepath.Dir(fullPath)
	extension := strings.TrimPrefix(filepath.Ext(name), ".")
	metadata := inspectFileMetadata(fullPath)
	tags := []string{"文件", "Everything"}
	icon := "file"
	if metadata.IsDirectory {
		icon = "folder"
		tags[0] = "目录"
	}
	if extension != "" {
		tags = append(tags, strings.ToUpper(extension))
	}
	meta := filePreviewMeta(fullPath, metadata)
	payload := map[string]interface{}{
		"path":        fullPath,
		"source":      "Everything SDK",
		"kind":        metadata.Kind,
		"isDirectory": metadata.IsDirectory,
	}
	if metadata.Available {
		payload["modifiedAt"] = metadata.ModifiedISO
		if !metadata.IsDirectory {
			payload["sizeBytes"] = metadata.SizeBytes
		}
	} else if metadata.Error != "" {
		payload["metadataError"] = metadata.Error
	}
	return contracts.SearchResult{
		ID:       "file-everything-" + stableID(fullPath),
		Type:     contracts.ResultFile,
		Title:    name,
		Subtitle: dir,
		Detail:   fullPath,
		Icon:     icon,
		Score:    score,
		Tags:     tags,
		Payload:  payload,
		Preview: contracts.PreviewDescriptor{
			Kind:     contracts.PreviewText,
			Title:    name,
			Subtitle: "Everything 文件搜索",
			Text:     fullPath,
			Meta:     meta,
		},
		Actions: []contracts.PreviewAction{
			{
				ID:       "open",
				Label:    "打开",
				Icon:     "open",
				Kind:     contracts.ActionOpen,
				Shortcut: "Enter",
				Payload:  map[string]interface{}{"path": fullPath},
				Feedback: &contracts.ActionFeedback{SuccessLabel: "已打开", DurationMS: 1400},
			},
			{ID: "open_parent", Label: "打开所在文件夹", Icon: "folder", Kind: contracts.ActionOpenParent, Payload: map[string]interface{}{"path": fullPath}},
			contracts.CopyAction("copy_path", "复制路径", fullPath, ""),
			contracts.RememberAction("remember", "加入记忆", "file-everything-"+stableID(fullPath)),
		},
	}
}

func inspectFileMetadata(path string) fileMetadata {
	metadata := fileMetadata{Kind: "文件"}
	info, err := os.Stat(path)
	if err != nil {
		metadata.Error = "元数据不可用"
		return metadata
	}
	metadata.Available = true
	metadata.IsDirectory = info.IsDir()
	if metadata.IsDirectory {
		metadata.Kind = "目录"
	} else {
		metadata.SizeBytes = info.Size()
		metadata.SizeLabel = formatFileSize(info.Size())
	}
	metadata.Modified = info.ModTime()
	metadata.ModifiedISO = info.ModTime().Format(time.RFC3339)
	return metadata
}

func filePreviewMeta(path string, metadata fileMetadata) []contracts.LabelValue {
	meta := []contracts.LabelValue{{Label: "类型", Value: metadata.Kind}}
	if metadata.Available {
		if metadata.SizeLabel != "" {
			meta = append(meta, contracts.LabelValue{Label: "大小", Value: metadata.SizeLabel})
		}
		if !metadata.Modified.IsZero() {
			meta = append(meta, contracts.LabelValue{Label: "修改时间", Value: metadata.Modified.Local().Format("2006-01-02 15:04")})
		}
	} else {
		meta = append(meta, contracts.LabelValue{Label: "元数据", Value: metadata.Error})
	}
	meta = append(meta,
		contracts.LabelValue{Label: "来源", Value: "Everything SDK"},
		contracts.LabelValue{Label: "路径", Value: path},
	)
	return meta
}

func formatFileSize(size int64) string {
	if size < 0 {
		return ""
	}
	const unit = 1024
	if size < unit {
		return formatInt(size) + " B"
	}
	value := float64(size)
	for _, suffix := range []string{"KiB", "MiB", "GiB", "TiB"} {
		value = value / unit
		if value < unit {
			return strings.TrimRight(strings.TrimRight(formatFloat(value), "0"), ".") + " " + suffix
		}
	}
	return strings.TrimRight(strings.TrimRight(formatFloat(value), "0"), ".") + " PiB"
}

func formatInt(value int64) string {
	return strconv.FormatInt(value, 10)
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 1, 64)
}

func fileScore(raw rawResult, query string) float64 {
	name := strings.ToLower(raw.Name)
	path := strings.ToLower(raw.Path)
	query = strings.ToLower(strings.TrimSpace(query))
	if name == query {
		return 95
	}
	if strings.HasPrefix(name, query) {
		return 88
	}
	if strings.Contains(name, query) {
		return 72
	}
	if strings.Contains(path, query) {
		return 52
	}
	return 30
}

func shouldShowCoverageHint(query string) bool {
	query = strings.TrimSpace(query)
	if query == "" {
		return false
	}
	lower := strings.ToLower(query)
	if strings.ContainsAny(query, `\/`) || strings.Contains(query, ":") {
		return true
	}
	if strings.Contains(filepath.Base(lower), ".") {
		return true
	}
	for _, suffix := range []string{".md", ".json", ".yaml", ".yml", ".go", ".ts", ".vue", ".py", ".exe", ".dll", ".png", ".jpg", ".jpeg", ".pdf", ".docx", ".xlsx"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func everythingCoverageHint(dllFound bool, lastError string, query string, resultCount int) string {
	if !dllFound {
		return "未定位到 Everything64.dll；请安装 Everything 或把 Everything64.dll 放到 Ariadne 工作目录/程序目录。"
	}
	if strings.TrimSpace(lastError) != "" {
		return "Everything 查询失败；请确认 Everything 后台服务正在运行，且 SDK DLL 与系统架构匹配。"
	}
	if strings.TrimSpace(query) != "" && resultCount == 0 && shouldShowCoverageHint(query) {
		return "Everything 可用但该文件/路径查询没有命中；请在 Everything 选项中确认目标盘或目录已加入索引，并等待索引完成。"
	}
	return ""
}

func coverageHintResult(query string, status EverythingStatus) contracts.SearchResult {
	hint := status.CoverageHint
	if hint == "" {
		hint = everythingCoverageHint(status.DLLFound, status.LastError, query, 0)
	}
	if hint == "" {
		hint = "Everything 未返回文件结果；请检查目标盘是否已加入索引。"
	}
	return contracts.SearchResult{
		ID:       "file-search-coverage-hint",
		Type:     contracts.ResultSettings,
		Title:    "Everything 未命中文件",
		Subtitle: "文件搜索索引提示",
		Detail:   hint,
		Icon:     "settings",
		Score:    38,
		Tags:     []string{"文件", "Everything", "诊断"},
		Payload: map[string]interface{}{
			"query":        query,
			"coverageHint": hint,
		},
		Preview: contracts.PreviewDescriptor{
			Kind:     contracts.PreviewText,
			Title:    "Everything 索引覆盖提示",
			Subtitle: "没有返回文件结果",
			Text:     hint,
			Meta: []contracts.LabelValue{
				{Label: "查询", Value: query},
				{Label: "最近状态", Value: status.LastQuery},
			},
		},
		Actions: []contracts.PreviewAction{
			contracts.CopyAction("copy_everything_hint", "复制提示", hint, ""),
			contracts.CopyAction("copy_everything_query", "复制查询", query, ""),
		},
	}
}

func stableID(value string) string {
	sum := sha1.Sum([]byte(strings.ToLower(value)))
	return hex.EncodeToString(sum[:])[:12]
}

func findDefaultEverythingDLL() string {
	workingDir, _ := os.Getwd()
	executablePath, _ := os.Executable()
	candidates := []string{
		findUp("Everything64.dll", workingDir, filepath.Dir(executablePath)),
		filepath.Join(workingDir, "Everything64.dll"),
		`C:\Program Files\Everything\Everything64.dll`,
		`C:\Program Files (x86)\Everything\Everything64.dll`,
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func findUp(filename string, roots ...string) string {
	seen := map[string]bool{}
	for _, root := range roots {
		for _, dir := range ancestorDirs(root, 8) {
			key := strings.ToLower(filepath.Clean(dir))
			if seen[key] {
				continue
			}
			seen[key] = true
			candidate := filepath.Join(dir, filename)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}
	return ""
}

func ancestorDirs(root string, limit int) []string {
	if root == "" {
		return nil
	}
	dirs := []string{}
	current := filepath.Clean(root)
	for i := 0; i < limit; i++ {
		dirs = append(dirs, current)
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return dirs
}
