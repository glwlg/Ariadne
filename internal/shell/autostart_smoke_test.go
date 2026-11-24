package shell

import (
	"strings"
	"testing"
)

func TestAutostartSmokeCommandIncludesHiddenArgument(t *testing.T) {
	command := autostartSmokeCommand(`C:\Program Files\Fancun\fancun.exe`)
	if command != `"C:\Program Files\Fancun\fancun.exe" --hidden` {
		t.Fatalf("unexpected smoke command: %q", command)
	}
	audit := buildAutostartAudit("com.glwlg.fancun.smoke", "com.glwlg.fancun.smoke", command, `C:\Program Files\Fancun\fancun.exe`)
	if !audit.CommandValid || !audit.HiddenArgPresent || !audit.CommandMatchesExe {
		t.Fatalf("smoke command should pass autostart audit: %#v", audit)
	}
}

func TestAutostartSmokeValueNameUsesExplicitName(t *testing.T) {
	if got := autostartSmokeValueName("FancunSmoke"); got != "FancunSmoke" {
		t.Fatalf("explicit value name should be preserved, got %q", got)
	}
}

func TestAutostartSmokeValueNameDefaultsToTemporaryFancunName(t *testing.T) {
	got := autostartSmokeValueName("")
	if !strings.HasPrefix(got, "com.glwlg.fancun.smoke.") {
		t.Fatalf("default smoke value should be scoped to Fancun, got %q", got)
	}
}
