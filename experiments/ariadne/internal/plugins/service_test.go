package plugins

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"ariadne/internal/contracts"
)

func TestListIncludesMigratedBuiltins(t *testing.T) {
	service := NewService()
	got := map[string]bool{}
	for _, plugin := range service.List() {
		got[plugin.ID] = true
	}

	want := []string{
		"calculator",
		"timestamp",
		"base64",
		"hash",
		"json",
		"json_compare",
		"url",
		"uuid",
		"custom_launch",
		"qr",
		"qr_scan",
		"system_commands",
		"hosts",
		"clipboard",
		"capture_overlay",
		"capture_history",
		"network_monitor",
		"workflow",
		"work_memory",
		"legacy_python",
	}
	for _, id := range want {
		if !got[id] {
			t.Fatalf("missing plugin manifest %q", id)
		}
	}
}

func TestLegacyPythonBuiltinsHaveNativeGoCoverage(t *testing.T) {
	service := NewService()
	nativeIDs := map[string]bool{}
	for _, plugin := range service.List() {
		nativeIDs[plugin.ID] = true
	}

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not resolve test file path")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", "..", ".."))
	legacyFiles, err := filepath.Glob(filepath.Join(repoRoot, "src", "plugins", "*.py"))
	if err != nil {
		t.Fatalf("glob legacy plugins: %v", err)
	}
	if len(legacyFiles) == 0 {
		t.Fatalf("expected legacy Python plugins under %s", filepath.Join(repoRoot, "src", "plugins"))
	}

	legacyToNative := map[string]string{
		"base64_tool.py":          "base64",
		"calculator.py":           "calculator",
		"capture_history_tool.py": "capture_history",
		"clipboard_tool.py":       "clipboard",
		"custom_launch_tool.py":   "custom_launch",
		"hash_tool.py":            "hash",
		"hosts_tool.py":           "hosts",
		"json_compare_tool.py":    "json_compare",
		"json_tool.py":            "json",
		"qr_tool.py":              "qr",
		"system_cmds.py":          "system_commands",
		"timestamp.py":            "timestamp",
		"url_tool.py":             "url",
		"uuid_tool.py":            "uuid",
		"workflow_tool.py":        "workflow",
		"work_memory_tool.py":     "work_memory",
	}

	seen := map[string]bool{}
	for _, file := range legacyFiles {
		base := filepath.Base(file)
		nativeID, ok := legacyToNative[base]
		if !ok {
			t.Fatalf("legacy plugin %s has no explicit native Go coverage mapping", base)
		}
		if nativeID == "legacy_python" {
			t.Fatalf("legacy plugin %s must not be covered only by the legacy bridge", base)
		}
		if !nativeIDs[nativeID] {
			t.Fatalf("legacy plugin %s maps to missing native plugin manifest %q", base, nativeID)
		}
		seen[base] = true
	}

	for legacyFile := range legacyToNative {
		if !seen[legacyFile] {
			t.Fatalf("native coverage map references missing legacy plugin %s", legacyFile)
		}
	}
}

func TestTextPluginsExecuteOnGoPath(t *testing.T) {
	service := NewService()
	cases := []struct {
		name  string
		query string
		want  string
	}{
		{name: "calculator", query: "calc 12*(8+3)", want: "132"},
		{name: "base64", query: "base64 hello", want: "aGVsbG8="},
		{name: "hash", query: "hash hello", want: "SHA256"},
		{name: "json", query: `json {"service":"ariadne","ok":true}`, want: `"service": "ariadne"`},
		{name: "url", query: "url hello world", want: "hello%20world"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			results := service.Search(tc.query)
			if len(results) == 0 {
				t.Fatalf("expected results for %s", tc.query)
			}
			if err := contracts.ValidateActionSurfaces(results); err != nil {
				t.Fatalf("invalid action surface: %v", err)
			}
			joined := joinResultText(results)
			if !strings.Contains(joined, tc.want) {
				t.Fatalf("expected %q in results:\n%s", tc.want, joined)
			}
		})
	}
}

func TestURLPluginMatchesLegacyQuoteSemantics(t *testing.T) {
	service := NewService()
	cases := []struct {
		name    string
		query   string
		want    string
		notWant string
	}{
		{name: "spaces use percent20", query: "url hello world", want: "编码结果: hello%20world"},
		{name: "slashes stay safe", query: "url https://a.com?q=中文", want: "https%3A//a.com%3Fq%3D%E4%B8%AD%E6%96%87"},
		{name: "percent escapes decode", query: "url hello%20world", want: "解码结果: hello world"},
		{name: "plus is literal", query: "url a+b", want: "编码结果: a%2Bb", notWant: "解码结果: a b"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			results := service.Search(tc.query)
			if len(results) == 0 {
				t.Fatalf("expected results for %s", tc.query)
			}
			if err := contracts.ValidateActionSurfaces(results); err != nil {
				t.Fatalf("invalid action surface: %v", err)
			}
			joined := joinResultText(results)
			if !strings.Contains(joined, tc.want) {
				t.Fatalf("expected %q in results:\n%s", tc.want, joined)
			}
			if tc.notWant != "" && strings.Contains(joined, tc.notWant) {
				t.Fatalf("did not expect %q in results:\n%s", tc.notWant, joined)
			}
		})
	}
}

func TestJSONPluginMatchesLegacyDumpSemantics(t *testing.T) {
	service := NewService()
	results := service.Search(`json {"name":"阿里阿德涅","html":"<tag>&","items":[1,2]}`)
	if len(results) < 2 {
		t.Fatalf("expected format and minify results, got %#v", results)
	}
	if err := contracts.ValidateActionSurfaces(results); err != nil {
		t.Fatalf("invalid action surface: %v", err)
	}
	joined := joinResultText(results)
	for _, want := range []string{
		"\n    \"name\": \"阿里阿德涅\"",
		"\"html\": \"<tag>&\"",
		"压缩结果: {\"html\":\"<tag>&\",\"items\":[1,2],\"name\":\"阿里阿德涅\"}",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected %q in results:\n%s", want, joined)
		}
	}
	for _, notWant := range []string{"\\u003c", "\\u003e", "\\u0026", "\\u963f"} {
		if strings.Contains(joined, notWant) {
			t.Fatalf("did not expect escaped value %q in results:\n%s", notWant, joined)
		}
	}
}

func TestBase64PluginMatchesLegacyUTF8DecodeSemantics(t *testing.T) {
	service := NewService()
	textResults := service.Search("base64 5L2g5aW9")
	textJoined := joinResultText(textResults)
	if !strings.Contains(textJoined, "解码结果: 你好") {
		t.Fatalf("expected UTF-8 base64 decode result:\n%s", textJoined)
	}

	binaryResults := service.Search("base64 /w==")
	if err := contracts.ValidateActionSurfaces(binaryResults); err != nil {
		t.Fatalf("invalid action surface: %v", err)
	}
	binaryJoined := joinResultText(binaryResults)
	if strings.Contains(binaryJoined, "解码结果:") {
		t.Fatalf("binary base64 should not expose decode result like legacy Python:\n%s", binaryJoined)
	}
	if !strings.Contains(binaryJoined, "编码结果:") {
		t.Fatalf("expected encode result to remain available:\n%s", binaryJoined)
	}
}

func TestCopyOnlyPluginResultsDoNotExposeFileActions(t *testing.T) {
	service := NewService()
	for _, query := range []string{"uuid 2", "qr https://example.test", "base64 hello", `json {"ok":true}`} {
		t.Run(query, func(t *testing.T) {
			results := service.Search(query)
			if len(results) == 0 {
				t.Fatal("expected results")
			}
			if err := contracts.ValidateActionSurfaces(results); err != nil {
				t.Fatalf("invalid action surface: %v", err)
			}
			for _, result := range results {
				for _, action := range result.Actions {
					if action.Kind == contracts.ActionOpenParent {
						t.Fatalf("%s exposed open_parent action", result.ID)
					}
					if action.Label == "打开文件" || action.Label == "打开所在文件夹" {
						t.Fatalf("%s exposed file-only label %q", result.ID, action.Label)
					}
				}
			}
		})
	}
}

func TestPluginTriggersExposeCommandSchemaForCompletion(t *testing.T) {
	service := NewService()
	results := service.triggerResults("")
	if len(results) == 0 {
		t.Fatal("expected plugin trigger results")
	}

	for _, result := range results {
		t.Run(result.ID, func(t *testing.T) {
			if result.Type != contracts.ResultPluginTrigger {
				t.Fatalf("expected plugin trigger, got %s", result.Type)
			}
			if err := contracts.ValidateActionSurface(result); err != nil {
				t.Fatalf("invalid action surface: %v", err)
			}
			schema, ok := result.Payload["commandSchema"].(CommandSchema)
			if !ok {
				t.Fatalf("expected commandSchema payload, got %#v", result.Payload["commandSchema"])
			}
			if strings.TrimSpace(schema.Usage) == "" {
				t.Fatal("command schema must expose usage for completion panel")
			}
			keyword, ok := result.Payload["keyword"].(string)
			if !ok || keyword == "" {
				t.Fatalf("expected keyword payload, got %#v", result.Payload["keyword"])
			}
			action := result.Actions[0]
			if action.ID != "prepare_command" || action.Kind != contracts.ActionRun {
				t.Fatalf("expected prepare command action, got %#v", action)
			}
			if action.Payload["command"] != keyword {
				t.Fatalf("prepare command should use completion keyword %q, got %#v", keyword, action.Payload)
			}
		})
	}
}

func TestPluginTriggerCompletionPrefersUsageKeyword(t *testing.T) {
	service := NewService()
	results := service.Search("calculator")
	if len(results) == 0 {
		t.Fatal("expected calculator trigger")
	}
	if results[0].Payload["keyword"] != "calc" {
		t.Fatalf("calculator completion should prefer usage keyword calc, got %#v", results[0].Payload["keyword"])
	}
	if results[0].Actions[0].Payload["command"] != "calc" {
		t.Fatalf("prepare action should fill calc, got %#v", results[0].Actions[0].Payload)
	}
}

func TestPluginEnabledSettingsHideTriggersAndBlockExecution(t *testing.T) {
	service := NewService()
	service.ApplyEnabled(map[string]bool{"uuid": false})

	for _, result := range service.triggerResults("") {
		if result.Payload["pluginId"] == "uuid" {
			t.Fatalf("disabled uuid plugin should not be listed in triggers: %#v", result)
		}
	}

	results := service.Search("uuid")
	if len(results) != 1 {
		t.Fatalf("expected disabled plugin feedback, got %#v", results)
	}
	if results[0].ID != "plugin-disabled-uuid" {
		t.Fatalf("expected disabled plugin result, got %#v", results[0])
	}

	service.ApplyEnabled(map[string]bool{"uuid": true})
	results = service.Search("uuid")
	if len(results) == 0 || results[0].ID == "plugin-disabled-uuid" {
		t.Fatalf("re-enabled uuid plugin should return normal results, got %#v", results)
	}
}

func TestSystemCommandsMarkDangerousActions(t *testing.T) {
	service := NewService()
	lockResults := service.Search("sys lock")
	if len(lockResults) != 1 {
		t.Fatalf("expected lock result, got %d", len(lockResults))
	}
	lockAction := lockResults[0].Actions[0]
	if lockAction.ID != "run_system" || lockAction.Kind != contracts.ActionRun || lockAction.Payload["requiresConfirmation"] != true {
		t.Fatalf("expected lock to be a guarded system action, got %#v", lockAction)
	}

	results := service.Search("sys shutdown")
	if len(results) != 1 {
		t.Fatalf("expected shutdown result, got %d", len(results))
	}
	shutdownAction := results[0].Actions[0]
	if shutdownAction.ID != "run_system" || shutdownAction.Kind != contracts.ActionDanger || shutdownAction.Payload["requiresConfirmation"] != true {
		t.Fatalf("expected shutdown to be a dangerous guarded system action, got %#v", shutdownAction)
	}
}

func TestNetworkMonitorOpensToolCenter(t *testing.T) {
	service := NewService()
	results := service.Search("net")
	if len(results) != 1 {
		t.Fatalf("expected one network monitor result, got %d", len(results))
	}
	action := results[0].Actions[0]
	if action.ID != "open_tool" || action.Payload["command"] != "open_network_monitor" {
		t.Fatalf("expected network monitor open_tool action, got %#v", action)
	}
	if len(results[0].Actions) < 2 || results[0].Actions[1].Payload["command"] != "open_network_mini" {
		t.Fatalf("expected network monitor to expose mini-window action, got %#v", results[0].Actions)
	}
}

func TestNetworkMonitorMiniOpensCompactToolWindow(t *testing.T) {
	service := NewService()
	results := service.Search("net mini")
	if len(results) != 1 {
		t.Fatalf("expected one network mini result, got %d", len(results))
	}
	if results[0].ID != "network-mini" {
		t.Fatalf("expected network-mini result, got %#v", results[0])
	}
	action := results[0].Actions[0]
	if action.ID != "open_tool" || action.Payload["command"] != "open_network_mini" {
		t.Fatalf("expected network mini open_tool action, got %#v", action)
	}
}

func TestCustomLaunchPluginOpensSettingsManager(t *testing.T) {
	service := NewService()
	results := service.Search("launch code")
	if len(results) != 1 {
		t.Fatalf("expected one custom launch manager result, got %#v", results)
	}
	if results[0].ID != "custom-launch-manager" {
		t.Fatalf("expected custom launch manager result, got %#v", results[0])
	}
	if err := contracts.ValidateActionSurface(results[0]); err != nil {
		t.Fatalf("invalid action surface: %v", err)
	}
	action := results[0].Actions[0]
	if action.ID != "open_tool" || action.Payload["command"] != "open_settings" {
		t.Fatalf("expected settings open_tool action, got %#v", action)
	}
	for _, action := range results[0].Actions {
		if action.Kind == contracts.ActionOpenParent || action.Label == "打开文件" || action.Label == "打开所在文件夹" {
			t.Fatalf("custom launch manager should not expose file-only action: %#v", action)
		}
	}
	if !strings.Contains(joinResultText(results), "code") {
		t.Fatalf("query should be reflected in preview text: %s", joinResultText(results))
	}
}

func TestCaptureOverlayOpensFromShotQuery(t *testing.T) {
	service := NewService()
	results := service.Search("shot")
	if len(results) < 1 || results[0].ID != "capture-overlay" {
		t.Fatalf("expected capture overlay first, got %#v", results)
	}
	action := results[0].Actions[0]
	if action.ID != "open_tool" || action.Payload["command"] != "open_capture_overlay" {
		t.Fatalf("expected capture overlay open_tool action, got %#v", action)
	}
}

func TestLegacyPythonWithoutBridgeReturnsDiagnostic(t *testing.T) {
	service := NewService()
	results := service.Search("legacy echo hello")
	if len(results) != 1 {
		t.Fatalf("expected one diagnostic result, got %#v", results)
	}
	if results[0].ID != "legacy-python-disabled" {
		t.Fatalf("expected disabled diagnostic, got %#v", results[0])
	}
	if err := contracts.ValidateActionSurfaces(results); err != nil {
		t.Fatalf("invalid action surface: %v", err)
	}
}

func TestLegacyPythonDelegatesToMountedBridge(t *testing.T) {
	service := NewServiceWithLegacyBridge(fakeLegacyBridge{})
	results := service.Search("legacy echo hello")
	if len(results) != 1 {
		t.Fatalf("expected one bridged result, got %#v", results)
	}
	if results[0].Title != "legacy:echo:hello" {
		t.Fatalf("unexpected bridge result: %#v", results[0])
	}
	if err := contracts.ValidateActionSurfaces(results); err != nil {
		t.Fatalf("invalid action surface: %v", err)
	}
}

type fakeLegacyBridge struct{}

func (fakeLegacyBridge) Execute(keyword string, query string) []contracts.SearchResult {
	return []contracts.SearchResult{copyResult("fake-legacy", "legacy:"+keyword+":"+query, "Python 旧插件", query, "fake bridge", []string{"legacy"})}
}

func joinResultText(results []contracts.SearchResult) string {
	var builder strings.Builder
	for _, result := range results {
		builder.WriteString(result.Title)
		builder.WriteString("\n")
		builder.WriteString(result.Detail)
		builder.WriteString("\n")
		builder.WriteString(result.Preview.Text)
		builder.WriteString("\n")
	}
	return builder.String()
}
