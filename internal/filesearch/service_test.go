package filesearch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ariadne/internal/contracts"
)

type fakeEverythingClient struct {
	results []rawResult
	err     error
}

func (f fakeEverythingClient) Search(query string, maxResults uint32) ([]rawResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.results, nil
}

type countingEverythingClient struct {
	results []rawResult
	called  int
}

func (f *countingEverythingClient) Search(query string, maxResults uint32) ([]rawResult, error) {
	f.called++
	return f.results, nil
}

type contextAwareEverythingClient struct {
	results        []rawResult
	contextCalled  int
	fallbackCalled int
}

func (f *contextAwareEverythingClient) Search(query string, maxResults uint32) ([]rawResult, error) {
	f.fallbackCalled++
	return f.results, nil
}

func (f *contextAwareEverythingClient) SearchContext(ctx context.Context, query string, maxResults uint32) ([]rawResult, error) {
	f.contextCalled++
	return f.results, nil
}

func TestSearchReturnsEverythingFileResults(t *testing.T) {
	service := NewServiceWithClient(fakeEverythingClient{
		results: []rawResult{
			{Name: "README.md", Path: `P:\workspace\glwlg\app\x-tools\README.md`},
		},
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
		t.Fatal("Everything file result should expose open_parent")
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
	service := NewServiceWithClient(fakeEverythingClient{
		results: []rawResult{{Name: "sample.log", Path: filePath}},
	})

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
	service := NewServiceWithClient(fakeEverythingClient{
		results: []rawResult{{Name: filepath.Base(dir), Path: dir}},
	})

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
	service := NewServiceWithClient(fakeEverythingClient{
		results: []rawResult{{Name: "a.txt", Path: `P:\a.txt`}},
	})

	if results := service.Search("a"); len(results) != 0 {
		t.Fatalf("short query should not hit Everything: %#v", results)
	}
}

func TestSearchContextSkipsEverythingWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &countingEverythingClient{
		results: []rawResult{{Name: "README.md", Path: `P:\workspace\glwlg\app\x-tools\README.md`}},
	}
	service := NewServiceWithClient(client)

	results := service.SearchContext(ctx, "readme")

	if len(results) != 0 {
		t.Fatalf("cancelled search should return no results, got %#v", results)
	}
	if client.called != 0 {
		t.Fatalf("cancelled search should not call Everything, called %d time(s)", client.called)
	}
	status := service.Status()
	if status.LastQuery != "" || status.LastUpdatedAt != 0 || status.LastError != "" {
		t.Fatalf("cancelled search should not update Everything diagnostics: %#v", status)
	}
}

func TestSearchContextUsesContextAwareEverythingClient(t *testing.T) {
	client := &contextAwareEverythingClient{
		results: []rawResult{{Name: "README.md", Path: `P:\workspace\glwlg\app\x-tools\README.md`}},
	}
	service := NewServiceWithClient(client)

	results := service.SearchContext(context.Background(), "readme")

	if len(results) != 1 {
		t.Fatalf("expected context-aware Everything result, got %#v", results)
	}
	if client.contextCalled != 1 || client.fallbackCalled != 0 {
		t.Fatalf("expected SearchContext path, context=%d fallback=%d", client.contextCalled, client.fallbackCalled)
	}
}

func TestSearchRecordsEverythingErrors(t *testing.T) {
	service := NewServiceWithClient(fakeEverythingClient{err: fmt.Errorf("Everything IPC unavailable")})

	results := service.Search("readme")

	if len(results) != 0 {
		t.Fatalf("Everything error should return no results: %#v", results)
	}
	if service.LastError() == "" {
		t.Fatal("Everything error should be recorded for diagnostics")
	}
	status := service.Status()
	if status.Ready || status.LastError == "" || status.LastQuery != "readme" || status.LastUpdatedAt == 0 {
		t.Fatalf("Everything diagnostic status should expose the failure: %#v", status)
	}
}

func TestStatusReportsLastSuccessfulEverythingQuery(t *testing.T) {
	service := NewServiceWithClient(fakeEverythingClient{
		results: []rawResult{
			{Name: "README.md", Path: `P:\workspace\glwlg\app\x-tools\README.md`},
			{Name: "main.py", Path: `P:\workspace\glwlg\app\x-tools\main.py`},
		},
	})

	service.Search("readme")
	status := service.Status()

	if !status.Ready || !status.DLLFound || status.LastError != "" {
		t.Fatalf("Everything should be ready after a successful injected query: %#v", status)
	}
	if status.LastQuery != "readme" || status.LastResultCount != 2 || status.LastUpdatedAt == 0 {
		t.Fatalf("Everything status should record query details: %#v", status)
	}
}

func TestFileLikeZeroResultsReturnsCoverageHint(t *testing.T) {
	service := NewServiceWithClient(fakeEverythingClient{})

	results := service.Search("P:\\workspace\\missing.json")

	if len(results) != 1 {
		t.Fatalf("expected coverage hint result, got %#v", results)
	}
	result := results[0]
	if result.ID != "file-search-coverage-hint" || result.Type != contracts.ResultSettings {
		t.Fatalf("expected explicit settings diagnostic result, got %#v", result)
	}
	if !strings.Contains(result.Detail, "目标盘或目录已加入索引") {
		t.Fatalf("coverage hint should explain index coverage, got %#v", result.Detail)
	}
	if err := contracts.ValidateActionSurface(result); err != nil {
		t.Fatalf("invalid coverage hint action surface: %v", err)
	}
	status := service.Status()
	if status.CoverageHint == "" {
		t.Fatalf("status should expose coverage hint: %#v", status)
	}
}

func TestOrdinaryZeroResultsDoNotReturnCoverageHint(t *testing.T) {
	service := NewServiceWithClient(fakeEverythingClient{})

	results := service.Search("gateway")

	if len(results) != 0 {
		t.Fatalf("ordinary empty file search should not add diagnostic result: %#v", results)
	}
	if status := service.Status(); status.CoverageHint != "" {
		t.Fatalf("ordinary empty query should not set coverage hint: %#v", status)
	}
}

func TestMissingEverythingDLLReturnsFileCoverageHint(t *testing.T) {
	service := NewServiceWithDLLPath("")

	results := service.Search("README.md")

	if len(results) != 1 || results[0].ID != "file-search-coverage-hint" {
		t.Fatalf("missing DLL should explain file-search setup for file-like query: %#v", results)
	}
	if !strings.Contains(results[0].Detail, "Everything64.dll") {
		t.Fatalf("missing DLL hint should mention SDK DLL, got %#v", results[0].Detail)
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

func metaValue(result contracts.SearchResult, label string) string {
	for _, item := range result.Preview.Meta {
		if item.Label == label {
			return item.Value
		}
	}
	return ""
}
