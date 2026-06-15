package legacybridge

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ariadne/internal/contracts"
)

func TestListManifestsFromLegacyPythonPlugin(t *testing.T) {
	python := requirePython(t)
	root := writeLegacyWorkspace(t, false)
	service := New(Options{Enabled: true, PythonPath: python, WorkspaceRoot: root, Timeout: 5 * time.Second})

	manifests, err := service.List()
	if err != nil {
		t.Fatalf("list manifests: %v", err)
	}
	for _, manifest := range manifests {
		if manifest.ID == "echo" && manifest.Name == "Legacy Echo" {
			if len(manifest.Keywords) != 1 || manifest.Keywords[0] != "echo" {
				t.Fatalf("unexpected keywords: %#v", manifest.Keywords)
			}
			return
		}
	}
	t.Fatalf("expected echo manifest, got %#v", manifests)
}

func TestExecuteMapsLegacyResultsToExplicitActions(t *testing.T) {
	python := requirePython(t)
	root := writeLegacyWorkspace(t, false)
	service := New(Options{Enabled: true, PythonPath: python, WorkspaceRoot: root, Timeout: 5 * time.Second})

	results := service.Execute("echo", "hello")
	if len(results) != 1 {
		t.Fatalf("expected one result, got %#v", results)
	}
	if results[0].Title != "Echo hello" {
		t.Fatalf("unexpected title: %#v", results[0])
	}
	if results[0].Type != contracts.ResultPluginResult {
		t.Fatalf("legacy result must stay a plugin result, got %s", results[0].Type)
	}
	if err := contracts.ValidateActionSurfaces(results); err != nil {
		t.Fatalf("invalid action surface: %v", err)
	}
	for _, action := range results[0].Actions {
		if action.Kind == contracts.ActionOpenParent || action.Label == "打开文件" || action.Label == "打开所在文件夹" {
			t.Fatalf("legacy result exposed file action: %#v", action)
		}
	}
}

func TestExecuteReturnsDiagnosticWhenBridgeDisabled(t *testing.T) {
	service := New(Options{Enabled: false})
	results := service.Execute("echo", "hello")
	if len(results) != 1 {
		t.Fatalf("expected one diagnostic result, got %#v", results)
	}
	if !strings.Contains(results[0].Title, "不可用") {
		t.Fatalf("expected unavailable diagnostic, got %#v", results[0])
	}
	if err := contracts.ValidateActionSurfaces(results); err != nil {
		t.Fatalf("invalid diagnostic action surface: %v", err)
	}
}

func TestExecuteConvertsLegacyExceptionsToDiagnostics(t *testing.T) {
	python := requirePython(t)
	root := writeLegacyWorkspace(t, true)
	service := New(Options{Enabled: true, PythonPath: python, WorkspaceRoot: root, Timeout: 5 * time.Second})

	results := service.Execute("echo", "hello")
	if len(results) != 1 {
		t.Fatalf("expected one error result, got %#v", results)
	}
	if results[0].Title != "旧插件执行失败" {
		t.Fatalf("unexpected title: %#v", results[0])
	}
	if err := contracts.ValidateActionSurfaces(results); err != nil {
		t.Fatalf("invalid action surface: %v", err)
	}
}

func TestExecuteFallsBackToEmbeddedRunnerWhenSourcePathMissing(t *testing.T) {
	python := requirePython(t)
	root := writeLegacyWorkspace(t, false)
	t.Setenv("LOCALAPPDATA", filepath.Join(t.TempDir(), "cache"))
	missingRunner := filepath.Join(t.TempDir(), "missing-runner.py")
	service := New(Options{Enabled: true, PythonPath: python, WorkspaceRoot: root, RunnerPath: missingRunner, Timeout: 5 * time.Second})

	results := service.Execute("echo", "hello")
	if len(results) != 1 {
		t.Fatalf("expected one result, got %#v", results)
	}
	if results[0].Title != "Echo hello" {
		t.Fatalf("unexpected result from embedded runner fallback: %#v", results[0])
	}
}

func requirePython(t *testing.T) string {
	t.Helper()
	python, err := exec.LookPath("python")
	if err != nil {
		t.Skip("python executable not available")
	}
	return python
}

func writeLegacyWorkspace(t *testing.T, failing bool) string {
	t.Helper()
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, "src", "core"))
	mkdirAll(t, filepath.Join(root, "src", "plugins"))
	writeFile(t, filepath.Join(root, "src", "__init__.py"), "")
	writeFile(t, filepath.Join(root, "src", "core", "__init__.py"), "")
	writeFile(t, filepath.Join(root, "src", "plugins", "__init__.py"), "")
	writeFile(t, filepath.Join(root, "src", "core", "plugin_base.py"), `
class PluginBase:
    def get_name(self):
        return self.__class__.__name__
    def get_description(self):
        return ""
    def get_keywords(self):
        return []
    def get_command_schema(self):
        return {}
    def get_supported_platforms(self):
        return ["windows"]
    def get_required_capabilities(self):
        return []
    def is_direct_action(self):
        return False
    def execute(self, query):
        return []
`)
	executeBody := `return [{"name": "Echo " + query, "path": query, "type": "copy_result"}]`
	if failing {
		executeBody = `raise RuntimeError("boom")`
	}
	writeFile(t, filepath.Join(root, "src", "plugins", "echo_plugin.py"), `
from src.core.plugin_base import PluginBase

class EchoPlugin(PluginBase):
    def get_name(self):
        return "Legacy Echo"
    def get_description(self):
        return "Echoes legacy results"
    def get_keywords(self):
        return ["echo"]
    def get_command_schema(self):
        return {"usage": "echo <text>", "examples": ["echo hello"]}
    def execute(self, query):
        `+executeBody+`
`)
	return root
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.TrimLeft(content, "\n")), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
