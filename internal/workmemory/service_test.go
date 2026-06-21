package workmemory

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ariadne/internal/appdb"
	"ariadne/internal/capturehistory"
	"ariadne/internal/clipboardhistory"
	"ariadne/internal/contracts"
)

type fakeOCRSummarizer struct {
	calls   int
	lastJob OCRSummaryJob
	result  OCRSummaryResult
	err     error
}

func (f *fakeOCRSummarizer) SummarizeOCR(ctx context.Context, job OCRSummaryJob) (OCRSummaryResult, error) {
	f.calls++
	f.lastJob = job
	if f.err != nil {
		return OCRSummaryResult{}, f.err
	}
	return f.result, nil
}

func TestSearchReturnsEvidenceBackedMemoryResults(t *testing.T) {
	service := NewServiceWithPath("", nil)
	results := service.Search("OpenWrt")
	if len(results) == 0 {
		t.Fatal("expected work memory search result")
	}
	if err := contracts.ValidateActionSurfaces(results); err != nil {
		t.Fatalf("invalid action surface: %v", err)
	}
	if results[0].Preview.Evidence[0].Value == "" {
		t.Fatal("expected evidence metadata")
	}
}

func TestPersistentStoreStartsEmptyWithoutDemoSeed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "work_memory.json")
	service := NewServiceWithPath(path, nil)
	defer service.Stop()

	if status := service.Status(); status.EntryCount != 0 {
		t.Fatalf("new persistent work memory store should start empty, got %#v", status)
	}
	if _, err := os.Stat(ftsPathForMemoryPath(path)); !os.IsNotExist(err) {
		t.Fatalf("empty persistent store should not create FTS database on startup, stat err=%v", err)
	}
}

func TestPrivacyModeBlocksTimeMachineAndManualCapture(t *testing.T) {
	capturer := &fakeCapturer{}
	service := NewServiceWithPath("", capturer)
	service.SetPrivacyMode(true)

	status := service.SetTimeMachineEnabled(true)
	if status.TimeMachineEnabled {
		t.Fatal("time machine should stay disabled in privacy mode")
	}

	entry := service.CaptureCurrentScreen()
	if entry.ID != "" {
		t.Fatal("manual capture should be blocked in privacy mode")
	}
	if capturer.calls != 0 {
		t.Fatalf("privacy mode should not call capturer, got %d calls", capturer.calls)
	}
	if !strings.Contains(service.Status().PauseReason, "隐私模式") {
		t.Fatalf("expected privacy pause reason, got %q", service.Status().PauseReason)
	}

	status = service.SetPrivacyMode(false)
	if status.PrivacyMode {
		t.Fatal("privacy mode should be disabled")
	}
	if status.PauseReason != "" {
		t.Fatalf("privacy pause reason should clear, got %q", status.PauseReason)
	}
}

func TestManualCaptureCreatesCaptureBackedMemoryEntry(t *testing.T) {
	capturer := &fakeCapturer{}
	service := NewServiceWithPath("", capturer)

	entry := service.CaptureCurrentScreen()

	if entry.ID == "" || entry.CaptureID == "" || entry.ImagePath == "" {
		t.Fatalf("expected capture-backed memory entry, got %#v", entry)
	}
	if entry.Source != "manual_capture" || entry.ContentType != "screenshot" {
		t.Fatalf("unexpected entry source/type: %#v", entry)
	}
	if service.Status().LastCaptureID != entry.ID || service.Status().CaptureCount != 1 {
		t.Fatalf("status should record capture: %#v", service.Status())
	}
	if capturer.calls != 1 || capturer.sources[0] != "work_memory_manual_capture" {
		t.Fatalf("unexpected capturer calls: %#v", capturer.sources)
	}
}

func TestTimeMachineOneShotCaptureAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "work_memory.json")
	capturer := &fakeCapturer{}
	service := NewServiceWithPath(path, capturer)
	service.ApplyCapturePolicy(CapturePolicy{PauseOnIdle: false, PauseOnLock: false})

	status := service.SetTimeMachineEnabled(true)
	if !status.TimeMachineEnabled {
		t.Fatalf("time machine should be enabled: %#v", status)
	}
	entry := service.CaptureTimeMachineNow()

	if entry.Source != "time_machine" || entry.CaptureID == "" {
		t.Fatalf("expected time machine capture entry, got %#v", entry)
	}
	if !strings.Contains(entry.Summary, "后台时间机器") {
		t.Fatalf("expected time-machine summary, got %q", entry.Summary)
	}
	service.Stop()

	reloaded := NewServiceWithPath(path, nil)
	defer reloaded.Stop()
	found := false
	for _, item := range reloaded.Timeline() {
		if item.ID == entry.ID && item.CaptureID == entry.CaptureID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected persisted time-machine entry in %#v", reloaded.Timeline())
	}
}

func TestTimeMachineMergesDuplicateScreenSignatures(t *testing.T) {
	capturer := &fakeCapturer{}
	service := NewServiceWithPath("", capturer)
	clearEntriesForTest(service)
	service.ApplyCapturePolicy(CapturePolicy{PauseOnIdle: false, PauseOnLock: false})

	first := service.CaptureTimeMachineNow()
	second := service.CaptureTimeMachineNow()

	if first.ID == "" || second.ID != first.ID {
		t.Fatalf("duplicate time-machine capture should return the existing entry, first=%#v second=%#v", first, second)
	}
	if second.MergedCount != 1 || second.LastMergedAt == 0 {
		t.Fatalf("duplicate merge metadata missing: %#v", second)
	}
	status := service.Status()
	if status.EntryCount != 1 || status.LastSkippedReason != "重复画面已合并" || status.LastCaptureID != first.ID {
		t.Fatalf("status should report duplicate merge, got %#v", status)
	}
	if !containsString(second.Tags, "重复画面合并") {
		t.Fatalf("expected duplicate merge tag, got %#v", second.Tags)
	}

	manualA := service.CaptureCurrentScreen()
	manualB := service.CaptureCurrentScreen()
	if manualA.ID == "" || manualB.ID == "" || manualA.ID == manualB.ID {
		t.Fatalf("manual captures should not be merged, got %#v %#v", manualA, manualB)
	}
	if service.Status().EntryCount != 3 {
		t.Fatalf("expected one merged time-machine entry plus two manual entries, got %#v", service.Status())
	}
}

func TestTimeMachineDoesNotMergeDuplicateScreenAcrossTimelineDays(t *testing.T) {
	service := NewServiceWithPath("", nil)
	clearEntriesForTest(service)
	yesterday := time.Date(2026, 6, 17, 23, 58, 0, 0, time.Local).Unix()
	today := time.Date(2026, 6, 18, 10, 56, 0, 0, time.Local).Unix()

	first := service.addEntry(Entry{
		ID:             "memory-time_machine-browser-yesterday",
		Source:         "time_machine",
		ContentType:    "screenshot",
		Title:          "浏览器任务页",
		AppName:        "msedge.exe",
		WindowTitle:    "数字化管理系统",
		CaptureID:      "capture-yesterday",
		ImagePath:      "browser-yesterday.png",
		ImageSignature: "same-browser-screen",
		CreatedAt:      yesterday,
	})
	second := service.addEntry(Entry{
		ID:             "memory-time_machine-browser-today",
		Source:         "time_machine",
		ContentType:    "screenshot",
		Title:          "浏览器任务页",
		AppName:        "msedge.exe",
		WindowTitle:    "数字化管理系统",
		CaptureID:      "capture-today",
		ImagePath:      "browser-today.png",
		ImageSignature: "same-browser-screen",
		CreatedAt:      today,
	})

	if first.ID == "" || second.ID == "" || second.ID == first.ID {
		t.Fatalf("same screen on a new day must stay visible on that day, first=%#v second=%#v", first, second)
	}
	if status := service.Status(); status.EntryCount != 2 || status.LastSkippedReason == "重复画面已合并" {
		t.Fatalf("cross-day duplicate should not be merged, got %#v", status)
	}
}

func TestTimeMachineDoesNotMergeDuplicateScreenAcrossWindows(t *testing.T) {
	service := NewServiceWithPath("", nil)
	clearEntriesForTest(service)
	now := time.Date(2026, 6, 18, 10, 56, 0, 0, time.Local).Unix()

	wechat := service.addEntry(Entry{
		ID:             "memory-time_machine-wechat",
		Source:         "time_machine",
		ContentType:    "screenshot",
		Title:          "微信",
		AppName:        "Weixin.exe",
		WindowTitle:    "微信",
		CaptureID:      "capture-wechat",
		ImagePath:      "wechat.png",
		ImageSignature: "same-rendered-screen",
		CreatedAt:      now,
	})
	browser := service.addEntry(Entry{
		ID:             "memory-time_machine-browser",
		Source:         "time_machine",
		ContentType:    "screenshot",
		Title:          "浏览器",
		AppName:        "msedge.exe",
		WindowTitle:    "数字化管理系统",
		CaptureID:      "capture-browser",
		ImagePath:      "browser.png",
		ImageSignature: "same-rendered-screen",
		CreatedAt:      now + 60,
	})

	if wechat.ID == "" || browser.ID == "" || browser.ID == wechat.ID {
		t.Fatalf("different windows must stay as separate timeline evidence, wechat=%#v browser=%#v", wechat, browser)
	}
	if status := service.Status(); status.EntryCount != 2 || status.LastSkippedReason == "重复画面已合并" {
		t.Fatalf("cross-window duplicate should not be merged, got %#v", status)
	}
}

func TestTimeMachineCaptureKeepsPreCaptureWindowContext(t *testing.T) {
	current := windowContext{title: "数字化管理系统", app: "msedge.exe"}
	capturer := &hookedCapturer{
		entry: testCaptureEntry("browser", "browser.png", "browser-screen", 1280, 720, time.Date(2026, 6, 18, 10, 56, 0, 0, time.Local).Unix()),
		onCapture: func() {
			current = windowContext{title: "微信", app: "Weixin.exe"}
		},
	}
	service := NewServiceWithPath("", capturer)
	clearEntriesForTest(service)
	service.context = func() windowContext { return current }
	service.ApplyCapturePolicy(CapturePolicy{PauseOnIdle: false, PauseOnLock: false})

	entry := service.CaptureTimeMachineNow()

	if entry.AppName != "msedge.exe" || entry.WindowTitle != "数字化管理系统" {
		t.Fatalf("capture should use pre-capture foreground context, got %#v", entry)
	}
}

func TestTimeMachineMergesSimilarScreenFingerprints(t *testing.T) {
	dir := t.TempDir()
	screenA := filepath.Join(dir, "screen-a.png")
	screenB := filepath.Join(dir, "screen-b.png")
	screenC := filepath.Join(dir, "screen-c.png")
	screenD := filepath.Join(dir, "screen-d.png")
	writeMemoryTestPNG(t, screenA, 96, 64, paintSimilarMemoryScreen(0))
	writeMemoryTestPNG(t, screenB, 96, 64, paintSimilarMemoryScreen(1))
	writeMemoryTestPNG(t, screenC, 96, 64, paintSimilarMemoryScreen(0))
	writeMemoryTestPNG(t, screenD, 96, 64, paintSimilarMemoryScreen(1))

	capturer := &sequenceCapturer{entries: []capturehistory.Entry{
		testCaptureEntry("similar-a", screenA, "fingerprint:a", 96, 64, 1770002101),
		testCaptureEntry("similar-b", screenB, "fingerprint:b", 96, 64, 1770002102),
		testCaptureEntry("manual-a", screenC, "fingerprint:c", 96, 64, 1770002103),
		testCaptureEntry("manual-b", screenD, "fingerprint:d", 96, 64, 1770002104),
	}}
	service := NewServiceWithPath("", capturer)
	clearEntriesForTest(service)
	service.ApplyCapturePolicy(CapturePolicy{PauseOnIdle: false, PauseOnLock: false})

	first := service.CaptureTimeMachineNow()
	second := service.CaptureTimeMachineNow()

	if first.ID == "" || second.ID != first.ID {
		t.Fatalf("similar time-machine capture should return the existing entry, first=%#v second=%#v", first, second)
	}
	if second.MergedCount != 1 || second.LastMergedAt == 0 {
		t.Fatalf("similar merge metadata missing: %#v", second)
	}
	status := service.Status()
	if status.EntryCount != 1 || status.LastSkippedReason != "相似画面已合并" || status.LastCaptureID != first.ID {
		t.Fatalf("status should report similar merge, got %#v", status)
	}
	if !containsString(second.Tags, "相似画面合并") {
		t.Fatalf("expected similar merge tag, got %#v", second.Tags)
	}

	manualA := service.CaptureCurrentScreen()
	manualB := service.CaptureCurrentScreen()
	if manualA.ID == "" || manualB.ID == "" || manualA.ID == manualB.ID {
		t.Fatalf("manual captures should not be merged by similar fingerprints, got %#v %#v", manualA, manualB)
	}
	if service.Status().EntryCount != 3 {
		t.Fatalf("expected one merged time-machine entry plus two manual entries, got %#v", service.Status())
	}
}

func TestTimeMachineDoesNotMergeDifferentScreenFingerprints(t *testing.T) {
	dir := t.TempDir()
	dark := filepath.Join(dir, "screen-dark.png")
	light := filepath.Join(dir, "screen-light.png")
	writeMemoryTestPNG(t, dark, 96, 64, paintSolidMemoryScreen(32))
	writeMemoryTestPNG(t, light, 96, 64, paintSolidMemoryScreen(232))

	capturer := &sequenceCapturer{entries: []capturehistory.Entry{
		testCaptureEntry("dark", dark, "fingerprint:dark", 96, 64, 1770002201),
		testCaptureEntry("light", light, "fingerprint:light", 96, 64, 1770002202),
	}}
	service := NewServiceWithPath("", capturer)
	clearEntriesForTest(service)
	service.ApplyCapturePolicy(CapturePolicy{PauseOnIdle: false, PauseOnLock: false})

	first := service.CaptureTimeMachineNow()
	second := service.CaptureTimeMachineNow()

	if first.ID == "" || second.ID == "" || second.ID == first.ID {
		t.Fatalf("different time-machine captures should both be kept, first=%#v second=%#v", first, second)
	}
	status := service.Status()
	if status.EntryCount != 2 || status.LastSkippedReason == "相似画面已合并" {
		t.Fatalf("different captures should not be marked as similar merge, got %#v", status)
	}
}

func TestTimeMachineMergeKeepsCollapsedFrames(t *testing.T) {
	service := NewServiceWithPath("", nil)
	clearEntriesForTest(service)
	baseAt := time.Date(2026, 6, 18, 9, 0, 0, 0, time.Local).Unix()

	first := service.addEntry(Entry{
		ID:             "memory-time_machine-weixin-first",
		Source:         "time_machine",
		ContentType:    "screenshot",
		Title:          "微信会话",
		AppName:        "Weixin.exe",
		WindowTitle:    "微信",
		CaptureID:      "capture-weixin-first",
		ImagePath:      "weixin-first.png",
		ImageSignature: "same-weixin-screen",
		Width:          1280,
		Height:         720,
		CreatedAt:      baseAt,
	})
	second := service.addEntry(Entry{
		ID:             "memory-time_machine-weixin-second",
		Source:         "time_machine",
		ContentType:    "screenshot",
		Title:          "微信会话",
		AppName:        "Weixin.exe",
		WindowTitle:    "微信",
		CaptureID:      "capture-weixin-second",
		ImagePath:      "weixin-second.png",
		ImageSignature: "same-weixin-screen",
		Width:          1280,
		Height:         720,
		CreatedAt:      baseAt + 5*60,
	})

	if first.ID == "" || second.ID != first.ID {
		t.Fatalf("recent same-window duplicate should collapse into the first entry, first=%#v second=%#v", first, second)
	}
	if second.FrameCount != 2 || len(second.Frames) != 2 {
		t.Fatalf("collapsed captures should remain available as frames, got %#v", second)
	}
	if second.Frames[0].CaptureID != "capture-weixin-first" || second.Frames[1].CaptureID != "capture-weixin-second" {
		t.Fatalf("unexpected collapsed frame order: %#v", second.Frames)
	}
	if second.CaptureID != "capture-weixin-second" {
		t.Fatalf("collapsed entry should display the newest frame, got %#v", second)
	}
}

func TestTimeMachineDoesNotMergeSameWindowAfterSimilarityWindow(t *testing.T) {
	service := NewServiceWithPath("", nil)
	clearEntriesForTest(service)
	baseAt := time.Date(2026, 6, 18, 9, 0, 0, 0, time.Local).Unix()

	first := service.addEntry(Entry{
		ID:             "memory-time_machine-browser-first",
		Source:         "time_machine",
		ContentType:    "screenshot",
		Title:          "浏览器任务页",
		AppName:        "msedge.exe",
		WindowTitle:    "数字化管理系统",
		CaptureID:      "capture-browser-first",
		ImagePath:      "browser-first.png",
		ImageSignature: "same-browser-screen",
		Width:          1280,
		Height:         720,
		CreatedAt:      baseAt,
	})
	second := service.addEntry(Entry{
		ID:             "memory-time_machine-browser-second",
		Source:         "time_machine",
		ContentType:    "screenshot",
		Title:          "浏览器任务页",
		AppName:        "msedge.exe",
		WindowTitle:    "数字化管理系统",
		CaptureID:      "capture-browser-second",
		ImagePath:      "browser-second.png",
		ImageSignature: "same-browser-screen",
		Width:          1280,
		Height:         720,
		CreatedAt:      baseAt + int64(timeMachineSimilarityMergeWindow.Seconds()) + 1,
	})

	if first.ID == "" || second.ID == "" || second.ID == first.ID {
		t.Fatalf("same window after merge window should stay visible as a new entry, first=%#v second=%#v", first, second)
	}
	if status := service.Status(); status.EntryCount != 2 || status.LastSkippedReason == "重复画面已合并" {
		t.Fatalf("expired duplicate should not be reported as collapsed, got %#v", status)
	}
}

func TestTimeMachineWindowSessionKeepsChangedScreensAsNewEntries(t *testing.T) {
	service := NewServiceWithPath("", nil)
	clearEntriesForTest(service)
	baseAt := time.Date(2026, 6, 18, 9, 0, 0, 0, time.Local).Unix()
	context := windowContext{title: "数字化管理系统", app: "msedge.exe"}
	service.mu.Lock()
	service.currentWindowSessionSignature = windowSignature(context)
	service.currentWindowSessionStartedAt = baseAt
	service.mu.Unlock()

	first := service.addEntry(Entry{
		ID:             "memory-time_machine-browser-list",
		Source:         "time_machine",
		ContentType:    "screenshot",
		Title:          "浏览器列表页",
		AppName:        context.app,
		WindowTitle:    context.title,
		CaptureID:      "capture-browser-list",
		ImagePath:      "browser-list.png",
		ImageSignature: "browser-list-screen",
		Width:          1280,
		Height:         720,
		CreatedAt:      baseAt,
	})
	second := service.addEntry(Entry{
		ID:             "memory-time_machine-browser-detail",
		Source:         "time_machine",
		ContentType:    "screenshot",
		Title:          "浏览器详情页",
		AppName:        context.app,
		WindowTitle:    context.title,
		CaptureID:      "capture-browser-detail",
		ImagePath:      "browser-detail.png",
		ImageSignature: "browser-detail-screen",
		Width:          1280,
		Height:         720,
		CreatedAt:      baseAt + 30,
	})

	if first.ID == "" || second.ID == "" || second.ID == first.ID {
		t.Fatalf("changed screen in the same window session should create a new entry, first=%#v second=%#v", first, second)
	}
	if status := service.Status(); status.EntryCount != 2 {
		t.Fatalf("changed session screens should both stay in timeline, got %#v", status)
	}
}

func TestDefaultWindowCaptureProfileUsesConfiguredProbeInterval(t *testing.T) {
	profile, ok := windowCaptureProfileForContext(
		windowContext{title: "数字化管理系统", app: "msedge.exe"},
		CapturePolicy{CaptureOnWindowChange: true, WindowChangeCooldown: 3},
		120,
	)
	if !ok {
		t.Fatal("expected default window capture profile")
	}
	if profile.ActiveIntervalSeconds != 120 {
		t.Fatalf("default window profile should keep configured probe interval, got %#v", profile)
	}
}

func TestTimeMachineWorkerCapturesOnInterval(t *testing.T) {
	capturer := &fakeCapturer{}
	service := NewServiceWithPath("", capturer)
	service.interval = 20 * time.Millisecond
	service.ApplyCapturePolicy(CapturePolicy{PauseOnIdle: false, PauseOnLock: false})
	defer service.Stop()

	status := service.SetTimeMachineEnabled(true)
	if !status.TimeMachineEnabled || !status.WorkerRunning {
		t.Fatalf("time machine worker should be running: %#v", status)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if capturer.calls > 0 && service.Status().CaptureCount > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected worker capture, calls=%d status=%#v", capturer.calls, service.Status())
}

func TestTimeMachineWorkerCapturesOnWindowChange(t *testing.T) {
	originalPoll := windowSwitchPollInterval
	windowSwitchPollInterval = 20 * time.Millisecond
	t.Cleanup(func() { windowSwitchPollInterval = originalPoll })

	capturer := &fakeCapturer{}
	service := NewServiceWithPath("", capturer)
	service.interval = time.Hour
	service.ApplyCapturePolicy(CapturePolicy{
		CaptureOnWindowChange: true,
		WindowChangeCooldown:  3,
		PauseOnIdle:           false,
		PauseOnLock:           false,
	})
	defer service.Stop()

	var mu sync.Mutex
	current := windowContext{title: "Ariadne settings", app: "ariadne.exe"}
	service.context = func() windowContext {
		mu.Lock()
		defer mu.Unlock()
		return current
	}

	status := service.SetTimeMachineEnabled(true)
	if !status.TimeMachineEnabled || !status.WorkerRunning || !status.WindowSwitchCaptureEnabled {
		t.Fatalf("time machine window-switch worker should be enabled: %#v", status)
	}

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if service.Status().LastWindowSwitchAt > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if capturer.calls != 0 {
		t.Fatalf("initial window baseline should not capture, got %d", capturer.calls)
	}

	mu.Lock()
	current = windowContext{title: "Terminal - go test", app: "WindowsTerminal.exe"}
	mu.Unlock()

	deadline = time.Now().Add(900 * time.Millisecond)
	for time.Now().Before(deadline) {
		if capturer.calls > 0 {
			t.Fatalf("window switch should wait for stable delay before capture, got %d calls", capturer.calls)
		}
		time.Sleep(20 * time.Millisecond)
	}

	deadline = time.Now().Add(3200 * time.Millisecond)
	for time.Now().Before(deadline) {
		status = service.Status()
		if capturer.calls > 0 && status.LastWindowSwitchCaptureAt > 0 {
			if status.CaptureCount == 0 {
				t.Fatalf("window switch should increment capture status: %#v", status)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected window switch capture, calls=%d status=%#v", capturer.calls, service.Status())
}

func TestTimeMachineAppProfileDelaysWindowSwitchCapture(t *testing.T) {
	originalPoll := windowSwitchPollInterval
	windowSwitchPollInterval = 20 * time.Millisecond
	t.Cleanup(func() { windowSwitchPollInterval = originalPoll })

	capturer := &fakeCapturer{}
	service := NewServiceWithPath("", capturer)
	service.interval = time.Hour
	service.ApplyCapturePolicy(CapturePolicy{
		AppCaptureProfiles: []AppCaptureProfile{
			{DisplayName: "微信", ProcessName: "Weixin.exe", Enabled: true, WindowSwitchDelaySeconds: 2, ActiveIntervalSeconds: 120},
		},
		PauseOnIdle: false,
		PauseOnLock: false,
	})
	defer service.Stop()

	var mu sync.Mutex
	current := windowContext{title: "Ariadne settings", app: "ariadne.exe"}
	service.context = func() windowContext {
		mu.Lock()
		defer mu.Unlock()
		return current
	}

	status := service.SetTimeMachineEnabled(true)
	if !status.TimeMachineEnabled || !status.WorkerRunning {
		t.Fatalf("time machine should be running: %#v", status)
	}
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if service.Status().LastWindowSwitchAt > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	current = windowContext{title: "微信 - 文件传输助手", app: "Weixin.exe"}
	mu.Unlock()

	time.Sleep(350 * time.Millisecond)
	if capturer.calls != 0 {
		t.Fatalf("profile should wait for switch delay before capture, got %d calls", capturer.calls)
	}

	deadline = time.Now().Add(2600 * time.Millisecond)
	for time.Now().Before(deadline) {
		status = service.Status()
		if capturer.calls > 0 && status.LastWindowSwitchCaptureAt > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("expected delayed app profile capture, calls=%d status=%#v", capturer.calls, service.Status())
}

func TestTimeMachineAppProfileSkipsGlobalIntervalCapture(t *testing.T) {
	originalPoll := windowSwitchPollInterval
	windowSwitchPollInterval = 20 * time.Millisecond
	t.Cleanup(func() { windowSwitchPollInterval = originalPoll })

	capturer := &fakeCapturer{}
	service := NewServiceWithPath("", capturer)
	service.interval = 40 * time.Millisecond
	service.ApplyCapturePolicy(CapturePolicy{
		AppCaptureProfiles: []AppCaptureProfile{
			{DisplayName: "微信", ProcessName: "Weixin.exe", Enabled: true, WindowSwitchDelaySeconds: 3, ActiveIntervalSeconds: 120},
		},
		PauseOnIdle: false,
		PauseOnLock: false,
	})
	defer service.Stop()

	service.context = func() windowContext {
		return windowContext{title: "微信 - 私人空对话", app: "Weixin.exe"}
	}
	status := service.SetTimeMachineEnabled(true)
	if !status.TimeMachineEnabled || !status.WorkerRunning {
		t.Fatalf("time machine should be running: %#v", status)
	}

	time.Sleep(240 * time.Millisecond)
	if capturer.calls != 0 {
		t.Fatalf("profiled app should be governed by profile interval, got global calls=%d", capturer.calls)
	}
}

func TestTimeMachineWindowSessionAppendsActiveFramesToOneEntry(t *testing.T) {
	capturer := &fakeCapturer{}
	service := NewServiceWithPath("", capturer)
	clearEntriesForTest(service)
	service.interval = time.Second
	service.ApplyCapturePolicy(CapturePolicy{
		CaptureOnWindowChange: true,
		WindowChangeCooldown:  3,
		PauseOnIdle:           false,
		PauseOnLock:           false,
	})
	now := time.Unix(1800000000, 0)
	service.now = func() time.Time { return now }
	service.context = func() windowContext {
		return windowContext{title: "Ariadne - 心流", app: "ariadne.exe"}
	}
	service.mu.Lock()
	service.status.TimeMachineEnabled = true
	service.mu.Unlock()

	service.captureIfWindowChanged()
	if capturer.calls != 0 {
		t.Fatalf("initial window observation should only start pending session, got %d calls", capturer.calls)
	}

	now = now.Add(2 * time.Second)
	service.captureIfWindowChanged()
	if capturer.calls != 0 {
		t.Fatalf("window should not capture before the 3s stable delay, got %d calls", capturer.calls)
	}

	now = now.Add(time.Second)
	service.captureIfWindowChanged()
	if capturer.calls != 1 {
		t.Fatalf("window should capture after stable delay, got %d calls", capturer.calls)
	}

	now = now.Add(time.Second)
	service.captureIfWindowChanged()
	if capturer.calls != 2 {
		t.Fatalf("active window should capture again on stay interval, got %d calls", capturer.calls)
	}

	timeline := service.Timeline()
	if len(timeline) != 1 {
		t.Fatalf("same window session should keep one memory entry, got %#v", timeline)
	}
	entry := timeline[0]
	if entry.FrameCount != 2 || len(entry.Frames) != 2 || entry.QualityStatus != qualityStatusPending {
		t.Fatalf("same window captures should append pending frames, got %#v", entry)
	}
	if !strings.Contains(entry.Summary, "已采集 2 帧") {
		t.Fatalf("entry should explain multi-frame pending collection, got %q", entry.Summary)
	}
}

func TestWindowChangeTriggersReviewForEndedPendingSession(t *testing.T) {
	capturer := &fakeCapturer{}
	service := NewServiceWithPath("", capturer)
	clearEntriesForTest(service)
	service.interval = time.Second
	service.ApplyCapturePolicy(CapturePolicy{
		CaptureOnWindowChange: true,
		WindowChangeCooldown:  3,
		PauseOnIdle:           false,
		PauseOnLock:           false,
	})
	now := time.Unix(1800000100, 0)
	service.now = func() time.Time { return now }
	current := windowContext{title: "Ariadne - 心流", app: "ariadne.exe"}
	service.context = func() windowContext { return current }
	service.mu.Lock()
	service.status.TimeMachineEnabled = true
	service.mu.Unlock()

	service.captureIfWindowChanged()
	now = now.Add(3 * time.Second)
	service.captureIfWindowChanged()
	if capturer.calls != 1 {
		t.Fatalf("expected first stable window capture, got %d", capturer.calls)
	}
	firstID := service.Status().LastCaptureID
	first := entryByIDForTest(service.Timeline(), firstID)
	if first.QualityStatus != qualityStatusPending {
		t.Fatalf("active session should stay pending before window switch, got %#v", first)
	}

	now = now.Add(time.Second)
	current = windowContext{title: "Terminal - go test", app: "WindowsTerminal.exe"}
	service.captureIfWindowChanged()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		first = entryByIDForTest(service.Timeline(), firstID)
		if first.QualityStatus == qualityStatusChecked {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("ended pending session should be reviewed after window switch, got %#v", first)
}

func TestReviewPendingCapturesCollapsesDuplicateFramesAndChecksEntry(t *testing.T) {
	service := NewServiceWithPath("", nil)
	clearEntriesForTest(service)
	service.now = func() time.Time { return time.Unix(1800000300, 0) }
	service.addEntry(Entry{
		ID:             "memory-time-machine-duplicate",
		Source:         "time_machine",
		ContentType:    "screenshot",
		Title:          "重复窗口采集",
		Summary:        "窗口保持期间采集到重复画面。",
		CaptureID:      "capture-a",
		ImagePath:      "a.png",
		ImageSignature: "same-screen",
		WindowTitle:    "Ariadne",
		AppName:        "ariadne.exe",
		Width:          100,
		Height:         100,
		Tags:           []string{"待质检"},
		QualityStatus:  qualityStatusPending,
		QualityReason:  "待质检：窗口保持期间多帧采集",
		Frames: []CaptureFrame{
			{CaptureID: "capture-a", ImagePath: "a.png", ImageSignature: "same-screen", Width: 100, Height: 100, CreatedAt: 1800000000},
			{CaptureID: "capture-b", ImagePath: "b.png", ImageSignature: "same-screen", Width: 100, Height: 100, CreatedAt: 1800000030},
		},
		FrameCount: 2,
		CreatedAt:  1800000000,
	})

	result := service.ReviewPendingCaptures()
	if !result.OK || result.Checked != 1 || result.CollapsedEntries != 1 || result.RemovedFrames != 1 || result.PendingRemaining != 0 {
		t.Fatalf("unexpected review result: %#v", result)
	}

	timeline := service.Timeline()
	if len(timeline) != 1 {
		t.Fatalf("expected one reviewed entry, got %#v", timeline)
	}
	entry := timeline[0]
	if entry.QualityStatus != qualityStatusChecked || entry.QualityCheckedAt != 1800000300 {
		t.Fatalf("entry should be marked checked, got %#v", entry)
	}
	if entry.FrameCount != 1 || len(entry.Frames) != 1 || entry.CaptureID != "capture-b" || entry.ImagePath != "b.png" {
		t.Fatalf("review should keep the latest duplicate frame, got %#v", entry)
	}
	if containsString(entry.Tags, "待质检") || !containsString(entry.Tags, "已质检") {
		t.Fatalf("review should update quality tags, got %#v", entry.Tags)
	}
}

func TestHealthSummaryExplainsQualityAndOCRState(t *testing.T) {
	service := NewServiceWithPath("", nil)
	clearEntriesForTest(service)
	service.now = func() time.Time { return time.Unix(1800000600, 0) }
	service.addEntry(Entry{
		ID:            "memory-pending-window",
		Source:        "time_machine",
		ContentType:   "screenshot",
		Title:         "待质检窗口",
		Summary:       "窗口切换后采集到截图。",
		CaptureID:     "capture-pending",
		ImagePath:     "pending.png",
		AppName:       "Weixin.exe",
		QualityStatus: qualityStatusPending,
		QualityReason: "待质检：时间机器自动采集",
		Frames: []CaptureFrame{
			{CaptureID: "capture-pending", ImagePath: "pending.png", CreatedAt: 1800000500},
		},
		FrameCount: 1,
		CreatedAt:  1800000500,
	})
	service.addEntry(Entry{
		ID:               "memory-checked-ocr",
		Source:           "time_machine",
		ContentType:      "ocr_text",
		Title:            "已质检记录",
		Summary:          "已经完成 OCR 与总结。",
		CaptureID:        "capture-checked",
		ImagePath:        "checked.png",
		AppName:          "Code.exe",
		OCRText:          "Ariadne 心流改造",
		OCRStatus:        "done:model",
		QualityStatus:    qualityStatusChecked,
		QualityCheckedAt: 1800000550,
		QualityReason:    "自动质检：2 帧重复或近似重复，保留最后一帧",
		MergedCount:      1,
		CreatedAt:        1800000520,
	})

	health := service.HealthSummary()
	if !health.OK || health.Total != 2 || health.Pending != 1 || health.Checked != 1 || health.OCRDone != 1 {
		t.Fatalf("unexpected health summary: %#v", health)
	}
	if health.CollapsedEntries != 1 || health.RemovedFrames != 1 || health.LastQualityCheckAt != 1800000550 {
		t.Fatalf("health should expose cleanup state: %#v", health)
	}
	if !strings.Contains(health.Message, "等待质检") {
		t.Fatalf("health message should explain pending gate, got %q", health.Message)
	}
	if len(health.AppStats) == 0 || health.AppStats[0].Count == 0 {
		t.Fatalf("expected app stats, got %#v", health.AppStats)
	}
}

func TestGenerateWorkflowDraftFromEvidence(t *testing.T) {
	service := NewServiceWithPath("", nil)
	clearEntriesForTest(service)
	first := service.AddNote(NoteRequest{
		Title: "剪贴板 JSON 格式化",
		Text:  "每天复制 JSON 到剪贴板后手动 json format，再复制结果。",
		Tags:  []string{"workflow", "clipboard", "json"},
	})
	second := service.AddNote(NoteRequest{
		Title: "URL hash 工作流",
		Text:  "clip url 后继续 hash {prev}，这个宏应该沉淀为候选 workflow。",
		Tags:  []string{"workflow", "hash"},
	})
	pending := service.addEntry(Entry{
		ID:            "workflow-pending-frame",
		Source:        "time_machine",
		Title:         "未质检窗口截图",
		Summary:       "pending evidence should not be used.",
		QualityStatus: qualityStatusPending,
		CreatedAt:     time.Now().Unix(),
	})

	draft := service.GenerateWorkflowDraft("剪贴板格式化自动化机会", []string{first.ID, pending.ID, second.ID})

	if draft.ID == "" || !draft.RequiresReview || draft.RiskLevel != "low" {
		t.Fatalf("unexpected workflow draft metadata: %#v", draft)
	}
	if draft.Trigger == "" || !strings.Contains(draft.Input, "剪贴板") || len(draft.Steps) < 3 {
		t.Fatalf("expected clipboard workflow draft, got %#v", draft)
	}
	if len(draft.Evidence) != 2 || !containsString(draft.Evidence, first.ID) || !containsString(draft.Evidence, second.ID) || containsString(draft.Evidence, pending.ID) {
		t.Fatalf("expected checked evidence only: %#v", draft.Evidence)
	}
	for _, step := range draft.Steps {
		if strings.TrimSpace(step.Label) == "" || strings.TrimSpace(step.Command) == "" {
			t.Fatalf("step should be actionable: %#v", step)
		}
	}
}

func TestGenerateChecklistDraftFromEvidence(t *testing.T) {
	service := NewServiceWithPath("", nil)
	clearEntriesForTest(service)
	first := service.AddNote(NoteRequest{
		Title: "OpenWrt 网关异常",
		Text:  "网络代理失败，先确认 DNS、Hosts、OpenWrt 网关和 Cloudflare tunnel。",
		Tags:  []string{"network", "hosts"},
	})
	second := service.AddNote(NoteRequest{
		Title: "Hosts 冲突",
		Text:  "hosts preview 发现域名和旧 X-TOOLS marker 冲突，需要保留回滚。",
		Tags:  []string{"hosts"},
	})

	draft := service.GenerateChecklistDraft("网络排查经验", []string{first.ID, second.ID})

	if draft.ID == "" || !draft.RequiresReview || len(draft.Items) < 4 {
		t.Fatalf("unexpected checklist draft: %#v", draft)
	}
	joined := strings.Join(draft.Items, "\n")
	if !strings.Contains(joined, "Hosts") || !strings.Contains(joined, "网关") || !strings.Contains(joined, "确认") {
		t.Fatalf("expected network checklist items, got %#v", draft.Items)
	}
	if len(draft.Evidence) != 2 {
		t.Fatalf("expected two evidence IDs, got %#v", draft.Evidence)
	}
}

func TestCapturePolicyBlocksExcludedApp(t *testing.T) {
	capturer := &fakeCapturer{}
	service := NewServiceWithPath("", capturer)
	service.context = func() windowContext {
		return windowContext{title: "Remote Desktop session", app: `C:\Windows\System32\mstsc.exe`}
	}
	service.ApplyCapturePolicy(CapturePolicy{ExcludeApps: []string{"mstsc.exe"}})

	entry := service.CaptureTimeMachineNow()
	status := service.Status()

	if entry.ID != "" {
		t.Fatalf("excluded app should block capture, got %#v", entry)
	}
	if capturer.calls != 0 {
		t.Fatalf("excluded app should not call capturer, got %d", capturer.calls)
	}
	if !strings.Contains(status.PauseReason, "排除规则命中应用") || status.LastSkippedAt == 0 || status.LastSkippedReason == "" {
		t.Fatalf("expected exclusion status, got %#v", status)
	}
}

func TestCapturePolicyBlocksExcludedWindowKeyword(t *testing.T) {
	capturer := &fakeCapturer{}
	service := NewServiceWithPath("", capturer)
	service.context = func() windowContext {
		return windowContext{title: "SSO token confirmation", app: "browser.exe"}
	}
	service.ApplyCapturePolicy(CapturePolicy{ExcludeWindowKeywords: []string{"token"}})

	entry := service.CaptureCurrentScreen()
	status := service.Status()

	if entry.ID != "" {
		t.Fatalf("excluded window should block capture, got %#v", entry)
	}
	if capturer.calls != 0 {
		t.Fatalf("excluded window should not call capturer, got %d", capturer.calls)
	}
	if !strings.Contains(status.PauseReason, "排除规则命中窗口") || status.LastSkippedAt == 0 {
		t.Fatalf("expected excluded window status, got %#v", status)
	}
}

func TestCapturePolicyBlocksExcludedWindowURL(t *testing.T) {
	capturer := &fakeCapturer{}
	service := NewServiceWithPath("", capturer)
	service.context = func() windowContext {
		return windowContext{title: "Incident - https://private.example.com/ticket/42", app: "browser.exe"}
	}
	service.ApplyCapturePolicy(CapturePolicy{ExcludeURLs: []string{"private.example.com/ticket"}})

	entry := service.CaptureCurrentScreen()
	status := service.Status()

	if entry.ID != "" {
		t.Fatalf("excluded URL should block capture, got %#v", entry)
	}
	if capturer.calls != 0 {
		t.Fatalf("excluded URL should not call capturer, got %d", capturer.calls)
	}
	if !strings.Contains(status.PauseReason, "排除规则命中 URL") || status.LastSkippedAt == 0 {
		t.Fatalf("expected excluded URL status, got %#v", status)
	}
}

func TestApplySettingsReportsWorkerStateAndInterval(t *testing.T) {
	service := NewServiceWithPath("", &fakeCapturer{})
	defer service.Stop()

	status := service.ApplySettings(true, false, true, 10)
	if !status.TimeMachineEnabled || !status.WorkerRunning || status.AutoCaptureIntervalSeconds != 10 {
		t.Fatalf("expected worker enabled with 10s interval, got %#v", status)
	}
	status = service.ApplySettings(true, false, true, 20)
	if !status.TimeMachineEnabled || !status.WorkerRunning || status.AutoCaptureIntervalSeconds != 20 {
		t.Fatalf("expected worker to remain running with updated interval, got %#v", status)
	}
	status = service.ApplySettings(true, false, false, 20)
	if status.WorkerRunning || status.TimeMachineEnabled {
		t.Fatalf("expected worker stopped, got %#v", status)
	}
}

func TestTimeMachinePauseOnIdleSkipsCapture(t *testing.T) {
	capturer := &fakeCapturer{}
	service := NewServiceWithPath("", capturer)
	service.activity = activityProviderFunc(func(now time.Time) activitySnapshot {
		return activitySnapshot{Available: true, IdleSeconds: 900, LastActivityAt: now.Add(-15 * time.Minute).Unix()}
	})
	service.ApplyCapturePolicy(CapturePolicy{PauseOnIdle: true, IdlePauseSeconds: 600})

	entry := service.CaptureTimeMachineNow()
	status := service.Status()

	if entry.ID != "" {
		t.Fatalf("idle pause should skip capture, got %#v", entry)
	}
	if capturer.calls != 0 {
		t.Fatalf("idle pause should not call capturer, got %d", capturer.calls)
	}
	if !strings.Contains(status.PauseReason, "空闲") || status.IdleSeconds != 900 || status.LastActivityAt == 0 {
		t.Fatalf("expected idle pause status, got %#v", status)
	}
}

func TestManualCaptureIgnoresIdlePause(t *testing.T) {
	capturer := &fakeCapturer{}
	service := NewServiceWithPath("", capturer)
	service.activity = activityProviderFunc(func(now time.Time) activitySnapshot {
		return activitySnapshot{Available: true, IdleSeconds: 900, LastActivityAt: now.Add(-15 * time.Minute).Unix()}
	})
	service.ApplyCapturePolicy(CapturePolicy{PauseOnIdle: true, IdlePauseSeconds: 600})

	entry := service.CaptureCurrentScreen()

	if entry.ID == "" {
		t.Fatal("manual capture should remain available when only idle pause is active")
	}
	if capturer.calls != 1 {
		t.Fatalf("manual capture should call capturer once, got %d", capturer.calls)
	}
}

func TestTimeMachinePauseOnLockSkipsCapture(t *testing.T) {
	capturer := &fakeCapturer{}
	service := NewServiceWithPath("", capturer)
	service.activity = activityProviderFunc(func(now time.Time) activitySnapshot {
		return activitySnapshot{Available: true, IdleSeconds: 5, LastActivityAt: now.Add(-5 * time.Second).Unix(), SessionLocked: true}
	})
	service.ApplyCapturePolicy(CapturePolicy{PauseOnLock: true})

	entry := service.CaptureTimeMachineNow()
	status := service.Status()

	if entry.ID != "" {
		t.Fatalf("lock pause should skip capture, got %#v", entry)
	}
	if capturer.calls != 0 {
		t.Fatalf("lock pause should not call capturer, got %d", capturer.calls)
	}
	if !strings.Contains(status.PauseReason, "锁定") || !status.SessionLocked {
		t.Fatalf("expected lock pause status, got %#v", status)
	}
}

func TestCaptureStrategyIsRecordedOnCaptureEntries(t *testing.T) {
	service := NewServiceWithPath("", &fakeCapturer{})
	service.ApplyCapturePolicy(CapturePolicy{
		CaptureScope:     "active_window",
		MultiMonitor:     "primary_only",
		PauseOnIdle:      true,
		IdlePauseSeconds: 120,
		PauseOnLock:      true,
	})

	entry := service.CaptureCurrentScreen()
	status := service.Status()

	if entry.ID == "" {
		t.Fatal("expected capture entry")
	}
	if !strings.Contains(entry.Text, "采集范围: 前台窗口") || !containsString(entry.Tags, "多屏:仅主屏") {
		t.Fatalf("expected strategy metadata on entry, got text=%q tags=%#v", entry.Text, entry.Tags)
	}
	if len(service.capturer.(*fakeCapturer).options) != 1 || service.capturer.(*fakeCapturer).options[0].CaptureScope != "active_window" || service.capturer.(*fakeCapturer).options[0].MultiMonitor != "primary_only" {
		t.Fatalf("expected scoped capture options to reach capturer, got %#v", service.capturer.(*fakeCapturer).options)
	}
	if status.CaptureScope != "active_window" || status.MultiMonitor != "primary_only" || status.IdlePauseSeconds != 120 || !status.PauseOnIdle || !status.PauseOnLock {
		t.Fatalf("expected strategy status, got %#v", status)
	}
}

func TestAutoOCRProcessorRunsAfterCaptureWhenEnabled(t *testing.T) {
	service := NewServiceWithPath("", &fakeCapturer{})
	service.ApplyCapturePolicy(CapturePolicy{AutoOCR: true, PauseOnIdle: false, PauseOnLock: false})
	processorCalls := 0
	RegisterAutoOCRProcessor(service, func(entry Entry) Entry {
		processorCalls++
		return service.ApplyOCRText(entry.ID, "gateway timeout from automatic OCR", "test-auto-ocr")
	})

	entry := service.CaptureTimeMachineNow()
	if processorCalls != 0 || entry.QualityStatus != qualityStatusPending {
		t.Fatalf("time-machine capture should wait for quality review before OCR, calls=%d entry=%#v", processorCalls, entry)
	}
	review := service.ReviewPendingCaptures()
	status := service.Status()
	updated := entryByIDForTest(service.Timeline(), entry.ID)

	if review.Checked != 1 || processorCalls != 1 {
		t.Fatalf("expected review to trigger one auto OCR call, review=%#v calls=%d", review, processorCalls)
	}
	if updated.OCRText != "gateway timeout from automatic OCR" || updated.OCRStatus != "done:test-auto-ocr" || updated.ContentType != "ocr_text" || updated.QualityStatus != qualityStatusChecked {
		t.Fatalf("expected auto OCR writeback after review, got %#v", updated)
	}
	if status.LastAutoOCRID != entry.ID || status.LastAutoOCRAt == 0 || status.LastAutoOCRError != "" || !status.AutoOCREnabled {
		t.Fatalf("expected successful auto OCR status, got %#v", status)
	}
	if len(service.Search("automatic OCR")) == 0 {
		t.Fatal("auto OCR text should be searchable")
	}
}

func TestPendingOCRIsUsedForQualityBeforeFormalWriteback(t *testing.T) {
	service := NewServiceWithPath("", nil)
	pending := service.addEntry(Entry{
		ID:            "memory-pending-precheck",
		Source:        "time_machine",
		ContentType:   "screenshot",
		Title:         "待质检截图",
		Summary:       "等待质检",
		Text:          "等待质检",
		ImagePath:     "pending.png",
		QualityStatus: qualityStatusPending,
		CreatedAt:     1800000400,
	})

	updated := service.ApplyOCRText(pending.ID, "zqxprecheckneedle", "test-precheck")
	if updated.QualityOCRText != "zqxprecheckneedle" || updated.QualityOCRStatus != "done:test-precheck" || updated.OCRText != "" || updated.OCRStatus != "" {
		t.Fatalf("pending OCR should stay in quality fields, got %#v", updated)
	}
	if updated.QualityStatus != qualityStatusPending {
		t.Fatalf("pending OCR should not approve quality, got %#v", updated)
	}
	if len(service.Search("zqxprecheckneedle")) != 0 {
		t.Fatal("quality OCR text should not be searchable before review")
	}

	review := service.ReviewPendingCaptures()
	final := entryByIDForTest(service.Timeline(), pending.ID)
	if review.Checked != 1 || review.OCRPromoted != 1 {
		t.Fatalf("expected review to promote precheck OCR, got %#v", review)
	}
	if final.QualityStatus != qualityStatusChecked || final.OCRText != "zqxprecheckneedle" || final.OCRStatus != "done:test-precheck" {
		t.Fatalf("expected quality OCR to become formal OCR after review, got %#v", final)
	}
	if len(service.Search("zqxprecheckneedle")) == 0 {
		t.Fatal("formal OCR text should be searchable after review")
	}
}

func TestAutoOCRProcessorDoesNotRunWhenDisabled(t *testing.T) {
	service := NewServiceWithPath("", &fakeCapturer{})
	service.ApplyCapturePolicy(CapturePolicy{AutoOCR: false, PauseOnIdle: false, PauseOnLock: false})
	processorCalls := 0
	RegisterAutoOCRProcessor(service, func(entry Entry) Entry {
		processorCalls++
		return service.ApplyOCRText(entry.ID, "should not run", "test-auto-ocr")
	})

	entry := service.CaptureTimeMachineNow()
	status := service.Status()

	if processorCalls != 0 {
		t.Fatalf("auto OCR should not run when disabled, got %d calls", processorCalls)
	}
	if entry.OCRText != "" || status.LastAutoOCRAt != 0 || status.AutoOCREnabled {
		t.Fatalf("unexpected auto OCR state, entry=%#v status=%#v", entry, status)
	}
}

func TestAutoOCRProcessorFailureIsRecorded(t *testing.T) {
	service := NewServiceWithPath("", &fakeCapturer{})
	service.ApplyCapturePolicy(CapturePolicy{AutoOCR: true, PauseOnIdle: false, PauseOnLock: false})
	RegisterAutoOCRProcessor(service, func(entry Entry) Entry {
		return service.ApplyOCRText(entry.ID, "", "failed: OCR 不可用")
	})

	entry := service.CaptureTimeMachineNow()
	if entry.OCRStatus != "" || entry.QualityStatus != qualityStatusPending {
		t.Fatalf("time-machine capture should stay pending before review, got %#v", entry)
	}
	service.ReviewPendingCaptures()
	status := service.Status()
	updated := entryByIDForTest(service.Timeline(), entry.ID)

	if updated.OCRStatus != "failed: OCR 不可用" {
		t.Fatalf("expected failed OCR status, got %#v", updated)
	}
	if status.LastAutoOCRID != entry.ID || status.LastAutoOCRError != "OCR 不可用" {
		t.Fatalf("expected auto OCR error status, got %#v", status)
	}
}

func TestDraftsAndAgentTaskPackageKeepEvidence(t *testing.T) {
	service := NewServiceWithPath("", nil)

	daily := service.GenerateDailyDraft()
	if len(daily.Evidence) == 0 {
		t.Fatal("daily draft should keep evidence")
	}

	knowledge := service.GenerateKnowledgeDraft([]string{"memory-gateway"})
	if len(knowledge.Evidence) != 1 || knowledge.Evidence[0] != "memory-gateway" {
		t.Fatalf("knowledge draft evidence mismatch: %#v", knowledge.Evidence)
	}

	task := service.GenerateAgentTaskPackage("迁移 Hosts 管理", []string{"memory-gateway"})
	if !task.RequiresReview {
		t.Fatal("external agent task package must require review")
	}
	if len(task.Boundaries) == 0 || len(task.Acceptance) == 0 {
		t.Fatal("task package should include boundaries and acceptance criteria")
	}
}

func TestGenerateDailyDraftBuildsLocalReportAndSkipsSensitive(t *testing.T) {
	service := NewServiceWithPath("", nil)
	service.entries = nil
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.Local)
	service.now = func() time.Time { return now }
	service.addEntry(Entry{
		ID:        "memory-network-a",
		Source:    "manual_note",
		Title:     "OpenWrt 网关失败",
		Summary:   "代理失败，需要确认 DNS 和网关。",
		Text:      "OpenWrt gateway proxy failed, 需要补复盘。",
		Tags:      []string{"network", "todo"},
		CreatedAt: now.Add(-1 * time.Hour).Unix(),
	})
	service.addEntry(Entry{
		ID:        "memory-network-b",
		Source:    "time_machine",
		Title:     "Cloudflare tunnel timeout",
		Summary:   "gateway timeout repeats",
		Text:      "Cloudflare tunnel gateway timeout repeats and should become checklist.",
		Tags:      []string{"network"},
		CreatedAt: now.Add(-20 * time.Minute).Unix(),
	})
	service.addEntry(Entry{
		ID:        "memory-sensitive",
		Source:    "manual_note",
		Title:     "token secret",
		Summary:   "password=secret",
		Text:      "token=secret should never enter daily report",
		Sensitive: true,
		CreatedAt: now.Add(-30 * time.Minute).Unix(),
	})
	service.addEntry(Entry{
		ID:            "memory-pending-empty-chat",
		Source:        "time_machine",
		Title:         "未质检空聊天截图",
		Summary:       "empty chat pending capture should not enter daily draft",
		Text:          "pending capture should stay out of summaries",
		QualityStatus: qualityStatusPending,
		CreatedAt:     now.Add(-10 * time.Minute).Unix(),
	})

	daily := service.GenerateDailyDraft()

	if daily.ID != "daily-"+now.Format("20060102") || len(daily.Evidence) != 2 {
		t.Fatalf("unexpected daily metadata: %#v", daily)
	}
	if containsString(daily.Evidence, "memory-sensitive") || containsString(daily.Evidence, "memory-pending-empty-chat") || strings.Contains(daily.Body, "token=secret") || strings.Contains(daily.Body, "password=secret") || strings.Contains(daily.Body, "pending capture") {
		t.Fatalf("daily draft should not include sensitive evidence or body: %#v", daily)
	}
	for _, expected := range []string{"## 今日概览", "## 主要工作", "## 待跟进", "## 复盘线索", "## 证据 ID", "memory-network-a", "memory-network-b", "已跳过敏感记忆 1 条"} {
		if !strings.Contains(daily.Body, expected) {
			t.Fatalf("daily body missing %q:\n%s", expected, daily.Body)
		}
	}
}

func TestAskFlowSummarizesTodayAndSkipsSensitiveEvidence(t *testing.T) {
	service := NewServiceWithPath("", nil)
	clearEntriesForTest(service)
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.Local)
	service.now = func() time.Time { return now }
	service.addEntry(Entry{
		ID:        "flow-ui",
		Source:    "manual_note",
		Title:     "心流界面整理",
		Summary:   "重构心流首页问答和证据抽屉。",
		Text:      "把心流主界面改成对话入口。",
		AppName:   "Code.exe",
		Tags:      []string{"flow"},
		CreatedAt: now.Add(-2 * time.Hour).Unix(),
	})
	service.addEntry(Entry{
		ID:        "flow-sensitive",
		Source:    "manual_note",
		Title:     "secret token",
		Summary:   "password=secret",
		Text:      "token=secret",
		AppName:   "Terminal",
		Sensitive: true,
		CreatedAt: now.Add(-1 * time.Hour).Unix(),
	})
	service.addEntry(Entry{
		ID:            "flow-pending-empty-chat",
		Source:        "time_machine",
		Title:         "未质检空聊天截图",
		Summary:       "pending flow evidence should not appear",
		Text:          "pending flow evidence should not appear",
		AppName:       "Weixin.exe",
		QualityStatus: qualityStatusPending,
		CreatedAt:     now.Add(-30 * time.Minute).Unix(),
	})

	answer := service.AskFlow(FlowAskRequest{Question: "我今天干了些什么？"})

	if !answer.OK || answer.Intent != "today" || len(answer.Evidence) != 1 {
		t.Fatalf("unexpected flow answer: %#v", answer)
	}
	if answer.Evidence[0].ID != "flow-ui" || strings.Contains(answer.Answer, "token=secret") || strings.Contains(answer.Answer, "password=secret") || strings.Contains(answer.Answer, "pending flow evidence") {
		t.Fatalf("flow answer should keep non-sensitive evidence only: %#v", answer)
	}
	if !strings.Contains(answer.Answer, "今天已经沉淀") || !strings.Contains(answer.Answer, "敏感记忆已自动跳过") {
		t.Fatalf("flow answer should summarize today and mention skipped sensitive memory: %s", answer.Answer)
	}
}

func TestAskFlowSummarizesCommunicationContext(t *testing.T) {
	service := NewServiceWithPath("", nil)
	clearEntriesForTest(service)
	now := time.Unix(1781458200, 0)
	service.now = func() time.Time { return now }
	service.addEntry(Entry{
		ID:          "flow-wechat",
		Source:      "time_machine",
		Title:       "微信群消息讨论截图",
		Summary:     "项目群里有人询问截图贴图问题。",
		Text:        "微信 群 消息",
		WindowTitle: "Ariadne 项目群 - 微信",
		AppName:     "Weixin.exe",
		CreatedAt:   now.Add(-30 * time.Minute).Unix(),
	})
	service.addEntry(Entry{
		ID:        "flow-build",
		Source:    "manual_note",
		Title:     "构建记录",
		Summary:   "wails3 package 完成。",
		AppName:   "Code.exe",
		CreatedAt: now.Add(-20 * time.Minute).Unix(),
	})

	answer := service.AskFlow(FlowAskRequest{Question: "今天有哪些人找过我？"})

	if answer.Intent != "contacts" || len(answer.Evidence) != 1 || answer.Evidence[0].ID != "flow-wechat" {
		t.Fatalf("expected contact evidence only, got %#v", answer)
	}
	if !strings.Contains(answer.Answer, "沟通有关") || !strings.Contains(answer.Answer, "Weixin.exe") {
		t.Fatalf("contact answer should mention communication source, got %s", answer.Answer)
	}
}

func TestAskFlowUsesFlowAgentRunnerWhenConfigured(t *testing.T) {
	service := NewServiceWithPath("", nil)
	clearEntriesForTest(service)
	now := time.Date(2026, 6, 15, 22, 0, 0, 0, time.Local)
	service.now = func() time.Time { return now }
	runner := &fakeFlowAgentRunner{result: FlowAgentResult{Answer: "这是 OpenAI Agents SDK 生成的动态回答。\n\n依据：flow-agent-a", Mode: "agent:openai-agents-sdk-shell-skill"}}
	RegisterFlowAgentRunner(service, runner)
	service.ApplyFlowAgentPolicy(FlowAgentPolicy{Enabled: true, Runner: "openai-agent", Provider: "openai-compatible", Model: "test-model", NativeSkills: true})
	service.addEntry(Entry{
		ID:        "flow-agent-a",
		Source:    "manual_note",
		Title:     "心流动态回答",
		Summary:   "把静态汇总改成 agent 生成。",
		Text:      "AskFlow 应该通过 Codex agent 基于 evidence 生成回答。",
		AppName:   "Code.exe",
		CreatedAt: now.Add(-30 * time.Minute).Unix(),
	})
	service.addEntry(Entry{
		ID:        "flow-agent-secret",
		Source:    "manual_note",
		Title:     "password token",
		Summary:   "password=secret",
		Text:      "token=secret",
		Sensitive: true,
		CreatedAt: now.Add(-10 * time.Minute).Unix(),
	})
	service.addEntry(Entry{
		ID:            "flow-agent-pending",
		Source:        "time_machine",
		Title:         "未质检 agent evidence",
		Summary:       "pending agent evidence should not be sent",
		QualityStatus: qualityStatusPending,
		CreatedAt:     now.Add(-5 * time.Minute).Unix(),
	})

	answer := service.AskFlow(FlowAskRequest{Question: "我今天干了些什么？"})

	if !answer.UsedAI || answer.Mode != "agent:openai-agents-sdk-shell-skill" || !strings.Contains(answer.Answer, "动态回答") {
		t.Fatalf("expected agent answer, got %#v", answer)
	}
	if runner.calls != 1 || runner.lastJob.Question == "" || runner.lastJob.LocalAnswer == "" {
		t.Fatalf("agent runner should receive question and tool-oriented seed answer: calls=%d job=%#v", runner.calls, runner.lastJob)
	}
	if len(runner.lastJob.Evidence) != 0 || !strings.Contains(runner.lastJob.LocalAnswer, "Flow Memory skill") {
		t.Fatalf("agent runner should query evidence through the Flow Memory skill instead of Go preselection: %#v", runner.lastJob)
	}
	if !runner.lastJob.NativeSkills {
		t.Fatalf("agent runner should receive native Responses skill preference: %#v", runner.lastJob)
	}
}

func TestSelfModelPersistsAndBuildsPromptSafeSummary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "work_memory.json")
	service := NewServiceWithPath(path, nil)
	now := time.Date(2026, 6, 18, 15, 0, 0, 0, time.Local)
	service.now = func() time.Time { return now }

	model := service.UpsertSelfAssertion(SelfAssertionRequest{
		Category: "identity",
		Key:      "name",
		Label:    "姓名",
		Value:    "luwei",
		Privacy:  "always",
	})
	model = service.UpsertSelfAssertion(SelfAssertionRequest{
		Category: "identity",
		Key:      "age",
		Label:    "年龄",
		Value:    "36",
		Privacy:  "relevant",
	})
	model = service.UpsertSelfAssertion(SelfAssertionRequest{
		Category: "relationship",
		Key:      "collaborator",
		Label:    "协作对象",
		Value:    "张笑腾",
		Privacy:  "always",
	})

	if len(model.Assertions) != 3 {
		t.Fatalf("expected assertions, got %#v", model)
	}
	if !strings.Contains(model.Summary.Prompt, "luwei") {
		t.Fatalf("confirmed low-risk identity should enter prompt summary: %#v", model.Summary)
	}
	if strings.Contains(model.Summary.Prompt, "36") || strings.Contains(model.Summary.Prompt, "张笑腾") {
		t.Fatalf("relevant/private relationship data should not enter default prompt summary: %#v", model.Summary)
	}

	reloaded := NewServiceWithPath(path, nil)
	reloadedModel := reloaded.SelfModel()
	if len(reloadedModel.Assertions) != 3 || !strings.Contains(reloadedModel.Summary.Prompt, "luwei") {
		t.Fatalf("self model should persist through service reload: %#v", reloadedModel)
	}
}

func TestTodosPersistAndFilterActiveItems(t *testing.T) {
	path := filepath.Join(t.TempDir(), "work_memory.json")
	service := NewServiceWithPath(path, nil)
	now := time.Date(2026, 6, 18, 17, 40, 0, 0, time.Local)
	service.now = func() time.Time { return now }

	list := service.UpsertTodo(TodoRequest{
		Title:    "补齐待办模块",
		Note:     "需要前端、CLI 和 Agent skill 都能读写",
		Priority: "high",
		Scope:    "Ariadne",
		Evidence: []string{"memory-todo-source"},
	})
	if list.Open != 1 || len(list.Items) != 1 {
		t.Fatalf("expected one open todo, got %#v", list)
	}
	todoID := list.Items[0].ID
	if todoID == "" || list.Items[0].Evidence[0] != "memory-todo-source" {
		t.Fatalf("todo should have id and evidence: %#v", list.Items[0])
	}

	now = now.Add(10 * time.Minute)
	list = service.UpdateTodo(TodoUpdateRequest{ID: todoID, Status: "done"})
	if list.Done != 1 || list.Items[0].CompletedAt == 0 {
		t.Fatalf("done todo should record completion: %#v", list)
	}
	active := service.Todos(TodoListRequest{})
	if len(active.Items) != 0 {
		t.Fatalf("default todo list should hide closed items, got %#v", active)
	}

	reloaded := NewServiceWithPath(path, nil)
	all := reloaded.Todos(TodoListRequest{IncludeDone: true})
	if len(all.Items) != 1 || all.Done != 1 || all.Items[0].ID != todoID {
		t.Fatalf("todo should persist through reload: %#v", all)
	}
}

func TestAskFlowSyncsTodoWritesFromAgentTool(t *testing.T) {
	path := filepath.Join(t.TempDir(), "work_memory.json")
	service := NewServiceWithPath(path, nil)
	defer service.Stop()

	runner := &fakeFlowAgentRunner{
		result: FlowAgentResult{
			Answer: "已保存端午值班待办。\n\n依据：memory-time_machine-duty",
			Mode:   "agent:openai-compatible-chat-tools",
		},
		onCall: func(FlowAgentJob) {
			external := NewServiceWithPath(path, nil)
			defer external.Stop()
			external.UpsertTodo(TodoRequest{
				Title:    "端午值班",
				Note:     "6 月 21 日白班 8:00-20:00",
				Priority: "high",
				Source:   "agent",
				Evidence: []string{"memory-time_machine-duty"},
			})
		},
	}
	RegisterFlowAgentRunner(service, runner)
	service.ApplyFlowAgentPolicy(FlowAgentPolicy{Enabled: true, Runner: "openai-agent", Provider: "openai-compatible", Model: "glm-5.1"})

	answer := service.AskFlow(FlowAskRequest{Question: "端午值班保存待办"})
	if !answer.OK || !answer.UsedAI {
		t.Fatalf("expected agent answer, got %#v", answer)
	}
	todos := service.Todos(TodoListRequest{IncludeDone: true})
	if len(todos.Items) != 1 || todos.Items[0].Title != "端午值班" || todos.Items[0].Source != "agent" {
		t.Fatalf("agent todo write should be visible to the running service, got %#v", todos)
	}
}

func TestAskFlowAgentReceivesSelfModelSummary(t *testing.T) {
	service := NewServiceWithPath("", nil)
	clearEntriesForTest(service)
	now := time.Date(2026, 6, 18, 15, 20, 0, 0, time.Local)
	service.now = func() time.Time { return now }
	runner := &fakeFlowAgentRunner{result: FlowAgentResult{Answer: "已按我模型和本地留痕回答。\n\n依据：本次未命中可引用证据", Mode: "agent:openai-agents-sdk"}}
	RegisterFlowAgentRunner(service, runner)
	service.ApplyFlowAgentPolicy(FlowAgentPolicy{Enabled: true, Runner: "openai-agent", Provider: "openai-compatible", Model: "test-model"})
	service.UpsertSelfAssertion(SelfAssertionRequest{
		Category: "identity",
		Key:      "name",
		Label:    "姓名",
		Value:    "luwei",
		Privacy:  "always",
	})
	service.UpsertSelfAssertion(SelfAssertionRequest{
		Category: "identity",
		Key:      "phone",
		Label:    "手机号",
		Value:    "13800000000",
		Privacy:  "never",
	})

	answer := service.AskFlow(FlowAskRequest{Question: "我今天干了什么？"})

	if !answer.UsedAI || runner.calls != 1 {
		t.Fatalf("expected agent answer, got answer=%#v calls=%d", answer, runner.calls)
	}
	if !strings.Contains(runner.lastJob.SelfModel, "luwei") {
		t.Fatalf("agent job should include allowed self model summary: %#v", runner.lastJob)
	}
	if strings.Contains(runner.lastJob.SelfModel, "13800000000") {
		t.Fatalf("agent job should not include never-send self assertions: %#v", runner.lastJob)
	}
}

func TestAskFlowShowsAgentErrorInsteadOfPretendingLocalSummaryWhenAgentFails(t *testing.T) {
	service := NewServiceWithPath("", nil)
	clearEntriesForTest(service)
	now := time.Date(2026, 6, 15, 22, 0, 0, 0, time.Local)
	service.now = func() time.Time { return now }
	runner := &fakeFlowAgentRunner{err: context.DeadlineExceeded}
	RegisterFlowAgentRunner(service, runner)
	service.ApplyFlowAgentPolicy(FlowAgentPolicy{Enabled: true, Runner: "codex"})
	service.addEntry(Entry{
		ID:        "flow-local-fallback",
		Source:    "manual_note",
		Title:     "心流本地兜底",
		Summary:   "agent 失败时保留本地摘要。",
		CreatedAt: now.Add(-10 * time.Minute).Unix(),
	})

	answer := service.AskFlow(FlowAskRequest{Question: "我今天干了些什么？"})

	if answer.OK || answer.UsedAI || answer.Mode != "agent_error" || strings.Contains(answer.Answer, "今天已经沉淀") || !strings.Contains(answer.Message, "Agent runner 调用失败") {
		t.Fatalf("expected visible agent error without local-summary masquerade, got %#v", answer)
	}
}

func TestAskFlowContactQuestionRanksNamedContactEvidence(t *testing.T) {
	service := NewServiceWithPath("", nil)
	clearEntriesForTest(service)
	now := time.Date(2026, 6, 16, 16, 30, 0, 0, time.Local)
	service.now = func() time.Time { return now }
	service.addEntry(Entry{
		ID:          "flow-contact-ye",
		Source:      "capture_history",
		Title:       "微信聊天",
		Summary:     "叶志伟讨论 Ariadne 心流问答。",
		Text:        "叶志伟：心流需要真的检索内容再回答。",
		WindowTitle: "微信 - 叶志伟",
		AppName:     "Weixin.exe",
		CreatedAt:   now.Add(-5 * time.Minute).Unix(),
	})
	service.addEntry(Entry{
		ID:          "flow-contact-other",
		Source:      "capture_history",
		Title:       "微信聊天",
		Summary:     "其他联系人讨论午饭。",
		Text:        "张三：中午吃什么。",
		WindowTitle: "微信 - 张三",
		AppName:     "Weixin.exe",
		CreatedAt:   now.Add(-4 * time.Minute).Unix(),
	})

	answer := service.AskFlow(FlowAskRequest{Question: "今天跟叶志伟聊了什么"})

	if answer.Intent != "contacts" || len(answer.Evidence) == 0 || answer.Evidence[0].ID != "flow-contact-ye" {
		t.Fatalf("expected named contact evidence first, got %#v", answer)
	}
	if strings.Contains(answer.Answer, "今天已经沉淀") {
		t.Fatalf("contact question should not be answered as today summary: %s", answer.Answer)
	}
}

func TestAskFlowConversationPersistsConversationAndMessages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "work_memory.json")
	service := NewServiceWithPath(path, nil)
	defer service.Stop()
	clearEntriesForTest(service)
	now := time.Date(2026, 6, 18, 13, 30, 0, 0, time.Local)
	service.now = func() time.Time { return now }
	service.addEntry(Entry{
		ID:        "flow-conversation-memory",
		Source:    "manual_note",
		Title:     "会话记录持久化",
		Summary:   "心流问答需要写入会话和消息表。",
		Text:      "用户的问题和心流回答都应该在中间对话框恢复。",
		AppName:   "Code.exe",
		CreatedAt: now.Add(-10 * time.Minute).Unix(),
	})

	result := service.AskFlowConversation(FlowConversationAskRequest{Question: "心流会话怎么持久化？"})
	if !result.OK || result.Conversation.ID == "" || len(result.Messages) != 2 {
		t.Fatalf("expected persisted conversation turn, got %#v", result)
	}
	if result.Messages[0].Role != "user" || result.Messages[0].Text != "心流会话怎么持久化？" {
		t.Fatalf("first message should be the user question: %#v", result.Messages[0])
	}
	if result.Messages[1].Role != "assistant" || result.Messages[1].Result == nil || result.Messages[1].Result.Question != "心流会话怎么持久化？" {
		t.Fatalf("second message should keep assistant answer and structured result: %#v", result.Messages[1])
	}

	reloaded := NewServiceWithPath(path, nil)
	defer reloaded.Stop()
	conversations := reloaded.FlowConversations()
	if len(conversations) != 1 || conversations[0].ID != result.Conversation.ID || conversations[0].MessageCount != 2 {
		t.Fatalf("expected one reloaded conversation with two messages, got %#v", conversations)
	}
	messages := reloaded.FlowMessages(result.Conversation.ID)
	if len(messages) != 2 || messages[0].Role != "user" || messages[1].Role != "assistant" || messages[1].Result == nil {
		t.Fatalf("expected reloaded user and assistant messages, got %#v", messages)
	}
	if messages[1].Result.Question != "心流会话怎么持久化？" || !strings.Contains(messages[1].Text, "非敏感") {
		t.Fatalf("assistant message should restore result payload and answer text, got %#v", messages[1])
	}
}

func TestAskFlowConversationPassesRecentMessagesToAgent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "work_memory.json")
	service := NewServiceWithPath(path, nil)
	defer service.Stop()
	clearEntriesForTest(service)
	runner := &fakeFlowAgentRunner{
		result: FlowAgentResult{
			Answer: "已处理。\n\n依据：本次未命中可引用证据",
			Mode:   "agent:openai-compatible-chat-tools",
		},
	}
	RegisterFlowAgentRunner(service, runner)
	service.ApplyFlowAgentPolicy(FlowAgentPolicy{Enabled: true, Runner: "openai-agent", Provider: "openai-compatible", Model: "glm-5.1"})

	first := service.AskFlowConversation(FlowConversationAskRequest{Question: "端午值班保存待办"})
	if !first.OK || runner.calls != 1 || len(runner.lastJob.Conversation) != 0 {
		t.Fatalf("first turn should not have prior context, result=%#v job=%#v", first, runner.lastJob)
	}

	second := service.AskFlowConversation(FlowConversationAskRequest{
		ConversationID: first.Conversation.ID,
		Question:       "你刚才没加成功，再加一次",
	})
	if !second.OK || runner.calls != 2 {
		t.Fatalf("expected second agent turn, result=%#v calls=%d", second, runner.calls)
	}
	if len(runner.lastJob.Conversation) < 2 {
		t.Fatalf("second turn should include recent conversation context: %#v", runner.lastJob)
	}
	if runner.lastJob.Conversation[0].Role != "user" || !strings.Contains(runner.lastJob.Conversation[0].Text, "端午值班保存待办") {
		t.Fatalf("conversation context should include prior user request: %#v", runner.lastJob.Conversation)
	}
	foundAssistant := false
	for _, message := range runner.lastJob.Conversation {
		if message.Role == "assistant" && strings.Contains(message.Text, "已处理") {
			foundAssistant = true
		}
	}
	if !foundAssistant {
		t.Fatalf("conversation context should include prior assistant answer: %#v", runner.lastJob.Conversation)
	}
}

func TestDeleteFlowConversationRemovesConversationAndMessages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "work_memory.json")
	service := NewServiceWithPath(path, nil)
	defer service.Stop()
	clearEntriesForTest(service)
	now := time.Date(2026, 6, 18, 14, 0, 0, 0, time.Local)
	service.now = func() time.Time { return now }
	service.addEntry(Entry{
		ID:        "flow-delete-memory",
		Source:    "manual_note",
		Title:     "会话删除",
		Summary:   "心流对话记录删除时，消息也应该清理。",
		Text:      "删除对话记录不能留下孤儿消息。",
		AppName:   "Code.exe",
		CreatedAt: now.Add(-10 * time.Minute).Unix(),
	})

	first := service.AskFlowConversation(FlowConversationAskRequest{Question: "第一条对话要删除"})
	second := service.AskFlowConversation(FlowConversationAskRequest{Question: "第二条对话要保留"})
	if first.Conversation.ID == "" || second.Conversation.ID == "" || first.Conversation.ID == second.Conversation.ID {
		t.Fatalf("expected two distinct conversations, got first=%#v second=%#v", first.Conversation, second.Conversation)
	}

	remaining := service.DeleteFlowConversation(first.Conversation.ID)
	if len(remaining) != 1 || remaining[0].ID != second.Conversation.ID {
		t.Fatalf("expected only second conversation to remain, got %#v", remaining)
	}
	if messages := service.FlowMessages(first.Conversation.ID); len(messages) != 0 {
		t.Fatalf("deleted conversation messages should be removed, got %#v", messages)
	}
	if messages := service.FlowMessages(second.Conversation.ID); len(messages) != 2 {
		t.Fatalf("remaining conversation messages should stay, got %#v", messages)
	}

	reloaded := NewServiceWithPath(path, nil)
	defer reloaded.Stop()
	reloadedConversations := reloaded.FlowConversations()
	if len(reloadedConversations) != 1 || reloadedConversations[0].ID != second.Conversation.ID {
		t.Fatalf("deleted conversation should stay deleted after reload, got %#v", reloadedConversations)
	}
}

func TestPolishDraftRequiresConfirmationAndSkipsExternalInPrivacyMode(t *testing.T) {
	service := NewServiceWithPath("", nil)
	polisher := &fakeDraftPolisher{}
	RegisterDraftPolisher(service, polisher)
	service.ApplyDraftPolishPolicy(DraftPolishPolicy{
		Enabled:  true,
		Provider: "openai-compatible",
		BaseURL:  "http://localhost/v1",
		Model:    "test-model",
	})
	draft := Draft{ID: "daily-20260614", Title: "今日工作日报草稿", Body: "原始日报", Evidence: []string{"daily-a"}}

	preview := service.PolishDraft(DraftPolishRequest{Draft: draft, Kind: "daily"})
	if preview.OK || !preview.RequiresConfirmation || !preview.External {
		t.Fatalf("expected confirmation preview, got %#v", preview)
	}
	if polisher.calls != 0 {
		t.Fatalf("preview should not call external polisher")
	}

	service.SetPrivacyMode(true)
	blocked := service.PolishDraft(DraftPolishRequest{Draft: draft, Kind: "daily", Confirmed: true})
	if blocked.OK || !strings.Contains(blocked.Message, "隐私模式") || polisher.calls != 0 {
		t.Fatalf("privacy mode should block external polish, got %#v calls=%d", blocked, polisher.calls)
	}
}

func TestPolishDraftUsesRegisteredPolisherAfterConfirmation(t *testing.T) {
	service := NewServiceWithPath("", nil)
	polisher := &fakeDraftPolisher{}
	RegisterDraftPolisher(service, polisher)
	service.ApplyDraftPolishPolicy(DraftPolishPolicy{
		Enabled:  true,
		Provider: "openai-compatible",
		BaseURL:  "http://localhost/v1",
		Model:    "test-model",
	})
	draft := Draft{ID: "daily-20260614", Title: "今日工作日报草稿", Body: "原始日报", Evidence: []string{"daily-a"}}

	result := service.PolishDraft(DraftPolishRequest{Draft: draft, Kind: "daily", Confirmed: true})
	if !result.OK || result.PolishedDraft.Body == "" {
		t.Fatalf("expected polished draft, got %#v", result)
	}
	if polisher.calls != 1 || polisher.lastJob.Model != "test-model" || polisher.lastJob.Provider != "openai-compatible" {
		t.Fatalf("polisher was not called with policy: calls=%d job=%#v", polisher.calls, polisher.lastJob)
	}
	if result.PolishedDraft.Evidence[0] != "daily-a" || !strings.Contains(result.PolishedDraft.Body, "润色") {
		t.Fatalf("polished draft lost evidence/body: %#v", result.PolishedDraft)
	}
}

func TestGenerateRetrospectiveDraftFromSelectedEvidence(t *testing.T) {
	service := NewServiceWithPath("", nil)
	service.entries = nil
	now := time.Unix(1781458200, 0)
	service.now = func() time.Time { return now }
	service.addEntry(Entry{
		ID:        "memory-network-a",
		Source:    "clipboard",
		Title:     "网关代理异常",
		Summary:   "OpenWrt 网关代理失败，默认网关仍指向 192.168.1.1。",
		Text:      "Cloudflare Tunnel 正常，但客户端网关指向错误，需要复盘。",
		Tags:      []string{"网络", "复盘"},
		CreatedAt: now.Add(-2 * time.Hour).Unix(),
	})
	service.addEntry(Entry{
		ID:        "memory-network-b",
		Source:    "manual_note",
		Title:     "修复验证",
		Summary:   "改回 192.168.1.10 后代理恢复，仍需补检查清单。",
		Text:      "todo: 把网关检查、DNS 检查和代理连通性命令沉淀成 checklist。",
		Tags:      []string{"验证"},
		CreatedAt: now.Add(-1 * time.Hour).Unix(),
	})
	service.addEntry(Entry{
		ID:        "memory-secret",
		Source:    "manual_note",
		Title:     "敏感凭据",
		Summary:   "token=secret",
		Text:      "password=secret",
		Sensitive: true,
		CreatedAt: now.Add(-30 * time.Minute).Unix(),
	})

	draft := service.GenerateRetrospectiveDraft([]string{"memory-network-b", "memory-secret", "memory-network-a"})

	if draft.ID == "" || !strings.Contains(draft.Title, "问题复盘草稿") {
		t.Fatalf("unexpected retrospective draft identity: %#v", draft)
	}
	if containsString(draft.Evidence, "memory-secret") || strings.Contains(draft.Body, "token=secret") || strings.Contains(draft.Body, "password=secret") {
		t.Fatalf("sensitive memory should not enter retrospective draft: %#v\n%s", draft.Evidence, draft.Body)
	}
	for _, expected := range []string{"## 复盘概览", "## 问题背景", "## 时间线", "## 初步原因", "## 处理过程", "## 遗留风险与后续动作", "## 证据 ID", "memory-network-a", "memory-network-b", "已跳过敏感记忆 1 条"} {
		if !strings.Contains(draft.Body, expected) {
			t.Fatalf("retrospective draft missing %q:\n%s", expected, draft.Body)
		}
	}
	if len(draft.Evidence) != 2 || draft.Evidence[0] != "memory-network-a" || draft.Evidence[1] != "memory-network-b" {
		t.Fatalf("expected non-sensitive evidence ordered by time, got %#v", draft.Evidence)
	}
}

func TestScheduledDraftsGenerateLocalArtifactsAndSkipSensitive(t *testing.T) {
	service := NewServiceWithPath("", nil)
	defer service.Stop()
	now := time.Date(2026, 6, 14, 16, 55, 0, 0, time.Local)
	service.now = func() time.Time { return now }
	service.entries = nil
	service.entries = append(service.entries,
		Entry{
			ID:        "schedule-network-a",
			Source:    "manual_note",
			Title:     "OpenWrt 代理故障",
			Summary:   "远程访问失败，需要复盘网络链路",
			Text:      "gateway timeout and proxy error after DNS change",
			Tags:      []string{"network", "issue"},
			CreatedAt: now.Add(-2 * time.Hour).Unix(),
		},
		Entry{
			ID:        "schedule-network-b",
			Source:    "clipboard",
			Title:     "验证修复路径",
			Summary:   "修复后恢复访问，TODO 补回滚步骤",
			Text:      "verified proxy fix and TODO write rollback checklist",
			Tags:      []string{"network", "fix"},
			CreatedAt: now.Add(-1 * time.Hour).Unix(),
		},
		Entry{
			ID:        "schedule-secret",
			Source:    "manual_note",
			Title:     "敏感凭据",
			Summary:   "token should not be scheduled",
			Text:      "token=secret password=hidden",
			Sensitive: true,
			CreatedAt: now.Add(-30 * time.Minute).Unix(),
		},
	)

	status := service.ApplyDraftSchedule(DraftSchedulePolicy{
		Enabled:                 true,
		IntervalMinutes:         1,
		DailyDraftEnabled:       true,
		RetrospectiveEnabled:    true,
		ExperienceReportEnabled: true,
		ExperiencePeriodDays:    7,
	})
	if !status.Enabled || !status.Running || status.IntervalMinutes != 240 {
		t.Fatalf("schedule should run with normalized interval: %#v", status)
	}

	status = service.RunScheduledDraftsNow()
	if status.LastRunAt != now.Unix() || status.LastEntryCount != 2 || status.LastError != "" {
		t.Fatalf("unexpected scheduled status: %#v", status)
	}
	if !containsString(status.DailyDraft.Evidence, "schedule-network-a") || containsString(status.DailyDraft.Evidence, "schedule-secret") || strings.Contains(status.DailyDraft.Body, "token=secret") {
		t.Fatalf("daily scheduled draft should keep only non-sensitive evidence: %#v\n%s", status.DailyDraft.Evidence, status.DailyDraft.Body)
	}
	if len(status.RetrospectiveDraft.Evidence) != 2 || containsString(status.RetrospectiveDraft.Evidence, "schedule-secret") || !strings.Contains(status.RetrospectiveDraft.Body, "## 时间线") {
		t.Fatalf("scheduled retrospective should use issue evidence only: %#v\n%s", status.RetrospectiveDraft.Evidence, status.RetrospectiveDraft.Body)
	}
	if status.ExperienceReport.ID == "" || status.ExperienceReport.EntryCount != 2 {
		t.Fatalf("expected scheduled experience report from non-sensitive entries: %#v", status.ExperienceReport)
	}

	status = service.runScheduledDrafts(false)
	if status.LastError != "没有新的非敏感工作记忆" {
		t.Fatalf("periodic run without new entries should not regenerate drafts: %#v", status)
	}
}

func TestDiscoverExperiencesFindsRepeatedIssuesAndAutomation(t *testing.T) {
	service := NewServiceWithPath("", nil)
	service.entries = nil
	now := time.Unix(1781404200, 0)
	service.now = func() time.Time { return now }
	service.addEntry(Entry{
		ID:        "memory-openwrt-timeout",
		Source:    "manual_note",
		Title:     "OpenWrt gateway timeout",
		Summary:   "Cloudflare proxy timeout after gateway change",
		Text:      "OpenWrt gateway proxy timeout needs the same DNS and route checks.",
		AppName:   "Terminal",
		Tags:      []string{"network"},
		Favorite:  true,
		CreatedAt: now.Add(-1 * time.Hour).Unix(),
	})
	service.addEntry(Entry{
		ID:        "memory-cloudflare-proxy",
		Source:    "clipboard",
		Title:     "Cloudflare tunnel proxy failed",
		Summary:   "Network proxy error repeats on gateway switch",
		Text:      "gateway proxy network timeout repeats, document the checklist.",
		AppName:   "Browser",
		Tags:      []string{"proxy"},
		CreatedAt: now.Add(-2 * time.Hour).Unix(),
	})
	service.addEntry(Entry{
		ID:        "memory-workflow-json",
		Source:    "workflow",
		Title:     "JSON workflow formatting",
		Summary:   "Repeated workflow macro for JSON clipboard formatting",
		Text:      "workflow macro uses clipboard JSON formatting then hash output.",
		AppName:   "Ariadne",
		Tags:      []string{"workflow", "json"},
		CreatedAt: now.Add(-3 * time.Hour).Unix(),
	})
	service.addEntry(Entry{
		ID:        "memory-workflow-hash",
		Source:    "workflow",
		Title:     "Hash workflow",
		Summary:   "Repeated macro hashes clipboard after URL transform",
		Text:      "workflow macro repeats hash and url encoding from clipboard.",
		AppName:   "Ariadne",
		Tags:      []string{"workflow"},
		CreatedAt: now.Add(-4 * time.Hour).Unix(),
	})

	report := service.DiscoverExperiences(7)

	if report.ID == "" || report.EntryCount != 4 || report.EvidenceCount == 0 || len(report.Insights) < 2 {
		t.Fatalf("expected experience report with insights, got %#v", report)
	}
	if !reportHasInsight(report, "repeated_issue", "memory-openwrt-timeout", "memory-cloudflare-proxy") {
		t.Fatalf("expected repeated network issue insight with evidence, got %#v", report.Insights)
	}
	if !reportHasInsight(report, "automation_opportunity", "memory-workflow-json", "memory-workflow-hash") {
		t.Fatalf("expected automation insight with evidence, got %#v", report.Insights)
	}
	for _, insight := range report.Insights {
		if !insight.RequiresReview || insight.Confidence <= 0 || insight.Recommendation == "" || insight.Reason == "" {
			t.Fatalf("insight should be explainable and require review: %#v", insight)
		}
	}
}

func TestDiscoverExperiencesExcludesSensitiveEntries(t *testing.T) {
	service := NewServiceWithPath("", nil)
	service.entries = nil
	now := time.Unix(1781404200, 0)
	service.now = func() time.Time { return now }
	service.addEntry(Entry{
		ID:        "memory-sensitive-token",
		Source:    "manual_note",
		Title:     "token auth failure",
		Summary:   "token secret repeated",
		Text:      "token=secret password=hidden auth failed",
		Tags:      []string{"auth"},
		Sensitive: true,
		CreatedAt: now.Unix(),
	})
	service.addEntry(Entry{
		ID:        "memory-auth-normal",
		Source:    "manual_note",
		Title:     "auth permission failure",
		Summary:   "forbidden permission check",
		Text:      "auth permission failed but no secret body",
		Tags:      []string{"auth"},
		CreatedAt: now.Unix(),
	})

	report := service.DiscoverExperiences(7)

	for _, insight := range report.Insights {
		for _, id := range insight.Evidence {
			if id == "memory-sensitive-token" {
				t.Fatalf("sensitive memory should not be used as experience evidence: %#v", report)
			}
		}
	}
}

func TestDiscoverExperiencesAIRequiresConfirmationAndBlocksPrivacy(t *testing.T) {
	service := NewServiceWithPath("", nil)
	service.entries = nil
	now := time.Unix(1781404200, 0)
	service.now = func() time.Time { return now }
	discoverer := &fakeExperienceDiscoverer{}
	RegisterExperienceDiscoverer(service, discoverer)
	service.ApplyExperienceDiscoveryPolicy(ExperienceDiscoveryPolicy{
		Enabled:  true,
		Provider: "openai-compatible",
		BaseURL:  "http://localhost/v1",
		Model:    "test-model",
	})
	service.addEntry(Entry{
		ID:        "memory-network-a",
		Source:    "manual_note",
		Title:     "OpenWrt proxy timeout",
		Summary:   "network proxy timeout repeats",
		Text:      "gateway timeout should become checklist",
		CreatedAt: now.Unix(),
	})

	preview := service.DiscoverExperiencesAI(ExperienceDiscoveryRequest{PeriodDays: 7, External: true})
	if preview.OK || !preview.RequiresConfirmation || !preview.External || preview.Provider != "openai-compatible" || preview.Model != "test-model" {
		t.Fatalf("expected external confirmation preview, got %#v", preview)
	}
	if len(preview.RiskReasons) == 0 || discoverer.calls != 0 {
		t.Fatalf("preview should expose risks without calling external discoverer: risks=%#v calls=%d", preview.RiskReasons, discoverer.calls)
	}

	service.SetPrivacyMode(true)
	blocked := service.DiscoverExperiencesAI(ExperienceDiscoveryRequest{PeriodDays: 7, External: true, Confirmed: true})
	if blocked.OK || !strings.Contains(blocked.Message, "隐私模式") || discoverer.calls != 0 {
		t.Fatalf("privacy mode should block external discovery, got %#v calls=%d", blocked, discoverer.calls)
	}
}

func TestDiscoverExperiencesAIUsesDiscovererAndFiltersSensitive(t *testing.T) {
	service := NewServiceWithPath("", nil)
	service.entries = nil
	now := time.Unix(1781404200, 0)
	service.now = func() time.Time { return now }
	discoverer := &fakeExperienceDiscoverer{
		report: ExperienceReport{
			Title:   "模型报告",
			Summary: "AI 找到重复网络问题",
			Insights: []ExperienceInsight{
				{
					Kind:           "repeated_issue",
					Title:          "代理排障可沉淀",
					Summary:        "网络超时重复出现",
					Reason:         "两条 evidence 都指向 proxy timeout",
					Recommendation: "沉淀为网络排障清单，执行前人工审核。",
					Evidence:       []string{"memory-network-a", "memory-secret", "unknown-id", "memory-network-b"},
					Confidence:     1.3,
					Severity:       "high",
				},
			},
		},
	}
	RegisterExperienceDiscoverer(service, discoverer)
	service.ApplyExperienceDiscoveryPolicy(ExperienceDiscoveryPolicy{
		Enabled:  true,
		Provider: "openai-compatible",
		BaseURL:  "http://localhost/v1",
		Model:    "test-model",
	})
	service.addEntry(Entry{
		ID:        "memory-network-a",
		Source:    "manual_note",
		Title:     "OpenWrt proxy timeout",
		Summary:   "network proxy timeout repeats",
		Text:      "gateway timeout should become checklist",
		Tags:      []string{"network"},
		CreatedAt: now.Add(-1 * time.Hour).Unix(),
	})
	service.addEntry(Entry{
		ID:        "memory-network-b",
		Source:    "clipboard",
		Title:     "Cloudflare tunnel timeout",
		Summary:   "network gateway timeout repeats",
		Text:      "proxy timeout again",
		Tags:      []string{"network"},
		CreatedAt: now.Add(-30 * time.Minute).Unix(),
	})
	service.addEntry(Entry{
		ID:        "memory-secret",
		Source:    "manual_note",
		Title:     "token auth failure",
		Summary:   "token secret repeated",
		Text:      "token=secret password=hidden auth failed",
		Tags:      []string{"auth"},
		Sensitive: true,
		CreatedAt: now.Unix(),
	})

	result := service.DiscoverExperiencesAI(ExperienceDiscoveryRequest{PeriodDays: 7, External: true, Confirmed: true})

	if !result.OK || result.Message == "" || !result.External {
		t.Fatalf("expected successful AI discovery, got %#v", result)
	}
	if discoverer.calls != 1 || discoverer.lastJob.Model != "test-model" || len(discoverer.lastJob.Evidence) != 2 {
		t.Fatalf("discoverer was not called with sanitized evidence: calls=%d job=%#v", discoverer.calls, discoverer.lastJob)
	}
	for _, evidence := range discoverer.lastJob.Evidence {
		if evidence.ID == "memory-secret" || strings.Contains(evidence.Text, "password=hidden") {
			t.Fatalf("sensitive evidence entered external job: %#v", discoverer.lastJob.Evidence)
		}
	}
	if len(result.Report.Insights) != 1 {
		t.Fatalf("expected one normalized insight, got %#v", result.Report.Insights)
	}
	insight := result.Report.Insights[0]
	if insight.ID == "" || !insight.RequiresReview || insight.Confidence != 0.95 || insight.Severity != "high" {
		t.Fatalf("unexpected normalized insight metadata: %#v", insight)
	}
	if containsString(insight.Evidence, "memory-secret") || containsString(insight.Evidence, "unknown-id") || len(insight.Evidence) != 2 {
		t.Fatalf("unexpected evidence filter result: %#v", insight.Evidence)
	}
}

func TestDiscoverExperiencesAIFallsBackToLocalReportOnError(t *testing.T) {
	service := NewServiceWithPath("", nil)
	service.entries = nil
	now := time.Unix(1781404200, 0)
	service.now = func() time.Time { return now }
	RegisterExperienceDiscoverer(service, &fakeExperienceDiscoverer{err: context.DeadlineExceeded})
	service.ApplyExperienceDiscoveryPolicy(ExperienceDiscoveryPolicy{
		Enabled:  true,
		Provider: "openai-compatible",
		Model:    "test-model",
	})
	service.addEntry(Entry{
		ID:        "memory-network-a",
		Source:    "manual_note",
		Title:     "OpenWrt proxy timeout",
		Summary:   "network proxy timeout repeats",
		Text:      "gateway timeout should become checklist",
		CreatedAt: now.Unix(),
	})

	result := service.DiscoverExperiencesAI(ExperienceDiscoveryRequest{PeriodDays: 7, External: true, Confirmed: true})

	if result.OK || result.Report.ID == "" || !strings.Contains(result.Message, "本地规则报告") {
		t.Fatalf("expected failed external discovery with local report fallback, got %#v", result)
	}
}

func TestExperienceDecisionPersistsAndDecoratesReports(t *testing.T) {
	path := filepath.Join(t.TempDir(), "work_memory.json")
	service := NewServiceWithPath(path, nil)
	defer service.Stop()
	service.entries = nil
	service.decisions = map[string]ExperienceDecision{}
	now := time.Unix(1781404200, 0)
	service.now = func() time.Time { return now }
	service.addEntry(Entry{
		ID:        "memory-db-timeout-a",
		Source:    "manual_note",
		Title:     "PostgreSQL connection refused",
		Summary:   "Database connection refused during deploy",
		Text:      "postgresql database connection refused; add checklist.",
		Tags:      []string{"database"},
		CreatedAt: now.Add(-1 * time.Hour).Unix(),
	})
	service.addEntry(Entry{
		ID:        "memory-db-timeout-b",
		Source:    "manual_note",
		Title:     "数据库连不上",
		Summary:   "database connection refused repeats",
		Text:      "数据库 connection refused again, document the issue.",
		Tags:      []string{"database"},
		CreatedAt: now.Add(-2 * time.Hour).Unix(),
	})

	report := service.DiscoverExperiences(7)
	var insightID string
	for _, insight := range report.Insights {
		if insight.Kind == "repeated_issue" {
			insightID = insight.ID
			break
		}
	}
	if insightID == "" {
		t.Fatalf("expected repeated issue insight, got %#v", report.Insights)
	}

	decision := service.SetExperienceInsightDecision(insightID, "accepted", "沉淀成数据库排障清单", "")
	if !decision.OK || decision.Decision.Status != "accepted" {
		t.Fatalf("expected accepted decision, got %#v", decision)
	}
	report = service.DiscoverExperiences(7)
	if !reportHasDecision(report, insightID, "accepted", "") {
		t.Fatalf("expected report to include accepted decision, got %#v", report.Insights)
	}

	reloaded := NewServiceWithPath(path, nil)
	defer reloaded.Stop()
	reloaded.now = func() time.Time { return now }
	reloadedReport := reloaded.DiscoverExperiences(7)
	if !reportHasDecision(reloadedReport, insightID, "accepted", "") {
		t.Fatalf("expected reloaded report to keep accepted decision, got %#v", reloadedReport.Insights)
	}

	taskDecision := reloaded.SetExperienceInsightDecision(insightID, "task-package", "", "agent-task-1")
	if !taskDecision.OK || taskDecision.Decision.Status != "task_package" {
		t.Fatalf("expected task package decision, got %#v", taskDecision)
	}
	reloadedReport = reloaded.DiscoverExperiences(7)
	if !reportHasDecision(reloadedReport, insightID, "task_package", "agent-task-1") {
		t.Fatalf("expected task package decision on report, got %#v", reloadedReport.Insights)
	}
}

func TestExperienceDecisionRejectsUnknownStatus(t *testing.T) {
	service := NewServiceWithPath("", nil)
	result := service.SetExperienceInsightDecision("insight-1", "ship-it-now", "", "")
	if result.OK {
		t.Fatalf("unknown status should not be accepted: %#v", result)
	}
}

func TestAddNoteClassifiesSensitiveContentAndSearches(t *testing.T) {
	service := NewServiceWithPath("", nil)

	entry := service.AddNote(NoteRequest{
		Title: "API 排查记录",
		Text:  "调用 https://example.test/api 报错，Authorization: Bearer secret-token",
		Tags:  []string{"接口"},
	})

	if entry.ID == "" || entry.Source != "manual_note" {
		t.Fatalf("expected manual note entry, got %#v", entry)
	}
	if !entry.Sensitive {
		t.Fatalf("expected sensitive note, got %#v", entry)
	}
	if !containsString(entry.Tags, "URL") || !containsString(entry.Tags, "错误") || !containsString(entry.Tags, "敏感") {
		t.Fatalf("expected classified tags, got %#v", entry.Tags)
	}
	if !containsString(entry.Tags, "API") {
		t.Fatalf("expected domain tags, got %#v", entry.Tags)
	}
	if len(service.Search("example.test")) == 0 {
		t.Fatal("expected manual note to be searchable")
	}
}

func TestSensitiveDetectionRequiresCredentialShape(t *testing.T) {
	normalTexts := []string{
		"登录页显示密码输入框和获取验证码按钮。",
		"OAuth token 配置说明，不包含任何真实 token 值。",
		"这张截图在讲 cookie、secret 和 API key 的概念。",
		"验证码: 点击获取，token: 配置说明。",
	}
	for _, text := range normalTexts {
		if LooksSensitiveText(text) {
			t.Fatalf("plain sensitive keywords should not mark text sensitive: %q", text)
		}
	}

	secretTexts := []string{
		"password=super-secret-value",
		`{"token":"abcd1234token"}`,
		"Authorization: Bearer abcdefghijklmnop",
		"验证码: 123456",
		"-----BEGIN PRIVATE KEY-----",
	}
	for _, text := range secretTexts {
		if !LooksSensitiveText(text) {
			t.Fatalf("credential-shaped text should be sensitive: %q", text)
		}
	}
}

func TestSensitiveRulesDoNotFlagPlainKeywordScreenshots(t *testing.T) {
	service := NewServiceWithPath("", nil)

	entry := service.AddNote(NoteRequest{
		Title: "登录页面说明",
		Text:  "截图里有密码输入框、验证码登录、token 配置说明，但没有真实凭据值。",
	})

	if entry.Sensitive || containsString(entry.Tags, "敏感") {
		t.Fatalf("plain security words should not isolate a normal screenshot note: %#v", entry)
	}
}

func TestSensitiveRulesCanBeDisabledForAutomaticClassification(t *testing.T) {
	service := NewServiceWithPath("", nil)
	service.ApplyCapturePolicy(CapturePolicy{SensitiveRulesEnabled: false, SensitiveRulesConfigured: true})

	entry := service.AddNote(NoteRequest{
		Title: "调试头",
		Text:  "Authorization: Bearer abcdefghijklmnop",
	})
	if entry.Sensitive {
		t.Fatalf("disabled sensitive rules should not auto-mark content: %#v", entry)
	}

	manual := service.AddNote(NoteRequest{
		Title:     "手动敏感",
		Text:      "普通内容",
		Sensitive: true,
	})
	if !manual.Sensitive || !containsString(manual.Tags, "敏感") {
		t.Fatalf("manual sensitive flag should still be respected: %#v", manual)
	}
}

func TestAddNoteEnrichesJSONContentTypeAndTags(t *testing.T) {
	service := NewServiceWithPath("", nil)

	entry := service.AddNote(NoteRequest{
		Title: "Gateway API response",
		Text:  `{"service":"gateway","ok":false,"error":"timeout","endpoint":"/api/v1/proxy","todo":"补复盘"}`,
	})

	if entry.ContentType != "json" {
		t.Fatalf("expected JSON content type, got %#v", entry)
	}
	for _, tag := range []string{"JSON", "API", "错误", "待办"} {
		if !containsString(entry.Tags, tag) {
			t.Fatalf("expected tag %q in %#v", tag, entry.Tags)
		}
	}
	if !strings.Contains(entry.Summary, "gateway") {
		t.Fatalf("expected local summary from content, got %q", entry.Summary)
	}
}

func TestRememberClipboardEntryDedupesBySignatureAndSearches(t *testing.T) {
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "work_memory.json"), nil)
	defer service.Stop()
	entry := clipboardhistory.Entry{
		ID:        "clip-a",
		Type:      clipboardhistory.EntryText,
		Text:      "gateway timeout copied from incident chat",
		Signature: "text:same-gateway",
		Source:    "clipboard_watcher",
		CreatedAt: time.Unix(1770001000, 0).Unix(),
		Tags:      []string{"incident"},
	}

	first := service.RememberClipboardEntry(entry)
	second := service.RememberClipboardEntry(clipboardhistory.Entry{
		ID:        "clip-b",
		Type:      clipboardhistory.EntryText,
		Text:      entry.Text,
		Signature: entry.Signature,
		Source:    "clipboard_watcher",
		CreatedAt: time.Unix(1770002000, 0).Unix(),
	})

	if first.ID == "" || second.ID != first.ID {
		t.Fatalf("expected stable memory id from clipboard signature, first=%#v second=%#v", first, second)
	}
	timeline := service.Timeline()
	if len(timeline) != 1 || timeline[0].Source != "clipboard" {
		t.Fatalf("expected one clipboard-backed memory, got %#v", timeline)
	}
	if !containsString(timeline[0].Tags, "主动沉淀") {
		t.Fatalf("expected proactive tag, got %#v", timeline[0].Tags)
	}
	if len(service.Search("incident chat")) == 0 {
		t.Fatal("clipboard memory should be searchable")
	}
}

func TestRememberCaptureHistoryEntryDedupesAndSkipsTimeMachine(t *testing.T) {
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "work_memory.json"), nil)
	defer service.Stop()
	entry := capturehistory.Entry{
		ID:        "cap-a",
		ImagePath: filepath.Join(t.TempDir(), "selection.png"),
		Source:    "overlay_selection",
		Signature: "png:same-selection",
		Width:     640,
		Height:    360,
		Bytes:     2048,
		CreatedAt: time.Unix(1770001000, 0).Unix(),
		Actions:   []string{"copy", "pin"},
	}

	first := service.RememberCaptureHistoryEntry(entry)
	second := service.RememberCaptureHistoryEntry(capturehistory.Entry{
		ID:        "cap-b",
		ImagePath: entry.ImagePath,
		Source:    "overlay_selection",
		Signature: entry.Signature,
		Width:     entry.Width,
		Height:    entry.Height,
		CreatedAt: time.Unix(1770002000, 0).Unix(),
	})
	skipped := service.RememberCaptureHistoryEntry(capturehistory.Entry{
		ID:        "cap-time-machine",
		Source:    "time_machine",
		Signature: "png:time-machine",
		CreatedAt: time.Unix(1770003000, 0).Unix(),
	})

	if first.ID == "" || second.ID != first.ID {
		t.Fatalf("expected stable memory id from capture signature, first=%#v second=%#v", first, second)
	}
	if skipped.ID != "" {
		t.Fatalf("time machine captures are already remembered directly and should be skipped, got %#v", skipped)
	}
	timeline := service.Timeline()
	if len(timeline) != 1 || timeline[0].Source != "capture_history" {
		t.Fatalf("expected one capture-history memory, got %#v", timeline)
	}
	if !containsString(timeline[0].Tags, "主动沉淀") {
		t.Fatalf("expected proactive/action tags, got %#v", timeline[0].Tags)
	}
}

func TestApplyOCRTextMakesImageMemorySearchable(t *testing.T) {
	service := NewServiceWithPath("", nil)
	entry := service.addEntry(Entry{
		ID:          "memory-ocr-image",
		Source:      "manual_capture",
		ContentType: "screenshot",
		Title:       "截图证据",
		Summary:     "截图尚未识别",
		Text:        "image evidence",
		ImagePath:   "P:\\captures\\screen.png",
		Tags:        []string{"截图"},
		CreatedAt:   time.Unix(1770003000, 0).Unix(),
	})

	updated := service.ApplyOCRText(entry.ID, "GET https://api.example.test failed with gateway timeout at OpenWrt 192.168.1.10", "test-ocr")

	if updated.OCRText == "" || updated.OCRStatus != "done:test-ocr" || updated.ContentType != "ocr_text" {
		t.Fatalf("unexpected OCR update: %#v", updated)
	}
	for _, tag := range []string{"OCR", "文字识别", "URL", "API", "错误", "网络"} {
		if !containsString(updated.Tags, tag) {
			t.Fatalf("expected OCR enrichment tag %q in %#v", tag, updated.Tags)
		}
	}
	if len(service.Search("OpenWrt")) == 0 {
		t.Fatal("OCR text should be searchable")
	}
}

func TestApplyOCRTextBuildsCleanTimelineText(t *testing.T) {
	service := NewServiceWithPath("", nil)
	entry := service.addEntry(Entry{
		ID:          "memory-ocr-clean",
		Source:      "work_memory_time_machine",
		ContentType: "screenshot",
		Title:       "work",
		Summary:     "截图尚未识别",
		Text:        "截图路径: P:\\captures\\screen.png\n尺寸: 3840x2160",
		ImagePath:   "P:\\captures\\screen.png",
		AppName:     "msedge.exe",
		WindowTitle: "work",
		Tags:        []string{"截图"},
		CreatedAt:   time.Unix(1770003000, 0).Unix(),
	})

	updated := service.ApplyOCRText(entry.ID, "work\nWails3重构项目\n时间线标题没有意义\nOCR内容需要自动整理\n截图路径: P:\\captures\\screen.png", "test-ocr")

	if updated.Title == "work" || updated.Title == "截图证据" || updated.Title == "" {
		t.Fatalf("expected cleaned OCR title, got %#v", updated.Title)
	}
	if !strings.Contains(updated.Title, "Wails3重构项目") {
		t.Fatalf("expected title to use OCR content, got %q", updated.Title)
	}
	if !strings.Contains(updated.Text, "## 画面文字整理") || !strings.Contains(updated.Text, "- 时间线标题没有意义") {
		t.Fatalf("expected cleaned OCR body, got %q", updated.Text)
	}
	if strings.Contains(updated.Text, "截图路径") {
		t.Fatalf("cleaned OCR body should remove capture metadata, got %q", updated.Text)
	}
	if !containsString(updated.Tags, "OCR整理") {
		t.Fatalf("expected OCR cleanup tag, got %#v", updated.Tags)
	}
}

func TestApplyOCRTextDoesNotMarkPlainSensitiveWords(t *testing.T) {
	service := NewServiceWithPath("", nil)
	entry := service.addEntry(Entry{
		ID:          "memory-ocr-plain-security-words",
		Source:      "work_memory_time_machine",
		ContentType: "screenshot",
		Title:       "登录设置",
		Summary:     "截图尚未识别",
		Text:        "image evidence",
		ImagePath:   "P:\\captures\\login.png",
		Tags:        []string{"截图"},
		CreatedAt:   time.Unix(1770003000, 0).Unix(),
	})

	updated := service.ApplyOCRText(entry.ID, "登录页包含密码输入框、验证码登录、token 配置说明，但没有任何真实凭据值。", "test-ocr")

	if updated.Sensitive || containsString(updated.Tags, "敏感") {
		t.Fatalf("plain sensitive wording should not isolate OCR entry: %#v", updated)
	}
	if len(service.Search("验证码登录")) == 0 {
		t.Fatal("normal OCR text should remain searchable")
	}
}

func TestApplyOCRTextUsesAISummarizerWhenConfigured(t *testing.T) {
	service := NewServiceWithPath("", nil)
	fake := &fakeOCRSummarizer{result: OCRSummaryResult{
		Title:   "时间线标题优化",
		Summary: "正在修正心流时间线截图后的标题与正文整理。",
		Text:    "## 可见内容\n- 时间线需要展示 AI 整理后的标题\n- 原始 OCR 只作为证据保留",
	}}
	RegisterOCRSummarizer(service, fake)
	service.ApplyOCRSummaryPolicy(OCRSummaryPolicy{
		Enabled:  true,
		Provider: "openai-compatible",
		BaseURL:  "http://127.0.0.1/v1",
		Model:    "glm-5.1",
	})
	entry := service.addEntry(Entry{
		ID:          "memory-ocr-ai",
		Source:      "work_memory_time_machine",
		ContentType: "screenshot",
		Title:       "work",
		Summary:     "截图尚未识别",
		Text:        "image evidence",
		ImagePath:   "P:\\captures\\screen.png",
		Tags:        []string{"截图"},
		CreatedAt:   time.Unix(1770003000, 0).Unix(),
	})

	updated := service.ApplyOCRText(entry.ID, "时间线里的标题还是没什么意义\n截图之后应该自动 OCR", "test-ocr")

	if fake.calls != 1 {
		t.Fatalf("expected AI OCR summarizer call, got %d", fake.calls)
	}
	if fake.lastJob.Provider != "openai-compatible" || fake.lastJob.Model != "glm-5.1" {
		t.Fatalf("unexpected summarizer job: %#v", fake.lastJob)
	}
	if updated.Title != "时间线标题优化" || updated.Summary != "正在修正心流时间线截图后的标题与正文整理。" {
		t.Fatalf("expected AI summary result, got %#v", updated)
	}
	if !strings.Contains(updated.Text, "AI 整理后的标题") || !containsString(updated.Tags, "AI整理") {
		t.Fatalf("expected AI body/tag, got text=%q tags=%#v", updated.Text, updated.Tags)
	}
}

func TestAutonomousFlowGeneratesArtifactsAndSuppressesRejectedSkill(t *testing.T) {
	now := time.Unix(1781589600, 0)
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "work_memory.json"), nil)
	defer service.Stop()
	service.now = func() time.Time { return now }
	service.ApplyDraftSchedule(DraftSchedulePolicy{
		Enabled:                 true,
		IntervalMinutes:         240,
		DailyDraftEnabled:       true,
		RetrospectiveEnabled:    true,
		ExperienceReportEnabled: true,
		ExperiencePeriodDays:    7,
	})
	for index, title := range []string{
		"JSON 文本格式化",
		"剪贴板 JSON 格式化",
		"URL Base64 文本处理",
		"剪贴板格式化复用",
		"剪贴板工作流宏沉淀",
		"剪贴板文本转换复用",
	} {
		service.addEntry(Entry{
			ID:          fmt.Sprintf("memory-auto-%d", index),
			Source:      "clipboard",
			ContentType: "clipboard_text",
			Title:       title,
			Summary:     "重复处理剪贴板中的 JSON、URL、Base64 文本，适合沉淀为低风险自动化 Skill。",
			Text:        "json base64 url clipboard workflow macro 格式化 剪贴板",
			AppName:     "Codex.exe",
			Tags:        []string{"clipboard", "json", "workflow"},
			CreatedAt:   now.Add(-time.Duration(index) * time.Hour).Unix(),
		})
	}

	result := service.RunAutonomousFlowNow()

	if !result.OK || result.Generated == 0 {
		t.Fatalf("expected autonomous artifacts, got %#v", result)
	}
	artifacts := service.AutonomousArtifacts()
	if len(artifacts) == 0 {
		t.Fatal("expected active autonomous artifacts")
	}
	var skill AutonomousArtifact
	for _, artifact := range artifacts {
		if artifact.Kind == "skill" {
			skill = artifact
			break
		}
	}
	if skill.ID == "" || !skill.AgentExecutable || !strings.Contains(skill.Body, "## Steps") {
		t.Fatalf("expected agent-executable skill artifact, got %#v", skill)
	}

	rejected := service.RejectAutonomousArtifact(AutonomousArtifactRejectRequest{ID: skill.ID, Reason: "这个流程不稳定，先不要自动生成"})
	if !rejected.OK || rejected.Artifact.DeleteReason == "" {
		t.Fatalf("expected rejection with reason, got %#v", rejected)
	}
	now = now.Add(24 * time.Hour)
	second := service.RunAutonomousFlowNow()
	for _, artifact := range second.Artifacts {
		if artifact.Kind == "skill" && artifact.DedupKey == skill.DedupKey {
			t.Fatalf("rejected skill should be suppressed, got %#v", second.Artifacts)
		}
	}
}

func TestAutonomousFlowRunsOncePerDayWithoutForcing(t *testing.T) {
	now := time.Unix(1781589600, 0)
	service := NewServiceWithPath("", nil)
	service.now = func() time.Time { return now }
	service.ApplyDraftSchedule(DraftSchedulePolicy{
		Enabled:                 true,
		IntervalMinutes:         240,
		DailyDraftEnabled:       true,
		RetrospectiveEnabled:    false,
		ExperienceReportEnabled: false,
		ExperiencePeriodDays:    7,
	})
	for index := 0; index < 3; index++ {
		service.addEntry(Entry{
			ID:        fmt.Sprintf("memory-daily-%d", index),
			Source:    "manual_note",
			Title:     fmt.Sprintf("今日上下文 %d", index),
			Summary:   "用于自动日报生成的非敏感证据。",
			Text:      "Ariadne 心流 自动日报 上下文",
			Tags:      []string{"daily"},
			CreatedAt: now.Add(-time.Duration(index) * time.Minute).Unix(),
		})
	}

	first := service.runAutonomousFlow(false)
	second := service.runAutonomousFlow(false)
	now = now.Add(24 * time.Hour)
	third := service.runAutonomousFlow(false)

	if !first.OK || first.Generated == 0 {
		t.Fatalf("expected first run to generate daily artifact, got %#v", first)
	}
	if second.OK || !strings.Contains(second.Message, "今天已经执行过") {
		t.Fatalf("expected second run to be skipped, got %#v", second)
	}
	if !third.OK {
		t.Fatalf("expected next-day run to execute, got %#v", third)
	}
}

func TestSemanticSearchFindsRelatedTechnicalMemory(t *testing.T) {
	service := NewServiceWithPath("", nil)
	service.addEntry(Entry{
		ID:          "memory-postgres-refused",
		Source:      "manual_note",
		ContentType: "incident",
		Title:       "PostgreSQL outage note",
		Summary:     "Service reported refused connection from API worker",
		Text:        "API worker cannot connect to PostgreSQL. Connection refused after deploy.",
		AppName:     "Windows Terminal",
		Tags:        []string{"incident"},
		CreatedAt:   time.Unix(1770004000, 0).Unix(),
	})

	results := service.Search("数据库连不上")

	found := false
	for _, result := range results {
		if result.ID != "memory-postgres-refused" {
			continue
		}
		found = true
		if result.Score <= 0 {
			t.Fatalf("semantic result should have positive score: %#v", result)
		}
		if !evidenceValueContains(result, "匹配", "本地语义") {
			t.Fatalf("semantic result should expose match evidence: %#v", result.Preview.Evidence)
		}
		if err := contracts.ValidateActionSurface(result); err != nil {
			t.Fatalf("semantic result actions should stay explicit: %v", err)
		}
	}
	if !found {
		t.Fatalf("expected semantic search to find postgres memory, got %#v", results)
	}
}

func TestSemanticStatusIsHonestLocalProvider(t *testing.T) {
	service := NewServiceWithPath("", nil)
	status := service.SemanticStatus()

	if !status.Enabled || status.Provider != "local_term_vector" || status.External {
		t.Fatalf("unexpected semantic status: %#v", status)
	}
	if !strings.Contains(status.Note, "Milvus") {
		t.Fatalf("semantic status should not imply external vector store is complete: %#v", status)
	}
}

func TestSQLiteFTSIndexPersistsAndReportsEvidence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "work_memory.json")
	service := NewServiceWithPath(path, nil)
	defer service.Stop()

	entry := service.AddNote(NoteRequest{
		Title: "FTS persistence note",
		Text:  "opaqueFTSNeedle survives process reload and should use the SQLite FTS index.",
		Tags:  []string{"fts-smoke"},
	})
	if entry.ID == "" {
		t.Fatal("expected note entry")
	}
	if _, err := os.Stat(ftsPathForMemoryPath(path)); err != nil {
		t.Fatalf("expected FTS database to exist: %v", err)
	}
	service.Stop()

	reloaded := NewServiceWithPath(path, nil)
	defer reloaded.Stop()
	results := reloaded.Search("opaqueFTSNeedle")

	if len(results) == 0 || results[0].ID != entry.ID {
		t.Fatalf("expected persisted FTS result first, got %#v", results)
	}
	if !evidenceValueContains(results[0], "匹配", "SQLite FTS") {
		t.Fatalf("expected FTS evidence, got %#v", results[0].Preview.Evidence)
	}
	if !evidenceValueContains(results[0], "命中", "opaqueFTSNeedle") {
		t.Fatalf("expected FTS snippet evidence, got %#v", results[0].Preview.Evidence)
	}
}

func TestSQLiteFTSIndexRebuildsAfterOCRAndDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "work_memory.json")
	service := NewServiceWithPath(path, nil)
	defer service.Stop()

	entry := service.addEntry(Entry{
		ID:          "memory-fts-ocr",
		Source:      "manual_capture",
		ContentType: "screenshot",
		Title:       "OCR image",
		Summary:     "Image before OCR",
		Text:        "screenshot without the OCR needle",
		ImagePath:   filepath.Join(t.TempDir(), "capture.png"),
		AppName:     "Ariadne",
		Tags:        []string{"截图"},
		CreatedAt:   time.Now().Unix(),
	})
	if entry.ID == "" {
		t.Fatal("expected capture-backed entry")
	}

	updated := service.ApplyOCRText(entry.ID, "ocrFTSNeedle appears only after OCR writeback", "test-ocr")
	if updated.OCRText == "" {
		t.Fatalf("expected OCR writeback, got %#v", updated)
	}
	results := service.Search("ocrFTSNeedle")
	if len(results) == 0 || results[0].ID != entry.ID || !evidenceValueContains(results[0], "匹配", "SQLite FTS") {
		t.Fatalf("expected OCR text to be indexed through FTS, got %#v", results)
	}

	service.Delete(entry.ID)
	results = service.Search("ocrFTSNeedle")
	for _, result := range results {
		if result.ID == entry.ID {
			t.Fatalf("deleted entry should not remain in FTS results: %#v", results)
		}
	}
}

func TestSemanticStatusReportsSQLiteFTSForPersistentStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "work_memory.json")
	service := NewServiceWithPath(path, nil)
	defer service.Stop()
	if entry := service.AddNote(NoteRequest{Title: "FTS status seed", Text: "SQLite FTS status should follow real user data."}); entry.ID == "" {
		t.Fatal("expected note entry")
	}

	status := service.SemanticStatus()

	if !status.Enabled || !status.FTSEnabled || !strings.Contains(status.Provider, "sqlite_fts5") || status.External {
		t.Fatalf("expected local SQLite FTS status, got %#v", status)
	}
	if status.FTSPath != ftsPathForMemoryPath(path) || status.IndexedEntries == 0 || status.LastIndexError != "" {
		t.Fatalf("unexpected FTS status details: %#v", status)
	}
	if !strings.Contains(status.Note, "SQLite FTS5") || !strings.Contains(status.Note, "Milvus") {
		t.Fatalf("status note should expose local FTS and pending vector store: %#v", status)
	}
}

func TestRefreshEmbeddingIndexPersistsNonSensitiveVectors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "work_memory.json")
	service := NewServiceWithPath(path, nil)
	defer service.Stop()
	clearEntriesForTest(service)
	RegisterEmbeddingClient(service, &fakeEmbeddingClient{})
	service.ApplyEmbeddingPolicy(EmbeddingPolicy{
		Enabled:          true,
		Provider:         "openai-compatible",
		BaseURL:          "http://embedding.local/v1",
		Model:            "/model/qwen_eb",
		VectorStoreType:  "embedded",
		VectorCollection: "ariadne_work_memory",
	})
	public := service.AddNote(NoteRequest{Title: "PostgreSQL timeout", Text: "database connection refused after deploy", Tags: []string{"db"}})
	secret := service.AddNote(NoteRequest{Title: "token secret", Text: "token=secret", Sensitive: true})

	result := service.RefreshEmbeddingIndex()
	if !result.OK || result.Indexed != 1 || result.Skipped == 0 {
		t.Fatalf("expected one non-sensitive vector, got %#v", result)
	}
	if result.Status.EmbeddingIndexed != 1 || !result.Status.External || !strings.Contains(result.Status.Provider, "external_embedding") {
		t.Fatalf("semantic status should report external embedding cache: %#v", result.Status)
	}
	if _, err := os.Stat(appdb.DatabasePathForPath(filepath.Join(filepath.Dir(path), "work_memory_vectors.json"))); err != nil {
		t.Fatalf("expected embedded vector cache database: %v", err)
	}

	search := service.SemanticSearchExternal("database refused")
	if !search.OK || len(search.Results) == 0 || search.Results[0].ID != public.ID {
		t.Fatalf("expected embedding search to find public entry, got %#v", search)
	}
	for _, item := range search.Results {
		if item.ID == secret.ID {
			t.Fatalf("sensitive entry should never appear in embedding search: %#v", search.Results)
		}
	}
}

func TestEmbeddingRefreshBlocksPrivacyAndMissingClient(t *testing.T) {
	service := NewServiceWithPath("", nil)
	service.ApplyEmbeddingPolicy(EmbeddingPolicy{Enabled: true, Provider: "openai-compatible", Model: "/model/qwen_eb"})
	missingClient := service.RefreshEmbeddingIndex()
	if missingClient.OK || !strings.Contains(missingClient.Message, "客户端") {
		t.Fatalf("expected missing client failure, got %#v", missingClient)
	}

	RegisterEmbeddingClient(service, &fakeEmbeddingClient{})
	service.SetPrivacyMode(true)
	blocked := service.RefreshEmbeddingIndex()
	if blocked.OK || !strings.Contains(blocked.Message, "隐私模式") {
		t.Fatalf("expected privacy failure, got %#v", blocked)
	}
}

func TestMilvusVectorStoreUsesRESTAdapter(t *testing.T) {
	var upserted []map[string]any
	var deleteFilter string
	collectionCreated := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("invalid request payload: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v2/vectordb/collections/list":
			_, _ = io.WriteString(w, `{"code":0,"data":[]}`)
		case "/v2/vectordb/collections/create":
			collectionCreated = true
			if payload["collectionName"] != "ariadne_work_memory_test" {
				t.Errorf("unexpected collection name: %#v", payload["collectionName"])
			}
			_, _ = io.WriteString(w, `{"code":0,"data":{}}`)
		case "/v2/vectordb/collections/load":
			_, _ = io.WriteString(w, `{"code":0,"data":{}}`)
		case "/v2/vectordb/entities/delete":
			deleteFilter, _ = payload["filter"].(string)
			_, _ = io.WriteString(w, `{"code":0,"data":{"deleteCount":0}}`)
		case "/v2/vectordb/entities/upsert":
			rawRows, _ := payload["data"].([]any)
			for _, rawRow := range rawRows {
				if row, ok := rawRow.(map[string]any); ok {
					upserted = append(upserted, row)
				}
			}
			_, _ = io.WriteString(w, `{"code":0,"data":{"upsertCount":1}}`)
		case "/v2/vectordb/entities/search":
			filter, _ := payload["filter"].(string)
			if !strings.Contains(filter, "ariadne_") {
				t.Errorf("search should include Ariadne namespace filter, got %q", filter)
			}
			_, _ = io.WriteString(w, `{"code":0,"data":[{"id":"ns::entry","entry_id":"`+upserted[0]["entry_id"].(string)+`","distance":0.93}],"topks":[1]}`)
		default:
			t.Errorf("unexpected Milvus path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "work_memory.json")
	service := NewServiceWithPath(path, nil)
	defer service.Stop()
	clearEntriesForTest(service)
	RegisterEmbeddingClient(service, &fakeEmbeddingClient{})
	service.ApplyEmbeddingPolicy(EmbeddingPolicy{
		Enabled:          true,
		Provider:         "openai-compatible",
		Model:            "/model/qwen_eb",
		VectorStoreType:  "milvus",
		VectorStoreURI:   server.URL,
		VectorCollection: "ariadne_work_memory_test",
	})
	public := service.AddNote(NoteRequest{Title: "PostgreSQL timeout", Text: "database connection refused after deploy"})
	secret := service.AddNote(NoteRequest{Title: "token secret", Text: "token=secret", Sensitive: true})

	refresh := service.RefreshEmbeddingIndex()
	if !refresh.OK || refresh.Indexed != 1 || refresh.Status.VectorStoreType != "milvus" || !refresh.Status.External {
		t.Fatalf("expected Milvus refresh success, got %#v", refresh)
	}
	if !collectionCreated || len(upserted) != 1 || upserted[0]["entry_id"] != public.ID {
		t.Fatalf("expected one public row to be upserted, created=%v rows=%#v", collectionCreated, upserted)
	}
	if strings.Contains(deleteFilter, secret.ID) || !strings.Contains(deleteFilter, "ariadne_") {
		t.Fatalf("delete filter should be namespace-scoped and not mention entry ids, got %q", deleteFilter)
	}
	metadata, ok, err := loadEmbeddingStateFromSQLite(filepath.Join(filepath.Dir(path), "work_memory_vectors.json"))
	if err != nil || !ok {
		t.Fatalf("Milvus refresh should persist metadata without local vectors, ok=%v err=%v", ok, err)
	}
	for _, record := range metadata.Records {
		if len(record.Vector) > 0 {
			t.Fatalf("Milvus metadata should not persist local vector values: %#v", metadata)
		}
	}
	reloaded := NewServiceWithPath(path, nil)
	defer reloaded.Stop()
	reloaded.ApplyEmbeddingPolicy(EmbeddingPolicy{
		Enabled:          true,
		Provider:         "openai-compatible",
		Model:            "/model/qwen_eb",
		VectorStoreType:  "milvus",
		VectorStoreURI:   server.URL,
		VectorCollection: "ariadne_work_memory_test",
	})
	reloadedStatus := reloaded.SemanticStatus()
	if reloadedStatus.EmbeddingIndexed != 1 || !reloadedStatus.External || reloadedStatus.Mode != "hybrid" {
		t.Fatalf("reloaded Milvus metadata should report hybrid index: %#v", reloadedStatus)
	}
	search := service.SemanticSearchExternal("数据库")
	if !search.OK || len(search.Results) != 1 || search.Results[0].ID != public.ID {
		t.Fatalf("expected Milvus semantic search hit, got %#v", search)
	}
	for _, item := range search.Results {
		if item.ID == secret.ID {
			t.Fatalf("sensitive entry should never appear in Milvus search results: %#v", search.Results)
		}
	}
}

func TestMilvusVectorStoreRequiresURI(t *testing.T) {
	service := NewServiceWithPath("", nil)
	defer service.Stop()
	RegisterEmbeddingClient(service, &fakeEmbeddingClient{})
	service.ApplyEmbeddingPolicy(EmbeddingPolicy{Enabled: true, Provider: "openai-compatible", Model: "/model/qwen_eb", VectorStoreType: "milvus"})
	service.AddNote(NoteRequest{Title: "数据库排查", Text: "database connection refused"})

	refresh := service.RefreshEmbeddingIndex()
	if refresh.OK || !strings.Contains(refresh.Message, "Milvus URI") {
		t.Fatalf("expected missing Milvus URI failure, got %#v", refresh)
	}
}

func TestDeleteAndClearUnpinnedKeepFavorites(t *testing.T) {
	service := NewServiceWithPath("", nil)
	transient := service.AddNote(NoteRequest{Title: "临时记录", Text: "稍后清理"})
	favorite := service.AddNote(NoteRequest{Title: "收藏记录", Text: "长期保留", Favorite: true})

	status := service.Delete(transient.ID)
	if status.EntryCount < 1 {
		t.Fatalf("expected entries after delete, got %#v", status)
	}
	for _, entry := range service.Timeline() {
		if entry.ID == transient.ID {
			t.Fatal("deleted entry should not remain in timeline")
		}
	}

	status = service.ClearUnpinned()
	if status.EntryCount == 0 {
		t.Fatal("clear unpinned should keep favorite entries")
	}
	foundFavorite := false
	for _, entry := range service.Timeline() {
		if !entry.Favorite {
			t.Fatalf("non-favorite entry should be cleared: %#v", entry)
		}
		if entry.ID == favorite.ID {
			foundFavorite = true
		}
	}
	if !foundFavorite {
		t.Fatal("favorite manual note should be kept")
	}
}

func TestChangeObserverPublishesEntryLifecycle(t *testing.T) {
	service := NewServiceWithPath("", nil)
	clearEntriesForTest(service)

	events := make(chan ChangeEvent, 8)
	RegisterChangeObserver(service, func(event ChangeEvent) {
		events <- event
	})

	entry := service.AddNote(NoteRequest{Title: "实时刷新", Text: "时间线应该实时出现新内容"})
	if entry.ID == "" {
		t.Fatal("expected note entry")
	}
	created := waitChangeEventForTest(t, events, "entry_upserted", entry.ID)
	if created.EntryCount != 1 || created.Source != "manual_note" {
		t.Fatalf("unexpected create event: %#v", created)
	}

	updated := service.ApplyOCRText(entry.ID, "ocr live update needle", "test-ocr")
	if updated.OCRText == "" {
		t.Fatalf("expected OCR update: %#v", updated)
	}
	changed := waitChangeEventForTest(t, events, "entry_updated", entry.ID)
	if changed.EntryCount != 1 {
		t.Fatalf("unexpected update event: %#v", changed)
	}

	status := service.Delete(entry.ID)
	if status.EntryCount != 0 {
		t.Fatalf("expected deleted entry count: %#v", status)
	}
	deleted := waitChangeEventForTest(t, events, "entry_deleted", entry.ID)
	if deleted.EntryCount != 0 {
		t.Fatalf("unexpected delete event: %#v", deleted)
	}
}

func TestRetentionPolicyRemovesOldEntriesButKeepsFavorites(t *testing.T) {
	service := NewServiceWithPath("", nil)
	now := time.Unix(1772000000, 0)
	service.now = func() time.Time { return now }
	service.entries = nil
	service.addEntry(Entry{
		ID:        "memory-old",
		Source:    "manual_note",
		Title:     "过期记录",
		Summary:   "应清理",
		Text:      "old transient note",
		CreatedAt: now.Add(-40 * 24 * time.Hour).Unix(),
	})
	service.addEntry(Entry{
		ID:        "memory-old-favorite",
		Source:    "manual_note",
		Title:     "过期收藏",
		Summary:   "应保留",
		Text:      "old favorite note",
		Favorite:  true,
		CreatedAt: now.Add(-50 * 24 * time.Hour).Unix(),
	})
	service.addEntry(Entry{
		ID:        "memory-recent",
		Source:    "manual_note",
		Title:     "近期记录",
		Summary:   "应保留",
		Text:      "recent note",
		CreatedAt: now.Add(-2 * 24 * time.Hour).Unix(),
	})

	result := service.ApplyRetentionPolicy(30, true)

	if !result.OK || result.Removed != 1 || result.KeptFavorites != 1 || result.RemainingCount != 2 {
		t.Fatalf("unexpected retention result: %#v", result)
	}
	if service.Entry("memory-old").ID != "" {
		t.Fatal("old non-favorite memory should be removed")
	}
	if service.Entry("memory-old-favorite").ID == "" || service.Entry("memory-recent").ID == "" {
		t.Fatal("favorite and recent memories should be kept")
	}
}

func TestExportDataSkipsSensitiveEntriesAndIncludesReadableFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "work_memory.json")
	imagePath := filepath.Join(dir, "screen.png")
	if err := os.WriteFile(imagePath, []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithPath(path, nil)
	defer service.Stop()
	normal := service.AddNote(NoteRequest{Title: "普通记录", Text: "可导出的工作记忆"})
	sensitive := service.AddNote(NoteRequest{Title: "敏感记录", Text: "password=secret"})
	service.addEntry(Entry{
		ID:          "memory-image",
		Source:      "manual_capture",
		ContentType: "screenshot",
		Title:       "截图证据",
		Summary:     "带图片证据",
		Text:        "image evidence",
		OCRText:     "可读 OCR 文本",
		OCRStatus:   "done:test-ocr",
		ImagePath:   imagePath,
		Tags:        []string{"截图"},
		CreatedAt:   time.Unix(1770002000, 0).Unix(),
	})

	result := service.ExportData(false)
	if !result.OK {
		t.Fatalf("export should succeed: %#v", result)
	}
	if result.SkippedSensitiveCount != 1 || result.IncludesSensitive {
		t.Fatalf("export should skip one sensitive entry: %#v", result)
	}
	if result.EntryCount == 0 || result.Path == "" || result.Bytes == 0 {
		t.Fatalf("export result missing metadata: %#v", result)
	}

	archive, err := zip.OpenReader(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()

	names := map[string]bool{}
	var exported struct {
		Entries []Entry `json:"entries"`
	}
	for _, file := range archive.File {
		names[file.Name] = true
		if file.Name == "work_memory.json" {
			reader, err := file.Open()
			if err != nil {
				t.Fatal(err)
			}
			if err := json.NewDecoder(reader).Decode(&exported); err != nil {
				reader.Close()
				t.Fatal(err)
			}
			reader.Close()
		}
	}
	if !names["README.md"] || !names["timeline.md"] || !names["work_memory.json"] {
		t.Fatalf("expected readable export files, got %#v", names)
	}
	if !names["evidence/memory-image/screen.png"] {
		t.Fatalf("expected image evidence in export, got %#v", names)
	}
	for _, entry := range exported.Entries {
		if entry.ID == sensitive.ID {
			t.Fatalf("sensitive entry should be skipped: %#v", exported.Entries)
		}
	}
	if !containsExportedEntry(exported.Entries, normal.ID) {
		t.Fatalf("normal entry should be exported: %#v", exported.Entries)
	}
	timeline := readZipText(t, archive, "timeline.md")
	if !strings.Contains(timeline, "可读 OCR 文本") {
		t.Fatalf("timeline should include OCR text, got %q", timeline)
	}
}

func TestExportDataWithOptionsFiltersByTimeTagsAndIDs(t *testing.T) {
	dir := t.TempDir()
	service := NewServiceWithPath(filepath.Join(dir, "work_memory.json"), nil)
	defer service.Stop()
	clearEntriesForTest(service)
	now := time.Unix(1770011000, 0)
	network := service.addEntry(Entry{
		ID:        "memory-network-export",
		Source:    "manual_note",
		Title:     "网络记录",
		Text:      "network export",
		Tags:      []string{"network", "incident"},
		CreatedAt: now.Add(-1 * time.Hour).Unix(),
	})
	ui := service.addEntry(Entry{
		ID:        "memory-ui-export",
		Source:    "manual_note",
		Title:     "UI 记录",
		Text:      "ui export",
		Tags:      []string{"ui"},
		CreatedAt: now.Add(-30 * time.Minute).Unix(),
	})
	service.addEntry(Entry{
		ID:        "memory-old-network-export",
		Source:    "manual_note",
		Title:     "旧网络记录",
		Text:      "old network export",
		Tags:      []string{"network"},
		CreatedAt: now.Add(-72 * time.Hour).Unix(),
	})
	service.now = func() time.Time { return now }

	filtered := service.ExportDataWithOptions(ExportRequest{
		IncludeSensitive: true,
		StartAt:          now.Add(-2 * time.Hour).Unix(),
		Tags:             []string{"network"},
	})
	if !filtered.OK || filtered.EntryCount != 1 || filtered.FilteredOutCount != 2 {
		t.Fatalf("expected time+tag filtered export, got %#v", filtered)
	}
	archive, err := zip.OpenReader(filtered.Path)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		FilteredOutCount int          `json:"filteredOutCount"`
		Filter           ExportFilter `json:"filter"`
		Entries          []Entry      `json:"entries"`
	}
	for _, file := range archive.File {
		if file.Name != "work_memory.json" {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		if err := json.NewDecoder(reader).Decode(&payload); err != nil {
			reader.Close()
			t.Fatal(err)
		}
		reader.Close()
	}
	if payload.FilteredOutCount != 2 || len(payload.Filter.Tags) != 1 || payload.Filter.Tags[0] != "network" || payload.Filter.StartAt != now.Add(-2*time.Hour).Unix() {
		t.Fatalf("expected filter metadata in package, got %#v", payload)
	}
	if !containsExportedEntry(payload.Entries, network.ID) || containsExportedEntry(payload.Entries, ui.ID) {
		t.Fatalf("filtered export should include network entry only, got %#v", payload.Entries)
	}
	readme := readZipText(t, archive, "README.md")
	if !strings.Contains(readme, "Filtered out entries: 2") || !strings.Contains(readme, "tags=network") {
		t.Fatalf("README should describe filter, got %q", readme)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}

	byID := service.ExportDataWithOptions(ExportRequest{IncludeSensitive: true, EntryIDs: []string{ui.ID}})
	if !byID.OK || byID.EntryCount != 1 || byID.FilteredOutCount != 2 {
		t.Fatalf("expected id-filtered export, got %#v", byID)
	}
}

func TestImportMaterialsImportsMarkdownAndImageEvidence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "work_memory.json")
	markdownPath := filepath.Join(dir, "migration.md")
	imagePath := filepath.Join(dir, "screen.png")
	if err := os.WriteFile(markdownPath, []byte("# 迁移记录\n\nOpenWrt gateway timeout needs review."), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(imagePath, []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}

	service := NewServiceWithPath(path, nil)
	defer service.Stop()
	clearEntriesForTest(service)
	service.now = func() time.Time { return time.Unix(1770010000, 0) }

	result := service.ImportMaterials(ImportMaterialRequest{
		Paths:    []string{markdownPath, imagePath},
		Tags:     []string{"迁移"},
		Favorite: true,
	})

	if !result.OK || result.Imported != 2 || len(result.Entries) != 2 {
		t.Fatalf("expected two imported entries, got %#v", result)
	}
	if result.Entries[0].Title != "迁移记录" || result.Entries[0].ContentType != "markdown" || !result.Entries[0].Favorite {
		t.Fatalf("unexpected markdown entry: %#v", result.Entries[0])
	}
	if len(service.Search("OpenWrt gateway")) == 0 {
		t.Fatal("imported markdown should be searchable")
	}

	imageEntry := result.Entries[1]
	if imageEntry.ContentType != "image" || imageEntry.ImagePath == "" || imageEntry.ImagePath == imagePath {
		t.Fatalf("expected copied image evidence, got %#v", imageEntry)
	}
	if !strings.Contains(imageEntry.ImagePath, "work_memory_images") {
		t.Fatalf("image evidence should be copied into work_memory_images, got %q", imageEntry.ImagePath)
	}
	raw, err := os.ReadFile(imageEntry.ImagePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "png" {
		t.Fatalf("unexpected copied image contents: %q", raw)
	}
	if service.Status().EntryCount != 2 {
		t.Fatalf("status should reflect imported entries: %#v", service.Status())
	}
}

func TestImportMaterialsImportsAriadneExportZipWithEvidence(t *testing.T) {
	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "work_memory.json")
	sourceImagePath := filepath.Join(sourceDir, "screen.png")
	if err := os.WriteFile(sourceImagePath, []byte("png-evidence"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := NewServiceWithPath(sourcePath, nil)
	defer source.Stop()
	clearEntriesForTest(source)
	source.AddNote(NoteRequest{Title: "普通记录", Text: "可迁移的工作记忆"})
	source.addEntry(Entry{
		ID:          "memory-image-import",
		Source:      "manual_capture",
		ContentType: "screenshot",
		Title:       "截图证据",
		Summary:     "带图片证据",
		Text:        "image evidence",
		ImagePath:   sourceImagePath,
		Tags:        []string{"截图"},
		CreatedAt:   time.Unix(1770010100, 0).Unix(),
	})
	exported := source.ExportData(true)
	if !exported.OK {
		t.Fatalf("export should succeed: %#v", exported)
	}

	targetDir := t.TempDir()
	target := NewServiceWithPath(filepath.Join(targetDir, "work_memory.json"), nil)
	defer target.Stop()
	clearEntriesForTest(target)
	result := target.ImportMaterials(ImportMaterialRequest{Paths: []string{exported.Path}, Tags: []string{"回灌"}})

	if !result.OK || result.Imported != 2 || len(result.Entries) != 2 {
		t.Fatalf("expected export package import, got %#v", result)
	}
	importedImage := target.Entry("memory-image-import")
	if importedImage.ID == "" || importedImage.ImagePath == "" {
		t.Fatalf("expected image entry with extracted evidence, got %#v", importedImage)
	}
	if !strings.Contains(importedImage.ImagePath, filepath.Join(targetDir, "work_memory_images")) {
		t.Fatalf("expected evidence extracted under target store, got %q", importedImage.ImagePath)
	}
	raw, err := os.ReadFile(importedImage.ImagePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "png-evidence" {
		t.Fatalf("unexpected extracted evidence contents: %q", raw)
	}
	if !containsString(importedImage.Tags, "ariadne_export") || !containsString(importedImage.Tags, "回灌") {
		t.Fatalf("expected import tags, got %#v", importedImage.Tags)
	}
}

func TestImportMaterialsImportsPDFAndOfficeDocuments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "work_memory.json")
	docxPath := filepath.Join(dir, "brief.docx")
	pdfPath := filepath.Join(dir, "incident.pdf")
	createTestZip(t, docxPath, map[string]string{
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Ariadne Office Import</w:t></w:r></w:p></w:body></w:document>`,
	})
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4\n1 0 obj\n<<>>\nstream\nBT (Ariadne PDF Import) Tj ET\nendstream\nendobj\n%%EOF"), 0o600); err != nil {
		t.Fatal(err)
	}

	service := NewServiceWithPath(path, nil)
	defer service.Stop()
	clearEntriesForTest(service)
	service.now = func() time.Time { return time.Unix(1770010200, 0) }

	result := service.ImportMaterials(ImportMaterialRequest{Paths: []string{docxPath, pdfPath}, Tags: []string{"材料"}})

	if !result.OK || result.Imported != 2 || len(result.Entries) != 2 {
		t.Fatalf("expected two document imports, got %#v", result)
	}
	if len(service.Search("Ariadne Office Import")) == 0 {
		t.Fatal("imported docx text should be searchable")
	}
	if len(service.Search("Ariadne PDF Import")) == 0 {
		t.Fatal("imported pdf text should be searchable")
	}
	if result.Entries[0].ContentType != "office_document" || !containsString(result.Entries[0].Tags, "Office 文档") {
		t.Fatalf("unexpected office entry: %#v", result.Entries[0])
	}
	if result.Entries[1].ContentType != "pdf" || !containsString(result.Entries[1].Tags, "PDF") {
		t.Fatalf("unexpected pdf entry: %#v", result.Entries[1])
	}
}

func TestImportMaterialsKeepsMetadataForLegacyOfficeDocument(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "work_memory.json")
	docPath := filepath.Join(dir, "legacy.doc")
	if err := os.WriteFile(docPath, []byte("binary-office"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithPath(path, nil)
	defer service.Stop()
	clearEntriesForTest(service)

	result := service.ImportMaterials(ImportMaterialRequest{Paths: []string{docPath}})

	if !result.OK || result.Imported != 1 || len(result.Entries) != 1 {
		t.Fatalf("expected metadata import for legacy office document, got %#v", result)
	}
	entry := result.Entries[0]
	if entry.ContentType != "office_document" || !strings.Contains(entry.Text, "旧版 Office 二进制格式") {
		t.Fatalf("expected legacy office metadata entry, got %#v", entry)
	}
	if len(service.Search("legacy.doc")) == 0 {
		t.Fatal("legacy office metadata should be searchable by filename")
	}
}

func TestImportMaterialsRespectsPathAndContentExclusions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "work_memory.json")
	excludedPath := filepath.Join(dir, "secret-note.md")
	excludedContentPath := filepath.Join(dir, "content.md")
	excludedURLPath := filepath.Join(dir, "url.md")
	allowedPath := filepath.Join(dir, "public.md")
	if err := os.WriteFile(excludedPath, []byte("# Secret\nshould never import"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(excludedContentPath, []byte("# Incident\nclassified incident notes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(excludedURLPath, []byte("# Link\nsee https://private.example.com/ticket/42 before importing"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(allowedPath, []byte("# Public\nsafe incident notes"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithPath(path, nil)
	defer service.Stop()
	clearEntriesForTest(service)
	service.ApplyCapturePolicy(CapturePolicy{
		ExcludePaths:           []string{"secret-note.md"},
		ExcludeURLs:            []string{"private.example.com/ticket"},
		ExcludeContentPatterns: []string{"classified"},
	})

	result := service.ImportMaterials(ImportMaterialRequest{Paths: []string{excludedPath, excludedContentPath, excludedURLPath, allowedPath}})

	if !result.OK || result.Imported != 1 || result.Skipped != 3 || len(result.Entries) != 1 {
		t.Fatalf("expected one import and three exclusion skips, got %#v", result)
	}
	if result.Entries[0].Title != "Public" {
		t.Fatalf("unexpected imported entry: %#v", result.Entries[0])
	}
	if service.Status().EntryCount != 1 {
		t.Fatalf("only allowed material should be persisted: %#v", service.Status())
	}
}

func TestApplyOCRTextBlocksExcludedContent(t *testing.T) {
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "work_memory.json"), nil)
	defer service.Stop()
	clearEntriesForTest(service)
	service.ApplyCapturePolicy(CapturePolicy{ExcludeContentPatterns: []string{"classified"}})
	entry := service.addEntry(Entry{
		ID:          "memory-image-ocr",
		Source:      "manual_capture",
		ContentType: "screenshot",
		Title:       "OCR target",
		Summary:     "screen",
		ImagePath:   filepath.Join(t.TempDir(), "screen.png"),
		CreatedAt:   time.Unix(1770010300, 0).Unix(),
	})

	updated := service.ApplyOCRText(entry.ID, "classified OCR text", "test")

	if !strings.HasPrefix(updated.OCRStatus, "blocked_excluded") {
		t.Fatalf("expected excluded OCR status, got %#v", updated)
	}
	if updated.OCRText != "" || updated.ContentType == "ocr_text" {
		t.Fatalf("excluded OCR text should not be persisted, got %#v", updated)
	}
}

func TestApplyOCRTextBlocksExcludedURL(t *testing.T) {
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "work_memory.json"), nil)
	defer service.Stop()
	clearEntriesForTest(service)
	service.ApplyCapturePolicy(CapturePolicy{ExcludeURLs: []string{"private.example.com/ticket"}})
	entry := service.addEntry(Entry{
		ID:          "memory-image-url-ocr",
		Source:      "manual_capture",
		ContentType: "screenshot",
		Title:       "OCR target",
		Summary:     "screen",
		ImagePath:   filepath.Join(t.TempDir(), "screen.png"),
		CreatedAt:   time.Unix(1770010300, 0).Unix(),
	})

	updated := service.ApplyOCRText(entry.ID, "open https://private.example.com/ticket/42", "test")

	if !strings.HasPrefix(updated.OCRStatus, "blocked_excluded:url:") {
		t.Fatalf("expected excluded URL OCR status, got %#v", updated)
	}
	if updated.OCRText != "" || updated.ContentType == "ocr_text" {
		t.Fatalf("excluded OCR URL should not be persisted, got %#v", updated)
	}
}

func TestExportDataSkipsExcludedEntriesAndRecordsCount(t *testing.T) {
	dir := t.TempDir()
	service := NewServiceWithPath(filepath.Join(dir, "work_memory.json"), nil)
	defer service.Stop()
	clearEntriesForTest(service)
	normal := service.AddNote(NoteRequest{Title: "普通记录", Text: "safe export"})
	excluded := service.AddNote(NoteRequest{Title: "排除记录", Text: "classified export"})
	excludedURL := service.AddNote(NoteRequest{Title: "URL 排除记录", Text: "open https://private.example.com/ticket/42"})
	service.ApplyCapturePolicy(CapturePolicy{
		ExcludeContentPatterns: []string{"classified"},
		ExcludeURLs:            []string{"private.example.com/ticket"},
	})

	result := service.ExportData(true)
	if !result.OK || result.EntryCount != 1 || result.SkippedExcludedCount != 2 {
		t.Fatalf("expected excluded export count, got %#v", result)
	}

	archive, err := zip.OpenReader(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	var exported struct {
		SkippedExcludedCount int     `json:"skippedExcludedCount"`
		Entries              []Entry `json:"entries"`
	}
	for _, file := range archive.File {
		if file.Name != "work_memory.json" {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		if err := json.NewDecoder(reader).Decode(&exported); err != nil {
			reader.Close()
			t.Fatal(err)
		}
		reader.Close()
	}
	if exported.SkippedExcludedCount != 2 {
		t.Fatalf("expected skipped excluded count in package, got %#v", exported)
	}
	if containsExportedEntry(exported.Entries, excluded.ID) || containsExportedEntry(exported.Entries, excludedURL.ID) || !containsExportedEntry(exported.Entries, normal.ID) {
		t.Fatalf("export should include normal and skip excluded, got %#v", exported.Entries)
	}
	readme := readZipText(t, archive, "README.md")
	if !strings.Contains(readme, "Skipped excluded entries: 2") {
		t.Fatalf("README should report excluded count, got %q", readme)
	}
}

func TestImportMaterialsRejectsUnsafeOrUnsupportedInputs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "work_memory.json")
	unsupported := filepath.Join(dir, "raw.bin")
	textPath := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(unsupported, []byte("raw"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(textPath, []byte("should not import in privacy mode"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithPath(path, nil)
	defer service.Stop()
	clearEntriesForTest(service)

	result := service.ImportMaterials(ImportMaterialRequest{Paths: []string{dir, unsupported}})
	if result.OK || result.Imported != 0 || result.Skipped != 2 || len(result.Items) != 2 {
		t.Fatalf("expected directory and unsupported file to be skipped, got %#v", result)
	}

	service.SetPrivacyMode(true)
	blocked := service.ImportMaterials(ImportMaterialRequest{Paths: []string{textPath}})
	if blocked.OK || blocked.Failed != 1 || !strings.Contains(blocked.Message, "隐私模式") {
		t.Fatalf("privacy mode should block imports, got %#v", blocked)
	}
	if service.Status().EntryCount != 0 {
		t.Fatalf("privacy mode import should not mutate timeline: %#v", service.Status())
	}
}

type fakeCapturer struct {
	calls   int
	sources []string
	options []capturehistory.CaptureOptions
}

func (f *fakeCapturer) CaptureScreenWithOptions(source string, options capturehistory.CaptureOptions) capturehistory.Status {
	f.options = append(f.options, options)
	return f.CaptureScreen(source)
}

func (f *fakeCapturer) CaptureScreen(source string) capturehistory.Status {
	f.calls++
	f.sources = append(f.sources, source)
	now := time.Unix(1770001000+int64(f.calls), 0)
	return capturehistory.Status{
		Entries: []capturehistory.Entry{
			{
				ID:        "capture-" + source + "-" + now.Format("150405"),
				ImagePath: filepath.Join("P:\\captures", source+".png"),
				CreatedAt: now.Unix(),
				Source:    source,
				Width:     1440,
				Height:    900,
				Bytes:     4096,
				Signature: "fake:" + source,
				Tags:      []string{"截图", "捕获历史", "1440x900"},
			},
		},
	}
}

type sequenceCapturer struct {
	calls   int
	sources []string
	options []capturehistory.CaptureOptions
	entries []capturehistory.Entry
}

func (f *sequenceCapturer) CaptureScreenWithOptions(source string, options capturehistory.CaptureOptions) capturehistory.Status {
	f.options = append(f.options, options)
	return f.CaptureScreen(source)
}

func (f *sequenceCapturer) CaptureScreen(source string) capturehistory.Status {
	f.calls++
	f.sources = append(f.sources, source)
	if len(f.entries) == 0 {
		return capturehistory.Status{LastCaptureError: "empty test capture sequence"}
	}
	index := f.calls - 1
	if index >= len(f.entries) {
		index = len(f.entries) - 1
	}
	entry := f.entries[index]
	entry.Source = source
	if entry.ID == "" {
		entry.ID = "capture-" + source + "-" + time.Unix(entry.CreatedAt, 0).Format("150405")
	}
	if entry.CreatedAt == 0 {
		entry.CreatedAt = time.Unix(1770002000+int64(f.calls), 0).Unix()
	}
	return capturehistory.Status{Entries: []capturehistory.Entry{entry}}
}

type hookedCapturer struct {
	calls     int
	sources   []string
	options   []capturehistory.CaptureOptions
	entry     capturehistory.Entry
	onCapture func()
}

func (f *hookedCapturer) CaptureScreenWithOptions(source string, options capturehistory.CaptureOptions) capturehistory.Status {
	f.options = append(f.options, options)
	return f.CaptureScreen(source)
}

func (f *hookedCapturer) CaptureScreen(source string) capturehistory.Status {
	f.calls++
	f.sources = append(f.sources, source)
	if f.onCapture != nil {
		f.onCapture()
	}
	entry := f.entry
	entry.Source = source
	if entry.ID == "" {
		entry.ID = "capture-" + source
	}
	return capturehistory.Status{Entries: []capturehistory.Entry{entry}}
}

func testCaptureEntry(id string, imagePath string, signature string, width int, height int, createdAt int64) capturehistory.Entry {
	return capturehistory.Entry{
		ID:        "capture-" + id,
		ImagePath: imagePath,
		CreatedAt: createdAt,
		Width:     width,
		Height:    height,
		Bytes:     int64(width * height * 4),
		Signature: signature,
		Tags:      []string{"截图", "捕获历史"},
	}
}

func writeMemoryTestPNG(t *testing.T, path string, width int, height int, paint func(*image.RGBA)) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	paint(img)
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, img); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func paintSimilarMemoryScreen(variant int) func(*image.RGBA) {
	return func(img *image.RGBA) {
		bounds := img.Bounds()
		width := bounds.Dx()
		height := bounds.Dy()
		denomX := width - 1
		if denomX <= 0 {
			denomX = 1
		}
		denomY := height - 1
		if denomY <= 0 {
			denomY = 1
		}
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				localX := x - bounds.Min.X
				localY := y - bounds.Min.Y
				shade := 70 + (localX*80)/denomX + (localY*30)/denomY
				if variant == 1 && localX >= width/3 && localX < width/3+8 && localY >= height/3 && localY < height/3+8 {
					shade += 6
				}
				img.SetRGBA(x, y, color.RGBA{
					R: uint8(clampByte(shade)),
					G: uint8(clampByte(shade + 4)),
					B: uint8(clampByte(shade + 8)),
					A: 255,
				})
			}
		}
	}
}

func paintSolidMemoryScreen(luma uint8) func(*image.RGBA) {
	return func(img *image.RGBA) {
		bounds := img.Bounds()
		fill := color.RGBA{R: luma, G: luma, B: luma, A: 255}
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				img.SetRGBA(x, y, fill)
			}
		}
	}
}

type fakeDraftPolisher struct {
	calls   int
	lastJob DraftPolishJob
}

func (f *fakeDraftPolisher) PolishDraft(_ context.Context, job DraftPolishJob) (Draft, error) {
	f.calls++
	f.lastJob = job
	return Draft{
		Title:    "AI 润色：" + job.Draft.Title,
		Body:     "润色后的日报草稿\n\n证据：" + strings.Join(job.Draft.Evidence, ", "),
		Evidence: append([]string(nil), job.Draft.Evidence...),
	}, nil
}

type fakeFlowAgentRunner struct {
	calls   int
	lastJob FlowAgentJob
	result  FlowAgentResult
	err     error
	onCall  func(FlowAgentJob)
}

func (f *fakeFlowAgentRunner) AnswerFlow(_ context.Context, job FlowAgentJob) (FlowAgentResult, error) {
	f.calls++
	f.lastJob = job
	if f.onCall != nil {
		f.onCall(job)
	}
	if f.err != nil {
		return FlowAgentResult{}, f.err
	}
	return f.result, nil
}

type fakeExperienceDiscoverer struct {
	calls   int
	lastJob ExperienceDiscoveryJob
	report  ExperienceReport
	err     error
}

func (f *fakeExperienceDiscoverer) DiscoverExperiences(_ context.Context, job ExperienceDiscoveryJob) (ExperienceReport, error) {
	f.calls++
	f.lastJob = job
	if f.err != nil {
		return ExperienceReport{}, f.err
	}
	if f.report.ID != "" || f.report.Title != "" || len(f.report.Insights) > 0 {
		return f.report, nil
	}
	return ExperienceReport{
		Title:   "AI 经验发现报告",
		Summary: "AI 发现 1 条经验线索",
		Insights: []ExperienceInsight{
			{
				Kind:           "repeated_issue",
				Title:          "默认 AI 线索",
				Summary:        "发现重复问题",
				Reason:         "多条 evidence 具有相同模式",
				Recommendation: "人工审核后沉淀为清单",
				Evidence:       []string{job.Evidence[0].ID},
				Confidence:     0.7,
				Severity:       "medium",
			},
		},
	}, nil
}

type fakeEmbeddingClient struct {
	calls  int
	inputs []string
}

func (f *fakeEmbeddingClient) EmbedTexts(_ context.Context, job EmbeddingJob) ([][]float64, error) {
	f.calls++
	f.inputs = append(f.inputs, job.Inputs...)
	vectors := make([][]float64, 0, len(job.Inputs))
	for _, input := range job.Inputs {
		text := strings.ToLower(input)
		switch {
		case strings.Contains(text, "database") || strings.Contains(text, "postgres") || strings.Contains(text, "refused"):
			vectors = append(vectors, []float64{1, 0, 0})
		case strings.Contains(text, "network") || strings.Contains(text, "gateway"):
			vectors = append(vectors, []float64{0, 1, 0})
		default:
			vectors = append(vectors, []float64{0, 0, 1})
		}
	}
	return vectors, nil
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func entryByIDForTest(entries []Entry, id string) Entry {
	for _, entry := range entries {
		if entry.ID == id {
			return entry
		}
	}
	return Entry{}
}

func clearEntriesForTest(service *Service) {
	for _, entry := range service.Timeline() {
		service.Delete(entry.ID)
	}
}

func waitChangeEventForTest(t *testing.T, events <-chan ChangeEvent, kind string, entryID string) ChangeEvent {
	t.Helper()
	deadline := time.After(700 * time.Millisecond)
	for {
		select {
		case event := <-events:
			if event.Kind == kind && (entryID == "" || event.EntryID == entryID) {
				return event
			}
		case <-deadline:
			t.Fatalf("timed out waiting for change event kind=%s entry=%s", kind, entryID)
		}
	}
}

func evidenceValueContains(result contracts.SearchResult, label string, text string) bool {
	for _, item := range result.Preview.Evidence {
		if item.Label == label && strings.Contains(item.Value, text) {
			return true
		}
	}
	return false
}

func reportHasInsight(report ExperienceReport, kind string, evidenceIDs ...string) bool {
	for _, insight := range report.Insights {
		if insight.Kind != kind {
			continue
		}
		found := map[string]bool{}
		for _, id := range insight.Evidence {
			found[id] = true
		}
		ok := true
		for _, id := range evidenceIDs {
			if !found[id] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func reportHasDecision(report ExperienceReport, insightID string, status string, taskPackageID string) bool {
	for _, insight := range report.Insights {
		if insight.ID != insightID {
			continue
		}
		return insight.DecisionStatus == status && insight.TaskPackageID == taskPackageID && insight.DecisionUpdatedAt > 0
	}
	return false
}

func containsExportedEntry(entries []Entry, id string) bool {
	for _, entry := range entries {
		if entry.ID == id {
			return true
		}
	}
	return false
}

func createTestZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	for name, body := range files {
		writer, err := archive.Create(name)
		if err != nil {
			archive.Close()
			file.Close()
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(body)); err != nil {
			archive.Close()
			file.Close()
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func readZipText(t *testing.T, archive *zip.ReadCloser, name string) string {
	t.Helper()
	for _, file := range archive.File {
		if file.Name != name {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()
		raw, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	t.Fatalf("zip file %s not found", name)
	return ""
}
