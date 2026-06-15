package workmemory

import (
	"archive/zip"
	"context"
	"encoding/json"
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

	"ariadne/internal/capturehistory"
	"ariadne/internal/clipboardhistory"
	"ariadne/internal/contracts"
)

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

	deadline = time.Now().Add(700 * time.Millisecond)
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

	draft := service.GenerateWorkflowDraft("剪贴板格式化自动化机会", []string{first.ID, second.ID})

	if draft.ID == "" || !draft.RequiresReview || draft.RiskLevel != "low" {
		t.Fatalf("unexpected workflow draft metadata: %#v", draft)
	}
	if draft.Trigger == "" || !strings.Contains(draft.Input, "剪贴板") || len(draft.Steps) < 3 {
		t.Fatalf("expected clipboard workflow draft, got %#v", draft)
	}
	if draft.Evidence[0] != first.ID || draft.Evidence[1] != second.ID {
		t.Fatalf("expected evidence preserved: %#v", draft.Evidence)
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
	service.ApplyCapturePolicy(CapturePolicy{AutoOCR: true})
	processorCalls := 0
	RegisterAutoOCRProcessor(service, func(entry Entry) Entry {
		processorCalls++
		return service.ApplyOCRText(entry.ID, "gateway timeout from automatic OCR", "test-auto-ocr")
	})

	entry := service.CaptureTimeMachineNow()
	status := service.Status()

	if processorCalls != 1 {
		t.Fatalf("expected one auto OCR call, got %d", processorCalls)
	}
	if entry.OCRText != "gateway timeout from automatic OCR" || entry.OCRStatus != "done:test-auto-ocr" || entry.ContentType != "ocr_text" {
		t.Fatalf("expected auto OCR writeback, got %#v", entry)
	}
	if status.LastAutoOCRID != entry.ID || status.LastAutoOCRAt == 0 || status.LastAutoOCRError != "" || !status.AutoOCREnabled {
		t.Fatalf("expected successful auto OCR status, got %#v", status)
	}
	if len(service.Search("automatic OCR")) == 0 {
		t.Fatal("auto OCR text should be searchable")
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
	service.ApplyCapturePolicy(CapturePolicy{AutoOCR: true})
	RegisterAutoOCRProcessor(service, func(entry Entry) Entry {
		return service.ApplyOCRText(entry.ID, "", "failed: OCR 不可用")
	})

	entry := service.CaptureTimeMachineNow()
	status := service.Status()

	if entry.OCRStatus != "failed: OCR 不可用" {
		t.Fatalf("expected failed OCR status, got %#v", entry)
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
	now := time.Unix(1781458200, 0)
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

	daily := service.GenerateDailyDraft()

	if daily.ID != "daily-"+now.Format("20060102") || len(daily.Evidence) != 2 {
		t.Fatalf("unexpected daily metadata: %#v", daily)
	}
	if containsString(daily.Evidence, "memory-sensitive") || strings.Contains(daily.Body, "token=secret") || strings.Contains(daily.Body, "password=secret") {
		t.Fatalf("daily draft should not include sensitive evidence or body: %#v", daily)
	}
	for _, expected := range []string{"## 今日概览", "## 主要工作", "## 待跟进", "## 复盘线索", "## 证据 ID", "memory-network-a", "memory-network-b", "已跳过敏感记忆 1 条"} {
		if !strings.Contains(daily.Body, expected) {
			t.Fatalf("daily body missing %q:\n%s", expected, daily.Body)
		}
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
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "work_memory_vectors.json")); err != nil {
		t.Fatalf("expected embedded vector cache file: %v", err)
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
	metadataPath := filepath.Join(filepath.Dir(path), "work_memory_vectors.json")
	rawMetadata, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("Milvus refresh should persist metadata without local vectors: %v", err)
	}
	if strings.Contains(string(rawMetadata), `"Vector":`) || strings.Contains(string(rawMetadata), `"vector":`) {
		t.Fatalf("Milvus metadata should not persist local vector values: %s", string(rawMetadata))
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

func clearEntriesForTest(service *Service) {
	for _, entry := range service.Timeline() {
		service.Delete(entry.ID)
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
