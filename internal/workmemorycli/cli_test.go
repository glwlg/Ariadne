package workmemorycli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestRunSearchesWorkMemoryThroughCLI(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	t.Setenv("LOCALAPPDATA", root)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{
		"add-note",
		"--title", "微信沟通",
		"--text", "叶志伟提醒心流应该通过 agent skill 查询本地记忆。",
		"--tags", "微信,心流",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("add-note failed: code=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"search", "--query", "叶志伟 心流", "--limit", "5"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("search failed: code=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var result Output
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	if !result.OK || len(result.Results) == 0 || result.Results[0].Title != "微信沟通" {
		t.Fatalf("expected search hit, got %#v", result)
	}
	if result.ConfigPath != filepath.Join(root, "Ariadne", "config.json") {
		t.Fatalf("unexpected config path: %s", result.ConfigPath)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"get", "--id", result.Results[0].ID}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("get failed: code=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var detail Output
	if err := json.Unmarshal(stdout.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v\n%s", err, stdout.String())
	}
	if detail.Entry == nil || detail.Entry.Text == "" {
		t.Fatalf("expected entry detail, got %#v", detail)
	}
}
