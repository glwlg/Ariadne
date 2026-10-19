package airplay

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func TestControlHelper(t *testing.T) {
	for _, arg := range os.Args {
		if arg == "--control-helper" {
			os.Exit(0)
		}
	}
}

func TestControlRequiresPackagedHelperAndNeverStartsReceiver(t *testing.T) {
	s := newService(filepath.Join(t.TempDir(), "uxplay"), t.TempDir())
	if s.Status().ControlReady || s.Execute("control").OK {
		t.Fatal("missing control helper must fail explicitly")
	}
	if err := os.MkdirAll(filepath.Dir(s.controlExecutable()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.controlExecutable(), []byte("helper"), 0o600); err != nil {
		t.Fatal(err)
	}
	var called bool
	s.command = func(path string, args ...string) *exec.Cmd {
		called = true
		if path != s.controlExecutable() || len(args) != 2 ||
			args[0] != "--owner-pid" || args[1] != strconv.Itoa(os.Getpid()) {
			t.Fatalf("incorrect control launch: %s %v", path, args)
		}
		return exec.Command(os.Args[0], "-test.run=^TestControlHelper$", "--", "--control-helper")
	}
	if result := s.Execute("control"); !result.OK || !called || !s.Status().ControlReady || s.Status().Running {
		t.Fatalf("control must launch independently of video: %+v", result)
	}
}

func TestControlLaunchErrorsAreReported(t *testing.T) {
	s := newService(filepath.Join(t.TempDir(), "uxplay"), t.TempDir())
	if err := os.MkdirAll(filepath.Dir(s.controlExecutable()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.controlExecutable(), []byte("helper"), 0o600); err != nil {
		t.Fatal(err)
	}
	s.command = func(string, ...string) *exec.Cmd { return exec.Command(filepath.Join(t.TempDir(), "missing.exe")) }
	if s.OpenControl().OK {
		t.Fatal("failed launch must not report success")
	}
}
