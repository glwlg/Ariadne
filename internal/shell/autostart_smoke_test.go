package shell

import (
	"strings"
	"testing"
)

func TestAutostartSmokeCommandIncludesHiddenArgument(t *testing.T) {
	command := autostartSmokeCommand(`C:\Program Files\Ariadne\ariadne.exe`)
	if command != `"C:\Program Files\Ariadne\ariadne.exe" --hidden` {
		t.Fatalf("unexpected smoke command: %q", command)
	}
	audit := buildAutostartAudit("com.glwlg.ariadne.smoke", "com.glwlg.ariadne.smoke", command, `C:\Program Files\Ariadne\ariadne.exe`)
	if !audit.CommandValid || !audit.HiddenArgPresent || !audit.CommandMatchesExe {
		t.Fatalf("smoke command should pass autostart audit: %#v", audit)
	}
}

func TestAutostartSmokeValueNameUsesExplicitName(t *testing.T) {
	if got := autostartSmokeValueName("AriadneSmoke"); got != "AriadneSmoke" {
		t.Fatalf("explicit value name should be preserved, got %q", got)
	}
}

func TestAutostartSmokeValueNameDefaultsToTemporaryAriadneName(t *testing.T) {
	got := autostartSmokeValueName("")
	if !strings.HasPrefix(got, "com.glwlg.ariadne.smoke.") {
		t.Fatalf("default smoke value should be scoped to Ariadne, got %q", got)
	}
}
