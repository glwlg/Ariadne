package ocr

import (
	"os/exec"
	"testing"
)

func TestConfigureOCRCommandHidesWindowsConsole(t *testing.T) {
	cmd := exec.Command("python.exe")
	configureOCRCommand(cmd)
	if cmd.SysProcAttr == nil {
		t.Fatal("expected Windows OCR command to set SysProcAttr")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Fatal("expected Windows OCR command to hide the Python window")
	}
	if cmd.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Fatalf("expected Windows OCR command to use CREATE_NO_WINDOW, got %#x", cmd.SysProcAttr.CreationFlags)
	}
}
