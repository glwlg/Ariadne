package airplay

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestReceiverHelper(t *testing.T) {
	for i, arg := range os.Args {
		if arg != "--airplay-helper" || i+1 >= len(os.Args) {
			continue
		}
		switch os.Args[i+1] {
		case "video":
			fmt.Println(`ARIADNE_VIDEO {"phase":"mirroring","width":480,"height":270,"title":"第2集","client":"Test iPhone"}`)
			time.Sleep(1300 * time.Millisecond)
			fmt.Println(`ARIADNE_VIDEO {"phase":"disconnected","width":480,"height":270,"title":"old episode"}`)
			time.Sleep(time.Minute)
			os.Exit(0)
		case "exit":
			fmt.Fprintln(os.Stderr, "receiver startup failed")
			os.Exit(7)
		case "listen":
			fmt.Println("receiver started")
			time.Sleep(time.Minute)
			os.Exit(0)
		}
	}
}

func testService(t *testing.T, mode string) *Service {
	t.Helper()
	base := t.TempDir()
	s := newService(filepath.Join(base, "runtime with spaces"), filepath.Join(base, "data"))
	if err := os.MkdirAll(s.runtimeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.runtimeDir, "uxplay.exe"), []byte("test receiver"), 0o600); err != nil {
		t.Fatal(err)
	}
	s.command = func(path string, args ...string) *exec.Cmd {
		if path != filepath.Join(s.runtimeDir, "uxplay.exe") {
			t.Errorf("unexpected executable: %s", path)
		}
		for _, flag := range []string{"-rc", "-pin", "-vs", "-as", "-nh", "-nofreeze"} {
			if !strings.Contains(strings.Join(args, "\x00"), flag) {
				t.Errorf("missing %s in %v", flag, args)
			}
		}
		return exec.Command(os.Args[0], "-test.run=^TestReceiverHelper$", "--", "--airplay-helper", mode)
	}
	t.Cleanup(func() { s.Stop() })
	return s
}

func TestStartIsIdempotentAndStopReapsOwnedProcess(t *testing.T) {
	s := testService(t, "listen")
	var group sync.WaitGroup
	for range 3 {
		group.Go(func() {
			if result := s.Start(); !result.OK {
				t.Errorf("start failed: %+v", result)
			}
		})
	}
	group.Wait()
	status := s.Status()
	if !status.Running || len(status.PIN) != 4 || status.ProcessID == 0 {
		t.Fatalf("missing running receiver state: %+v", status)
	}
	if result := s.Execute("status"); !result.OK || !strings.Contains(result.Message, status.PIN) {
		t.Fatalf("status must expose pairing code: %+v", result)
	}
	if result := s.Start(); !result.OK || s.Status().ProcessID != status.ProcessID {
		t.Fatalf("repeated start should reuse receiver: %+v", result)
	}
	if result := s.Stop(); !result.OK {
		t.Fatalf("stop failed: %+v", result)
	}
	stopped := s.Status()
	if stopped.Running || stopped.PIN != "" || stopped.ProcessID != 0 || stopped.LastError != "" {
		t.Fatalf("stopped process retained live state: %+v", stopped)
	}
	if result := s.Stop(); !result.OK {
		t.Fatalf("repeated stop should succeed: %+v", result)
	}
}

func TestStartupExitIsReportedAndCanRestart(t *testing.T) {
	s := testService(t, "exit")
	if result := s.Start(); result.OK || !strings.Contains(result.Message, "exit status 7") {
		t.Fatalf("startup exit was not reported: %+v", result)
	}
	if s.Status().Running || s.Status().LastError == "" {
		t.Fatalf("failed receiver is marked running: %+v", s.Status())
	}
	logText, err := ReadLog(s.Status().LogPath)
	if err != nil || !strings.Contains(logText, "receiver startup failed") {
		t.Fatalf("missing failure diagnostics: %q, %v", logText, err)
	}
	s.command = func(string, ...string) *exec.Cmd {
		return exec.Command(os.Args[0], "-test.run=^TestReceiverHelper$", "--", "--airplay-helper", "listen")
	}
	if result := s.Start(); !result.OK || s.Status().LastError != "" {
		t.Fatalf("restart should clear failure: %+v, %+v", result, s.Status())
	}
}

func TestMissingRuntimeAndUnknownCommandsFail(t *testing.T) {
	s := newService(t.TempDir(), t.TempDir())
	if s.Status().Ready || s.Start().OK || s.Execute("firewall").OK {
		t.Fatal("missing runtime must not start or configure the firewall")
	}
	if result := s.Execute("start; calc.exe"); result.OK {
		t.Fatalf("arbitrary command accepted: %+v", result)
	}
}

func TestReceiverEnvironmentIsolatedFromExternalGStreamer(t *testing.T) {
	s := testService(t, "listen")
	t.Setenv("GST_PLUGIN_PATH_1_0", "external plugins")
	t.Setenv("GST_PLUGIN_SCANNER_1_0", "external scanner")
	t.Setenv("GST_REGISTRY_1_0", "external cache")
	env := strings.Join(s.environment(), "\n")
	if strings.Contains(env, "external ") {
		t.Fatalf("external GStreamer configuration leaked: %s", env)
	}
	for _, want := range []string{
		"GST_REGISTRY_1_0=" + filepath.Join(s.dataDir, "gstreamer-registry.bin"),
		"GST_PLUGIN_SYSTEM_PATH_1_0=" + filepath.Join(s.runtimeDir, "lib", "gstreamer-1.0"),
		"GST_PLUGIN_SCANNER_1_0=" + filepath.Join(s.runtimeDir, "libexec", "gstreamer-1.0", "gst-plugin-scanner.exe"),
	} {
		if !strings.Contains(env, want) {
			t.Fatalf("missing receiver environment: %s", want)
		}
	}
}

func TestFirewallIsScopedToReceiverOnPrivateLocalNetwork(t *testing.T) {
	s := testService(t, "listen")
	called := false
	s.elevate = func(command string, args []string) error {
		called = true
		line := strings.Join(args, " ")
		if command != "powershell.exe" {
			t.Fatalf("unexpected firewall program: %s", command)
		}
		for _, want := range []string{"-Profile Private", "-RemoteAddress LocalSubnet", "-Program '" + filepath.Join(s.runtimeDir, "uxplay.exe") + "'", "-WindowStyle Hidden"} {
			if !strings.Contains(line, want) {
				t.Errorf("missing firewall scope: %s", want)
			}
		}
		return nil
	}
	if result := s.Execute("firewall"); !result.OK || !called {
		t.Fatalf("firewall action not routed: %+v", result)
	}
}

func TestReadLogReturnsBoundedTailAndMissingFileError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "uxplay.log")
	if _, err := ReadLog(path); !os.IsNotExist(err) {
		t.Fatalf("missing log should preserve error: %v", err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("old\n", 2000)+"latest\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	text, err := ReadLog(path)
	if err != nil || len(text) > 4096 || !strings.HasSuffix(text, "latest\n") {
		t.Fatalf("incorrect log tail: bytes=%d, err=%v", len(text), err)
	}
}

func TestBundledRuntimeSmoke(t *testing.T) {
	runtimeDir := os.Getenv("ARIADNE_TEST_UXPLAY_RUNTIME")
	if runtimeDir == "" {
		t.Skip("set ARIADNE_TEST_UXPLAY_RUNTIME to test a built UxPlay runtime")
	}
	s := newService(runtimeDir, filepath.Join(t.TempDir(), "投屏验证"))
	s.name = "Ariadne · 接收测试"
	t.Cleanup(func() { s.Stop() })
	if result := s.Start(); !result.OK {
		text, _ := ReadLog(s.Status().LogPath)
		t.Fatalf("real receiver failed: %+v\n%s", result, text)
	}
	var text string
	var err error
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); {
		text, err = ReadLog(s.Status().LogPath)
		if !s.Status().Running || strings.Contains(text, "Initialized server socket(s)") {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil || !s.Status().Running || !strings.Contains(text, "Initialized server socket(s)") {
		t.Fatalf("real receiver did not initialize: %+v, %v\n%s", s.Status(), err, text)
	}
	t.Logf("UxPlay PID %d initialized; log:\n%s", s.Status().ProcessID, text)
	if result := s.Stop(); !result.OK || s.Status().Running {
		t.Fatalf("real receiver did not stop: %+v", result)
	}
}
