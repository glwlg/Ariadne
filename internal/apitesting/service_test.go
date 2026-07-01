package apitesting

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunAppliesVariablesAndAssertions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/run-42" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("mode"); got != "smoke" {
			t.Fatalf("unexpected mode query: %q", got)
		}
		if got := r.Header.Get("X-Trace"); got != "run-42" {
			t.Fatalf("unexpected trace header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Service", "ariadne")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"data": map[string]any{
				"id": "run-42",
			},
		})
	}))
	defer server.Close()

	service := NewServiceWithPath(filepath.Join(t.TempDir(), "api_testing.json"), server.Client())
	collection := Collection{
		ID:   "col-test",
		Name: "测试集合",
		Variables: []Variable{
			{ID: "var-base", Name: "baseUrl", Value: server.URL, Enabled: true},
		},
		Environments: []Environment{{
			ID:   "env-test",
			Name: "测试环境",
			Variables: []Variable{
				{ID: "var-trace", Name: "trace", Value: "run-42", Enabled: true},
			},
		}},
		Requests: []Request{{
			ID:     "req-test",
			Name:   "ping",
			Method: http.MethodGet,
			URL:    "{{baseUrl}}/v1/:trace",
			Params: []Param{
				{ID: "param-trace", Name: "trace", Value: "{{trace}}", Type: "path", Enabled: true},
				{ID: "param-mode", Name: "mode", Value: "smoke", Type: "query", Enabled: true},
				{ID: "param-off", Name: "ignored", Value: "true", Type: "query", Enabled: false},
			},
			Headers: []Header{
				{ID: "hdr-trace", Name: "X-Trace", Value: "{{trace}}", Enabled: true},
			},
			Assertions: []Assertion{
				{ID: "ast-status", Kind: "status", Operator: "equals", Expected: "200", Enabled: true},
				{ID: "ast-header", Kind: "header", Target: "X-Service", Operator: "equals", Expected: "ariadne", Enabled: true},
				{ID: "ast-json", Kind: "json", Target: "data.id", Operator: "equals", Expected: "run-42", Enabled: true},
				{ID: "ast-time", Kind: "response_time", Operator: "less_than", Expected: "1000", Enabled: true},
			},
			BodyType: "none",
		}},
		ActiveEnvironmentID: "env-test",
		ActiveRequestID:     "req-test",
	}
	service.UpsertCollection(collection)

	result := service.Run(RunRequest{
		CollectionID:  "col-test",
		EnvironmentID: "env-test",
		Request:       collection.Requests[0],
	})
	if !result.OK {
		t.Fatalf("expected run ok, got %#v", result)
	}
	if result.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", result.StatusCode)
	}
	if result.Failed != 0 || result.Passed != 4 {
		t.Fatalf("unexpected assertion counts: passed=%d failed=%d results=%#v", result.Passed, result.Failed, result.AssertionResults)
	}
}

func TestRunReportsFailingJSONAssertion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"queued"}`))
	}))
	defer server.Close()

	service := NewServiceWithPath(filepath.Join(t.TempDir(), "api_testing.json"), server.Client())
	request := Request{
		ID:     "req-json",
		Name:   "status",
		Method: http.MethodGet,
		URL:    server.URL,
		Assertions: []Assertion{
			{ID: "ast-json", Kind: "json", Target: "status", Operator: "equals", Expected: "done", Enabled: true},
		},
	}

	result := service.Run(RunRequest{Request: request})
	if !result.OK {
		t.Fatalf("expected completed request, got %#v", result)
	}
	if result.Failed != 1 || result.AssertionResults[0].Passed {
		t.Fatalf("expected one failing assertion, got %#v", result.AssertionResults)
	}
}

func TestRunKeepsServerSentEventsOpenUntilStopped(t *testing.T) {
	firstEventSent := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "event: ready\ndata: {\"ok\":true}\n\n")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		close(firstEventSent)
		<-r.Context().Done()
	}))
	defer server.Close()

	service := NewServiceWithPath(filepath.Join(t.TempDir(), "api_testing.json"), server.Client())
	resultCh := make(chan RunResult, 1)
	go func() {
		resultCh <- service.Run(RunRequest{
			RunID: "run-sse",
			Request: Request{
				ID:     "req-sse",
				Name:   "sse",
				Method: http.MethodGet,
				URL:    server.URL,
			},
			TimeoutSeconds: 10,
		})
	}()

	select {
	case <-firstEventSent:
	case <-time.After(2 * time.Second):
		t.Fatal("server did not send first SSE event")
	}

	select {
	case result := <-resultCh:
		t.Fatalf("SSE request returned before it was stopped: %#v", result)
	case <-time.After(300 * time.Millisecond):
	}

	snapshot := service.RunSnapshot("run-sse")
	if !snapshot.OK || !snapshot.Running {
		t.Fatalf("expected running SSE snapshot, got %#v", snapshot)
	}
	if !snapshot.Result.Streaming || !strings.Contains(snapshot.Result.Body, "event: ready") {
		t.Fatalf("expected live SSE body in snapshot, got %#v", snapshot.Result)
	}

	stopResult := service.StopRun("run-sse")
	if !stopResult.OK {
		t.Fatalf("expected stop to be accepted, got %#v", stopResult)
	}
	var result RunResult
	select {
	case result = <-resultCh:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE request did not finish after stop")
	}
	if !result.OK || !result.Streaming {
		t.Fatalf("expected streaming result, got %#v", result)
	}
	if !strings.Contains(result.Body, "event: ready") {
		t.Fatalf("expected first SSE event in body, got %q", result.Body)
	}
}

func TestImportRequestsImportsPostmanCollection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api_testing.json")
	service := NewServiceWithPath(path, nil)
	status := service.UpsertCollection(Collection{
		ID:           "col-import",
		Name:         "导入目标",
		Environments: []Environment{{ID: "env-main", Name: "主环境"}},
		Requests:     []Request{{ID: "req-existing", Name: "已有请求", Method: http.MethodGet, URL: "https://example.test/existing"}},
	})
	if status.LastSaveError != "" {
		t.Fatalf("unexpected save error: %s", status.LastSaveError)
	}

	importPath := filepath.Join(t.TempDir(), "postman.json")
	if err := os.WriteFile(importPath, []byte(`{
		"info": { "name": "ops" },
		"item": [
			{
				"name": "监控",
				"item": [
					{
						"name": "首页大屏 sse",
						"request": {
							"method": "GET",
							"header": [{ "key": "Accept", "value": "text/event-stream" }],
							"url": { "raw": "{{host}}/ops-mg/api/open/dashboard/stream/digital-guolian" }
						}
					}
				]
			}
		]
	}`), 0o600); err != nil {
		t.Fatal(err)
	}

	result := service.ImportRequests(importPath, "col-import")
	if !result.OK || result.ImportedCount != 1 {
		t.Fatalf("unexpected import result: %#v", result)
	}
	var importedCollection *Collection
	for index := range result.Status.Collections {
		if result.Status.Collections[index].ID == "col-import" {
			importedCollection = &result.Status.Collections[index]
			break
		}
	}
	if importedCollection == nil {
		t.Fatalf("target collection not found: %#v", result.Status.Collections)
	}
	var found *Request
	for index := range importedCollection.Requests {
		if importedCollection.Requests[index].Name == "首页大屏 sse" {
			found = &importedCollection.Requests[index]
			break
		}
	}
	if found == nil {
		t.Fatalf("imported request not found: %#v", importedCollection.Requests)
	}
	if found.Folder != "监控" || found.URL != "{{host}}/ops-mg/api/open/dashboard/stream/digital-guolian" {
		t.Fatalf("unexpected imported request: %#v", found)
	}
}

func TestNewCollectionKeepsExistingCollectionsAndActivatesNewOne(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api_testing.json")
	service := NewServiceWithPath(path, nil)
	initial := service.UpsertCollection(Collection{
		ID:           "col-existing",
		Name:         "opscore",
		Environments: []Environment{{ID: "env-main", Name: "prod"}},
		Requests:     []Request{{ID: "req-main", Name: "首页大屏", Folder: "监控大屏", Method: http.MethodGet, URL: "{{host}}/dashboard"}},
	})

	status := service.NewCollection()
	if len(status.Collections) != len(initial.Collections)+1 {
		t.Fatalf("expected existing collections to remain, got %#v", status.Collections)
	}
	if status.ActiveCollectionID == "" || status.ActiveCollectionID == "col-existing" {
		t.Fatalf("expected new collection to be active, got %q", status.ActiveCollectionID)
	}
	var existingFound bool
	var activeFound bool
	for _, collection := range status.Collections {
		if collection.ID == "col-existing" {
			existingFound = true
		}
		if collection.ID == status.ActiveCollectionID {
			activeFound = true
		}
	}
	if !existingFound {
		t.Fatalf("existing collection missing: %#v", status.Collections)
	}
	if !activeFound {
		t.Fatalf("active collection missing: %#v", status.Collections)
	}
}

func TestStatePersistsCollections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api_testing.json")
	service := NewServiceWithPath(path, nil)
	status := service.UpsertCollection(Collection{
		ID:   "col-persist",
		Name: "持久化集合",
		Variables: []Variable{
			{ID: "var-base", Name: "baseUrl", Value: "https://example.test", Enabled: true},
		},
		Environments: []Environment{{ID: "env-main", Name: "主环境"}},
		Requests:     []Request{{ID: "req-main", Name: "检查状态", Folder: "监控", Method: http.MethodGet, URL: "{{baseUrl}}/status"}},
	})
	if status.LastSaveError != "" {
		t.Fatalf("unexpected save error: %s", status.LastSaveError)
	}

	reloaded := NewServiceWithPath(path, nil)
	next := reloaded.Status()
	var found *Collection
	for index := range next.Collections {
		if next.Collections[index].ID == "col-persist" {
			found = &next.Collections[index]
			break
		}
	}
	if found == nil {
		t.Fatalf("persisted collection not found: %#v", next.Collections)
	}
	if found.Requests[0].URL != "{{baseUrl}}/status" {
		t.Fatalf("unexpected request url: %s", found.Requests[0].URL)
	}
	if found.Requests[0].Folder != "监控" {
		t.Fatalf("unexpected request folder: %s", found.Requests[0].Folder)
	}
}

func TestGitSyncWritesCollectionFileOnSave(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "api_testing.json"), nil)
	collection := Collection{
		ID:   "col-git",
		Name: "opscore",
		Requests: []Request{{
			ID:     "req-main",
			Name:   "首页大屏",
			Method: http.MethodGet,
			URL:    "https://example.test/one",
		}},
		Environments: []Environment{{ID: "env-main", Name: "prod"}},
	}
	status := service.UpsertCollection(collection)
	if status.LastSaveError != "" {
		t.Fatalf("unexpected save error: %s", status.LastSaveError)
	}
	repoPath := filepath.Join(t.TempDir(), "api-repo")
	gitStatus := service.ConfigureGit("col-git", repoPath, "")
	if !gitStatus.OK {
		t.Fatalf("expected git config ok, got %#v", gitStatus)
	}

	collection.Git = GitConfig{Path: repoPath}
	collection.Requests[0].URL = "https://example.test/two"
	status = service.UpsertCollection(collection)
	if status.LastSaveError != "" {
		t.Fatalf("unexpected save error after git sync: %s", status.LastSaveError)
	}

	raw, err := os.ReadFile(filepath.Join(repoPath, gitCollectionFileName))
	if err != nil {
		t.Fatal(err)
	}
	var saved Collection
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Name != "opscore" || len(saved.Requests) != 1 || saved.Requests[0].URL != "https://example.test/two" {
		t.Fatalf("unexpected synced collection: %#v", saved)
	}
	if saved.Git.Path != "" || strings.Contains(string(raw), "\"git\"") || strings.Contains(string(raw), repoPath) {
		t.Fatalf("git collection file leaked local git config: %s", raw)
	}
}
