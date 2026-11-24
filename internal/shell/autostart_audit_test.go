package shell

import (
	"strings"
	"testing"
)

func TestBuildAutostartAuditAcceptsCurrentExeWithHiddenArg(t *testing.T) {
	audit := buildAutostartAudit(
		"com.glwlg.fancun",
		"com.glwlg.fancun",
		`"C:\Program Files\Fancun\fancun.exe" --hidden`,
		`C:\Program Files\Fancun\fancun.exe`,
	)

	if !audit.CommandValid || !audit.CommandMatchesExe || !audit.HiddenArgPresent {
		t.Fatalf("expected valid hidden autostart command, got %#v", audit)
	}
	if len(audit.Notes) != 0 {
		t.Fatalf("valid command should not produce notes: %#v", audit.Notes)
	}
}

func TestBuildAutostartAuditFlagsVisibleStartup(t *testing.T) {
	audit := buildAutostartAudit(
		"com.glwlg.fancun",
		"com.glwlg.fancun",
		`C:\Tools\fancun.exe`,
		`C:\Tools\fancun.exe`,
	)

	if audit.CommandValid || audit.HiddenArgPresent {
		t.Fatalf("missing --hidden should invalidate startup command: %#v", audit)
	}
	if !strings.Contains(strings.Join(audit.Notes, "\n"), "--hidden") {
		t.Fatalf("expected --hidden note, got %#v", audit.Notes)
	}
}

func TestBuildAutostartAuditFlagsWrongExecutableAndValueName(t *testing.T) {
	audit := buildAutostartAudit(
		"com.glwlg.fancun",
		"FancunOld",
		`"C:\Old\x-tools.exe" --hidden`,
		`C:\Tools\fancun.exe`,
	)

	if audit.CommandValid || audit.CommandMatchesExe {
		t.Fatalf("wrong executable should invalidate startup command: %#v", audit)
	}
	notes := strings.Join(audit.Notes, "\n")
	if !strings.Contains(notes, "identifier") || !strings.Contains(notes, "可执行文件") {
		t.Fatalf("expected value-name and executable notes, got %#v", audit.Notes)
	}
}

func TestWindowsCommandLineTokensPreserveQuotedPathAndArgs(t *testing.T) {
	tokens := windowsCommandLineTokens(`"C:\Program Files\Fancun\fancun.exe" --hidden --flag "quoted value"`)
	want := []string{`C:\Program Files\Fancun\fancun.exe`, "--hidden", "--flag", "quoted value"}
	if len(tokens) != len(want) {
		t.Fatalf("tokens=%#v want %#v", tokens, want)
	}
	for index := range want {
		if tokens[index] != want[index] {
			t.Fatalf("tokens=%#v want %#v", tokens, want)
		}
	}
}
