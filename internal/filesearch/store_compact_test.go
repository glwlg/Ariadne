package filesearch

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
)

func testRootVolume() string {
	if runtime.GOOS == "windows" {
		return `P:\`
	}
	return `/p`
}

func TestCompactFileIndexSearchResolvesChineseFilenamePath(t *testing.T) {
	root := testRootVolume()
	index := newCompactFileIndex([]string{root})
	if err := index.AddNode(root, fileIndexNode{Ref: 10, Parent: 0, Name: "项目", IsDirectory: true}); err != nil {
		t.Fatalf("add directory node: %v", err)
	}
	if err := index.AddNode(root, fileIndexNode{Ref: 11, Parent: 10, Name: "工作日历.xlsx"}); err != nil {
		t.Fatalf("add file node: %v", err)
	}

	results := index.Search("工作日历", 10)
	if len(results) != 1 {
		t.Fatalf("expected one result, got %#v", results)
	}
	expected := filepath.Join(root, "项目", "工作日历.xlsx")
	if results[0].Path != expected {
		t.Fatalf("expected resolved full path %q, got %#v", expected, results[0])
	}
}

func TestCompactFileIndexSearchFindsTailItemInLargeIndex(t *testing.T) {
	index := buildCompactFileIndexFixture(t, 300000)

	results := index.Search("工作日历", 10)
	expected := filepath.Join(testRootVolume(), "项目", "工作日历.xlsx")
	if !hasRawPath(results, expected) {
		t.Fatalf("expected target result %q, got %#v", expected, results)
	}
	if index.Count() != 300003 {
		t.Fatalf("expected all nodes to stay indexed, got %d", index.Count())
	}
}

func BenchmarkCompactFileIndexSearch300K(b *testing.B) {
	index := buildCompactFileIndexFixture(b, 300000)
	index.Finalize()
	expected := filepath.Join(testRootVolume(), "项目", "工作日历.xlsx")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results := index.Search("工作日历", 24)
		if !hasRawPath(results, expected) {
			b.Fatalf("expected target result %q, got %#v", expected, results)
		}
	}
}

func BenchmarkCompactFileIndexBuild300K(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		index := buildCompactFileIndexFixture(b, 300000)
		if index.Count() != 300003 {
			b.Fatalf("expected full fixture, got %d", index.Count())
		}
	}
}

func TestLineFileIndexSearchResolvesChineseFilenamePath(t *testing.T) {
	index := buildCompactFileIndexFixture(t, 1000)
	lineIndex, err := writeLineFileIndex(filepath.Join(t.TempDir(), "file-index.tsv"), index)
	if err != nil {
		t.Fatalf("write line index: %v", err)
	}

	results := lineIndex.Search("工作日历", 10)
	expected := filepath.Join(testRootVolume(), "项目", "工作日历.xlsx")
	if !hasRawPath(results, expected) {
		t.Fatalf("expected target result %q, got %#v", expected, results)
	}
	if lineIndex.Count() != 1003 {
		t.Fatalf("expected full line index count, got %d", lineIndex.Count())
	}
}

func TestOpenLineFileIndexReusesPersistedTSV(t *testing.T) {
	index := buildCompactFileIndexFixture(t, 1000)
	path := filepath.Join(t.TempDir(), "file-index.tsv")
	if _, err := writeLineFileIndex(path, index); err != nil {
		t.Fatalf("write line index: %v", err)
	}

	reopened, err := openLineFileIndex(path)
	if err != nil {
		t.Fatalf("open line index: %v", err)
	}

	if reopened.Count() != 1003 {
		t.Fatalf("expected persisted count, got %d", reopened.Count())
	}
	if runtime.GOOS == "windows" {
		if volumes := reopened.Volumes(); len(volumes) != 1 || volumes[0] != `P:\` {
			t.Fatalf("expected persisted volume metadata, got %#v", volumes)
		}
	}
	expected := filepath.Join(testRootVolume(), "项目", "工作日历.xlsx")
	if !hasRawPath(reopened.Search("工作日历", 10), expected) {
		t.Fatalf("reopened line index should be searchable")
	}
}

func TestDefaultReadLineIndexPathsPreferSharedIndex(t *testing.T) {
	root := t.TempDir()
	programData := filepath.Join(root, "ProgramData")
	localAppData := filepath.Join(root, "LocalAppData")
	t.Setenv(lineIndexPathEnv, "")
	t.Setenv("PROGRAMDATA", programData)
	t.Setenv("LOCALAPPDATA", localAppData)

	paths := defaultReadLineIndexPaths()
	if len(paths) < 2 {
		t.Fatalf("expected shared and user index paths, got %#v", paths)
	}
	if paths[0] != filepath.Join(programData, "Fancun", "file-index.tsv") {
		t.Fatalf("shared index should be preferred, got %#v", paths)
	}
	if paths[1] != filepath.Join(localAppData, "Fancun", "file-index.tsv") {
		t.Fatalf("user index should be fallback, got %#v", paths)
	}
}

func TestLineFileIndexAppendNewFileIsSearchable(t *testing.T) {
	index := buildCompactFileIndexFixture(t, 1000)
	lineIndex, err := writeLineFileIndex(filepath.Join(t.TempDir(), "file-index.tsv"), index)
	if err != nil {
		t.Fatalf("write line index: %v", err)
	}
	if results := lineIndex.Search("搜索测试", 10); len(results) != 0 {
		t.Fatalf("new file should not exist before append, got %#v", results)
	}

	targetPath := filepath.Join(testRootVolume(), "桌面", "搜索测试.txt")
	added, err := lineIndex.AppendRawResults([]rawResult{{Name: "搜索测试.txt", Path: targetPath}})
	if err != nil {
		t.Fatalf("append line index: %v", err)
	}

	if added != 1 {
		t.Fatalf("expected one appended entry, got %d", added)
	}
	if !hasRawPath(lineIndex.Search("搜索测试", 10), targetPath) {
		t.Fatalf("appended file should be searchable")
	}
	if lineIndex.Count() != 1004 {
		t.Fatalf("expected appended count, got %d", lineIndex.Count())
	}
}

func TestLineFileIndexWriteExcludesPolicyFolders(t *testing.T) {
	index := buildCompactFileIndexFixture(t, 1000)
	lineIndex, err := writeLineFileIndex(filepath.Join(t.TempDir(), "file-index.tsv"), index, FileSearchPolicy{
		ExcludeFolders: []string{filepath.Join(testRootVolume(), "项目")},
	})
	if err != nil {
		t.Fatalf("write line index: %v", err)
	}

	if lineIndex.Count() != 0 {
		t.Fatalf("expected excluded index to be empty, got %d", lineIndex.Count())
	}
	if results := lineIndex.Search("工作日历", 10); len(results) != 0 {
		t.Fatalf("excluded paths should not be searchable, got %#v", results)
	}
}

func TestLineFileIndexSearchContextAbortsWhenCancelled(t *testing.T) {
	index := buildCompactFileIndexFixture(t, 20000)
	lineIndex, err := writeLineFileIndex(filepath.Join(t.TempDir(), "file-index.tsv"), index)
	if err != nil {
		t.Fatalf("write line index: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if results := lineIndex.SearchContext(ctx, "工作日历", 24); len(results) != 0 {
		t.Fatalf("cancelled SearchContext should abort and return no results, got %#v", results)
	}
}

func TestLineFileIndexAppendedCacheIsBounded(t *testing.T) {
	index := buildCompactFileIndexFixture(t, 10)
	lineIndex, err := writeLineFileIndex(filepath.Join(t.TempDir(), "file-index.tsv"), index)
	if err != nil {
		t.Fatalf("write line index: %v", err)
	}

	total := lineIndexAppendedCacheLimit + 500
	batch := make([]rawResult, 0, total)
	root := testRootVolume()
	for i := 0; i < total; i++ {
		name := fmt.Sprintf("extra-%06d.txt", i)
		batch = append(batch, rawResult{Name: name, Path: filepath.Join(root, "extra", name)})
	}
	added, err := lineIndex.AppendRawResults(batch)
	if err != nil {
		t.Fatalf("append batch: %v", err)
	}
	if added != total {
		t.Fatalf("expected %d added entries, got %d", total, added)
	}
	if len(lineIndex.appended) > lineIndexAppendedCacheLimit {
		t.Fatalf("appended cache exceeded limit %d: got %d", lineIndexAppendedCacheLimit, len(lineIndex.appended))
	}
}

func BenchmarkLineFileIndexSearch300K(b *testing.B) {
	index := buildCompactFileIndexFixture(b, 300000)
	lineIndex, err := writeLineFileIndex(filepath.Join(b.TempDir(), "file-index.tsv"), index)
	if err != nil {
		b.Fatalf("write line index: %v", err)
	}
	index = nil
	releaseFileIndexBuildMemory()
	expected := filepath.Join(testRootVolume(), "项目", "工作日历.xlsx")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results := lineIndex.Search("工作日历", 24)
		if !hasRawPath(results, expected) {
			b.Fatalf("expected target result %q, got %#v", expected, results)
		}
	}
}

func BenchmarkLineFileIndexBuild300K(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		index := buildCompactFileIndexFixture(b, 300000)
		lineIndex, err := writeLineFileIndex(filepath.Join(b.TempDir(), fmt.Sprintf("file-index-%d.tsv", i)), index)
		if err != nil {
			b.Fatalf("write line index: %v", err)
		}
		if lineIndex.Count() != 300003 {
			b.Fatalf("expected full line index count, got %d", lineIndex.Count())
		}
	}
}

func buildCompactFileIndexFixture(tb testing.TB, fillerCount int) *compactFileIndex {
	tb.Helper()
	root := testRootVolume()
	index := newCompactFileIndex([]string{root})
	if err := index.AddNode(root, fileIndexNode{Ref: 10, Parent: 0, Name: "项目", IsDirectory: true}); err != nil {
		tb.Fatalf("add project node: %v", err)
	}
	for i := 0; i < fillerCount; i++ {
		if err := index.AddNode(root, fileIndexNode{
			Ref:    uint64(100 + i),
			Parent: 10,
			Name:   fmt.Sprintf("文件-%06d.txt", i),
		}); err != nil {
			tb.Fatalf("add filler node %d: %v", i, err)
		}
	}
	if err := index.AddNode(root, fileIndexNode{Ref: uint64(100 + fillerCount), Parent: 10, Name: "工作日历.xlsx"}); err != nil {
		tb.Fatalf("add target node: %v", err)
	}
	if err := index.AddNode(root, fileIndexNode{Ref: uint64(101 + fillerCount), Parent: 10, Name: "工作日历-归档.xlsx"}); err != nil {
		tb.Fatalf("add archive node: %v", err)
	}
	return index
}

func hasRawPath(results []rawResult, path string) bool {
	for _, result := range results {
		if result.Path == path {
			return true
		}
	}
	return false
}
