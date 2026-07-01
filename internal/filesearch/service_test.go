package filesearch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ariadne/internal/contracts"
)

type countingIndexBuilder struct {
	called int
}

func (b *countingIndexBuilder) Build(ctx context.Context) (IndexBuildResult, error) {
	b.called++
	return IndexBuildResult{}, nil
}

type failingPrivilegeIndexBuilder struct{}

func (failingPrivilegeIndexBuilder) Build(ctx context.Context) (IndexBuildResult, error) {
	return IndexBuildResult{
		RequiresElevation: true,
		Elevated:          false,
		Errors:            []string{"需要以管理员身份运行 Ariadne 后才能读取 NTFS USN/MFT"},
	}, errors.New("文件索引需要管理员权限")
}

type fakeIndex struct {
	count   int
	volumes []string
	results []rawResult
}

func (idx fakeIndex) Search(query string, limit int) []rawResult {
	return append([]rawResult(nil), idx.results...)
}

func (idx fakeIndex) Count() int {
	return idx.count
}

func (idx fakeIndex) Volumes() []string {
	return append([]string(nil), idx.volumes...)
}

func (idx fakeIndex) Close() {}

type fullDiskIndexBuilder struct{}

func (fullDiskIndexBuilder) Build(ctx context.Context) (IndexBuildResult, error) {
	index := fakeIndex{
		count:   250001,
		volumes: []string{`P:\`},
		results: []rawResult{{Name: "target.xlsx", Path: `P:\workspace\target.xlsx`}},
	}
	return IndexBuildResult{
		Index:        index,
		IndexedCount: index.Count(),
		Volumes:      index.Volumes(),
		Elevated:     true,
	}, nil
}

type cachedRefreshIndexBuilder struct {
	cached  fileIndex
	started chan struct{}
	release chan struct{}
}

func (b cachedRefreshIndexBuilder) CachedIndex(ctx context.Context) (IndexBuildResult, error) {
	return IndexBuildResult{
		Index:        b.cached,
		IndexedCount: b.cached.Count(),
		Volumes:      b.cached.Volumes(),
	}, nil
}

func (b cachedRefreshIndexBuilder) Build(ctx context.Context) (IndexBuildResult, error) {
	close(b.started)
	select {
	case <-b.release:
		return IndexBuildResult{}, nil
	case <-ctx.Done():
		return IndexBuildResult{}, ctx.Err()
	}
}

type blockingIndexBuilder struct {
	started chan struct{}
	release chan struct{}
}

func (b blockingIndexBuilder) Build(ctx context.Context) (IndexBuildResult, error) {
	close(b.started)
	select {
	case <-b.release:
		return IndexBuildResult{}, nil
	case <-ctx.Done():
		return IndexBuildResult{}, ctx.Err()
	}
}

func TestStartIndexingMarksStatusAsIndexing(t *testing.T) {
	builder := blockingIndexBuilder{started: make(chan struct{}), release: make(chan struct{})}
	service := NewServiceWithIndexer(builder)
	t.Cleanup(func() { close(builder.release) })

	status := service.StartIndexing()
	<-builder.started

	if !status.Indexing || status.IndexStartedAt == 0 {
		t.Fatalf("StartIndexing should expose indexing state immediately: %#v", status)
	}
}

func TestStartIndexingReportsAdminRequirementWithoutReadyIndex(t *testing.T) {
	service := NewServiceWithIndexer(failingPrivilegeIndexBuilder{})
	service.StartIndexing()

	status := waitForFileIndexStatus(t, service, func(status FileIndexStatus) bool {
		return !status.Indexing && status.LastError != ""
	})

	if !status.RequiresAdmin || status.Elevated || status.Ready {
		t.Fatalf("permission failure should be explicit and degraded: %#v", status)
	}
	if !strings.Contains(status.CoverageHint, "搜索服务未安装") {
		t.Fatalf("coverage hint should tell the user how to recover through the search service: %#v", status)
	}
	results := service.Search("README.md")
	if len(results) != 1 || results[0].ID != "file-search-coverage-hint" {
		t.Fatalf("file-like search should return an actionable permission hint, got %#v", results)
	}
	if !hasActionID(results[0], "install_file_search_service") {
		t.Fatalf("permission hint should expose search service install action: %#v", results[0].Actions)
	}
	action, ok := actionByID(results[0], "install_file_search_service")
	if !ok {
		t.Fatalf("missing search service install action: %#v", results[0].Actions)
	}
	if action.Label != "安装搜索服务" || action.Kind != contracts.ActionRun {
		t.Fatalf("search service install action should be a direct product action: %#v", action)
	}
	results = service.Search("工作日历")
	if len(results) != 1 || !strings.Contains(results[0].Detail, "搜索服务未安装") {
		t.Fatalf("plain filename search should still expose permission recovery, got %#v", results)
	}
}

func TestSearchWhileIndexingShowsProgressHintForPlainFilename(t *testing.T) {
	builder := blockingIndexBuilder{started: make(chan struct{}), release: make(chan struct{})}
	service := NewServiceWithIndexer(builder)
	t.Cleanup(func() { close(builder.release) })

	results := service.Search("工作日历")
	<-builder.started

	if len(results) != 1 || !strings.Contains(results[0].Detail, "正在建立") {
		t.Fatalf("search should expose indexing progress for plain filenames, got %#v", results)
	}
}

func TestStartIndexingUsesFullDiskIndexWithoutCoverageLimit(t *testing.T) {
	service := NewServiceWithIndexer(fullDiskIndexBuilder{})
	service.StartIndexing()

	status := waitForFileIndexStatus(t, service, func(status FileIndexStatus) bool {
		return !status.Indexing && status.Ready
	})

	if status.IndexedCount != 250001 || status.CoverageHint != "" {
		t.Fatalf("full disk index should not be reported as capped or degraded: %#v", status)
	}
	results := service.Search("target")
	if len(results) != 1 || results[0].Title != "target.xlsx" {
		t.Fatalf("search should use the full disk-backed index: %#v", results)
	}
}

func TestSearchUsesCachedLineIndexWhileRefreshing(t *testing.T) {
	builder := cachedRefreshIndexBuilder{
		cached: fakeIndex{
			count:   1,
			volumes: []string{`P:\`},
			results: []rawResult{{Name: "target.xlsx", Path: `P:\workspace\target.xlsx`}},
		},
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	service := NewServiceWithIndexer(builder)
	t.Cleanup(func() { close(builder.release) })

	results := service.Search("target")
	<-builder.started
	status := service.Status()

	if len(results) != 1 || results[0].Title != "target.xlsx" {
		t.Fatalf("cached index should be searchable immediately, got %#v", results)
	}
	if !status.Ready || !status.Indexing || status.IndexedCount != 1 {
		t.Fatalf("cached index should be ready while refresh continues: %#v", status)
	}
}

func TestChangedPathIsSearchableImmediately(t *testing.T) {
	service := NewServiceWithIndex([]rawResult{{Name: "README.md", Path: `P:\workspace\README.md`}})

	service.applyChangedPath(rawResult{Name: "搜索测试.txt", Path: `P:\桌面\搜索测试.txt`})

	results := service.Search("搜索测试")
	if len(results) != 1 || results[0].Detail != `P:\桌面\搜索测试.txt` {
		t.Fatalf("changed path should be searchable immediately, got %#v", results)
	}
	status := service.Status()
	if status.IndexedCount != 2 {
		t.Fatalf("changed path should update indexed count, got %#v", status)
	}
}

func TestSearchFiltersExcludedFolder(t *testing.T) {
	service := NewServiceWithIndex([]rawResult{
		{Name: "搜索测试.txt.lnk", Path: `C:\Users\luwei\AppData\Roaming\Microsoft\Windows\Recent\搜索测试.txt.lnk`},
		{Name: "搜索测试.txt", Path: `C:\Users\luwei\OneDrive\桌面\搜索测试.txt`},
	})
	service.ApplyPolicy(FileSearchPolicy{ExcludeFolders: []string{`C:\Users\luwei\AppData\Roaming\Microsoft\Windows\Recent`}})

	results := service.Search("搜索测试")

	if len(results) != 1 || results[0].Title != "搜索测试.txt" {
		t.Fatalf("excluded Recent shortcut should not be returned, got %#v", results)
	}
}

func TestSearchFiltersExcludedRegex(t *testing.T) {
	service := NewServiceWithIndex([]rawResult{
		{Name: "keep.txt", Path: `P:\workspace\keep.txt`},
		{Name: "drop.tmp", Path: `P:\workspace\drop.tmp`},
	})
	service.ApplyPolicy(FileSearchPolicy{ExcludePatterns: []string{`\.tmp$`}})

	results := service.Search("p:\\workspace")

	if len(results) != 1 || results[0].Title != "keep.txt" {
		t.Fatalf("regex-excluded file should not be returned, got %#v", results)
	}
}

func TestInvalidExcludeRegexIsReported(t *testing.T) {
	service := NewServiceWithIndex([]rawResult{{Name: "README.md", Path: `P:\workspace\README.md`}})
	service.ApplyPolicy(FileSearchPolicy{ExcludePatterns: []string{"["}})

	status := service.Status()

	if len(status.PolicyErrors) != 1 || !strings.Contains(status.PolicyErrors[0], "missing closing") {
		t.Fatalf("invalid regex should be exposed in status, got %#v", status.PolicyErrors)
	}
}

func TestSearchReturnsAriadneIndexedFileResults(t *testing.T) {
	service := NewServiceWithIndex([]rawResult{
		{Name: "README.md", Path: `P:\workspace\glwlg\app\Ariadne\README.md`},
	})

	results := service.Search("readme")

	if len(results) != 1 {
		t.Fatalf("expected one file result, got %#v", results)
	}
	result := results[0]
	if result.Type != contracts.ResultFile {
		t.Fatalf("expected file result, got %s", result.Type)
	}
	if result.Actions[0].Label != "打开" {
		t.Fatalf("expected generic open action, got %#v", result.Actions[0])
	}
	if !hasActionKind(result, contracts.ActionOpenParent) {
		t.Fatal("Ariadne indexed file result should expose open_parent")
	}
	if source := result.Payload["source"]; source != fileIndexProvider {
		t.Fatalf("expected Ariadne file index source, got %#v", source)
	}
	if err := contracts.ValidateActionSurface(result); err != nil {
		t.Fatalf("invalid action surface: %v", err)
	}
}

func TestFileResultIncludesFilesystemMetadata(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "sample.log")
	if err := os.WriteFile(filePath, []byte(strings.Repeat("x", 1536)), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	modified := time.Date(2026, 6, 14, 19, 30, 0, 0, time.Local)
	if err := os.Chtimes(filePath, modified, modified); err != nil {
		t.Fatalf("set temp file time: %v", err)
	}
	service := NewServiceWithIndex([]rawResult{{Name: "sample.log", Path: filePath}})

	results := service.Search("sample.log")

	if len(results) != 1 {
		t.Fatalf("expected one file result, got %#v", results)
	}
	result := results[0]
	if result.Icon != "file" {
		t.Fatalf("expected file icon, got %q", result.Icon)
	}
	if metaValue(result, "大小") != "1.5 KiB" {
		t.Fatalf("expected formatted size metadata, got %#v", result.Preview.Meta)
	}
	if metaValue(result, "修改时间") != "2026-06-14 19:30" {
		t.Fatalf("expected modified time metadata, got %#v", result.Preview.Meta)
	}
	if result.Payload["sizeBytes"] != int64(1536) {
		t.Fatalf("expected sizeBytes payload, got %#v", result.Payload)
	}
	if result.Payload["modifiedAt"] == "" {
		t.Fatalf("expected modifiedAt payload, got %#v", result.Payload)
	}
}

func TestDirectoryResultUsesFolderMetadata(t *testing.T) {
	dir := t.TempDir()
	service := NewServiceWithIndex([]rawResult{{Name: filepath.Base(dir), Path: dir, IsDirectory: true}})

	results := service.Search(filepath.Base(dir))

	if len(results) != 1 {
		t.Fatalf("expected one directory result, got %#v", results)
	}
	result := results[0]
	if result.Icon != "folder" || result.Tags[0] != "目录" {
		t.Fatalf("expected directory result decoration, got icon=%q tags=%#v", result.Icon, result.Tags)
	}
	if metaValue(result, "类型") != "目录" {
		t.Fatalf("expected directory metadata, got %#v", result.Preview.Meta)
	}
	if _, ok := result.Payload["sizeBytes"]; ok {
		t.Fatalf("directory payload should not expose file size bytes: %#v", result.Payload)
	}
}

func TestSearchSkipsShortQueries(t *testing.T) {
	service := NewServiceWithIndex([]rawResult{{Name: "a.txt", Path: `P:\a.txt`}})

	if results := service.Search("a"); len(results) != 0 {
		t.Fatalf("short query should not hit file index: %#v", results)
	}
}

func TestSearchContextSkipsIndexingWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	builder := &countingIndexBuilder{}
	service := NewServiceWithIndexer(builder)

	results := service.SearchContext(ctx, "readme")

	if len(results) != 0 {
		t.Fatalf("cancelled search should return no results, got %#v", results)
	}
	if builder.called != 0 {
		t.Fatalf("cancelled search should not start indexing, called %d time(s)", builder.called)
	}
	status := service.Status()
	if status.LastQuery != "" || status.LastUpdatedAt != 0 || status.LastError != "" {
		t.Fatalf("cancelled search should not update file index diagnostics: %#v", status)
	}
}

func TestStatusReportsLastSuccessfulFileIndexQuery(t *testing.T) {
	service := NewServiceWithIndex([]rawResult{
		{Name: "README.md", Path: `P:\workspace\glwlg\app\Ariadne\README.md`},
		{Name: "main.go", Path: `P:\workspace\glwlg\app\Ariadne\main.go`},
	})

	service.Search("readme")
	status := service.Status()

	if !status.Ready || status.Provider != fileIndexProvider || status.LastError != "" {
		t.Fatalf("file index should be ready after an injected query: %#v", status)
	}
	if status.LastQuery != "readme" || status.LastResultCount != 1 || status.LastUpdatedAt == 0 {
		t.Fatalf("file index status should record query details: %#v", status)
	}
	if status.IndexedCount != 2 {
		t.Fatalf("file index status should expose indexed count: %#v", status)
	}
}

func TestFileLikeZeroResultsDoNotReturnCoverageHintWhenIndexReady(t *testing.T) {
	service := NewServiceWithIndex([]rawResult{{Name: "README.md", Path: `P:\workspace\README.md`}})

	results := service.Search("P:\\workspace\\missing.json")

	if len(results) != 0 {
		t.Fatalf("ready index should not add diagnostic result for an ordinary miss, got %#v", results)
	}
	status := service.Status()
	if status.CoverageHint != "" {
		t.Fatalf("ready index miss should not set coverage hint: %#v", status)
	}
}

func TestReadyIndexServiceWarningDoesNotPolluteNoMatchResults(t *testing.T) {
	service := NewServiceWithIndex([]rawResult{{Name: "README.md", Path: `P:\workspace\README.md`}})
	service.mu.Lock()
	service.requiresAdmin = true
	service.elevated = false
	service.mu.Unlock()

	results := service.Search("explorer.ex")

	if len(results) != 0 {
		t.Fatalf("service warning should stay in settings and not appear as a search result: %#v", results)
	}
	status := service.Status()
	if !strings.Contains(status.CoverageHint, "搜索服务未运行") {
		t.Fatalf("status should still expose service warning for settings: %#v", status)
	}
}

func TestOrdinaryZeroResultsDoNotReturnCoverageHint(t *testing.T) {
	service := NewServiceWithIndex([]rawResult{{Name: "README.md", Path: `P:\workspace\README.md`}})

	results := service.Search("gateway")

	if len(results) != 0 {
		t.Fatalf("ordinary empty file search should not add diagnostic result: %#v", results)
	}
	if status := service.Status(); status.CoverageHint != "" {
		t.Fatalf("ordinary empty query should not set coverage hint: %#v", status)
	}
}

func hasActionKind(result contracts.SearchResult, kind contracts.PreviewActionKind) bool {
	for _, action := range result.Actions {
		if action.Kind == kind {
			return true
		}
	}
	return false
}

func hasActionID(result contracts.SearchResult, id string) bool {
	_, ok := actionByID(result, id)
	return ok
}

func actionByID(result contracts.SearchResult, id string) (contracts.PreviewAction, bool) {
	for _, action := range result.Actions {
		if action.ID == id {
			return action, true
		}
	}
	return contracts.PreviewAction{}, false
}

func waitForFileIndexStatus(t *testing.T, service *Service, done func(FileIndexStatus) bool) FileIndexStatus {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var status FileIndexStatus
	for time.Now().Before(deadline) {
		status = service.Status()
		if done(status) {
			return status
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("file index status did not reach expected state: %#v", status)
	return status
}

func metaValue(result contracts.SearchResult, label string) string {
	for _, item := range result.Preview.Meta {
		if item.Label == label {
			return item.Value
		}
	}
	return ""
}
