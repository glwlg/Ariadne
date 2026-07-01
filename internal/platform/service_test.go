package platform

import (
	"archive/zip"
	"ariadne/internal/contracts"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestStatusReportsHonestDesktopShellCapabilities(t *testing.T) {
	status := NewService().Status()

	if status.AppName != "Ariadne" || status.LegacyName != "x-tools" {
		t.Fatalf("unexpected app identity: %#v", status)
	}
	if !capability(status.Capabilities, "preview_actions").Enabled {
		t.Fatal("preview actions should be marked as implemented")
	}
	if !capability(status.Capabilities, "settings").Enabled {
		t.Fatal("settings should be marked as implemented")
	}
	if !capability(status.Capabilities, "app_scan").Enabled {
		t.Fatal("app scan should be marked complete after Start Menu shortcut provider integration")
	}
	if !capability(status.Capabilities, "custom_launchers").Enabled {
		t.Fatal("custom launchers should be marked complete after launcher provider integration")
	}
	if !capability(status.Capabilities, "search_ranking").Enabled {
		t.Fatal("search ranking should be marked complete after usage state integration")
	}
	if !capability(status.Capabilities, "json_compare").Enabled {
		t.Fatal("json compare should be marked complete after Go service and Vue center integration")
	}
	systemCommands := capability(status.Capabilities, "system_commands")
	if systemCommands.Enabled != (status.Diagnostics.OS == "windows") || !strings.Contains(systemCommands.Note, "二次确认") {
		t.Fatalf("system commands capability should be honest and guarded: %#v", systemCommands)
	}
	screenshotOverlay := capability(status.Capabilities, "screenshot_overlay")
	if strings.Contains(screenshotOverlay.Note, "待接入") || !strings.Contains(screenshotOverlay.Note, "标注") || !strings.Contains(screenshotOverlay.Note, "自动贴图") {
		t.Fatalf("screenshot overlay capability note should reflect migrated overlay tools and side effects: %#v", screenshotOverlay)
	}
	pinnedImage := capability(status.Capabilities, "pinned_image")
	if strings.Contains(pinnedImage.Note, "待接入") || !strings.Contains(pinnedImage.Note, "拖动") || !strings.Contains(pinnedImage.Note, "右键菜单") || !strings.Contains(pinnedImage.Note, "OCR") {
		t.Fatalf("pinned image capability note should reflect migrated pin window behavior: %#v", pinnedImage)
	}
	workMemory := capability(status.Capabilities, "work_memory")
	if strings.Contains(workMemory.Note, "embedding 待接入") || !strings.Contains(workMemory.Note, "Milvus") {
		t.Fatalf("work memory capability note should reflect embedding and Milvus support: %#v", workMemory)
	}
	networkMonitor := capability(status.Capabilities, "network_monitor")
	if networkMonitor.Enabled != (status.Diagnostics.OS == "windows") {
		t.Fatalf("network monitor capability should follow Windows runtime support: %#v", networkMonitor)
	}
	fileSearch := capability(status.Capabilities, "file_search")
	if fileSearch.Provider != "Ariadne USN/MFT" || !strings.Contains(fileSearch.Note, "文件索引") {
		t.Fatalf("file search capability should report Ariadne file index status: %#v", fileSearch)
	}
	if capability(status.Capabilities, "global_hotkey").Enabled {
		t.Fatal("global hotkey should not be marked complete before Alt+Q integration")
	}
	if capability(status.Capabilities, "tray").Enabled {
		t.Fatal("tray should not be marked complete before tray menu integration")
	}
	if capability(status.Capabilities, "single_instance").Enabled {
		t.Fatal("single instance should not be marked complete without shell runtime status")
	}
	if status.Diagnostics.ProcessID <= 0 {
		t.Fatalf("diagnostics should include process id: %#v", status.Diagnostics)
	}
	if len(status.Metrics) == 0 {
		t.Fatal("status should expose runtime metrics")
	}
}

func TestStatusUsesShellRuntimeStatus(t *testing.T) {
	service := NewService(WithShellStatus(func() ShellStatus {
		return ShellStatus{
			SingleInstanceConfigured:  true,
			TrayConfigured:            true,
			GlobalHotkeyRegistered:    true,
			GlobalHotkey:              "alt+q",
			AutostartSupported:        true,
			AutostartEnabled:          true,
			AutostartPath:             `HKCU\Software\Microsoft\Windows\CurrentVersion\Run\com.glwlg.ariadne`,
			AutostartCommandValid:     true,
			AutostartHiddenArgPresent: true,
		}
	}))

	status := service.Status()

	for _, id := range []string{"single_instance", "global_hotkey", "tray", "autostart"} {
		if !capability(status.Capabilities, id).Enabled {
			t.Fatalf("%s should follow shell runtime status: %#v", id, status.Capabilities)
		}
	}
	if !status.Shell.AutostartCommandValid || !strings.Contains(capability(status.Capabilities, "autostart").Note, "隐藏启动参数") {
		t.Fatalf("autostart diagnostics should expose hidden startup validation: shell=%#v note=%q", status.Shell, capability(status.Capabilities, "autostart").Note)
	}
}

func TestStatusReportsInvalidAutostartCommand(t *testing.T) {
	service := NewService(WithShellStatus(func() ShellStatus {
		return ShellStatus{
			AutostartSupported:        true,
			AutostartEnabled:          true,
			AutostartPath:             `HKCU\Software\Microsoft\Windows\CurrentVersion\Run\com.glwlg.ariadne`,
			AutostartCommand:          `C:\Tools\ariadne.exe`,
			AutostartHiddenArgPresent: false,
			AutostartCommandValid:     false,
			AutostartNotes:            []string{"开机启动命令缺少 --hidden，登录后可能直接弹出启动器"},
		}
	}))

	status := service.Status()
	note := capability(status.Capabilities, "autostart").Note
	if !strings.Contains(note, "需检查") || !strings.Contains(note, "--hidden") {
		t.Fatalf("autostart capability should surface invalid command details: %q", note)
	}
}

func TestStatusReportsLegacyRuntimeConflict(t *testing.T) {
	service := NewService(
		WithShellStatus(func() ShellStatus {
			return ShellStatus{
				GlobalHotkeyRegistered: false,
				GlobalHotkey:           "alt+q",
				LastError:              `register global hotkey "alt+q": hotkey is already registered (1409)`,
			}
		}),
		WithLegacyRuntime(func(shell ShellStatus, diagnostics RuntimeDiagnostics) LegacyRuntimeStatus {
			return LegacyRuntimeStatus{
				ProcessRunning:       true,
				ProcessID:            1234,
				ProcessName:          "x-tools.exe",
				ConfigPath:           filepath.Join(t.TempDir(), "x-tools", "config.json"),
				HotkeyConflictLikely: legacyHotkeyConflictLikely(shell, true),
				Notes:                []string{"旧版进程正在运行：x-tools.exe pid 1234"},
			}
		}),
	)

	status := service.Status()
	legacy := status.LegacyRuntime
	if !legacy.ProcessRunning || !legacy.HotkeyConflictLikely || legacy.ProcessName != "x-tools.exe" {
		t.Fatalf("legacy runtime conflict should be exposed: %#v", legacy)
	}
	coexistence := capability(status.Capabilities, "legacy_coexistence")
	if coexistence.Enabled {
		t.Fatalf("legacy coexistence should be degraded during hotkey conflict: %#v", coexistence)
	}
	if !strings.Contains(coexistence.Note, "Alt+Q") {
		t.Fatalf("legacy coexistence note should mention Alt+Q: %#v", coexistence)
	}
}

func TestResolveLegacyConflictRequiresConfirmation(t *testing.T) {
	called := false
	service := NewService(
		WithShellStatus(func() ShellStatus {
			return ShellStatus{GlobalHotkeyRegistered: false, LastError: "already registered (1409)"}
		}),
		WithLegacyRuntime(func(shell ShellStatus, diagnostics RuntimeDiagnostics) LegacyRuntimeStatus {
			return LegacyRuntimeStatus{
				ProcessRunning:       true,
				ProcessID:            1234,
				ProcessName:          "x-tools.exe",
				ConfigPath:           filepath.Join(t.TempDir(), "x-tools", "config.json"),
				HotkeyConflictLikely: legacyHotkeyConflictLikely(shell, true),
			}
		}),
		WithLegacyHandoff(func(request LegacyHandoffRequest, before LegacyRuntimeStatus) legacyHandoffOutcome {
			called = true
			return legacyHandoffOutcome{}
		}),
	)

	result := service.ResolveLegacyConflict(LegacyHandoffRequest{})
	if !result.RequiresConfirmation || result.OK || called {
		t.Fatalf("handoff should require explicit confirmation and avoid side effects: result=%#v called=%v", result, called)
	}
}

func TestResolveLegacyConflictClosesLegacyAndRetriesHotkey(t *testing.T) {
	legacyRunning := true
	service := NewService(
		WithShellStatus(func() ShellStatus {
			return ShellStatus{GlobalHotkeyRegistered: false, GlobalHotkey: "alt+q", LastError: "already registered (1409)"}
		}),
		WithLegacyRuntime(func(shell ShellStatus, diagnostics RuntimeDiagnostics) LegacyRuntimeStatus {
			return LegacyRuntimeStatus{
				ProcessRunning:       legacyRunning,
				ProcessID:            1234,
				ProcessName:          "x-tools.exe",
				ConfigPath:           filepath.Join(t.TempDir(), "x-tools", "config.json"),
				HotkeyConflictLikely: legacyHotkeyConflictLikely(shell, legacyRunning),
			}
		}),
		WithLegacyHandoff(func(request LegacyHandoffRequest, before LegacyRuntimeStatus) legacyHandoffOutcome {
			if !request.Confirm {
				t.Fatal("confirmed request expected")
			}
			legacyRunning = false
			return legacyHandoffOutcome{
				Actions:       []string{"旧版 x-tools 已退出"},
				ProcessExited: true,
			}
		}),
		WithHotkeyRetry(func() ShellStatus {
			return ShellStatus{GlobalHotkeyRegistered: true, GlobalHotkey: "alt+q"}
		}),
	)

	result := service.ResolveLegacyConflict(LegacyHandoffRequest{Confirm: true, TimeoutMs: 10})
	if !result.OK || result.After.ProcessRunning || !result.Shell.GlobalHotkeyRegistered || !result.HotkeyRetried {
		t.Fatalf("legacy handoff should close legacy and retry hotkey: %#v", result)
	}
	if !strings.Contains(strings.Join(result.Actions, "\n"), "旧版 x-tools 已退出") {
		t.Fatalf("handoff actions should include close evidence: %#v", result.Actions)
	}
}

func TestStatusIncludesSearchAndFileIndexDiagnostics(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "logs", "ariadne.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, []byte("ariadne log line\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewService(
		WithSearchPerformance(func() SearchPerformanceStatus {
			return SearchPerformanceStatus{
				SampleCount:     12,
				TargetP95Ms:     100,
				LastQuery:       "README.md",
				LastElapsedMs:   24,
				LastResultCount: 2,
				AverageMs:       18,
				P95Ms:           42,
				MaxMs:           55,
				WithinTarget:    true,
				LastUpdatedAt:   1770000000,
			}
		}),
		WithFileSearchStatus(func() FileSearchStatus {
			return FileSearchStatus{
				Ready:           true,
				Provider:        "Ariadne USN/MFT",
				IndexedCount:    1200,
				VolumeCount:     2,
				LastQuery:       "README.md",
				LastElapsedMs:   7,
				LastResultCount: 1,
				LastUpdatedAt:   1770000000,
				CoverageHint:    "Ariadne 文件索引已完成，但该文件或路径没有命中。",
			}
		}),
		WithFileSearchServiceStatus(func(status FileSearchStatus) FileSearchStatus {
			return status
		}),
		WithLogStatus(func() LogStatus {
			return logStatusForTest(logPath)
		}),
	)

	status := service.Status()

	if status.SearchPerformance.P95Ms != 42 || status.SearchPerformance.SampleCount != 12 {
		t.Fatalf("search performance should be surfaced: %#v", status.SearchPerformance)
	}
	if !status.FileSearch.Ready || status.FileSearch.LastResultCount != 1 || status.FileSearch.CoverageHint == "" {
		t.Fatalf("file index status should be surfaced: %#v", status.FileSearch)
	}
	if !capability(status.Capabilities, "search_performance").Enabled {
		t.Fatalf("search performance capability should be implemented: %#v", status.Capabilities)
	}
	if !capability(status.Capabilities, "file_search").Enabled {
		t.Fatalf("file search should follow ready status: %#v", status.Capabilities)
	}
	if !strings.Contains(capability(status.Capabilities, "file_search").Note, "文件索引") {
		t.Fatalf("file search note should expose coverage hint: %#v", capability(status.Capabilities, "file_search"))
	}
	if !status.Logs.Exists || status.Logs.Bytes == 0 {
		t.Fatalf("log status should be surfaced: %#v", status.Logs)
	}
	if !capability(status.Capabilities, "diagnostic_logs").Enabled {
		t.Fatalf("diagnostic logs capability should be implemented: %#v", status.Capabilities)
	}
	if !metricValue(status.Metrics, "search_p95", 42) || !metricValue(status.Metrics, "file_index_last", 7) || !metricValue(status.Metrics, "log_file_size", status.Logs.Bytes) {
		t.Fatalf("metrics should include search and file index timings: %#v", status.Metrics)
	}
}

func TestExportDiagnosticsIncludesPlatformStatusAndLog(t *testing.T) {
	root := t.TempDir()
	appData := filepath.Join(root, "AppData")
	oldAppData := os.Getenv("APPDATA")
	if err := os.Setenv("APPDATA", appData); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Setenv("APPDATA", oldAppData) })
	logPath := filepath.Join(root, "logs", "ariadne.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, []byte("diagnostic log\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewService(WithLogStatus(func() LogStatus {
		return logStatusForTest(logPath)
	}))

	result := service.ExportDiagnostics()
	if !result.OK || result.Path == "" || result.Bytes == 0 || !result.LogIncluded {
		t.Fatalf("unexpected diagnostics export: %#v", result)
	}
	reader, err := zip.OpenReader(result.Path)
	if err != nil {
		t.Fatalf("open diagnostics zip: %v", err)
	}
	defer reader.Close()
	names := map[string]bool{}
	for _, file := range reader.File {
		names[file.Name] = true
	}
	for _, name := range []string{"README.md", "diagnostics/platform_status.json", "diagnostics/metrics.json", "logs/ariadne.log"} {
		if !names[name] {
			t.Fatalf("diagnostics zip missing %s, entries=%#v", name, names)
		}
	}
}

func TestStatusDegradesFileSearchOnIndexError(t *testing.T) {
	service := NewService(
		WithFileSearchStatus(func() FileSearchStatus {
			return FileSearchStatus{
				Ready:     false,
				Provider:  "Ariadne USN/MFT",
				LastError: "line index parse failed",
			}
		}),
		WithFileSearchServiceStatus(func(status FileSearchStatus) FileSearchStatus {
			return status
		}),
	)

	status := service.Status()
	fileSearch := capability(status.Capabilities, "file_search")
	if fileSearch.Enabled {
		t.Fatalf("file search should be degraded when recent file index status is not ready: %#v", fileSearch)
	}
	if !strings.Contains(fileSearch.Note, "最近查询失败") {
		t.Fatalf("file search note should expose the recent error: %#v", fileSearch)
	}
}

func TestStatusExplainsMissingSearchService(t *testing.T) {
	service := NewService(
		WithFileSearchStatus(func() FileSearchStatus {
			return FileSearchStatus{
				Ready:         false,
				Provider:      "Ariadne USN/MFT",
				RequiresAdmin: true,
				Elevated:      false,
				LastError:     "搜索服务未安装",
				CoverageHint:  "搜索服务未安装。安装后会自动维护本机文件索引。",
			}
		}),
		WithFileSearchServiceStatus(func(status FileSearchStatus) FileSearchStatus {
			return status
		}),
	)

	status := service.Status()
	fileSearch := capability(status.Capabilities, "file_search")
	if fileSearch.Enabled {
		t.Fatalf("file search should be degraded until Ariadne is elevated: %#v", fileSearch)
	}
	if !strings.Contains(fileSearch.Note, "搜索服务未安装") {
		t.Fatalf("file search note should make the missing search service visible: %#v", fileSearch)
	}
}

func TestNormalizeFileSearchStatusClearsStaleServiceNotice(t *testing.T) {
	status := normalizeFileSearchStatus(FileSearchStatus{
		Ready:          true,
		ServiceRunning: true,
		CoverageHint:   "搜索服务未运行；当前索引可搜索，后台刷新需安装搜索服务。",
	})

	if status.CoverageHint != "" {
		t.Fatalf("running service should clear stale service notice: %#v", status)
	}
}

func TestLegacyHotkeyConflictHeuristics(t *testing.T) {
	if !legacyHotkeyConflictLikely(ShellStatus{LastError: "register global hotkey: already registered"}, false) {
		t.Fatal("already registered hotkey errors should be treated as conflicts")
	}
	if !legacyHotkeyConflictLikely(ShellStatus{LastError: "register global hotkey: access denied"}, true) {
		t.Fatal("legacy process plus hotkey registration failure should be treated as a likely conflict")
	}
	if legacyHotkeyConflictLikely(ShellStatus{GlobalHotkeyRegistered: true, LastError: "old error"}, true) {
		t.Fatal("registered Ariadne hotkey should override stale errors")
	}
}

func TestExecuteOpenActionRequiresPath(t *testing.T) {
	result := NewService().ExecuteAction(contracts.PreviewAction{
		ID:    "open_app",
		Label: "打开应用",
		Kind:  contracts.ActionOpen,
	})

	if result.OK {
		t.Fatalf("open action without path should fail: %#v", result)
	}
}

func TestActionSuccessUsesInlineFeedback(t *testing.T) {
	message := actionSuccess(contracts.PreviewAction{
		Feedback: &contracts.ActionFeedback{SuccessLabel: "已启动"},
	}, "已打开")

	if message != "已启动" {
		t.Fatalf("expected inline feedback message, got %q", message)
	}
}

func TestRememberActionUsesConfiguredHandler(t *testing.T) {
	called := false
	service := NewService(WithRememberActionHandler(func(action contracts.PreviewAction) contracts.ActionResult {
		called = true
		if action.Payload["targetId"] != "clipboard-1" {
			t.Fatalf("unexpected target payload: %#v", action.Payload)
		}
		return contracts.ActionResult{OK: true, Message: "已加入工作记忆"}
	}))

	result := service.ExecuteAction(contracts.PreviewAction{
		ID:    "remember_clipboard",
		Label: "加入记忆",
		Kind:  contracts.ActionRemember,
		Payload: map[string]interface{}{
			"targetId": "clipboard-1",
		},
	})

	if !called || !result.OK || result.Message != "已加入工作记忆" {
		t.Fatalf("remember action should use handler, called=%v result=%#v", called, result)
	}
}

func TestInstallFileSearchServiceUsesElevatedRunner(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only service install action")
	}
	commandRunnerCalled := false
	var elevatedFile string
	var elevatedArgs []string
	service := NewService(
		WithCommandRunner(func(request commandRunRequest) error {
			commandRunnerCalled = true
			return nil
		}),
		WithElevatedRunner(func(file string, args []string) error {
			elevatedFile = file
			elevatedArgs = append([]string(nil), args...)
			return nil
		}),
	)

	result := service.InstallFileSearchService()

	if !result.OK || commandRunnerCalled {
		t.Fatalf("search service install should use native elevated runner only: result=%#v commandRunnerCalled=%v", result, commandRunnerCalled)
	}
	if !strings.HasSuffix(strings.ToLower(elevatedFile), ".exe") || !reflect.DeepEqual(elevatedArgs, []string{"filesearch-service-install"}) {
		t.Fatalf("unexpected elevated request: file=%q args=%#v", elevatedFile, elevatedArgs)
	}
}

func TestDangerActionRequiresConfirmation(t *testing.T) {
	called := false
	result := NewService(WithCommandRunner(func(request commandRunRequest) error {
		called = true
		return nil
	})).ExecuteAction(contracts.PreviewAction{
		ID:    "run_launcher",
		Label: "确认运行",
		Kind:  contracts.ActionDanger,
		Payload: map[string]interface{}{
			"command":              "ipconfig",
			"requiresConfirmation": true,
		},
	})

	if result.OK || !result.RequiresConfirmation || called || !strings.Contains(result.Message, "再次点击确认") {
		t.Fatalf("danger action should not silently execute: %#v", result)
	}
}

func TestDangerActionConfirmationUsesProductLabel(t *testing.T) {
	called := false
	result := NewService(WithCommandRunner(func(request commandRunRequest) error {
		called = true
		return nil
	})).ExecuteAction(contracts.PreviewAction{
		ID:    "restart_elevated",
		Label: "以管理员身份启动",
		Kind:  contracts.ActionDanger,
		Payload: map[string]interface{}{
			"command":              "powershell.exe",
			"arguments":            []string{"-NoProfile", "-Command", "Start-Process -Verb RunAs"},
			"requiresConfirmation": true,
			"confirmationLabel":    "以管理员身份启动 Ariadne",
		},
	})

	if result.OK || !result.RequiresConfirmation || called {
		t.Fatalf("danger action should require confirmation: %#v", result)
	}
	if !strings.Contains(result.Message, "以管理员身份启动 Ariadne") || strings.Contains(result.Message, "powershell.exe") || strings.Contains(result.Message, "Start-Process") {
		t.Fatalf("confirmation should hide implementation command: %#v", result)
	}
}

func TestConfirmedDangerActionRunsConfiguredCommand(t *testing.T) {
	var captured commandRunRequest
	service := NewService(WithCommandRunner(func(request commandRunRequest) error {
		captured = request
		return nil
	}))
	dir := t.TempDir()

	result := service.ExecuteAction(contracts.PreviewAction{
		ID:    "run_launcher",
		Label: "确认运行",
		Kind:  contracts.ActionDanger,
		Payload: map[string]interface{}{
			"command":              "cmd.exe",
			"arguments":            `/c "echo hello"`,
			"workingDir":           dir,
			"requiresConfirmation": true,
			"confirmed":            true,
		},
		Feedback: &contracts.ActionFeedback{SuccessLabel: "已启动"},
	})

	if !result.OK || result.RequiresConfirmation || result.Message != "已启动" {
		t.Fatalf("confirmed danger action should run once: %#v", result)
	}
	if captured.Command != "cmd.exe" || captured.WorkingDir != dir || !reflect.DeepEqual(captured.Arguments, []string{"/c", "echo hello"}) {
		t.Fatalf("unexpected command request: %#v", captured)
	}
}

func TestConfirmedDangerActionCanWaitAndQuitAfterStart(t *testing.T) {
	var captured commandRunRequest
	quit := make(chan struct{}, 1)
	service := NewService(
		WithCommandRunner(func(request commandRunRequest) error {
			captured = request
			return nil
		}),
		WithApplicationQuit(func() {
			quit <- struct{}{}
		}),
	)

	result := service.ExecuteAction(contracts.PreviewAction{
		ID:    "restart_elevated",
		Label: "以管理员身份启动",
		Kind:  contracts.ActionDanger,
		Payload: map[string]interface{}{
			"command":              "powershell.exe",
			"arguments":            []string{"-NoProfile", "-Command", "Start-Process -Verb RunAs"},
			"requiresConfirmation": true,
			"confirmed":            true,
			"waitForExit":          true,
			"quitAfterStart":       true,
		},
		Feedback: &contracts.ActionFeedback{SuccessLabel: "正在切换到管理员实例"},
	})

	if !result.OK || captured.Command != "powershell.exe" || !captured.Wait {
		t.Fatalf("confirmed elevated restart should wait for command success: result=%#v request=%#v", result, captured)
	}
	select {
	case <-quit:
	case <-time.After(time.Second):
		t.Fatal("confirmed elevated restart should request old instance quit")
	}
}

func TestSystemCommandRequiresConfirmation(t *testing.T) {
	called := false
	result := NewService(WithCommandRunner(func(request commandRunRequest) error {
		called = true
		return nil
	})).ExecuteAction(contracts.PreviewAction{
		ID:    "run_system",
		Label: "执行",
		Kind:  contracts.ActionRun,
		Payload: map[string]interface{}{
			"command":              "lock",
			"requiresConfirmation": true,
		},
	})

	if result.OK || !result.RequiresConfirmation || called || !strings.Contains(result.Message, "锁定工作站") {
		t.Fatalf("system command should require confirmation before execution: %#v", result)
	}
}

func TestConfirmedSystemCommandUsesControlledMapping(t *testing.T) {
	var captured commandRunRequest
	service := NewService(WithCommandRunner(func(request commandRunRequest) error {
		captured = request
		return nil
	}))

	result := service.ExecuteAction(contracts.PreviewAction{
		ID:    "run_system",
		Label: "执行",
		Kind:  contracts.ActionRun,
		Payload: map[string]interface{}{
			"command":              "empty",
			"requiresConfirmation": true,
			"confirmed":            true,
		},
		Feedback: &contracts.ActionFeedback{SuccessLabel: "已请求清空回收站"},
	})

	if !result.OK || result.RequiresConfirmation || result.Message != "已请求清空回收站" {
		t.Fatalf("confirmed system command should execute once with inline feedback: %#v", result)
	}
	expectedArgs := []string{"-NoProfile", "-Command", "Clear-RecycleBin -Force -ErrorAction Stop"}
	if captured.Command != "powershell.exe" || !reflect.DeepEqual(captured.Arguments, expectedArgs) {
		t.Fatalf("unexpected system command mapping: %#v", captured)
	}
}

func TestSystemCommandRejectsUnknownCommand(t *testing.T) {
	called := false
	result := NewService(WithCommandRunner(func(request commandRunRequest) error {
		called = true
		return nil
	})).ExecuteAction(contracts.PreviewAction{
		ID:    "run_system",
		Label: "执行",
		Kind:  contracts.ActionRun,
		Payload: map[string]interface{}{
			"command":   "format",
			"confirmed": true,
		},
	})

	if result.OK || called || !strings.Contains(result.Message, "不支持的系统命令") {
		t.Fatalf("unknown system command should be rejected: %#v", result)
	}
}

func TestCommandActionReportsInvalidWorkingDirectory(t *testing.T) {
	missingDir := filepath.Join(t.TempDir(), "missing")
	result := NewService().ExecuteAction(contracts.PreviewAction{
		ID:    "run_launcher",
		Label: "确认运行",
		Kind:  contracts.ActionDanger,
		Payload: map[string]interface{}{
			"command":              "cmd.exe",
			"workingDir":           missingDir,
			"requiresConfirmation": true,
			"confirmed":            true,
		},
	})

	if result.OK || !strings.Contains(result.Message, "工作目录不可用") || !strings.Contains(result.Message, missingDir) {
		t.Fatalf("invalid working directory should be actionable: %#v", result)
	}
}

func TestCommandActionReportsRunnerFailureWithContext(t *testing.T) {
	dir := t.TempDir()
	result := NewService(WithCommandRunner(func(request commandRunRequest) error {
		return errors.New("executable file not found")
	})).ExecuteAction(contracts.PreviewAction{
		ID:    "run_launcher",
		Label: "确认运行",
		Kind:  contracts.ActionDanger,
		Payload: map[string]interface{}{
			"command":              "missing-tool.exe",
			"arguments":            "--version",
			"workingDir":           dir,
			"requiresConfirmation": true,
			"confirmed":            true,
		},
	})

	if result.OK || !strings.Contains(result.Message, "missing-tool.exe --version") || !strings.Contains(result.Message, dir) || !strings.Contains(result.Message, "executable file not found") {
		t.Fatalf("runner failure should include command context: %#v", result)
	}
}

func TestSplitCommandArgumentsHandlesQuotedValues(t *testing.T) {
	args, err := splitCommandArguments(`/c "echo hello" --name 'Ariadne Launcher'`)
	if err != nil {
		t.Fatalf("split should succeed: %v", err)
	}
	expected := []string{"/c", "echo hello", "--name", "Ariadne Launcher"}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("expected %#v, got %#v", expected, args)
	}
}

func TestSplitCommandArgumentsPreservesWindowsPaths(t *testing.T) {
	args, err := splitCommandArguments(`--input "C:\Program Files\Ariadne\config.json" C:\Temp\out.txt`)
	if err != nil {
		t.Fatalf("split should succeed: %v", err)
	}
	expected := []string{"--input", `C:\Program Files\Ariadne\config.json`, `C:\Temp\out.txt`}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("expected %#v, got %#v", expected, args)
	}
}

func capability(items []Capability, id string) Capability {
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	return Capability{}
}

func metricValue(metrics []RuntimeMetric, id string, value int64) bool {
	for _, metric := range metrics {
		if metric.ID == id {
			return metric.Value == value
		}
	}
	return false
}

func logStatusForTest(path string) LogStatus {
	status := LogStatus{Path: path, Directory: filepath.Dir(path)}
	if info, err := os.Stat(status.Directory); err == nil && info.IsDir() {
		status.DirectoryExists = true
	}
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		status.Exists = true
		status.Bytes = info.Size()
		status.LastModifiedAt = info.ModTime().Unix()
	}
	return status
}
