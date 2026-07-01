//go:build windows

package elevation

import (
	"strings"
	"testing"
)

func TestCommandLineEscapesArguments(t *testing.T) {
	line := commandLine([]string{"filesearch-service-install", `C:\Program Files\Ariadne\ariadne.exe`})
	if !strings.Contains(line, "filesearch-service-install") || !strings.Contains(line, `"C:\Program Files\Ariadne\ariadne.exe"`) {
		t.Fatalf("arguments should be Windows-escaped, got %q", line)
	}
}

func TestRunasUsesVisibleFiniteProcessWait(t *testing.T) {
	if shellExecuteShowCommand() != swShowNormal {
		t.Fatalf("runas should not request a hidden elevated window")
	}
	if shellExecuteMask() != seeMaskNoCloseProcess {
		t.Fatalf("runas should only keep a process handle for bounded waiting, mask=%#x", shellExecuteMask())
	}
	if elevatedProcessTimeout <= 0 {
		t.Fatal("runas wait timeout should be finite")
	}
}
