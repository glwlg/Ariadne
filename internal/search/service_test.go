package search

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"ariadne/internal/apps"
	"ariadne/internal/capturehistory"
	"ariadne/internal/contracts"
	"ariadne/internal/plugins"
	"ariadne/internal/workflows"
)

func TestSearchSeedResultsExcludeWorkMemoryContent(t *testing.T) {
	service := NewServiceWithState(NewStateStore(""))

	response := service.Search(context.Background(), "OpenWrt")
	if err := contracts.ValidateActionSurfaces(response.Results); err != nil {
		t.Fatalf("invalid action surface: %v", err)
	}
	for _, result := range response.Results {
		if result.Type == contracts.ResultMemory || result.ID == "memory-gateway" {
			t.Fatalf("launcher seed search should not expose work memory content: %#v", result)
		}
	}
}

func TestSearchEmptyQueryReturnsNoResults(t *testing.T) {
	service := NewService(fakeProvider{results: []contracts.SearchResult{scoredResult("launcher-alpha", "Alpha", 10)}})

	response := service.Search(context.Background(), "")

	if len(response.Results) != 0 {
		t.Fatalf("empty query should keep launcher collapsed, got %#v", response.Results)
	}
}

func TestSearchSkipsProvidersWhenContextIsAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	provider := &recordingProvider{results: []contracts.SearchResult{scoredResult("launcher-alpha", "Alpha", 10)}}
	service := NewServiceWithState(NewStateStore(""), provider)

	response := service.Search(ctx, "cancel-query")

	if provider.called != 0 {
		t.Fatalf("cancelled search should not call providers, called %d time(s)", provider.called)
	}
	if len(response.Results) != 0 {
		t.Fatalf("cancelled search should return no results, got %#v", response.Results)
	}
	if status := service.PerformanceStatus(); status.SampleCount != 0 {
		t.Fatalf("cancelled search should not record performance samples: %#v", status)
	}
}

func TestSearchCancellationStopsRemainingProviders(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	first := &cancelingContextProvider{
		cancel:  cancel,
		results: []contracts.SearchResult{scoredResult("launcher-alpha", "Alpha", 10)},
	}
	second := &recordingProvider{results: []contracts.SearchResult{scoredResult("launcher-beta", "Beta", 20)}}
	service := NewServiceWithState(NewStateStore(""), first, second)

	response := service.Search(ctx, "cancel-query")

	if first.contextCalled != 1 {
		t.Fatalf("expected context-aware provider to be called once, got %d", first.contextCalled)
	}
	if first.fallbackCalled != 0 {
		t.Fatalf("expected SearchContext path, fallback Search called %d time(s)", first.fallbackCalled)
	}
	if second.called != 0 {
		t.Fatalf("cancelled search should not call remaining providers, called %d time(s)", second.called)
	}
	if len(response.Results) != 0 {
		t.Fatalf("cancelled provider results should be discarded, got %#v", response.Results)
	}
	if status := service.PerformanceStatus(); status.SampleCount != 0 {
		t.Fatalf("cancelled search should not record performance samples: %#v", status)
	}
}

func TestSearchAggregatesIndexedFiles(t *testing.T) {
	service := NewService(fakeProvider{results: []contracts.SearchResult{{
		ID:       "file-ariadne-readme",
		Type:     contracts.ResultFile,
		Title:    "README.md",
		Subtitle: `P:\workspace\glwlg\app\x-tools`,
		Detail:   `P:\workspace\glwlg\app\x-tools\README.md`,
		Icon:     "file",
		Score:    90,
		Preview: contracts.PreviewDescriptor{
			Kind:  contracts.PreviewText,
			Title: "README.md",
			Text:  `P:\workspace\glwlg\app\x-tools\README.md`,
		},
		Actions: []contracts.PreviewAction{
			{ID: "open", Label: "打开", Kind: contracts.ActionOpen, Payload: map[string]interface{}{"path": `P:\workspace\glwlg\app\x-tools\README.md`}},
			{ID: "open_parent", Label: "打开所在文件夹", Kind: contracts.ActionOpenParent, Payload: map[string]interface{}{"path": `P:\workspace\glwlg\app\x-tools\README.md`}},
			contracts.CopyAction("copy_path", "复制路径", `P:\workspace\glwlg\app\x-tools\README.md`, ""),
		},
	}}})

	response := service.Search(context.Background(), "readme")

	if len(response.Results) == 0 {
		t.Fatal("expected file search result")
	}
	if response.Results[0].Type != contracts.ResultFile {
		t.Fatalf("expected scored file result first, got %#v", response.Results[0])
	}
	if err := contracts.ValidateActionSurfaces(response.Results); err != nil {
		t.Fatalf("invalid action surface: %v", err)
	}
}

func TestSearchAggregatesStartMenuApps(t *testing.T) {
	root := t.TempDir()
	shortcutPath := filepath.Join(root, "Ariadne Notes.lnk")
	if err := os.WriteFile(shortcutPath, []byte("shortcut"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewService(apps.NewServiceWithRoots(root))

	response := service.Search(context.Background(), "ariadne")

	if len(response.Results) == 0 {
		t.Fatal("expected app search result")
	}
	if response.Results[0].Type != contracts.ResultApp {
		t.Fatalf("expected scored app result first, got %#v", response.Results[0])
	}
	if err := contracts.ValidateActionSurfaces(response.Results); err != nil {
		t.Fatalf("invalid action surface: %v", err)
	}
}

func TestSearchAggregatesCustomLaunchers(t *testing.T) {
	service := NewServiceWithState(NewStateStore(""), fakeProvider{results: []contracts.SearchResult{{
		ID:       "launcher-docs",
		Type:     contracts.ResultCommand,
		Title:    "项目文档",
		Subtitle: "自定义启动项",
		Detail:   `P:\workspace\glwlg\app\x-tools\docs`,
		Icon:     "folder",
		Score:    110,
		Preview: contracts.PreviewDescriptor{
			Kind:  contracts.PreviewText,
			Title: "项目文档",
			Text:  `P:\workspace\glwlg\app\x-tools\docs`,
		},
		Actions: []contracts.PreviewAction{
			{ID: "open_launcher", Label: "打开", Kind: contracts.ActionOpen, Payload: map[string]interface{}{"path": `P:\workspace\glwlg\app\x-tools\docs`}},
			contracts.CopyAction("copy_target", "复制目标", `P:\workspace\glwlg\app\x-tools\docs`, ""),
		},
	}}})

	response := service.Search(context.Background(), "docs")

	if len(response.Results) == 0 || response.Results[0].ID != "launcher-docs" {
		t.Fatalf("expected custom launcher first, got %#v", response.Results)
	}
	if !hasActionKind(response.Results[0], contracts.ActionPin) {
		t.Fatal("search should append favorite action")
	}
}

func TestSearchSeedResultsExcludeClipboardHistoryContent(t *testing.T) {
	service := NewServiceWithState(NewStateStore(""))

	response := service.Search(context.Background(), "gateway degraded")

	for _, result := range response.Results {
		if result.Type == contracts.ResultClipboard || result.ID == "clipboard-json" {
			t.Fatalf("launcher seed search should not expose clipboard history content: %#v", result)
		}
	}
}

func TestSearchAggregatesCaptureHistory(t *testing.T) {
	root := t.TempDir()
	captureService := capturehistory.NewServiceWithPaths(filepath.Join(root, "capture_history.json"), filepath.Join(root, "capture_images"))
	captureService.AddPNG([]byte("png-bytes"), 1440, 900, "test-screen", "", []string{"screen"})
	service := NewServiceWithState(NewStateStore(""), captureService)

	response := service.Search(context.Background(), "cap 1440x900")

	if len(response.Results) == 0 || response.Results[0].Type != contracts.ResultCapture {
		t.Fatalf("expected capture history result, got %#v", response.Results)
	}
	if !hasActionKind(response.Results[0], contracts.ActionOpenParent) {
		t.Fatal("capture history result should expose open_parent")
	}
	if err := contracts.ValidateActionSurfaces(response.Results); err != nil {
		t.Fatalf("invalid action surface: %v", err)
	}
}

func TestSearchAggregatesWorkflowMacros(t *testing.T) {
	pluginService := plugins.NewService()
	workflowService := workflows.NewServiceWithPaths(filepath.Join(t.TempDir(), "workflows.json"), "", pluginService)
	service := NewServiceWithState(NewStateStore(""), workflowService, pluginService)

	response := service.Search(context.Background(), "wf clip-md5 smoke")

	if len(response.Results) == 0 || response.Results[0].Type != contracts.ResultWorkflow {
		t.Fatalf("expected workflow result, got %#v", response.Results)
	}
	if response.Results[0].Payload["workflowId"] != "clip-md5" {
		t.Fatalf("expected real workflow payload, got %#v", response.Results[0].Payload)
	}
	if err := contracts.ValidateActionSurfaces(response.Results); err != nil {
		t.Fatalf("invalid action surface: %v", err)
	}
}

func TestExactPluginCommandBeatsFileIndexResults(t *testing.T) {
	pluginService := plugins.NewService()
	fileProvider := fakeProvider{results: []contracts.SearchResult{{
		ID:       "file-ariadne-net",
		Type:     contracts.ResultFile,
		Title:    "net",
		Subtitle: `C:\docs`,
		Detail:   `C:\docs\net`,
		Icon:     "file",
		Score:    95,
		Tags:     []string{"文件", "Ariadne 索引"},
		Preview:  contracts.PreviewDescriptor{Kind: contracts.PreviewText, Title: "net", Text: `C:\docs\net`},
		Actions:  []contracts.PreviewAction{{ID: "open", Label: "打开", Kind: contracts.ActionOpen, Payload: map[string]interface{}{"path": `C:\docs\net`}}},
	}}}
	service := NewServiceWithState(NewStateStore(""), fileProvider, pluginService)

	response := service.Search(context.Background(), "net")

	if len(response.Results) == 0 || response.Results[0].ID != "network-monitor" {
		t.Fatalf("exact plugin command should rank above file result, got %#v", response.Results)
	}
}

func TestSettingsSearchOpensSettingsCenter(t *testing.T) {
	service := NewServiceWithState(NewStateStore(""))

	response := service.Search(context.Background(), "设置")

	if len(response.Results) == 0 || response.Results[0].ID != "settings-center" {
		t.Fatalf("expected settings center result, got %#v", response.Results)
	}
	action := response.Results[0].Actions[0]
	if action.ID != "open_tool" || action.Payload["command"] != "open_settings" {
		t.Fatalf("expected settings open_tool action, got %#v", action)
	}
	if err := contracts.ValidateActionSurfaces(response.Results); err != nil {
		t.Fatalf("invalid action surface: %v", err)
	}
}

func TestSettingsSearchBeatsFileResults(t *testing.T) {
	service := NewServiceWithState(NewStateStore(""), fakeProvider{results: []contracts.SearchResult{{
		ID:       "file-user-settings",
		Type:     contracts.ResultFile,
		Title:    "设置.docx",
		Subtitle: `C:\Users\luwei\Desktop`,
		Detail:   `C:\Users\luwei\Desktop\设置.docx`,
		Icon:     "file",
		Score:    95,
		Preview:  contracts.PreviewDescriptor{Kind: contracts.PreviewText, Title: "设置.docx", Text: `C:\Users\luwei\Desktop\设置.docx`},
		Actions: []contracts.PreviewAction{
			{ID: "open", Label: "打开", Kind: contracts.ActionOpen, Payload: map[string]interface{}{"path": `C:\Users\luwei\Desktop\设置.docx`}},
			contracts.CopyAction("copy_path", "复制路径", `C:\Users\luwei\Desktop\设置.docx`, ""),
		},
	}}})

	response := service.Search(context.Background(), "设置")

	if len(response.Results) < 2 || response.Results[0].ID != "settings-center" {
		t.Fatalf("settings center should beat file results, got %#v", response.Results)
	}
}

func TestSeedPluginTriggerUsesPrepareCommandAction(t *testing.T) {
	service := NewServiceWithState(NewStateStore(""))

	response := service.Search(context.Background(), "UUID 生成器")

	if len(response.Results) == 0 || response.Results[0].ID != "plugin-uuid" {
		t.Fatalf("expected seeded UUID plugin trigger, got %#v", response.Results)
	}
	action := response.Results[0].Actions[0]
	if action.ID != "prepare_command" || action.Payload["command"] != "uuid" {
		t.Fatalf("seeded plugin trigger should prepare command, got %#v", action)
	}
	if _, ok := response.Results[0].Payload["commandSchema"]; !ok {
		t.Fatalf("seeded plugin trigger should expose command schema: %#v", response.Results[0].Payload)
	}
}

func TestSearchFavoriteAndRecentUsageBoostRanking(t *testing.T) {
	state := NewStateStore("")
	service := NewServiceWithState(state, fakeProvider{results: []contracts.SearchResult{
		scoredResult("launcher-alpha", "Alpha", 10),
		scoredResult("launcher-beta", "Beta", 20),
	}})

	service.RecordUse("launcher-alpha")
	service.SetFavorite("launcher-alpha", true)
	response := service.Search(context.Background(), "launcher")

	if len(response.Results) < 2 || response.Results[0].ID != "launcher-alpha" {
		t.Fatalf("favorite/recent usage should boost alpha first: %#v", response.Results)
	}
	if !hasTag(response.Results[0], "收藏") || !hasTag(response.Results[0], "最近使用") {
		t.Fatalf("expected favorite and recent tags: %#v", response.Results[0].Tags)
	}
}

func TestSearchUsageStatePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "search_state.json")
	first := NewStateStore(path)
	first.RecordUse("launcher-alpha")
	first.SetFavorite("launcher-alpha", true)

	second := NewStateStore(path)
	record := second.Get("launcher-alpha")

	if !record.Favorite || record.UseCount != 1 {
		t.Fatalf("expected persisted usage record, got %#v", record)
	}
}

func TestSearchUnfavoriteWithoutRecentUseRemovesEmptyRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "search_state.json")
	store := NewStateStore(path)
	store.SetFavorite("launcher-alpha", true)

	record := store.SetFavorite("launcher-alpha", false)
	if record.Favorite {
		t.Fatalf("expected favorite flag to be false after unfavorite, got %#v", record)
	}
	if status := store.Status(); status.Count != 0 || len(status.Records) != 0 {
		t.Fatalf("empty unfavorited record should not remain in status: %#v", status)
	}

	reloaded := NewStateStore(path)
	if status := reloaded.Status(); status.Count != 0 || len(status.Records) != 0 {
		t.Fatalf("empty unfavorited record should not persist after reload: %#v", status)
	}
}

func TestSearchUnfavoriteKeepsRecentUsageButRemovesFavoriteBoost(t *testing.T) {
	state := NewStateStore("")
	service := NewServiceWithState(state, fakeProvider{results: []contracts.SearchResult{
		scoredResult("launcher-alpha", "Alpha", 10),
		scoredResult("launcher-beta", "Beta", 20),
	}})

	service.RecordUse("launcher-alpha")
	service.SetFavorite("launcher-alpha", true)
	service.SetFavorite("launcher-alpha", false)

	status := service.UsageStatus()
	if status.Count != 1 || status.Records[0].ResultID != "launcher-alpha" || status.Records[0].Favorite || status.Records[0].UseCount != 1 {
		t.Fatalf("unfavorite should keep recent usage only: %#v", status)
	}

	response := service.Search(context.Background(), "launcher")
	if len(response.Results) < 2 || response.Results[0].ID != "launcher-alpha" {
		t.Fatalf("recent usage should still boost alpha first: %#v", response.Results)
	}
	if hasTag(response.Results[0], "收藏") || !hasTag(response.Results[0], "最近使用") {
		t.Fatalf("expected only recent usage tag after unfavorite: %#v", response.Results[0].Tags)
	}
	if !hasFavoriteAction(response.Results[0], "favorite", true) {
		t.Fatalf("unfavorited recent result should expose favorite action: %#v", response.Results[0].Actions)
	}
}

func TestSearchUsageStateCanBeCleared(t *testing.T) {
	path := filepath.Join(t.TempDir(), "search_state.json")
	service := NewServiceWithState(NewStateStore(path))
	service.RecordUse("launcher-alpha")
	service.SetFavorite("launcher-alpha", true)
	service.RecordUse("launcher-beta")

	result := service.ClearUsage()
	if !result.OK {
		t.Fatalf("expected clear ok, got %#v", result)
	}
	if result.Cleared != 2 || result.Status.Count != 0 {
		t.Fatalf("unexpected clear result: %#v", result)
	}

	reloaded := NewStateStore(path)
	if status := reloaded.Status(); status.Count != 0 || len(status.Records) != 0 {
		t.Fatalf("expected cleared persisted state, got %#v", status)
	}
}

func TestSearchDoesNotDuplicateProviderResults(t *testing.T) {
	duplicate := scoredResult("file-readme", "README.md", 10)
	service := NewService(
		fakeProvider{results: []contracts.SearchResult{duplicate}},
		fakeProvider{results: []contracts.SearchResult{duplicate}},
	)
	response := service.Search(context.Background(), "readme")

	count := 0
	for _, result := range response.Results {
		if result.ID == "file-readme" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected one file-readme result, got %d", count)
	}
}

func TestSearchRecordsPerformanceStatus(t *testing.T) {
	service := NewServiceWithState(NewStateStore(""))
	if status := service.PerformanceStatus(); status.SampleCount != 0 || !status.WithinTarget || status.TargetP95Ms != 100 {
		t.Fatalf("empty performance status should be target-ready: %#v", status)
	}

	service.Search(context.Background(), "设置")
	status := service.PerformanceStatus()

	if status.SampleCount != 1 || status.LastQuery != "设置" || status.LastResultCount == 0 {
		t.Fatalf("search should record non-empty query performance: %#v", status)
	}
}

func TestSearchPerformanceCalculatesP95(t *testing.T) {
	service := NewServiceWithState(NewStateStore(""))
	service.recordPerformance("fast", 10, 1)
	service.recordPerformance("medium", 50, 2)
	service.recordPerformance("slow", 120, 3)

	status := service.PerformanceStatus()

	if status.SampleCount != 3 || status.P95Ms != 120 || status.AverageMs != 60 || status.MaxMs != 120 {
		t.Fatalf("unexpected performance status: %#v", status)
	}
	if status.WithinTarget {
		t.Fatalf("p95 above target should be reported: %#v", status)
	}
}

func TestFileSeedResultKeepsFileActions(t *testing.T) {
	results := seedResults()
	if err := contracts.ValidateActionSurfaces(results); err != nil {
		t.Fatalf("invalid action surface: %v", err)
	}

	for _, result := range results {
		if result.ID == "file-readme" && !hasActionKind(result, contracts.ActionOpenParent) {
			t.Fatal("file result should expose open_parent")
		}
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

func hasTag(result contracts.SearchResult, tag string) bool {
	for _, item := range result.Tags {
		if item == tag {
			return true
		}
	}
	return false
}

func hasFavoriteAction(result contracts.SearchResult, id string, favorite bool) bool {
	for _, action := range result.Actions {
		if action.ID == id && action.Payload["favorite"] == favorite {
			return true
		}
	}
	return false
}

func scoredResult(id string, title string, score float64) contracts.SearchResult {
	return contracts.SearchResult{
		ID:       id,
		Type:     contracts.ResultCommand,
		Title:    title,
		Subtitle: "自定义启动项",
		Icon:     "command",
		Score:    score,
		Tags:     []string{"启动项"},
		Preview:  contracts.PreviewDescriptor{Kind: contracts.PreviewText, Title: title, Text: title},
		Actions: []contracts.PreviewAction{
			{ID: "open", Label: "打开", Kind: contracts.ActionOpen, Payload: map[string]interface{}{"path": title}},
		},
	}
}

type fakeProvider struct {
	results []contracts.SearchResult
}

func (f fakeProvider) Search(query string) []contracts.SearchResult {
	return f.results
}

type recordingProvider struct {
	results []contracts.SearchResult
	called  int
}

func (p *recordingProvider) Search(query string) []contracts.SearchResult {
	p.called++
	return p.results
}

type cancelingContextProvider struct {
	results        []contracts.SearchResult
	cancel         func()
	contextCalled  int
	fallbackCalled int
}

func (p *cancelingContextProvider) Search(query string) []contracts.SearchResult {
	p.fallbackCalled++
	return p.results
}

func (p *cancelingContextProvider) SearchContext(ctx context.Context, query string) []contracts.SearchResult {
	p.contextCalled++
	if p.cancel != nil {
		p.cancel()
	}
	return p.results
}
