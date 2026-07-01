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

const (
	defaultMaxResults uint32 = 24
	fileIndexProvider        = "Ariadne USN/MFT"
)

type rawResult struct {
	Name        string
	Path        string
	IsDirectory bool
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

type IndexBuildResult struct {
	Entries           []rawResult
	Index             fileIndex
	IndexedCount      int
	Volumes           []string
	Errors            []string
	RequiresElevation bool
	Elevated          bool
}

type fileIndex interface {
	Search(query string, limit int) []rawResult
	Count() int
	Volumes() []string
	Close()
}

type indexBuilder interface {
	Build(ctx context.Context) (IndexBuildResult, error)
}

type cachedIndexBuilder interface {
	CachedIndex(ctx context.Context) (IndexBuildResult, error)
}

type changeWatchingIndexBuilder interface {
	WatchChanges(ctx context.Context, volumes []string, emit func(rawResult)) error
}

type mutableFileIndex interface {
	AppendRawResults(entries []rawResult) (int, error)
}

type policyAwareIndexBuilder interface {
	ApplyPolicy(policy FileSearchPolicy)
}

type Service struct {
	mu              sync.Mutex
	builder         indexBuilder
	index           fileIndex
	indexStarted    bool
	indexing        bool
	indexStartedAt  int64
	indexFinishedAt int64
	indexErrors     []string
	watchCancel     context.CancelFunc
	watching        bool
	policy          FileSearchPolicy
	filter          fileSearchFilter
	requiresAdmin   bool
	elevated        bool
	volumeCount     int
	indexedCount    int
	lastErr         string
	lastQuery       string
	lastElapsedMs   int64
	lastResultCount int
	lastUpdatedAt   int64
}

type FileIndexStatus struct {
	DLLPath         string   `json:"dllPath,omitempty"`
	DLLFound        bool     `json:"dllFound"`
	Ready           bool     `json:"ready"`
	Provider        string   `json:"provider,omitempty"`
	Indexing        bool     `json:"indexing"`
	IndexedCount    int      `json:"indexedCount"`
	VolumeCount     int      `json:"volumeCount"`
	RequiresAdmin   bool     `json:"requiresAdmin"`
	Elevated        bool     `json:"elevated"`
	IndexStartedAt  int64    `json:"indexStartedAt,omitempty"`
	IndexFinishedAt int64    `json:"indexFinishedAt,omitempty"`
	LastError       string   `json:"lastError,omitempty"`
	LastQuery       string   `json:"lastQuery,omitempty"`
	LastElapsedMs   int64    `json:"lastElapsedMs"`
	LastResultCount int      `json:"lastResultCount"`
	LastUpdatedAt   int64    `json:"lastUpdatedAt,omitempty"`
	CoverageHint    string   `json:"coverageHint,omitempty"`
	PolicyErrors    []string `json:"policyErrors,omitempty"`
}

type memoryIndex struct {
	mu      sync.RWMutex
	entries []rawResult
	volumes []string
}

type scoredRawResult struct {
	raw   rawResult
	score float64
}

func NewService() *Service {
	return NewServiceWithIndexer(newDefaultIndexBuilder())
}

func NewServiceWithIndexer(builder indexBuilder) *Service {
	if builder == nil {
		builder = noopIndexBuilder{}
	}
	policy := DefaultFileSearchPolicy()
	service := &Service{builder: builder, policy: policy, filter: newFileSearchFilter(policy)}
	service.applyPolicyToBuilder(policy)
	return service
}

func NewServiceWithIndex(entries []rawResult) *Service {
	index := newMemoryIndex(entries, nil)
	policy := DefaultFileSearchPolicy()
	return &Service{
		builder:         noopIndexBuilder{},
		index:           index,
		indexStarted:    true,
		indexFinishedAt: time.Now().Unix(),
		indexedCount:    index.Count(),
		volumeCount:     len(index.Volumes()),
		elevated:        true,
		policy:          policy,
		filter:          newFileSearchFilter(policy),
	}
}

func (s *Service) Search(query string) []contracts.SearchResult {
	return s.SearchContext(context.Background(), query)
}

func (s *Service) StartIndexing() FileIndexStatus {
	s.startIndexing()
	return s.Status()
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
	s.startIndexing()
	index, status := s.currentIndexAndStatus(query)
	if ctx.Err() != nil {
		return nil
	}
	if index == nil && s.tryLoadCachedIndex(ctx) {
		index, status = s.currentIndexAndStatus(query)
	}
	if index == nil {
		s.recordQueryStatus(query, time.Since(started).Milliseconds(), 0)
		if shouldReturnCoverageHintResult(status) {
			return []contracts.SearchResult{coverageHintResult(query, status)}
		}
		return nil
	}

	rawResults := index.Search(query, int(defaultMaxResults)*4)
	if ctx.Err() != nil {
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
		if s.excludesPath(fullPath) {
			continue
		}
		seen[key] = true
		results = append(results, fileToResult(raw, fileScore(raw, query)))
		if len(results) >= int(defaultMaxResults) {
			break
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	s.recordQueryStatus(query, time.Since(started).Milliseconds(), len(results))
	return results
}

func (s *Service) ApplyPolicy(policy FileSearchPolicy) {
	policy = NormalizeFileSearchPolicy(policy)
	filter := newFileSearchFilter(policy)
	s.mu.Lock()
	s.policy = policy
	s.filter = filter
	builder := s.builder
	s.mu.Unlock()
	if setter, ok := builder.(policyAwareIndexBuilder); ok {
		setter.ApplyPolicy(policy)
	}
}

func (s *Service) LastError() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastErr
}

func (s *Service) Status() FileIndexStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statusLocked("")
}

func (s *Service) Close() {
	s.mu.Lock()
	index := s.index
	s.index = nil
	watchCancel := s.watchCancel
	s.watchCancel = nil
	s.watching = false
	s.mu.Unlock()
	if watchCancel != nil {
		watchCancel()
	}
	if index != nil {
		index.Close()
	}
}

func (s *Service) startIndexing() {
	s.mu.Lock()
	if s.indexStarted || s.index != nil {
		s.mu.Unlock()
		return
	}
	s.indexStarted = true
	s.indexing = true
	s.indexStartedAt = time.Now().Unix()
	builder := s.builder
	s.mu.Unlock()

	if cachedBuilder, ok := builder.(cachedIndexBuilder); ok {
		if result, err := cachedBuilder.CachedIndex(context.Background()); err == nil && (result.Index != nil || len(result.Entries) > 0) {
			s.mu.Lock()
			s.applyIndexBuildResultLocked(result, nil)
			s.indexing = true
			volumes := append([]string(nil), result.Volumes...)
			if len(volumes) == 0 && s.index != nil {
				volumes = s.index.Volumes()
			}
			s.mu.Unlock()
			s.startChangeWatcher(builder, volumes)
		}
	}

	go func() {
		result, err := builder.Build(context.Background())
		s.mu.Lock()
		s.indexing = false
		s.indexFinishedAt = time.Now().Unix()
		s.applyIndexBuildResultLocked(result, err)
		volumes := append([]string(nil), result.Volumes...)
		if len(volumes) == 0 && s.index != nil {
			volumes = s.index.Volumes()
		}
		s.mu.Unlock()
		s.startChangeWatcher(builder, volumes)
	}()
}

func (s *Service) tryLoadCachedIndex(ctx context.Context) bool {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return false
	}
	s.mu.Lock()
	if s.index != nil {
		s.mu.Unlock()
		return true
	}
	builder := s.builder
	s.mu.Unlock()
	cachedBuilder, ok := builder.(cachedIndexBuilder)
	if !ok {
		return false
	}
	result, err := cachedBuilder.CachedIndex(ctx)
	if err != nil || (result.Index == nil && len(result.Entries) == 0) {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.index != nil {
		if result.Index != nil {
			result.Index.Close()
		}
		return true
	}
	s.applyIndexBuildResultLocked(result, nil)
	return s.index != nil
}

func (s *Service) startChangeWatcher(builder indexBuilder, volumes []string) {
	watcher, ok := builder.(changeWatchingIndexBuilder)
	if !ok || len(volumes) == 0 {
		return
	}
	s.mu.Lock()
	if s.watching || s.index == nil {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.watchCancel = cancel
	s.watching = true
	s.mu.Unlock()

	go func() {
		err := watcher.WatchChanges(ctx, volumes, s.applyChangedPath)
		if err != nil && ctx.Err() == nil {
			s.mu.Lock()
			s.lastErr = err.Error()
			s.indexErrors = append(s.indexErrors, err.Error())
			s.watching = false
			s.watchCancel = nil
			s.mu.Unlock()
			return
		}
		s.mu.Lock()
		s.watchCancel = nil
		s.watching = false
		s.mu.Unlock()
	}()
}

func (s *Service) applyChangedPath(raw rawResult) {
	raw, ok := normalizeLineFileRawResult(raw)
	if !ok {
		return
	}
	if s.excludesPath(raw.Path) {
		return
	}
	s.mu.Lock()
	index := s.index
	mutable, ok := index.(mutableFileIndex)
	s.mu.Unlock()
	if !ok || mutable == nil {
		return
	}
	added, err := mutable.AppendRawResults([]rawResult{raw})
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.lastErr = err.Error()
		s.indexErrors = append(s.indexErrors, err.Error())
		return
	}
	if added > 0 {
		s.indexedCount += added
		s.lastErr = ""
	}
}

func (s *Service) applyIndexBuildResultLocked(result IndexBuildResult, err error) {
	s.indexErrors = append([]string(nil), result.Errors...)
	if result.RequiresElevation || result.Elevated {
		s.requiresAdmin = result.RequiresElevation
		s.elevated = result.Elevated
	}
	if len(result.Volumes) > 0 {
		s.volumeCount = len(result.Volumes)
	}
	index := result.Index
	if index == nil && len(result.Entries) > 0 {
		index = newMemoryIndex(result.Entries, result.Volumes)
	}
	if err != nil {
		if errText := strings.TrimSpace(err.Error()); errText != "" {
			s.lastErr = errText
			s.indexErrors = append(s.indexErrors, errText)
		}
		if index == nil {
			return
		}
	}
	if index == nil {
		if result.IndexedCount > 0 {
			s.indexedCount = result.IndexedCount
		}
		return
	}
	previous := s.index
	s.index = index
	if previous != nil && previous != index {
		previous.Close()
	}
	if result.IndexedCount > 0 {
		s.indexedCount = result.IndexedCount
	} else {
		s.indexedCount = index.Count()
	}
	if s.volumeCount == 0 {
		s.volumeCount = len(index.Volumes())
	}
	if s.indexedCount > 0 && err == nil {
		s.lastErr = ""
	}
}

func (s *Service) currentIndexAndStatus(query string) (fileIndex, FileIndexStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.index, s.statusLocked(query)
}

func (s *Service) statusLocked(query string) FileIndexStatus {
	if strings.TrimSpace(query) == "" {
		query = s.lastQuery
	}
	ready := s.index != nil && s.indexedCount > 0
	status := FileIndexStatus{
		Ready:           ready,
		Provider:        fileIndexProvider,
		Indexing:        s.indexing,
		IndexedCount:    s.indexedCount,
		VolumeCount:     s.volumeCount,
		RequiresAdmin:   s.requiresAdmin,
		Elevated:        s.elevated,
		IndexStartedAt:  s.indexStartedAt,
		IndexFinishedAt: s.indexFinishedAt,
		LastError:       s.lastErr,
		LastQuery:       s.lastQuery,
		LastElapsedMs:   s.lastElapsedMs,
		LastResultCount: s.lastResultCount,
		LastUpdatedAt:   s.lastUpdatedAt,
		PolicyErrors:    s.filter.Errors(),
	}
	status.CoverageHint = fileIndexCoverageHint(status, append([]string(nil), s.indexErrors...), query)
	return status
}

func (s *Service) excludesPath(path string) bool {
	s.mu.Lock()
	filter := s.filter
	s.mu.Unlock()
	return filter.Excludes(path)
}

func (s *Service) applyPolicyToBuilder(policy FileSearchPolicy) {
	if setter, ok := s.builder.(policyAwareIndexBuilder); ok {
		setter.ApplyPolicy(policy)
	}
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

func newMemoryIndex(entries []rawResult, volumes []string) *memoryIndex {
	cleaned := make([]rawResult, 0, len(entries))
	seen := map[string]bool{}
	for _, entry := range entries {
		entry.Name = strings.TrimSpace(entry.Name)
		entry.Path = filepath.Clean(strings.TrimSpace(entry.Path))
		if entry.Path == "." || entry.Path == "" {
			continue
		}
		if entry.Name == "" {
			entry.Name = filepath.Base(entry.Path)
		}
		key := strings.ToLower(entry.Path)
		if seen[key] {
			continue
		}
		seen[key] = true
		cleaned = append(cleaned, entry)
	}
	index := &memoryIndex{
		entries: cleaned,
		volumes: append([]string(nil), volumes...),
	}
	return index
}

func (s *memoryIndex) Search(query string, limit int) []rawResult {
	if s == nil || limit <= 0 {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	normalized := strings.ToLower(strings.TrimSpace(query))
	if len([]rune(normalized)) < 2 {
		return nil
	}
	scored := make([]scoredRawResult, 0, minInt(limit*4, len(s.entries)))
	for _, entry := range s.entries {
		lowerName := strings.ToLower(entry.Name)
		lowerPath := strings.ToLower(entry.Path)
		if !strings.Contains(lowerName, normalized) && !strings.Contains(lowerPath, normalized) {
			continue
		}
		item := scoredRawResult{raw: entry, score: fileScoreLower(lowerName, lowerPath, normalized)}
		if len(scored) < limit*4 {
			scored = append(scored, item)
			continue
		}
		replaceIndex := -1
		replaceScore := item.score
		for index, existing := range scored {
			if existing.score < replaceScore {
				replaceIndex = index
				replaceScore = existing.score
			}
		}
		if replaceIndex >= 0 {
			scored[replaceIndex] = item
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return strings.ToLower(scored[i].raw.Path) < strings.ToLower(scored[j].raw.Path)
		}
		return scored[i].score > scored[j].score
	})
	if len(scored) > limit {
		scored = scored[:limit]
	}
	results := make([]rawResult, 0, len(scored))
	for _, item := range scored {
		results = append(results, item.raw)
	}
	return results
}

func (s *memoryIndex) Count() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

func (s *memoryIndex) Volumes() []string {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string(nil), s.volumes...)
}

func (s *memoryIndex) Close() {}

func (s *memoryIndex) AppendRawResults(entries []rawResult) (int, error) {
	if s == nil || len(entries) == 0 {
		return 0, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[string]bool{}
	for _, existing := range s.entries {
		seen[strings.ToLower(filepath.Clean(existing.Path))] = true
	}
	added := 0
	for _, entry := range entries {
		entry, ok := normalizeLineFileRawResult(entry)
		if !ok {
			continue
		}
		key := strings.ToLower(filepath.Clean(entry.Path))
		if seen[key] {
			continue
		}
		seen[key] = true
		s.entries = append(s.entries, entry)
		volume := filepath.VolumeName(entry.Path)
		if volume != "" {
			volume += string(filepath.Separator)
			hasVolume := false
			for _, existing := range s.volumes {
				if strings.EqualFold(existing, volume) {
					hasVolume = true
					break
				}
			}
			if !hasVolume {
				s.volumes = append(s.volumes, volume)
			}
		}
		added++
	}
	return added, nil
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
	if !metadata.Available && raw.IsDirectory {
		metadata.Kind = "目录"
		metadata.IsDirectory = true
	}
	tags := []string{"文件", "Ariadne 索引"}
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
		"source":      fileIndexProvider,
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
		ID:       "file-ariadne-" + stableID(fullPath),
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
			Subtitle: "Ariadne 文件索引",
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
			contracts.RememberAction("remember", "加入记忆", "file-ariadne-"+stableID(fullPath)),
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
		contracts.LabelValue{Label: "来源", Value: fileIndexProvider},
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
	return fileScoreLower(name, path, query)
}

func fileScoreLower(name string, path string, query string) float64 {
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

func shouldReturnCoverageHintResult(status FileIndexStatus) bool {
	return !status.Ready || status.Indexing
}

func isPathLikeQuery(query string) bool {
	return strings.ContainsAny(query, `\/`) || strings.Contains(query, ":")
}

func fileIndexCoverageHint(status FileIndexStatus, indexErrors []string, query string) string {
	if status.RequiresAdmin && !status.Elevated && !status.Ready {
		return "搜索服务未安装。请安装搜索服务后再搜索本机文件。"
	}
	if status.IndexStartedAt == 0 && !status.Ready && !status.Indexing {
		return "Ariadne 文件索引尚未启动；应用启动后会自动建立，也可通过首次文件搜索触发。"
	}
	if status.Indexing && !status.Ready {
		return "Ariadne 文件索引正在建立；完成后会返回本机文件结果。"
	}
	if strings.TrimSpace(status.LastError) != "" && status.IndexedCount == 0 {
		return "Ariadne 文件索引不可用：" + status.LastError
	}
	if !status.Ready {
		return "Ariadne 文件索引尚未就绪；NTFS USN/MFT 读取可能需要更高权限。"
	}
	if status.Indexing {
		return "Ariadne 文件索引可用；后台正在刷新。"
	}
	if status.RequiresAdmin && !status.Elevated {
		return "搜索服务未运行；当前索引可搜索，后台刷新需安装搜索服务。"
	}
	if len(indexErrors) > 0 {
		return "Ariadne 文件索引已完成，部分磁盘未纳入索引：" + strings.Join(indexErrors, "；")
	}
	return ""
}

func coverageHintResult(query string, status FileIndexStatus) contracts.SearchResult {
	hint := status.CoverageHint
	if hint == "" {
		hint = fileIndexCoverageHint(status, nil, query)
	}
	if hint == "" {
		hint = "Ariadne 文件索引没有返回文件结果。"
	}
	title := "文件索引未命中"
	subtitle := "文件搜索索引提示"
	previewSubtitle := "没有返回文件结果"
	if status.RequiresAdmin && !status.Elevated && !status.Ready {
		title = "搜索服务未安装"
		subtitle = "安装后可搜索本机文件"
		previewSubtitle = "等待安装"
	} else if status.RequiresAdmin && !status.Elevated {
		title = "文件索引可用"
		subtitle = "搜索服务未运行"
		previewSubtitle = "可搜索"
	} else if status.Indexing && !status.Ready {
		title = "文件索引正在建立"
		subtitle = "索引完成后会返回本机文件结果"
		previewSubtitle = "索引中"
	} else if status.Indexing {
		title = "文件索引可用"
		subtitle = "后台正在刷新"
		previewSubtitle = "可搜索"
	} else if !status.Ready {
		title = "文件索引尚未就绪"
		subtitle = "文件搜索索引提示"
		previewSubtitle = "尚未就绪"
	}
	return contracts.SearchResult{
		ID:       "file-search-coverage-hint",
		Type:     contracts.ResultSettings,
		Title:    title,
		Subtitle: subtitle,
		Detail:   hint,
		Icon:     "settings",
		Score:    38,
		Tags:     []string{"文件", "索引", "诊断"},
		Payload: map[string]interface{}{
			"query":        query,
			"coverageHint": hint,
		},
		Preview: contracts.PreviewDescriptor{
			Kind:     contracts.PreviewText,
			Title:    title,
			Subtitle: previewSubtitle,
			Text:     hint,
			Meta: []contracts.LabelValue{
				{Label: "查询", Value: query},
				{Label: "最近状态", Value: status.LastQuery},
			},
		},
		Actions: coverageHintActions(hint, query, status),
	}
}

func coverageHintActions(hint string, query string, status FileIndexStatus) []contracts.PreviewAction {
	actions := []contracts.PreviewAction{
		contracts.CopyAction("copy_file_index_hint", "复制提示", hint, ""),
	}
	if strings.TrimSpace(query) != "" {
		actions = append(actions, contracts.CopyAction("copy_file_index_query", "复制查询", query, ""))
	}
	if status.RequiresAdmin && !status.Elevated {
		actions = append([]contracts.PreviewAction{installFileSearchServiceAction()}, actions...)
	}
	return actions
}

func installFileSearchServiceAction() contracts.PreviewAction {
	return contracts.PreviewAction{
		ID:       "install_file_search_service",
		Label:    "安装搜索服务",
		Icon:     "shield",
		Kind:     contracts.ActionRun,
		Feedback: &contracts.ActionFeedback{SuccessLabel: "正在安装搜索服务", DurationMS: 1800},
		Payload: map[string]interface{}{
			"source": "file_search",
		},
	}
}

func stableID(value string) string {
	sum := sha1.Sum([]byte(strings.ToLower(value)))
	return hex.EncodeToString(sum[:])[:12]
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

type noopIndexBuilder struct{}

func (noopIndexBuilder) Build(ctx context.Context) (IndexBuildResult, error) {
	return IndexBuildResult{}, nil
}
