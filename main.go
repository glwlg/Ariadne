package main

import (
	"embed"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"ariadne/internal/aiclient"
	"ariadne/internal/applog"
	"ariadne/internal/apps"
	"ariadne/internal/capturehistory"
	"ariadne/internal/captureoverlay"
	"ariadne/internal/checklists"
	"ariadne/internal/clipboardhistory"
	"ariadne/internal/contracts"
	"ariadne/internal/filesearch"
	"ariadne/internal/hosts"
	"ariadne/internal/imageindex"
	"ariadne/internal/jsoncompare"
	"ariadne/internal/launchers"
	"ariadne/internal/migration"
	"ariadne/internal/networkmonitor"
	"ariadne/internal/ocr"
	"ariadne/internal/pinnedimage"
	"ariadne/internal/platform"
	"ariadne/internal/plugins"
	"ariadne/internal/qrscan"
	"ariadne/internal/release"
	"ariadne/internal/search"
	"ariadne/internal/secrets"
	"ariadne/internal/settings"
	"ariadne/internal/shell"
	"ariadne/internal/skills"
	"ariadne/internal/toolwindows"
	"ariadne/internal/workflows"
	"ariadne/internal/workmemory"
	"ariadne/internal/workmemorycli"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed assets/logo.png
var appIcon []byte

//go:embed assets/logo.png
var trayIcon []byte

func main() {
	if len(os.Args) > 1 && strings.EqualFold(os.Args[1], "workmemory") {
		os.Exit(workmemorycli.Run(os.Args[2:], os.Stdout, os.Stderr))
	}

	logService := applog.NewService()
	if err := logService.Start(); err != nil {
		log.Printf("start app log: %v", err)
	} else {
		log.SetOutput(io.MultiWriter(os.Stderr, logService))
		log.Println("Ariadne starting")
	}
	appService := apps.NewService()
	fileSearchService := filesearch.NewService()
	launcherService := launchers.NewService()
	captureService := capturehistory.NewService()
	clipboardService := clipboardhistory.NewService(captureService)
	pinnedImageService := pinnedimage.NewService(captureService, clipboardService)
	captureOverlayService := captureoverlay.NewService(captureService, pinnedImageService)
	hostsService := hosts.NewService()
	jsonCompareService := jsoncompare.NewService()
	networkMonitorService := networkmonitor.NewService()
	qrScanService := qrscan.NewService(captureService)
	pluginService := plugins.NewService()
	workflowService := workflows.NewService(pluginService)
	checklistService := checklists.NewService()
	skillService := skills.NewService()
	secretsService := secrets.NewService()
	settingsService := settings.NewService()
	pluginService.ApplyEnabled(settingsService.GetSettings().Plugins.Enabled)
	workMemoryService := workmemory.NewService(captureService)
	memorySinkJobs := make(chan func(), 256)
	go func() {
		for job := range memorySinkJobs {
			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						log.Printf("work memory sink panic: %v", recovered)
					}
				}()
				job()
			}()
		}
	}()
	enqueueMemorySink := func(label string, job func()) {
		select {
		case memorySinkJobs <- job:
		default:
			log.Printf("work memory sink queue full, dropping %s", label)
		}
	}
	clipboardhistory.RegisterEntryObserver(clipboardService, func(entry clipboardhistory.Entry) {
		config := settingsService.GetSettings().WorkMemory
		if !config.Enabled || config.PrivacyMode || !config.SourceClipboard {
			return
		}
		captured := entry
		enqueueMemorySink("clipboard entry", func() {
			workMemoryService.RememberClipboardEntry(captured)
		})
	})
	capturehistory.RegisterEntryObserver(captureService, func(entry capturehistory.Entry) {
		config := settingsService.GetSettings().WorkMemory
		if !config.Enabled || config.PrivacyMode || !config.SourceCaptureHistory {
			return
		}
		captured := entry
		enqueueMemorySink("capture history entry", func() {
			workMemoryService.RememberCaptureHistoryEntry(captured)
		})
	})
	migrationService := migration.NewService(clipboardService, captureService, workMemoryService)
	releaseService := release.NewService()
	ocrService := ocr.NewService(captureService, clipboardService, workMemoryService)
	ocr.RegisterAIClient(ocrService, aiclient.NewOpenAICompatibleImageOCR())
	workmemory.RegisterAutoOCRProcessor(workMemoryService, func(entry workmemory.Entry) workmemory.Entry {
		result := ocrService.RecognizeWorkMemory(entry.ID)
		if result.WorkMemory == nil {
			return entry
		}
		return *result.WorkMemory
	})
	workmemory.RegisterOCRSummarizer(workMemoryService, aiclient.NewOpenAICompatibleOCRSummarizer())
	workmemory.RegisterDraftPolisher(workMemoryService, aiclient.NewOpenAICompatiblePolisher())
	workmemory.RegisterFlowAgentRunner(workMemoryService, aiclient.NewFlowAgentRouter())
	workmemory.RegisterExperienceDiscoverer(workMemoryService, aiclient.NewOpenAICompatibleExperienceDiscoverer())
	workmemory.RegisterEmbeddingClient(workMemoryService, aiclient.NewOpenAICompatibleEmbedder())
	imageIndexService := imageindex.NewService(captureService, clipboardService, ocrService)
	searchService := search.NewService(fileSearchService, appService, launcherService, clipboardService, captureService, imageIndexService, workflowService, pluginService, workMemoryService)
	toolWindowService := toolwindows.NewService()
	toolWindowService.SetWindowIcon(appIcon)
	initialHotkeys := settingsService.GetSettings().Hotkeys
	shellManager := shell.NewManager(
		initialHotkeys.ToggleWindow,
		initialHotkeys.Screenshot,
		initialHotkeys.PinClipboard,
		toolWindowService.OpenFromShell,
		func() bool {
			result := captureOverlayService.Open()
			if !result.OK {
				log.Printf("open screenshot overlay: %s", result.Message)
			}
			return result.OK
		},
		func() bool {
			result := pinnedImageService.OpenCurrentClipboard()
			if !result.OK {
				log.Printf("pin clipboard: %s", result.Message)
			}
			return result.OK
		},
	)
	platformService := platform.NewService(
		platform.WithShellStatus(func() platform.ShellStatus {
			return platformShellStatus(shellManager.Status())
		}),
		platform.WithHotkeyRetry(func() platform.ShellStatus {
			return platformShellStatus(shellManager.RetryHotkeyRegistration())
		}),
		platform.WithRememberActionHandler(func(action contracts.PreviewAction) contracts.ActionResult {
			return rememberActionResult(action, clipboardService, captureService, workMemoryService)
		}),
		platform.WithSearchPerformance(func() platform.SearchPerformanceStatus {
			status := searchService.PerformanceStatus()
			return platform.SearchPerformanceStatus{
				SampleCount:     status.SampleCount,
				TargetP95Ms:     status.TargetP95Ms,
				LastQuery:       status.LastQuery,
				LastElapsedMs:   status.LastElapsedMs,
				LastResultCount: status.LastResultCount,
				AverageMs:       status.AverageMs,
				P95Ms:           status.P95Ms,
				MaxMs:           status.MaxMs,
				WithinTarget:    status.WithinTarget,
				LastUpdatedAt:   status.LastUpdatedAt,
			}
		}),
		platform.WithFileSearchStatus(func() platform.FileSearchStatus {
			status := fileSearchService.Status()
			return platform.FileSearchStatus{
				DLLPath:         status.DLLPath,
				DLLFound:        status.DLLFound,
				Ready:           status.Ready,
				LastError:       status.LastError,
				LastQuery:       status.LastQuery,
				LastElapsedMs:   status.LastElapsedMs,
				LastResultCount: status.LastResultCount,
				LastUpdatedAt:   status.LastUpdatedAt,
				CoverageHint:    status.CoverageHint,
			}
		}),
		platform.WithLogStatus(func() platform.LogStatus {
			return appLogStatus(logService.Status())
		}),
	)

	app := application.New(application.Options{
		Name:        "Ariadne",
		Description: "Ariadne command launcher and work memory center",
		Icon:        appIcon,
		Windows: application.WindowsOptions{
			DisableQuitOnLastWindowClosed: true,
		},
		SingleInstance: shellManager.SingleInstanceOptions(),
		OnShutdown: func() {
			log.Println("Ariadne shutting down")
			if err := shellManager.Stop(); err != nil {
				log.Printf("stop shell: %v", err)
			}
			toolWindowService.Stop()
			clipboardService.StopWatcher()
			workMemoryService.Stop()
			if err := logService.Stop(); err != nil {
				log.Printf("stop app log: %v", err)
			}
		},
		Services: []application.Service{
			application.NewService(searchService),
			application.NewService(pluginService),
			application.NewService(launcherService),
			application.NewService(clipboardService),
			application.NewService(captureService),
			application.NewService(captureOverlayService),
			application.NewService(pinnedImageService),
			application.NewService(hostsService),
			application.NewService(jsonCompareService),
			application.NewService(networkMonitorService),
			application.NewService(qrScanService),
			application.NewService(ocrService),
			application.NewService(imageIndexService),
			application.NewService(workflowService),
			application.NewService(checklistService),
			application.NewService(skillService),
			application.NewService(secretsService),
			application.NewService(migrationService),
			application.NewService(releaseService),
			application.NewService(toolWindowService),
			application.NewService(settingsService),
			application.NewService(workMemoryService),
			application.NewService(platformService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})
	workmemory.RegisterChangeObserver(workMemoryService, func(event workmemory.ChangeEvent) {
		app.Event.Emit("ariadne:work-memory-changed", event)
	})
	captureOverlayService.Attach(app)
	pinnedImageService.Attach(app)
	toolWindowService.Attach(app)

	settings.RegisterChangeHandler(settingsService, func(next settings.AppSettings) {
		shellManager.ApplyHotkeys(next.Hotkeys.ToggleWindow, next.Hotkeys.Screenshot, next.Hotkeys.PinClipboard)
		if err := shellManager.ApplyAutostart(next.General.RunOnStartup); err != nil {
			log.Printf("apply autostart: %v", err)
		}
		applyCaptureOverlayRuntime(captureOverlayService, next.Screenshot)
		applyOCRAIRuntime(ocrService, next.AI)
		applyWorkMemoryRuntime(workMemoryService, next.WorkMemory)
		applyWorkMemoryAIRuntime(workMemoryService, next.AI, next.WorkMemory)
		applyRetentionPolicies(workMemoryService, captureService, clipboardService, imageIndexService, next.WorkMemory)
		clipboardService.ApplyWatcherSettings(next.WorkMemory.PrivacyMode, next.WorkMemory.SourceClipboard)
		pluginService.ApplyEnabled(next.Plugins.Enabled)
	})
	initialSettings := settingsService.GetSettings()
	applyCaptureOverlayRuntime(captureOverlayService, initialSettings.Screenshot)
	applyOCRAIRuntime(ocrService, initialSettings.AI)
	applyWorkMemoryAIRuntime(workMemoryService, initialSettings.AI, initialSettings.WorkMemory)

	mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "Ariadne - 心流",
		Width:            1280,
		Height:           820,
		MinWidth:         1040,
		MinHeight:        640,
		AlwaysOnTop:      false,
		Frameless:        false,
		DisableResize:    false,
		BackgroundColour: application.NewRGB(244, 244, 245),
		InitialPosition:  application.WindowCentered,
		Hidden:           shouldStartHidden(),
		Windows: application.WindowsWindow{
			Theme:                             application.Light,
			DisableIcon:                       false,
			DisableFramelessWindowDecorations: false,
			HiddenOnTaskbar:                   false,
		},
	})
	toolWindowService.ApplyMainWindowPolicy()
	shellManager.Attach(app, mainWindow, trayIcon)
	if err := shellManager.ApplyAutostart(settingsService.GetSettings().General.RunOnStartup); err != nil {
		log.Printf("apply autostart: %v", err)
	}
	deferStartupMaintenance(func() {
		toolWindowService.ApplyMainWindowPolicy()
		latest := settingsService.GetSettings()
		applyOCRAIRuntime(ocrService, latest.AI)
		applyWorkMemoryRuntime(workMemoryService, latest.WorkMemory)
		applyWorkMemoryAIRuntime(workMemoryService, latest.AI, latest.WorkMemory)
		applyRetentionPolicies(workMemoryService, captureService, clipboardService, imageIndexService, latest.WorkMemory)
		clipboardService.ApplyWatcherSettings(latest.WorkMemory.PrivacyMode, latest.WorkMemory.SourceClipboard)
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func platformShellStatus(status shell.Status) platform.ShellStatus {
	return platform.ShellStatus{
		SingleInstanceConfigured:     status.SingleInstanceConfigured,
		TrayConfigured:               status.TrayConfigured,
		GlobalHotkeyRegistered:       status.GlobalHotkeyRegistered,
		GlobalHotkey:                 status.GlobalHotkey,
		ScreenshotHotkeyRegistered:   status.ScreenshotHotkeyRegistered,
		ScreenshotHotkey:             status.ScreenshotHotkey,
		PinClipboardHotkeyRegistered: status.PinClipboardHotkeyRegistered,
		PinClipboardHotkey:           status.PinClipboardHotkey,
		AutostartSupported:           status.AutostartSupported,
		AutostartEnabled:             status.AutostartEnabled,
		AutostartPath:                status.AutostartPath,
		AutostartIdentifier:          status.AutostartIdentifier,
		AutostartValueName:           status.AutostartValueName,
		AutostartCommand:             status.AutostartCommand,
		AutostartCommandValid:        status.AutostartCommandValid,
		AutostartHiddenArgPresent:    status.AutostartHiddenArgPresent,
		AutostartNotes:               status.AutostartNotes,
		LastError:                    status.LastError,
	}
}

func appLogStatus(status applog.Status) platform.LogStatus {
	return platform.LogStatus{
		Path:            status.Path,
		Directory:       status.Directory,
		DirectoryExists: status.DirectoryExists,
		Exists:          status.Exists,
		Bytes:           status.Bytes,
		LastModifiedAt:  status.LastModifiedAt,
		LastError:       status.LastError,
	}
}

func shouldStartHidden() bool {
	for _, arg := range os.Args[1:] {
		if arg == "--hidden" || arg == "/hidden" {
			return true
		}
	}
	return false
}

func deferStartupMaintenance(fn func()) {
	go func() {
		time.Sleep(1500 * time.Millisecond)
		fn()
	}()
}

func applyRetentionPolicies(
	workMemoryService *workmemory.Service,
	captureService *capturehistory.Service,
	clipboardService *clipboardhistory.Service,
	imageIndexService *imageindex.Service,
	config settings.WorkMemorySettings,
) {
	workMemoryService.ApplyRetentionPolicy(config.RetentionDays, config.KeepFavoritesForever)
	captureService.ApplyRetentionPolicy(config.RetentionDays, config.KeepFavoritesForever)
	captureService.ApplyStoragePolicy(config.MaxStorageMB, config.KeepFavoritesForever)
	clipboardService.ApplyRetentionPolicy(config.RetentionDays, config.KeepFavoritesForever)
	imageIndexService.ApplyRetentionPolicy(config.RetentionDays)
}

func applyCaptureOverlayRuntime(service *captureoverlay.Service, config settings.ScreenshotSettings) {
	service.ApplyScreenshotPolicy(captureoverlay.ScreenshotPolicy{
		AutoCopy:         config.AutoCopy,
		AutoPin:          config.AutoPin,
		AutoSave:         config.AutoSave,
		SaveDir:          config.SaveDir,
		FilenameTemplate: config.FilenameTemplate,
	})
}

func applyOCRAIRuntime(service *ocr.Service, config settings.AISettings) {
	provider := firstNonEmpty(config.OCRProvider, "openai-compatible")
	service.ApplyAIOCRPolicy(ocr.AIOCRPolicy{
		Enabled:  config.OCRModelEnabled,
		Provider: provider,
		BaseURL:  ocrAIBaseURL(provider, config),
		Model:    firstNonEmpty(config.OCRModel, os.Getenv("ARIADNE_OCR_MODEL")),
	})
}

func ocrAIBaseURL(provider string, config settings.AISettings) string {
	if isOllamaOCRProvider(provider) {
		return firstNonEmpty(config.OCRBaseURL, os.Getenv("ARIADNE_OCR_BASE_URL"))
	}
	return firstNonEmpty(config.OCRBaseURL, os.Getenv("ARIADNE_OCR_BASE_URL"), config.BaseURL, os.Getenv("OPENAI__BASE_URL"))
}

func isOllamaOCRProvider(provider string) bool {
	switch strings.TrimSpace(strings.ToLower(provider)) {
	case "ollama", "ollama-generate", "ollama_generate":
		return true
	default:
		return false
	}
}

func applyWorkMemoryRuntime(service *workmemory.Service, config settings.WorkMemorySettings) {
	service.ApplyCapturePolicy(workmemory.CapturePolicy{
		ExcludeApps:              config.ExcludeApps,
		ExcludeWindowKeywords:    config.ExcludeWindowKeywords,
		ExcludePaths:             config.ExcludePaths,
		ExcludeURLs:              config.ExcludeURLs,
		ExcludeContentPatterns:   config.ExcludeContentPatterns,
		SensitiveRulesEnabled:    config.SensitiveRulesEnabled,
		SensitiveRulesConfigured: true,
		AppCaptureProfiles:       workMemoryAppProfiles(config.AppCaptureProfiles),
		AutoOCR:                  config.AutoOCR,
		CaptureScope:             config.CaptureScope,
		MultiMonitor:             config.MultiMonitor,
		CaptureOnWindowChange:    config.WindowSwitchCaptureEnabled,
		WindowChangeCooldown:     config.WindowSwitchCooldownSecs,
		PauseOnIdle:              config.PauseOnIdle,
		IdlePauseSeconds:         config.IdlePauseSeconds,
		PauseOnLock:              config.PauseOnLock,
	})
	service.ApplySettings(
		config.Enabled,
		config.PrivacyMode,
		config.TimeMachineEnabled,
		config.AutoCaptureIntervalSeconds,
	)
	service.ApplyDraftSchedule(workmemory.DraftSchedulePolicy{
		Enabled:                 config.DraftScheduleEnabled,
		IntervalMinutes:         config.DraftScheduleIntervalMin,
		DailyDraftEnabled:       config.DailyDraftScheduleEnabled,
		RetrospectiveEnabled:    config.RetroDraftScheduleEnabled,
		ExperienceReportEnabled: config.ExperienceScheduleEnabled,
		ExperiencePeriodDays:    config.ExperienceDiscoveryDays,
	})
}

func workMemoryAppProfiles(config []settings.WorkMemoryAppCaptureProfile) []workmemory.AppCaptureProfile {
	profiles := make([]workmemory.AppCaptureProfile, 0, len(config))
	for _, profile := range config {
		profiles = append(profiles, workmemory.AppCaptureProfile{
			ID:                       profile.ID,
			DisplayName:              profile.DisplayName,
			ProcessName:              profile.ProcessName,
			Icon:                     profile.Icon,
			Enabled:                  profile.Enabled,
			WindowSwitchDelaySeconds: profile.WindowSwitchDelaySeconds,
			ActiveIntervalSeconds:    profile.ActiveIntervalSeconds,
		})
	}
	return profiles
}

func applyWorkMemoryAIRuntime(service *workmemory.Service, config settings.AISettings, memory settings.WorkMemorySettings) {
	service.ApplyDraftPolishPolicy(workmemory.DraftPolishPolicy{
		Enabled:  config.Enabled,
		Provider: firstNonEmpty(config.Provider, "openai-compatible"),
		BaseURL:  firstNonEmpty(config.BaseURL, os.Getenv("OPENAI__BASE_URL")),
		Model:    firstNonEmpty(config.Model, os.Getenv("OPENAI__MODEL")),
	})
	service.ApplyOCRSummaryPolicy(workmemory.OCRSummaryPolicy{
		Enabled:  config.Enabled,
		Provider: firstNonEmpty(config.Provider, "openai-compatible"),
		BaseURL:  firstNonEmpty(config.BaseURL, os.Getenv("OPENAI__BASE_URL")),
		Model:    firstNonEmpty(config.Model, os.Getenv("OPENAI__MODEL")),
	})
	service.ApplyFlowAgentPolicy(workmemory.FlowAgentPolicy{
		Enabled:      flowAgentEnabled(config, memory),
		Runner:       flowAgentRunner(config),
		Provider:     firstNonEmpty(config.Provider, "openai-compatible"),
		BaseURL:      firstNonEmpty(config.BaseURL, os.Getenv("OPENAI__BASE_URL")),
		Model:        flowAgentModel(config),
		NativeSkills: config.AgentResponsesEnabled,
	})
	service.ApplyExperienceDiscoveryPolicy(workmemory.ExperienceDiscoveryPolicy{
		Enabled:  config.Enabled && memory.ExperienceDiscoveryEnabled,
		Provider: firstNonEmpty(config.Provider, "openai-compatible"),
		BaseURL:  firstNonEmpty(config.BaseURL, os.Getenv("OPENAI__BASE_URL")),
		Model:    firstNonEmpty(config.Model, os.Getenv("OPENAI__MODEL")),
	})
	service.ApplyEmbeddingPolicy(workmemory.EmbeddingPolicy{
		Enabled:          config.EmbeddingEnabled,
		Provider:         firstNonEmpty(config.EmbeddingProvider, "openai-compatible"),
		BaseURL:          firstNonEmpty(config.EmbeddingBaseURL, os.Getenv("EMBED__BASE_URL")),
		Model:            firstNonEmpty(config.EmbeddingModel, os.Getenv("EMBED__MODEL")),
		VectorStoreType:  config.VectorStoreType,
		VectorStoreURI:   config.VectorStoreURI,
		VectorCollection: config.VectorCollection,
	})
}

func flowAgentEnabled(config settings.AISettings, memory settings.WorkMemorySettings) bool {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("ARIADNE_FLOW_AGENT")))
	switch env {
	case "off", "none", "disabled":
		return false
	case "codex", "openai-agent", "agent", "agents-sdk", "openai-agents-sdk":
		return memory.Enabled
	}
	return memory.Enabled && (config.Enabled || config.AgentsSDKEnabled || config.CodexCollaborationEnabled)
}

func flowAgentRunner(config settings.AISettings) string {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("ARIADNE_FLOW_AGENT")))
	if env == "codex" {
		return "codex"
	}
	if env == "openai-agent" || env == "agent" || env == "agents-sdk" || env == "openai-agents-sdk" {
		return "openai-agent"
	}
	if config.CodexCollaborationEnabled {
		return "codex"
	}
	if config.Enabled || config.AgentsSDKEnabled {
		return "openai-agent"
	}
	return "disabled"
}

func flowAgentModel(config settings.AISettings) string {
	if runner := strings.ToLower(strings.TrimSpace(os.Getenv("ARIADNE_FLOW_AGENT"))); runner == "codex" {
		return strings.TrimSpace(os.Getenv("ARIADNE_FLOW_CODEX_MODEL"))
	}
	return firstNonEmpty(config.Model, os.Getenv("OPENAI__MODEL"))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func rememberActionResult(
	action contracts.PreviewAction,
	clipboardService *clipboardhistory.Service,
	captureService *capturehistory.Service,
	workMemoryService *workmemory.Service,
) contracts.ActionResult {
	targetID := actionPayloadString(action.Payload, "targetId")
	if targetID == "" {
		return contracts.ActionResult{OK: false, Message: "缺少加入记忆目标"}
	}
	switch {
	case strings.HasPrefix(targetID, "clipboard-"):
		clipboardID := strings.TrimPrefix(targetID, "clipboard-")
		entry := clipboardService.Entry(clipboardID)
		if entry.ID == "" {
			return contracts.ActionResult{OK: false, Message: "未找到剪贴板记录"}
		}
		memory := workMemoryService.RememberClipboardEntry(entry)
		return rememberMemoryResult(memory, workMemoryService.Status())
	case strings.HasPrefix(targetID, "capture-"):
		captureID := strings.TrimPrefix(targetID, "capture-")
		entry := captureService.Entry(captureID)
		if entry.ID == "" {
			return contracts.ActionResult{OK: false, Message: "未找到截图记录"}
		}
		memory := workMemoryService.RememberCaptureHistoryEntry(entry)
		return rememberMemoryResult(memory, workMemoryService.Status())
	default:
		return contracts.ActionResult{OK: false, Message: "当前结果暂不支持加入工作记忆"}
	}
}

func rememberMemoryResult(memory workmemory.Entry, status workmemory.Status) contracts.ActionResult {
	if memory.ID != "" {
		return contracts.ActionResult{OK: true, Message: "已加入工作记忆"}
	}
	if !status.Enabled {
		return contracts.ActionResult{OK: false, Message: "工作记忆已停用"}
	}
	if status.PrivacyMode {
		return contracts.ActionResult{OK: false, Message: "隐私模式已开启，已阻断加入记忆"}
	}
	return contracts.ActionResult{OK: false, Message: "未能加入工作记忆"}
}

func actionPayloadString(payload map[string]interface{}, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(strings.Trim(fmt.Sprint(value), `"`))
}
