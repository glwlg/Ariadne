package airplay

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestControlHelper(t *testing.T) {
	for _, arg := range os.Args {
		if arg == "--control-helper" {
			os.Exit(0)
		}
		if arg == "--control-helper-crash" {
			time.Sleep(600 * time.Millisecond)
			os.Exit(7)
		}
		if arg == "--control-helper-wait" {
			time.Sleep(time.Minute)
			os.Exit(0)
		}
	}
}

func TestMirroringStartsControlInBackgroundOnceAndReusesIt(t *testing.T) {
	s := testService(t, "listen")
	s.queryControl = func() (ControlStatus, error) { return ControlStatus{}, os.ErrNotExist }
	if err := os.MkdirAll(filepath.Dir(s.controlExecutable()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.controlExecutable(), []byte("helper"), 0o600); err != nil {
		t.Fatal(err)
	}
	receiverCommand := s.command
	var starts atomic.Int32
	s.command = func(path string, args ...string) *exec.Cmd {
		if path != s.controlExecutable() {
			return receiverCommand(path, args...)
		}
		starts.Add(1)
		if strings.Join(args, " ") != "--owner-pid "+strconv.Itoa(os.Getpid())+" --background" {
			t.Errorf("automatic control must launch hidden: %v", args)
		}
		return exec.Command(os.Args[0], "-test.run=^TestControlHelper$", "--", "--control-helper-wait")
	}
	t.Cleanup(func() {
		s.Stop()
		s.operations.Lock()
		s.mu.Lock()
		p := s.controlProcess
		s.mu.Unlock()
		if p != nil {
			p.cmd.Process.Kill()
			<-p.done
		}
		s.operations.Unlock()
	})
	if result := s.Start(); !result.OK {
		t.Fatal(result.Message)
	}
	if starts.Load() != 0 {
		t.Fatal("waiting for a phone must not start control")
	}
	s.mu.Lock()
	receiver := s.process
	s.mu.Unlock()
	for range 5 {
		s.acceptVideoEvent(receiver, videoEvent{Phase: "mirroring", Width: 480, Height: 270})
	}
	for deadline := time.Now().Add(5 * time.Second); starts.Load() == 0; {
		if time.Now().After(deadline) {
			t.Fatal("mirroring did not start control")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Wait for the background start to complete before testing reconnect.
	s.operations.Lock()
	s.operations.Unlock()
	s.acceptVideoEvent(receiver, videoEvent{Phase: "disconnected"})
	s.acceptVideoEvent(receiver, videoEvent{Phase: "mirroring", Width: 270, Height: 480})
	time.Sleep(400 * time.Millisecond)
	if starts.Load() != 1 || !s.Status().Running {
		t.Fatalf("repeated video events and reconnect must reuse control: %d starts", starts.Load())
	}
}

func TestBackgroundControlReusesLiveHelperAndRejectsStaleReceiverEvents(t *testing.T) {
	s := newService(t.TempDir(), t.TempDir())
	s.queryControl = func() (ControlStatus, error) {
		return ControlStatus{Running: true, Phase: "direct"}, nil
	}
	s.command = func(string, ...string) *exec.Cmd {
		t.Error("a live helper or stale receiver must not start another control process")
		return exec.Command(os.Args[0], "-test.run=^TestControlHelper$", "--", "--control-helper")
	}
	if result := s.startControlLocked(true); !result.OK {
		t.Fatal(result.Message)
	}
	s.queryControl = func() (ControlStatus, error) { return ControlStatus{}, os.ErrNotExist }
	stale := &receiverProcess{}
	s.acceptVideoEvent(stale, videoEvent{Phase: "mirroring"})
	time.Sleep(100 * time.Millisecond)
}

func TestControlRequiresPackagedHelperAndNeverStartsReceiver(t *testing.T) {
	s := newService(filepath.Join(t.TempDir(), "uxplay"), t.TempDir())
	s.queryControl = func() (ControlStatus, error) { return ControlStatus{}, os.ErrNotExist }
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

func TestControlTracksExitAfterStartupWindow(t *testing.T) {
	s := newService(filepath.Join(t.TempDir(), "uxplay"), t.TempDir())
	s.queryControl = func() (ControlStatus, error) { return ControlStatus{}, os.ErrNotExist }
	if err := os.MkdirAll(filepath.Dir(s.controlExecutable()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.controlExecutable(), []byte("helper"), 0o600); err != nil {
		t.Fatal(err)
	}
	s.command = func(string, ...string) *exec.Cmd {
		return exec.Command(os.Args[0], "-test.run=^TestControlHelper$", "--", "--control-helper-crash")
	}
	if result := s.OpenControl(); !result.OK {
		t.Fatalf("helper should still be starting after 300ms: %+v", result)
	}
	s.mu.Lock()
	p := s.controlProcess
	s.mu.Unlock()
	if p == nil {
		t.Fatal("control process must be tracked")
	}
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
		t.Fatal("helper exit was not observed")
	}
	status := s.Status()
	if status.Control.Phase != "exited" || status.Control.Running || !strings.Contains(status.Control.LastError, "exit status 7") {
		t.Fatalf("late helper crash must appear in status: %+v", status.Control)
	}
}

func TestControlStatusUsesLiveReadinessAndReportsReadErrors(t *testing.T) {
	s := newService(filepath.Join(t.TempDir(), "uxplay"), t.TempDir())
	s.queryControl = func() (ControlStatus, error) {
		return ControlStatus{Phase: "direct", Running: true, ProcessID: 42, Advertising: true,
			ConnectedDevices: 1, TargetName: "Test iPhone", Readiness: 2}, nil
	}
	status := s.controlStatus(true)
	if status.Phase != "direct" || status.Readiness != 2 || status.TargetName != "Test iPhone" {
		t.Fatalf("must expose selected-device readiness: %+v", status)
	}
	s.queryControl = func() (ControlStatus, error) { return ControlStatus{}, errors.New("pipe timeout") }
	status = s.controlStatus(true)
	if status.Phase != "unavailable" || !strings.Contains(status.LastError, "pipe timeout") {
		t.Fatalf("failed status read must not report stopped or ready: %+v", status)
	}
	if status = s.controlStatus(false); status.Phase != "not-installed" {
		t.Fatalf("missing component: %+v", status)
	}
}

func TestDiagnosticsIncludesBothLogsAndBoundedNativeStatus(t *testing.T) {
	s := newService(filepath.Join(t.TempDir(), "uxplay"), t.TempDir())
	s.queryControl = func() (ControlStatus, error) { return ControlStatus{}, os.ErrNotExist }
	for name, content := range map[string]string{"uxplay.log": "receiver log", "control.log": strings.Repeat("x", 5000) + "control log"} {
		if err := os.WriteFile(filepath.Join(s.dataDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	text, err := s.Diagnostics()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "receiver log") || !strings.Contains(text, "control log") ||
		!strings.Contains(text, `"phase": "not-installed"`) || len(text) > 6500 {
		t.Fatalf("diagnostics must include bounded receiver/control logs and state: len=%d\n%s", len(text), text)
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
