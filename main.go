package main

import (
	"embed"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"ariadne/internal/aiclient"
	"ariadne/internal/airplay"
	"ariadne/internal/apitesting"
	"ariadne/internal/applog"
	"ariadne/internal/apps"
	"ariadne/internal/appupdate"
	"ariadne/internal/capturehistory"
	"ariadne/internal/captureoverlay"

	"ariadne/internal/clipboardhistory"

	"ariadne/internal/filesearch"

	"ariadne/internal/hosts"
	"ariadne/internal/imageindex"
	"ariadne/internal/jsoncompare"
	"ariadne/internal/launchers"
	"ariadne/internal/migration"
	"ariadne/internal/nativecapture"
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

	"ariadne/internal/toolwindows"
	"ariadne/internal/workflows"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed assets/logo.png
var appIcon []byte

//go:embed assets/logo.png
var trayIcon []byte

var appVersion = "dev"

func main() {
	if _, err := handleRestartReplacementArgs(os.Args[1:], os.Getpid(), waitForRestartReplacementProcess); err != nil {
		fmt.Fprintf(os.Stderr, "Ariadne restart replacement: %v\n", err)
	}

	if len(os.Args) > 1 && strings.EqualFold(os.Args[1], "filesearch-service") {
		if err := filesearch.RunWindowsServiceWithPolicyProvider(os.Args[2:], fileSearchPolicyProvider(os.Args[2:])); err != nil {
			fmt.Fprintf(os.Stderr, "Ariadne file search service: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && strings.EqualFold(os.Args[1], "filesearch-service-install") {
		exePath, err := os.Executable()
		if err == nil {
			err = filesearch.InstallWindowsService("Ariadne", exePath, "--settings-config", settings.NewService().ConfigPath())
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Ariadne search service install: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && strings.EqualFold(os.Args[1], "filesearch-service-remove") {
		if err := filesearch.RemoveWindowsService(); err != nil {
			fmt.Fprintf(os.Stderr, "Ariadne search service remove: %v\n", err)
			os.Exit(1)
		}
		return
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
	hostsService := hosts.NewService()
	jsonCompareService := jsoncompare.NewService()
	apiTestingService := apitesting.NewService()
	networkMonitorService := networkmonitor.NewService()
	networkMonitorService.SetProcessIconResolver(func(path string) string {
		if icon := fileSearchService.ResolveFileIcon(path); icon != nil {
			return icon.URL
		}
		return ""
	})
	qrScanService := qrscan.NewService(captureService)
	airplayService := airplay.NewService()
	pluginService := plugins.NewService()
	workflowService := workflows.NewService(pluginService)
	secretsService := secrets.NewService()
	settingsService := settings.NewService()
	pluginService.ApplyEnabled(settingsService.GetSettings().Plugins.Enabled)
	migrationService := migration.NewService(clipboardService, captureService)
	releaseService := release.NewService()
	ocrService := ocr.NewService(captureService, clipboardService)
	ocr.RegisterAIClient(ocrService, aiclient.NewOpenAICompatibleImageOCR())
	toolWindowService := toolwindows.NewService()
	toolWindowService.SetWindowIcon(appIcon)
	var nativeCaptureManager *nativecapture.Manager
	if runtime.GOOS == "windows" {
		nativeCaptureManager = nativecapture.NewManager(nativecapture.Options{})
	}
	captureOverlayService := captureoverlay.NewServiceWithNative(captureService, pinnedImageService, nativeCaptureManager, ocrService)
	captureOverlayService.SetFocusSuspension(toolWindowService.SetCaptureFocusSuspended)
	imageIndexService := imageindex.NewService(captureService, clipboardService, ocrService)
	searchService := search.NewService(fileSearchService, appService, launcherService, captureService, imageIndexService, workflowService, pluginService)
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
				Provider:        status.Provider,
				Indexing:        status.Indexing,
				IndexedCount:    status.IndexedCount,
				VolumeCount:     status.VolumeCount,
				RequiresAdmin:   status.RequiresAdmin,
				Elevated:        status.Elevated,
				IndexStartedAt:  status.IndexStartedAt,
				IndexFinishedAt: status.IndexFinishedAt,
				LastError:       status.LastError,
				LastQuery:       status.LastQuery,
				LastElapsedMs:   status.LastElapsedMs,
				LastResultCount: status.LastResultCount,
				LastUpdatedAt:   status.LastUpdatedAt,
				CoverageHint:    status.CoverageHint,
				PolicyErrors:    status.PolicyErrors,
			}
		}),
		platform.WithLogStatus(func() platform.LogStatus {
			return appLogStatus(logService.Status())
		}),
		platform.WithApplicationQuit(func() {
			shellManager.Quit()
		}),
	)

	app := application.New(application.Options{
		Name:        "Ariadne",
		Description: "Ariadne desktop productivity tools",
		Icon:        appIcon,
		Windows: application.WindowsOptions{
			DisableQuitOnLastWindowClosed: true,
		},
		SingleInstance: shellManager.SingleInstanceOptions(),
		OnShutdown: func() {
			log.Println("Ariadne shutting down")
			if result := airplayService.Stop(); !result.OK {
				log.Printf("stop airplay: %s", result.Message)
			}
			if err := shellManager.Stop(); err != nil {
				log.Printf("stop shell: %v", err)
			}
			toolWindowService.Stop()
			if nativeCaptureManager != nil {
				if err := nativeCaptureManager.Stop(); err != nil {
					log.Printf("stop native capture host: %v", err)
				}
			}
			fileSearchService.Close()
			clipboardService.StopWatcher()
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
			application.NewService(apiTestingService),
			application.NewService(airplayService),
			application.NewService(networkMonitorService),
			application.NewService(qrScanService),
			application.NewService(ocrService),
			application.NewService(imageIndexService),
			application.NewService(workflowService),
			application.NewService(secretsService),
			application.NewService(migrationService),
			application.NewService(releaseService),
			application.NewService(toolWindowService),
			application.NewService(settingsService),
			application.NewService(platformService),
		},
		Assets: application.AssetOptions{
			Handler: captureoverlay.CaptureOverlayAssetHandler(captureOverlayService, fileSearchService.FileIconAssetHandler(application.AssetFileServerFS(assets))),
		},
	})
	appUpdateService, err := appupdate.Configure(app.Updater, appVersion, func() error {
		checkpoint := releaseService.CreateRollbackCheckpoint(release.BackupRequest{Reason: "pre_app_update:" + appVersion})
		if !checkpoint.OK {
			return fmt.Errorf("%s", checkpoint.Message)
		}
		return nil
	})
	if err != nil {
		log.Printf("configure updater: %v", err)
	}
	app.RegisterService(application.NewService(appUpdateService))
	app.HandleStream(networkmonitor.TelemetryStreamName, func(connection *application.StreamConn) {
		networkmonitor.HandleTelemetryStream(networkMonitorService, connection)
	})
	captureOverlayService.Attach(app)
	search.RegisterProgressHandler(searchService, func(progress search.Progress) {
		app.Event.Emit("ariadne:search-progress", progress)
	})
	pinnedImageService.Attach(app)
	toolWindowService.Attach(app)

	settings.RegisterChangeHandler(settingsService, func(next settings.AppSettings) {
		shellManager.ApplyHotkeys(next.Hotkeys.ToggleWindow, next.Hotkeys.Screenshot, next.Hotkeys.PinClipboard)
		if err := shellManager.ApplyAutostart(next.General.RunOnStartup); err != nil {
			log.Printf("apply autostart: %v", err)
		}
		applyCaptureOverlayRuntime(captureOverlayService, next.Screenshot)
		applyOCRAIRuntime(ocrService, next.AI)
		applyRetentionPolicies(captureService, clipboardService, imageIndexService, next.History)
		clipboardService.ApplyWatcherSettings(next.History.PrivacyMode, next.History.ClipboardEnabled)
		pluginService.ApplyEnabled(next.Plugins.Enabled)
		applyFileSearchRuntime(fileSearchService, next.Search)
	})
	initialSettings := settingsService.GetSettings()
	applyFileSearchRuntime(fileSearchService, initialSettings.Search)
	applyCaptureOverlayRuntime(captureOverlayService, initialSettings.Screenshot)
	applyOCRAIRuntime(ocrService, initialSettings.AI)
	if nativeCaptureManager != nil {
		if err := nativeCaptureManager.Start(); err != nil {
			log.Printf("start native capture host: %v", err)
		}
	}

	shellManager.Attach(app, nil, trayIcon)
	if err := shellManager.ApplyAutostart(settingsService.GetSettings().General.RunOnStartup); err != nil {
		log.Printf("apply autostart: %v", err)
	}
	deferStartupMaintenance(func() {
		latest := settingsService.GetSettings()
		applyOCRAIRuntime(ocrService, latest.AI)
		applyRetentionPolicies(captureService, clipboardService, imageIndexService, latest.History)
		clipboardService.ApplyWatcherSettings(latest.History.PrivacyMode, latest.History.ClipboardEnabled)
		fileSearchService.StartIndexing()
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

func deferStartupMaintenance(fn func()) {
	go func() {
		time.Sleep(1500 * time.Millisecond)
		fn()
	}()
}

func applyRetentionPolicies(
	captureService *capturehistory.Service,
	clipboardService *clipboardhistory.Service,
	imageIndexService *imageindex.Service,
	config settings.HistorySettings,
) {
	captureService.ApplyRetentionPolicy(config.RetentionDays, config.KeepFavoritesForever)
	captureService.ApplyStoragePolicy(config.MaxStorageMB, config.KeepFavoritesForever)
	clipboardService.ApplyRetentionPolicy(config.RetentionDays, config.KeepFavoritesForever)
	imageIndexService.ApplyRetentionPolicy(config.RetentionDays)
}

func applyFileSearchRuntime(service *filesearch.Service, config settings.SearchSettings) {
	service.ApplyPolicy(fileSearchPolicyFromSettings(config))
}

func fileSearchPolicyProvider(args []string) filesearch.PolicyProvider {
	configPath := fileSearchSettingsConfigPath(args)
	return func() filesearch.FileSearchPolicy {
		if strings.TrimSpace(configPath) != "" {
			return fileSearchPolicyFromSettings(settings.NewServiceWithPaths(configPath, "").GetSettings().Search)
		}
		return fileSearchPolicyFromSettings(settings.NewService().GetSettings().Search)
	}
}

func fileSearchSettingsConfigPath(args []string) string {
	configPath := ""
	for index := 0; index < len(args); index++ {
		arg := strings.TrimSpace(args[index])
		if strings.EqualFold(arg, "--settings-config") || strings.EqualFold(arg, "-settings-config") {
			if index+1 < len(args) {
				configPath = args[index+1]
			}
			break
		}
		if strings.HasPrefix(strings.ToLower(arg), "--settings-config=") {
			configPath = strings.TrimSpace(arg[len("--settings-config="):])
			break
		}
	}
	configPath = strings.TrimSpace(configPath)
	if strings.EqualFold(filepath.Base(configPath), "ariadne.sqlite") {
		return filepath.Join(filepath.Dir(configPath), "config.json")
	}
	return configPath
}

func fileSearchPolicyFromSettings(config settings.SearchSettings) filesearch.FileSearchPolicy {
	return filesearch.FileSearchPolicy{
		ExcludeFolders:    config.FileExcludeFolders,
		ExcludePatterns:   config.FileExcludePatterns,
		IncludeExtensions: config.FileIncludeExtensions,
		ExcludeExtensions: config.FileExcludeExtensions,
	}
}

func applyCaptureOverlayRuntime(service *captureoverlay.Service, config settings.ScreenshotSettings) {
	service.ApplyScreenshotPolicy(captureoverlay.ScreenshotPolicy{
		AutoCopy:         config.AutoCopy,
		AutoPin:          config.AutoPin,
		AutoSave:         config.AutoSave,
		SaveDir:          config.SaveDir,
		FilenameTemplate: config.FilenameTemplate,
		AutoRedact:       config.AutoRedact,
		RedactPhones:     config.RedactPhones,
		RedactKeywords:   config.RedactKeywords,
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
	return firstNonEmpty(config.OCRBaseURL, os.Getenv("ARIADNE_OCR_BASE_URL"), os.Getenv("OPENAI__BASE_URL"))
}

func isOllamaOCRProvider(provider string) bool {
	switch strings.TrimSpace(strings.ToLower(provider)) {
	case "ollama", "ollama-generate", "ollama_generate":
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
