package airplay

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestToolWindowStatusJSONAndLogs(t *testing.T) {
	service := testService(t, "listen")
	status := service.Status()
	raw, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if wire["receiverName"] != status.ReceiverName || wire["frameRate"] != float64(30) || wire["running"] != false {
		t.Fatalf("tool window status fields are missing: %s", raw)
	}
	if logs, err := service.Logs(); err != nil || logs != "" {
		t.Fatalf("new receiver should have empty logs: %q %v", logs, err)
	}
	if err := os.MkdirAll(service.dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(status.LogPath, []byte("receiver diagnostic"), 0o600); err != nil {
		t.Fatal(err)
	}
	if logs, err := service.Logs(); err != nil || logs != "receiver diagnostic" {
		t.Fatalf("tool window did not read receiver logs: %q %v", logs, err)
	}
}

func TestVideoEventsHandleChunksAndClearDisconnectedMetadata(t *testing.T) {
	var received []videoEvent
	writer := &videoEventWriter{accept: func(event videoEvent) { received = append(received, event) }}
	stream := "ordinary log\nARIADNE_VIDEO {\"phase\":\"mirroring\",\"width\":270,\"height\":480,\"title\":\"第1集\\n片名\",\"client\":\"iPhone\"}\n" +
		"ARIADNE_VIDEO {\"phase\":\"disconnected\",\"width\":270,\"height\":480,\"title\":\"stale\"}\n"
	for _, value := range []byte(stream) {
		if n, err := writer.Write([]byte{value}); n != 1 || err != nil {
			t.Fatalf("write failed: %d %v", n, err)
		}
	}
	if len(received) != 2 || received[0].Title != "第1集\n片名" ||
		received[1].Width != 0 || received[1].Title != "" || received[1].Client != "" {
		t.Fatalf("incorrect stream transitions: %+v", received)
	}
	for _, line := range []string{
		`ARIADNE_VIDEO {"phase":"mirroring","width":0,"height":480}`,
		`ARIADNE_VIDEO {"phase":"mirroring","width":999999,"height":480}`,
		`ARIADNE_VIDEO {"phase":"unknown"}`,
		"ARIADNE_VIDEO {invalid",
		strings.Repeat("x", 9000),
	} {
		fmt.Fprintln(writer, line)
	}
	if len(received) != 2 {
		t.Fatal("invalid events changed receiver state")
	}
	fmt.Fprintln(writer, `ARIADNE_VIDEO {"phase":"mirroring","width":480,"height":270}`)
	if len(received) != 3 || received[2].Width != 480 {
		t.Fatal("parser did not resume after oversized line")
	}
}

func TestFrameRatePersistsAndRestartIsExplicit(t *testing.T) {
	s := testService(t, "listen")
	if s.Status().FrameRate != 30 {
		t.Fatal("default should be 30 fps")
	}
	if result := s.Execute("fps 60"); !result.OK {
		t.Fatal(result.Message)
	}
	if s.Status().Running {
		t.Fatal("changing settings must not start an inactive receiver")
	}
	reloaded := newService(s.runtimeDir, s.dataDir)
	if reloaded.Status().FrameRate != 60 {
		t.Fatal("frame rate was not persisted")
	}
	if result := s.Start(); !result.OK {
		t.Fatal(result.Message)
	}
	pid := s.Status().ProcessID
	if result := s.Execute("fps 30"); !result.OK || !strings.Contains(result.Message, "重新连接") {
		t.Fatalf("running receiver change must explain reconnection: %+v", result)
	}
	status := s.Status()
	if !status.Running || status.FrameRate != 30 || status.ProcessID == pid || status.Phase != "waiting" {
		t.Fatalf("receiver was not restarted with the selected frame rate: %+v", status)
	}
	if result := s.Execute("fps 30"); !result.OK || s.Status().ProcessID != status.ProcessID {
		t.Fatal("same frame rate must not restart a receiver")
	}
	if result := s.SetFrameRate(120); result.OK || s.Status().FrameRate != 30 {
		t.Fatal("unsupported frame rate accepted")
	}
}

func TestInvalidPreferencesAreReported(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "preferences.json"), []byte(`{"frameRate":120}`), 0o600); err != nil {
		t.Fatal(err)
	}
	service := newService(t.TempDir(), dir)
	if status := service.Status(); !strings.Contains(status.LastError, "30 或 60") {
		t.Fatalf("invalid preferences were not reported: %+v", status)
	}
	if result := service.SetFrameRate(30); !result.OK || service.Status().LastError != "" {
		t.Fatalf("explicit frame rate selection must repair invalid preferences: %+v", result)
	}
}

func TestStateLabelsDistinguishListeningFromVideo(t *testing.T) {
	status := Status{Ready: true, Running: true, Phase: "waiting"}
	if status.StateLabel() != "等待 iPhone 连接" {
		t.Fatal("receiver process alone must not imply mirroring")
	}
	status.Phase = "mirroring"
	if status.StateLabel() != "正在投屏" {
		t.Fatal("missing active stream state")
	}
	status.Phase = "disconnected"
	if !strings.Contains(status.StateLabel(), "已断开") {
		t.Fatal("missing disconnected state")
	}
	status.Running = false
	if status.StateLabel() != "未启动" {
		t.Fatal("stopped receiver retained connection state")
	}
}

func TestReceiverConsumesNativeVideoEvents(t *testing.T) {
	s := testService(t, "video")
	if result := s.Start(); !result.OK {
		t.Fatal(result.Message)
	}
	status := s.Status()
	if status.Phase != "mirroring" || status.Width != 480 || status.ContentTitle != "第2集" {
		t.Fatalf("native stdout event was not applied: %+v", status)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && s.Status().Phase != "disconnected" {
		time.Sleep(20 * time.Millisecond)
	}
	if status := s.Status(); status.Phase != "disconnected" || status.ContentTitle != "" || !status.Running {
		t.Fatalf("disconnect must clear media while retaining receiver: %+v", status)
	}
}
