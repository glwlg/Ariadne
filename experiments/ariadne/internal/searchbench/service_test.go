package searchbench

import (
	"strings"
	"testing"

	"ariadne/internal/contracts"
	"ariadne/internal/search"
)

func TestRunWithRunnerSummarizesSearchP95AndEverythingHits(t *testing.T) {
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
	if report.ProviderStatus.EverythingFileHits != 2 {
		t.Fatalf("EverythingFileHits = %d, want 2", report.ProviderStatus.EverythingFileHits)
	}
	if len(report.SlowestSamples) != 1 || report.SlowestSamples[0].Query != "beta" {
		t.Fatalf("slowest samples = %#v, want beta", report.SlowestSamples)
	}
	if len(report.QuerySummaries) != 2 || report.QuerySummaries[1].EverythingFileSamples != 2 {
		t.Fatalf("query summaries = %#v, want beta Everything samples", report.QuerySummaries)
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
	result := validResult("file-everything-readme", contracts.ResultFile, []string{"文件", "Everything"})
	result.Icon = "file"
	result.Payload = map[string]interface{}{"source": "Everything SDK", "path": `P:\workspace\README.md`}
	result.Actions = append(result.Actions, contracts.PreviewAction{
		ID:      "open_parent",
		Label:   "打开所在文件夹",
		Kind:    contracts.ActionOpenParent,
		Payload: map[string]interface{}{"path": `P:\workspace\README.md`},
	})
	return result
}
