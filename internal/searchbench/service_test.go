package searchbench

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"fancun/internal/contracts"
	"fancun/internal/filesearch"
	"fancun/internal/search"
)

func TestRunWithRunnerSummarizesSearchP95AndFileIndexHits(t *testing.T) {
	runner := &fakeRunner{responses: map[string]contracts.SearchResponse{
		"alpha": response("alpha", 12, validResult("alpha-result", contracts.ResultPluginResult, nil)),
		"beta":  response("beta", 120, everythingFileResult()),
	}}

	report := RunWithRunner(runner, Options{
		Iterations:   2,
		TargetP95Ms:  100,
		Queries:      []string{"alpha", "beta"},
		SlowestLimit: 1,
	})

	if report.Summary.Count != 4 || report.Summary.P95 != 120 {
		t.Fatalf("summary = %#v, want 4 samples with 120ms p95", report.Summary)
	}
	if report.Verdict.WithinTarget {
		t.Fatalf("p95 above target should fail verdict: %#v", report.Verdict)
	}
	if !report.ActionValidation.OK {
		t.Fatalf("expected valid actions: %#v", report.ActionValidation)
	}
	if report.ProviderStatus.FileIndexFileHits != 2 {
		t.Fatalf("file index hits = %d, want 2", report.ProviderStatus.FileIndexFileHits)
	}
	if len(report.SlowestSamples) != 1 || report.SlowestSamples[0].Query != "beta" {
		t.Fatalf("slowest samples = %#v, want beta", report.SlowestSamples)
	}
	if len(report.QuerySummaries) != 2 || report.QuerySummaries[1].FileIndexFileSamples != 2 {
		t.Fatalf("query summaries = %#v, want beta file index samples", report.QuerySummaries)
	}
}

func TestRunWithRunnerReportsActionValidationFailure(t *testing.T) {
	runner := &fakeRunner{responses: map[string]contracts.SearchResponse{
		"bad": {
			Query:   "bad",
			Elapsed: 8,
			Results: []contracts.SearchResult{{
				ID:      "bad-copy",
				Type:    contracts.ResultPluginResult,
				Title:   "Bad Copy",
				Icon:    "plugin",
				Preview: contracts.PreviewDescriptor{Kind: contracts.PreviewText, Title: "Bad Copy"},
				Actions: []contracts.PreviewAction{{
					ID:      "copy",
					Label:   "复制",
					Kind:    contracts.ActionCopy,
					Payload: map[string]interface{}{"text": "bad"},
				}},
			}},
		},
	}}

	report := RunWithRunner(runner, Options{Iterations: 1, Queries: []string{"bad"}})

	if report.ActionValidation.OK || report.ActionValidation.InvalidSamples != 1 {
		t.Fatalf("expected one invalid action sample: %#v", report.ActionValidation)
	}
	if !strings.Contains(report.ActionValidation.LastError, "inline feedback") {
		t.Fatalf("unexpected action error: %q", report.ActionValidation.LastError)
	}
	if len(report.Verdict.Warnings) == 0 {
		t.Fatalf("expected verdict warning: %#v", report.Verdict)
	}
}

func TestNormalizeQueriesDeduplicatesAndTrims(t *testing.T) {
	queries := normalizeQueries([]string{" settings ", "", "SETTINGS", "net"})
	if len(queries) != 2 || queries[0] != "settings" || queries[1] != "net" {
		t.Fatalf("queries = %#v, want settings/net", queries)
	}
}

type fakeRunner struct {
	responses map[string]contracts.SearchResponse
}

func (f *fakeRunner) Search(query string) contracts.SearchResponse {
	if response, ok := f.responses[query]; ok {
		return response
	}
	return contracts.SearchResponse{Query: query, Elapsed: 1}
}

func (f *fakeRunner) PerformanceStatus() search.PerformanceStatus {
	return search.PerformanceStatus{
		SampleCount:  len(f.responses),
		TargetP95Ms:  100,
		P95Ms:        120,
		WithinTarget: false,
	}
}

func response(query string, elapsed int64, results ...contracts.SearchResult) contracts.SearchResponse {
	return contracts.SearchResponse{Query: query, Elapsed: elapsed, Results: results}
}

func validResult(id string, resultType contracts.SearchResultType, tags []string) contracts.SearchResult {
	return contracts.SearchResult{
		ID:      id,
		Type:    resultType,
		Title:   id,
		Icon:    "plugin",
		Tags:    tags,
		Preview: contracts.PreviewDescriptor{Kind: contracts.PreviewText, Title: id, Text: id},
		Actions: []contracts.PreviewAction{contracts.CopyAction("copy_value", "复制结果", id, "Enter")},
	}
}

func everythingFileResult() contracts.SearchResult {
	result := validResult("file-fancun-readme", contracts.ResultFile, []string{"文件", "Fancun 索引"})
	result.Icon = "file"
	result.Payload = map[string]interface{}{"source": "Fancun USN/MFT", "path": `P:\workspace\README.md`}
	result.Actions = append(result.Actions, contracts.PreviewAction{
		ID:      "open_parent",
		Label:   "打开所在文件夹",
		Kind:    contracts.ActionOpenParent,
		Payload: map[string]interface{}{"path": `P:\workspace\README.md`},
	})
	return result
}

func TestRunWithSyntheticFileIndexFixtureExercisesFileHits(t *testing.T) {
	tempRoot := t.TempDir()
	appData := filepath.Join(tempRoot, "AppData", "Roaming")
	localAppData := filepath.Join(tempRoot, "AppData", "Local")
	if err := os.MkdirAll(appData, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(localAppData, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPDATA", appData)
	t.Setenv("LOCALAPPDATA", localAppData)

	volumeRoot := "/fancun-bench/workspace"
	if runtime.GOOS == "windows" {
		volumeRoot = `P:\workspace`
	}
	var builder strings.Builder
	for i := 0; i < 5000; i++ {
		name := fmt.Sprintf("sample-doc-%05d.txt", i)
		fmt.Fprintf(&builder, "%s\t0\t%s\n", strings.ToLower(name), filepath.Join(volumeRoot, "docs", name))
	}
	fmt.Fprintf(&builder, "readme.md\t0\t%s\n", filepath.Join(volumeRoot, "README.md"))
	fmt.Fprintf(&builder, "fancun.exe\t0\t%s\n", filepath.Join(volumeRoot, "fancun.exe"))
	fmt.Fprintf(&builder, "gateway.yaml\t0\t%s\n", filepath.Join(volumeRoot, "gateway.yaml"))

	tsvPath := filepath.Join(tempRoot, "file-index.tsv")
	if err := os.WriteFile(tsvPath, []byte(builder.String()), 0o600); err != nil {
		t.Fatal(err)
	}

	stack := NewDefaultStackWithFileSearch(filesearch.NewServiceWithLineIndexPath(tsvPath))
	defer stack.Close()

	report := RunWithRunner(stack, Options{
		Iterations:  2,
		Warmup:      1,
		TargetP95Ms: 200,
	})

	if !report.ProviderStatus.FileIndexStatusAvailable {
		t.Fatalf("expected file index status to be available: %#v", report.ProviderStatus)
	}
	if !report.ProviderStatus.FileIndex.Ready || report.ProviderStatus.FileIndex.IndexedCount != 5003 {
		t.Fatalf("expected synthetic file index ready with 5003 entries, got %#v", report.ProviderStatus.FileIndex)
	}
	if report.ProviderStatus.FileIndexFileHits <= 0 {
		t.Fatalf("expected file_hits > 0 from synthetic index fixture, got %#v", report.ProviderStatus)
	}
	if len(report.ProviderStatus.FileIndexHitQueries) == 0 {
		t.Fatalf("expected non-empty hit queries, got %#v", report.ProviderStatus)
	}
	if !report.ActionValidation.OK {
		t.Fatalf("action validation failed on synthetic hits: %#v", report.ActionValidation)
	}
	if !report.Verdict.WithinTarget {
		t.Fatalf("expected benchmark verdict within target, got %#v", report.Verdict)
	}
}
