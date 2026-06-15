package workflows

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ariadne/internal/contracts"
	"ariadne/internal/workmemory"
)

type fakeExecutor struct{}

func (fakeExecutor) Execute(keyword string, query string) []contracts.SearchResult {
	switch keyword {
	case "hash":
		sum := md5.Sum([]byte(query))
		value := hex.EncodeToString(sum[:])
		return []contracts.SearchResult{copyResult("MD5: "+value, value)}
	case "url":
		return []contracts.SearchResult{copyResult("编码结果: hello+world", "hello+world")}
	case "base64":
		return []contracts.SearchResult{copyResult("编码结果: aGVsbG8=", "aGVsbG8=")}
	default:
		return nil
	}
}

type countingExecutor struct {
	calls int
}

func (executor *countingExecutor) Execute(keyword string, query string) []contracts.SearchResult {
	executor.calls++
	return []contracts.SearchResult{copyResult("ok", keyword+" "+query)}
}

func TestWorkflowRunChainsContextAndPicksCopyOutput(t *testing.T) {
	service := NewServiceWithPaths(filepath.Join(t.TempDir(), "workflows.json"), "", fakeExecutor{})
	service.Upsert(Workflow{
		ID:   "chain-test",
		Name: "链式测试",
		Steps: []Step{
			{Command: "hash {input}", Pick: "MD5"},
			{Command: "url {prev}", Pick: "编码结果"},
		},
	})

	result := service.Run(RunRequest{WorkflowID: "chain-test", Input: "hello world", ClipboardText: "ignored"})

	if !result.OK {
		t.Fatalf("expected workflow to pass, got %#v", result)
	}
	if result.Output != "hello+world" {
		t.Fatalf("expected final output from second step, got %q", result.Output)
	}
	if len(result.Steps) != 2 || !strings.Contains(result.Steps[1].RenderedCommand, "url ") {
		t.Fatalf("expected rendered step chain, got %#v", result.Steps)
	}
}

func TestWorkflowRunRequiresConfirmationForHighRiskCommands(t *testing.T) {
	executor := &countingExecutor{}
	service := NewServiceWithPaths(filepath.Join(t.TempDir(), "workflows.json"), "", executor)
	service.Upsert(Workflow{
		ID:    "risky-flow",
		Name:  "Risky Flow",
		Steps: []Step{{Command: "sys shutdown", Pick: "ok"}},
	})

	result := service.Run(RunRequest{WorkflowID: "risky-flow"})
	if result.OK || !result.RequiresConfirmation {
		t.Fatalf("expected confirmation boundary, got %#v", result)
	}
	if len(result.RiskReasons) == 0 || !strings.Contains(result.RiskReasons[0], "shutdown") {
		t.Fatalf("expected shutdown risk reason, got %#v", result.RiskReasons)
	}
	if executor.calls != 0 {
		t.Fatalf("risk preview should not execute steps, got %d calls", executor.calls)
	}

	confirmed := service.Run(RunRequest{WorkflowID: "risky-flow", Confirmed: true})
	if !confirmed.OK {
		t.Fatalf("confirmed run should execute fake result, got %#v", confirmed)
	}
	if executor.calls != 1 {
		t.Fatalf("confirmed run should execute once, got %d", executor.calls)
	}
}

func TestWorkflowSearchReturnsExplicitWorkflowActions(t *testing.T) {
	service := NewServiceWithPaths(filepath.Join(t.TempDir(), "workflows.json"), "", fakeExecutor{})
	results := service.Search("wf clip-md5 custom input")

	if len(results) == 0 {
		t.Fatal("expected workflow search results")
	}
	result := results[0]
	if result.Type != contracts.ResultWorkflow {
		t.Fatalf("expected workflow result, got %s", result.Type)
	}
	if err := contracts.ValidateActionSurface(result); err != nil {
		t.Fatal(err)
	}
	if result.Payload["workflowId"] != "clip-md5" || result.Payload["input"] != "custom input" {
		t.Fatalf("expected workflow payload with input suffix, got %#v", result.Payload)
	}
	if !hasAction(result.Actions, "run_workflow") || !hasAction(result.Actions, "open_tool") {
		t.Fatalf("expected run and editor actions, got %#v", result.Actions)
	}
}

func TestWorkflowSearchMarksRiskyWorkflowAction(t *testing.T) {
	service := NewServiceWithPaths(filepath.Join(t.TempDir(), "workflows.json"), "", fakeExecutor{})
	service.Upsert(Workflow{
		ID:    "risky-flow",
		Name:  "Risky Flow",
		Steps: []Step{{Command: "sys restart", Pick: "ok"}},
	})

	results := service.Search("wf risky-flow")
	if len(results) == 0 {
		t.Fatal("expected risky workflow search result")
	}
	action := findAction(results[0].Actions, "run_workflow")
	if action == nil || action.Kind != contracts.ActionDanger {
		t.Fatalf("expected danger action, got %#v", action)
	}
	if action.Payload["requiresConfirmation"] != true {
		t.Fatalf("expected requiresConfirmation payload, got %#v", action.Payload)
	}
}

func TestWorkflowImportsLegacyConfig(t *testing.T) {
	root := t.TempDir()
	legacyPath := filepath.Join(root, "legacy", "config.json")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]interface{}{
		"workflows": []map[string]interface{}{
			{
				"id":          "legacy-flow",
				"name":        "Legacy Flow",
				"description": "from python config",
				"steps":       []map[string]string{{"command": "hash {clipboard}", "pick": "MD5"}},
			},
		},
	})
	if err := os.WriteFile(legacyPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	service := NewServiceWithPaths(filepath.Join(root, "workflows.json"), legacyPath, fakeExecutor{})
	status := service.Status()

	if !status.LegacyImported {
		t.Fatal("expected legacy import flag")
	}
	if len(status.Workflows) != 1 || status.Workflows[0].ID != "legacy-flow" {
		t.Fatalf("expected imported legacy workflow, got %#v", status.Workflows)
	}
	if _, err := os.Stat(status.Path); err != nil {
		t.Fatalf("expected imported workflows to be persisted: %v", err)
	}
}

func TestWorkflowRunRejectsUnknownPlaceholder(t *testing.T) {
	service := NewServiceWithPaths(filepath.Join(t.TempDir(), "workflows.json"), "", fakeExecutor{})
	service.Upsert(Workflow{
		ID:    "bad-variable",
		Name:  "Bad Variable",
		Steps: []Step{{Command: "hash {secret}", Pick: "MD5"}},
	})

	result := service.Run(RunRequest{WorkflowID: "bad-variable"})

	if result.OK {
		t.Fatalf("expected workflow to fail, got %#v", result)
	}
	if !strings.Contains(result.Steps[0].Message, "secret") {
		t.Fatalf("expected unknown placeholder evidence, got %#v", result.Steps)
	}
}

func TestWorkflowRemovePersistsTombstoneForDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflows.json")
	service := NewServiceWithPaths(path, "", fakeExecutor{})
	service.Remove("clip-md5")

	reloaded := NewServiceWithPaths(path, "", fakeExecutor{})
	for _, workflow := range reloaded.List() {
		if workflow.ID == "clip-md5" {
			t.Fatalf("default workflow should stay removed after reload: %#v", reloaded.List())
		}
	}
}

func TestWorkflowExportAndImportData(t *testing.T) {
	root := t.TempDir()
	service := NewServiceWithPaths(filepath.Join(root, "workflows.json"), "", fakeExecutor{})
	service.Upsert(Workflow{
		ID:          "export-flow",
		Name:        "Export Flow",
		Description: "round trip",
		Steps:       []Step{{Command: "hash {input}", Pick: "MD5"}},
	})

	exported := service.ExportData()
	if !exported.OK || exported.Path == "" || exported.JSON == "" || exported.Count == 0 {
		t.Fatalf("expected export metadata, got %#v", exported)
	}
	if _, err := os.Stat(exported.Path); err != nil {
		t.Fatalf("expected export file: %v", err)
	}

	importedService := NewServiceWithPaths(filepath.Join(root, "imported", "workflows.json"), "", fakeExecutor{})
	result := importedService.ImportData(exported.JSON)
	if !result.OK || result.ImportedCount == 0 {
		t.Fatalf("expected import success, got %#v", result)
	}
	found := false
	for _, workflow := range result.Status.Workflows {
		if workflow.ID == "export-flow" && workflow.Description == "round trip" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected exported workflow after import, got %#v", result.Status.Workflows)
	}
}

func TestSaveWorkflowDraftRequiresConfirmationAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflows.json")
	service := NewServiceWithPaths(path, "", fakeExecutor{})
	draft := workmemory.WorkflowDraft{
		ID:        "workflow-draft-20260614132000",
		Title:     "Hosts 切换草稿",
		Trigger:   "需要切换 Hosts 时从经验发现触发",
		Input:     "目标环境和 Hosts 片段",
		Output:    "Hosts 差异预览",
		RiskLevel: "high",
		Evidence:  []string{"memory-a", "memory-b"},
		Steps: []workmemory.WorkflowDraftStep{
			{ID: "preview", Label: "生成预览", Command: "hosts preview {input}"},
			{ID: "apply", Label: "应用 Hosts", Command: "hosts apply {prev}", RequiresConfirm: true},
		},
		RequiresReview: true,
		CreatedAt:      1781414000,
	}

	preview := service.SaveWorkflowDraft(DraftSaveRequest{Draft: draft})
	if preview.OK || !preview.RequiresConfirmation {
		t.Fatalf("draft save should require confirmation, got %#v", preview)
	}
	if len(preview.RiskReasons) == 0 || preview.Status.Count != len(defaultWorkflows()) {
		t.Fatalf("confirmation preview should include risks without saving, got %#v", preview)
	}

	result := service.SaveWorkflowDraft(DraftSaveRequest{Draft: draft, Confirmed: true})
	if !result.OK {
		t.Fatalf("confirmed draft save failed: %#v", result)
	}
	if result.Workflow.ID != "memory-20260614132000" {
		t.Fatalf("unexpected workflow id: %#v", result.Workflow)
	}
	if !strings.Contains(result.Workflow.Description, "memory-a") || !strings.Contains(result.Workflow.Description, "Hosts 差异预览") {
		t.Fatalf("workflow description should preserve evidence and output: %q", result.Workflow.Description)
	}

	reloaded := NewServiceWithPaths(path, "", fakeExecutor{})
	found := false
	for _, workflow := range reloaded.List() {
		if workflow.ID == result.Workflow.ID && len(workflow.Steps) == 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("saved workflow should reload from disk: %#v", reloaded.List())
	}
}

func TestSaveWorkflowDraftRejectsInvalidDraft(t *testing.T) {
	service := NewServiceWithPaths(filepath.Join(t.TempDir(), "workflows.json"), "", fakeExecutor{})
	result := service.SaveWorkflowDraft(DraftSaveRequest{
		Draft: workmemory.WorkflowDraft{
			ID:    "workflow-draft-empty",
			Title: "空草稿",
		},
		Confirmed: true,
	})
	if result.OK || !strings.Contains(result.Message, "无效") {
		t.Fatalf("expected invalid draft rejection, got %#v", result)
	}
}

func copyResult(title string, value string) contracts.SearchResult {
	return contracts.SearchResult{
		ID:      "fake-" + stableID(title),
		Type:    contracts.ResultPluginResult,
		Title:   title,
		Icon:    "plugin",
		Preview: contracts.PreviewDescriptor{Kind: contracts.PreviewText, Title: title, Text: value},
		Actions: []contracts.PreviewAction{
			contracts.CopyAction("copy_value", "复制结果", value, ""),
		},
	}
}

func hasAction(actions []contracts.PreviewAction, id string) bool {
	for _, action := range actions {
		if action.ID == id {
			return true
		}
	}
	return false
}

func findAction(actions []contracts.PreviewAction, id string) *contracts.PreviewAction {
	for i := range actions {
		if actions[i].ID == id {
			return &actions[i]
		}
	}
	return nil
}
