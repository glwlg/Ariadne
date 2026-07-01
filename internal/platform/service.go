package platform

import (
	"archive/zip"
	"ariadne/internal/contracts"
	"ariadne/internal/elevation"
	"ariadne/internal/ocr"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Capability struct {
	ID       string `json:"id"`
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"`
	Note     string `json:"note,omitempty"`
}

type RuntimeDiagnostics struct {
	OS              string `json:"os"`
	Arch            string `json:"arch"`
	GoVersion       string `json:"goVersion"`
	ProcessID       int    `json:"processId"`
	WorkingDir      string `json:"workingDir"`
	ExecutablePath  string `json:"executablePath"`
	ExecutableBytes int64  `json:"executableBytes"`
	AppDataEnv      string `json:"appDataEnv"`
	LocalAppDataEnv string `json:"localAppDataEnv"`
	GoToolPath      string `json:"goToolPath,omitempty"`
	WailsToolPath   string `json:"wailsToolPath,omitempty"`
}

type LegacyRuntimeStatus struct {
	ProcessRunning       bool     `json:"processRunning"`
	ProcessID            int      `json:"processId,omitempty"`
	ProcessName          string   `json:"processName,omitempty"`
	ProcessPath          string   `json:"processPath,omitempty"`
	ConfigPath           string   `json:"configPath"`
	ConfigExists         bool     `json:"configExists"`
	ConfigBytes          int64    `json:"configBytes,omitempty"`
	HotkeyConflictLikely bool     `json:"hotkeyConflictLikely"`
	Notes                []string `json:"notes,omitempty"`
}

type RuntimeMetric struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Value int64  `json:"value"`
	Unit  string `json:"unit"`
}

type SearchPerformanceStatus struct {
	SampleCount     int    `json:"sampleCount"`
	TargetP95Ms     int64  `json:"targetP95Ms"`
	LastQuery       string `json:"lastQuery,omitempty"`
	LastElapsedMs   int64  `json:"lastElapsedMs"`
	LastResultCount int    `json:"lastResultCount"`
	AverageMs       int64  `json:"averageMs"`
	P95Ms           int64  `json:"p95Ms"`
	MaxMs           int64  `json:"maxMs"`
	WithinTarget    bool   `json:"withinTarget"`
	LastUpdatedAt   int64  `json:"lastUpdatedAt,omitempty"`
}

type FileSearchStatus struct {
	DLLPath          string   `json:"dllPath,omitempty"`
	DLLFound         bool     `json:"dllFound"`
	Ready            bool     `json:"ready"`
	Provider         string   `json:"provider,omitempty"`
	ServiceName      string   `json:"serviceName,omitempty"`
	ServiceInstalled bool     `json:"serviceInstalled"`
	ServiceRunning   bool     `json:"serviceRunning"`
	ServiceState     string   `json:"serviceState,omitempty"`
	ServiceError     string   `json:"serviceError,omitempty"`
	Indexing         bool     `json:"indexing"`
	IndexedCount     int      `json:"indexedCount"`
	VolumeCount      int      `json:"volumeCount"`
	RequiresAdmin    bool     `json:"requiresAdmin"`
	Elevated         bool     `json:"elevated"`
	IndexStartedAt   int64    `json:"indexStartedAt,omitempty"`
	IndexFinishedAt  int64    `json:"indexFinishedAt,omitempty"`
	LastError        string   `json:"lastError,omitempty"`
	LastQuery        string   `json:"lastQuery,omitempty"`
	LastElapsedMs    int64    `json:"lastElapsedMs"`
	LastResultCount  int      `json:"lastResultCount"`
	LastUpdatedAt    int64    `json:"lastUpdatedAt,omitempty"`
	CoverageHint     string   `json:"coverageHint,omitempty"`
	PolicyErrors     []string `json:"policyErrors,omitempty"`
}

type LogStatus struct {
	Path            string `json:"path"`
	Directory       string `json:"directory"`
	DirectoryExists bool   `json:"directoryExists"`
	Exists          bool   `json:"exists"`
	Bytes           int64  `json:"bytes"`
	LastModifiedAt  int64  `json:"lastModifiedAt,omitempty"`
	LastError       string `json:"lastError,omitempty"`
}

type EnvironmentStatus struct {
	AppName           string                  `json:"appName"`
	LegacyName        string                  `json:"legacyName"`
	Capabilities      []Capability            `json:"capabilities"`
	Diagnostics       RuntimeDiagnostics      `json:"diagnostics"`
	Shell             ShellStatus             `json:"shell"`
	LegacyRuntime     LegacyRuntimeStatus     `json:"legacyRuntime"`
	SearchPerformance SearchPerformanceStatus `json:"searchPerformance"`
	FileSearch        FileSearchStatus        `json:"fileSearch"`
	Logs              LogStatus               `json:"logs"`
	Metrics           []RuntimeMetric         `json:"metrics"`
}

type DiagnosticsExportResult struct {
	OK          bool     `json:"ok"`
	Message     string   `json:"message"`
	Path        string   `json:"path,omitempty"`
	Bytes       int64    `json:"bytes,omitempty"`
	CreatedAt   int64    `json:"createdAt,omitempty"`
	Included    []string `json:"included,omitempty"`
	LogIncluded bool     `json:"logIncluded"`
}

type LegacyHandoffRequest struct {
	Confirm   bool `json:"confirm"`
	Force     bool `json:"force"`
	TimeoutMs int  `json:"timeoutMs,omitempty"`
}

type LegacyHandoffResult struct {
	OK                   bool                `json:"ok"`
	Message              string              `json:"message"`
	Before               LegacyRuntimeStatus `json:"before"`
	After                LegacyRuntimeStatus `json:"after"`
	Shell                ShellStatus         `json:"shell"`
	Actions              []string            `json:"actions"`
	RequiresConfirmation bool                `json:"requiresConfirmation"`
	ForceUsed            bool                `json:"forceUsed"`
	HotkeyRetried        bool                `json:"hotkeyRetried"`
	CreatedAt            int64               `json:"createdAt"`
}

type ShellStatus struct {
	SingleInstanceConfigured     bool     `json:"singleInstanceConfigured"`
	TrayConfigured               bool     `json:"trayConfigured"`
	GlobalHotkeyRegistered       bool     `json:"globalHotkeyRegistered"`
	GlobalHotkey                 string   `json:"globalHotkey"`
	ScreenshotHotkeyRegistered   bool     `json:"screenshotHotkeyRegistered"`
	ScreenshotHotkey             string   `json:"screenshotHotkey"`
	PinClipboardHotkeyRegistered bool     `json:"pinClipboardHotkeyRegistered"`
	PinClipboardHotkey           string   `json:"pinClipboardHotkey"`
	AutostartSupported           bool     `json:"autostartSupported"`
	AutostartEnabled             bool     `json:"autostartEnabled"`
	AutostartPath                string   `json:"autostartPath"`
	AutostartIdentifier          string   `json:"autostartIdentifier,omitempty"`
	AutostartValueName           string   `json:"autostartValueName,omitempty"`
	AutostartCommand             string   `json:"autostartCommand,omitempty"`
	AutostartCommandValid        bool     `json:"autostartCommandValid"`
	AutostartHiddenArgPresent    bool     `json:"autostartHiddenArgPresent"`
	AutostartNotes               []string `json:"autostartNotes,omitempty"`
	LastError                    string   `json:"lastError"`
}

type legacyHandoffOutcome struct {
	Actions        []string
	Error          string
	ForceUsed      bool
	ProcessExited  bool
	WindowsReached int
}

type commandRunRequest struct {
	Command    string
	Arguments  []string
	WorkingDir string
	Wait       bool
}

type systemCommand struct {
	Label        string
	Command      string
	Arguments    []string
	SuccessLabel string
	RiskReasons  []string
}

type Option func(*Service)

type Service struct {
	shellStatus       func() ShellStatus
	legacyRuntime     func(ShellStatus, RuntimeDiagnostics) LegacyRuntimeStatus
	searchPerformance func() SearchPerformanceStatus
	fileSearchStatus  func() FileSearchStatus
	serviceFileSearch func(FileSearchStatus) FileSearchStatus
	logStatus         func() LogStatus
	hotkeyRetry       func() ShellStatus
	legacyHandoff     func(LegacyHandoffRequest, LegacyRuntimeStatus) legacyHandoffOutcome
	commandRunner     func(commandRunRequest) error
	elevatedRunner    func(string, []string) error
	applicationQuit   func()
	rememberAction    func(contracts.PreviewAction) contracts.ActionResult
}

func NewService(options ...Option) *Service {
	service := &Service{}
	for _, option := range options {
		option(service)
	}
	return service
}

func WithShellStatus(provider func() ShellStatus) Option {
	return func(service *Service) {
		service.shellStatus = provider
	}
}

func WithLegacyRuntime(provider func(ShellStatus, RuntimeDiagnostics) LegacyRuntimeStatus) Option {
	return func(service *Service) {
		service.legacyRuntime = provider
	}
}

func WithSearchPerformance(provider func() SearchPerformanceStatus) Option {
	return func(service *Service) {
		service.searchPerformance = provider
	}
}

func WithFileSearchStatus(provider func() FileSearchStatus) Option {
	return func(service *Service) {
		service.fileSearchStatus = provider
	}
}

func WithFileSearchServiceStatus(provider func(FileSearchStatus) FileSearchStatus) Option {
	return func(service *Service) {
		service.serviceFileSearch = provider
	}
}

func WithLogStatus(provider func() LogStatus) Option {
	return func(service *Service) {
		service.logStatus = provider
	}
}

func WithHotkeyRetry(provider func() ShellStatus) Option {
	return func(service *Service) {
		service.hotkeyRetry = provider
	}
}

func WithLegacyHandoff(handler func(LegacyHandoffRequest, LegacyRuntimeStatus) legacyHandoffOutcome) Option {
	return func(service *Service) {
		service.legacyHandoff = handler
	}
}

func WithCommandRunner(handler func(commandRunRequest) error) Option {
	return func(service *Service) {
		service.commandRunner = handler
	}
}

func WithElevatedRunner(handler func(string, []string) error) Option {
	return func(service *Service) {
		service.elevatedRunner = handler
	}
}

func WithApplicationQuit(handler func()) Option {
	return func(service *Service) {
		service.applicationQuit = handler
	}
}

func WithRememberActionHandler(handler func(contracts.PreviewAction) contracts.ActionResult) Option {
	return func(service *Service) {
		service.rememberAction = handler
	}
}

func (s *Service) Status() EnvironmentStatus {
	diagnostics := runtimeDiagnostics()
	shell := s.currentShellStatus()
	legacy := s.currentLegacyRuntime(shell, diagnostics)
	searchPerformance := s.currentSearchPerformance()
	fileSearch := s.currentFileSearchStatus(diagnostics)
	logs := s.currentLogStatus(diagnostics)
	return EnvironmentStatus{
		AppName:    "Ariadne",
		LegacyName: "x-tools",
		Capabilities: []Capability{
			{ID: "preview_actions", Enabled: true, Provider: "Ariadne contracts", Note: "结果动作由后端显式声明，非文件结果不继承文件动作。"},
			{ID: "settings", Enabled: true, Provider: "Ariadne settings service", Note: "支持旧配置安全导入、写后读回校验和 MSIX AppData virtualization 诊断。"},
			{ID: "work_memory", Enabled: true, Provider: "Ariadne work memory", Note: "时间线、手动补记、手动笔记、删除/清理、可读数据包导出、后台屏幕时间机器、日报、知识草稿、外部代理任务包、SQLite FTS、本地语义检索、外部 embedding、内置向量缓存和 Milvus 向量存储已接入。"},
			{ID: "file_search", Enabled: fileSearch.Ready || fileSearch.Indexing || fileSearch.ServiceRunning, Provider: firstNonEmpty(fileSearch.Provider, "Ariadne USN/MFT"), Note: fileSearchNote(fileSearch)},
			{ID: "app_scan", Enabled: true, Provider: "Start Menu shortcuts", Note: "已接入用户和系统开始菜单 .lnk 扫描。"},
			{ID: "custom_launchers", Enabled: true, Provider: "Ariadne launcher registry", Note: "自定义启动项已接入搜索 provider；支持应用、文件、文件夹、URL 和需要确认的命令。"},
			{ID: "clipboard_history", Enabled: true, Provider: "Ariadne clipboard history", Note: "文本和图片剪贴板历史已接入自动监听、持久化、搜索、置顶、预览和中心 UI。"},
			{ID: "capture_history", Enabled: runtime.GOOS == "windows", Provider: "Ariadne capture history", Note: "Windows GDI 截屏、区域截图覆盖层、PNG 持久化、搜索、置顶和中心 UI 已接入。"},
			{ID: "screenshot_overlay", Enabled: runtime.GOOS == "windows", Provider: "Ariadne capture overlay", Note: "区域选择覆盖层已接入独立无边框 Wails 窗口，支持保存、复制、自动保存、自动贴图、二维码识别、放大镜取色、选区缩放、标注和已有标注选择/移动/删除。"},
			{ID: "pinned_image", Enabled: true, Provider: "Ariadne pinned image service", Note: "截图历史、区域截图、剪贴板图片和二维码结果可创建独立置顶无边框贴图窗口，支持拖动、缩放、右键菜单、阴影切换、复制和图片 OCR。"},
			{ID: "ocr", Enabled: ocr.RuntimeAvailable(), Provider: "RapidOCR local bridge", Note: ocr.RuntimeNote()},
			{ID: "qr_recognition", Enabled: true, Provider: "gozxing QR reader", Note: "可识别截图历史或用户显式触发当前屏幕中的二维码；不后台扫描屏幕。"},
			{ID: "system_commands", Enabled: runtime.GOOS == "windows", Provider: "Ariadne platform action runner", Note: "系统命令插件通过受控 Windows 命令映射执行锁屏、休眠、清空回收站、关机和重启，所有动作都需要用户二次确认。"},
			{ID: "network_monitor", Enabled: runtime.GOOS == "windows", Provider: "Windows IP Helper API", Note: "实时上下行速率和网卡累计流量已接入 Go 服务、搜索入口和中心 UI。"},
			{ID: "hosts", Enabled: runtime.GOOS == "windows", Provider: "Ariadne hosts service", Note: "Hosts 方案管理、远程拉取、冲突检测和应用前预览已接入；写入系统 Hosts 需要用户确认并经 UAC。"},
			{ID: "json_compare", Enabled: true, Provider: "Ariadne JSON compare service", Note: "JSON 语义差异、规范化 diff 和中心 UI 已接入 Go/TS 主路径。"},
			{ID: "workflow_macros", Enabled: true, Provider: "Ariadne workflow service", Note: "工作流宏已接入持久化、旧配置迁移、搜索入口、变量渲染、命令链执行和中心 UI。"},
			{ID: "search_ranking", Enabled: true, Provider: "Ariadne search state", Note: "收藏和最近使用排序已接入本地状态文件。"},
			{ID: "search_performance", Enabled: true, Provider: "Ariadne search metrics", Note: searchPerformanceNote(searchPerformance)},
			{ID: "legacy_coexistence", Enabled: !legacy.ProcessRunning && !legacy.HotkeyConflictLikely, Provider: "Ariadne migration guard", Note: legacyCoexistenceNote(legacy)},
			{ID: "single_instance", Enabled: shell.SingleInstanceConfigured, Provider: "Wails SingleInstance", Note: shellSingleInstanceNote(shell)},
			{ID: "global_hotkey", Enabled: shell.GlobalHotkeyRegistered, Provider: "Windows RegisterHotKey", Note: shellHotkeyNote(shell)},
			{ID: "tray", Enabled: shell.TrayConfigured, Provider: "Wails SystemTray", Note: shellTrayNote(shell)},
			{ID: "autostart", Enabled: shell.AutostartSupported, Provider: "Wails Autostart", Note: shellAutostartNote(shell)},
			{ID: "diagnostic_logs", Enabled: logs.DirectoryExists && logs.LastError == "", Provider: "Ariadne local log", Note: diagnosticLogNote(logs)},
		},
		Diagnostics:       diagnostics,
		Shell:             shell,
		LegacyRuntime:     legacy,
		SearchPerformance: searchPerformance,
		FileSearch:        fileSearch,
		Logs:              logs,
		Metrics:           runtimeMetrics(diagnostics, searchPerformance, fileSearch, logs),
	}
}

func (s *Service) ResolveLegacyConflict(request LegacyHandoffRequest) LegacyHandoffResult {
	request = normalizeLegacyHandoffRequest(request)
	createdAt := time.Now().Unix()
	diagnostics := runtimeDiagnostics()
	shell := s.currentShellStatus()
	before := s.currentLegacyRuntime(shell, diagnostics)
	result := LegacyHandoffResult{
		Before:    before,
		After:     before,
		Shell:     shell,
		CreatedAt: createdAt,
		Actions:   []string{},
	}
	if !request.Confirm {
		result.RequiresConfirmation = true
		result.Message = "需要确认后关闭旧版 x-tools 并重试 Ariadne Alt+Q"
		return result
	}

	var handoffError string
	if before.ProcessRunning {
		outcome := s.performLegacyHandoff(request, before)
		result.Actions = append(result.Actions, outcome.Actions...)
		result.ForceUsed = outcome.ForceUsed
		if outcome.Error != "" {
			handoffError = outcome.Error
		}
	} else {
		result.Actions = append(result.Actions, "未发现正在运行的旧版 x-tools 进程")
	}

	if s.hotkeyRetry != nil {
		result.Shell = s.hotkeyRetry()
		result.HotkeyRetried = true
		result.Actions = append(result.Actions, "已重试 Ariadne 全局快捷键注册")
	} else {
		result.Shell = s.currentShellStatus()
		result.Actions = append(result.Actions, "当前运行态未提供热键重试接口")
	}

	result.After = s.currentLegacyRuntime(result.Shell, runtimeDiagnostics())
	result.OK = !result.After.ProcessRunning && (result.Shell.GlobalHotkeyRegistered || !result.After.HotkeyConflictLikely)
	if result.OK {
		if result.Shell.GlobalHotkeyRegistered {
			result.Message = "旧版交接完成，Ariadne 已接管 Alt+Q"
		} else {
			result.Message = "旧版交接完成，未发现 Alt+Q 冲突"
		}
		return result
	}
	if handoffError != "" {
		result.Message = "旧版交接未完成：" + handoffError
		return result
	}
	if result.After.ProcessRunning {
		result.Message = "旧版 x-tools 仍在运行；可确认后强制结束，或手动退出旧版"
		return result
	}
	if result.After.HotkeyConflictLikely {
		result.Message = "旧版已退出，但 Alt+Q 仍被其他进程占用"
		return result
	}
	result.Message = "旧版交接未完成"
	return result
}

func (s *Service) ExportDiagnostics() DiagnosticsExportResult {
	createdAt := time.Now()
	status := s.Status()
	exportDir := diagnosticsExportDir(status.Diagnostics)
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		return DiagnosticsExportResult{OK: false, Message: err.Error(), CreatedAt: createdAt.Unix()}
	}
	path := filepath.Join(exportDir, "ariadne-diagnostics-"+createdAt.Format("20060102-150405")+".zip")
	file, err := os.Create(path)
	if err != nil {
		return DiagnosticsExportResult{OK: false, Message: err.Error(), Path: path, CreatedAt: createdAt.Unix()}
	}
	archive := zip.NewWriter(file)
	included := []string{}
	add := func(name string, data []byte) error {
		if err := writeZipBytes(archive, name, data); err != nil {
			return err
		}
		included = append(included, name)
		return nil
	}
	if err := add("README.md", []byte(diagnosticsReadme(createdAt))); err != nil {
		_ = archive.Close()
		_ = file.Close()
		return DiagnosticsExportResult{OK: false, Message: err.Error(), Path: path, CreatedAt: createdAt.Unix()}
	}
	statusJSON, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		_ = archive.Close()
		_ = file.Close()
		return DiagnosticsExportResult{OK: false, Message: err.Error(), Path: path, CreatedAt: createdAt.Unix()}
	}
	if err := add("diagnostics/platform_status.json", statusJSON); err != nil {
		_ = archive.Close()
		_ = file.Close()
		return DiagnosticsExportResult{OK: false, Message: err.Error(), Path: path, CreatedAt: createdAt.Unix()}
	}
	metricsJSON, err := json.MarshalIndent(status.Metrics, "", "  ")
	if err != nil {
		_ = archive.Close()
		_ = file.Close()
		return DiagnosticsExportResult{OK: false, Message: err.Error(), Path: path, CreatedAt: createdAt.Unix()}
	}
	if err := add("diagnostics/metrics.json", metricsJSON); err != nil {
		_ = archive.Close()
		_ = file.Close()
		return DiagnosticsExportResult{OK: false, Message: err.Error(), Path: path, CreatedAt: createdAt.Unix()}
	}
	logIncluded := false
	if status.Logs.Exists && status.Logs.Path != "" {
		if err := writeZipFile(archive, "logs/ariadne.log", status.Logs.Path); err != nil {
			_ = archive.Close()
			_ = file.Close()
			return DiagnosticsExportResult{OK: false, Message: err.Error(), Path: path, CreatedAt: createdAt.Unix()}
		}
		included = append(included, "logs/ariadne.log")
		logIncluded = true
	}
	if err := archive.Close(); err != nil {
		_ = file.Close()
		return DiagnosticsExportResult{OK: false, Message: err.Error(), Path: path, CreatedAt: createdAt.Unix()}
	}
	if err := file.Close(); err != nil {
		return DiagnosticsExportResult{OK: false, Message: err.Error(), Path: path, CreatedAt: createdAt.Unix()}
	}
	bytes := int64(0)
	if info, err := os.Stat(path); err == nil {
		bytes = info.Size()
	}
	return DiagnosticsExportResult{
		OK:          true,
		Message:     "诊断包已导出",
		Path:        path,
		Bytes:       bytes,
		CreatedAt:   createdAt.Unix(),
		Included:    included,
		LogIncluded: logIncluded,
	}
}

func (s *Service) InstallFileSearchService() contracts.ActionResult {
	if runtime.GOOS != "windows" {
		return contracts.ActionResult{OK: false, Message: "搜索服务仅支持 Windows"}
	}
	exePath, err := os.Executable()
	if err != nil || strings.TrimSpace(exePath) == "" {
		if err == nil {
			err = fmt.Errorf("缺少 Ariadne 程序路径")
		}
		return contracts.ActionResult{OK: false, Message: "搜索服务安装失败: " + err.Error()}
	}
	if err := s.runElevated(exePath, []string{"filesearch-service-install"}); err != nil {
		return contracts.ActionResult{OK: false, Message: "搜索服务安装未完成: " + err.Error()}
	}
	return contracts.ActionResult{OK: true, Message: "搜索服务已安装"}
}

func (s *Service) ExecuteAction(action contracts.PreviewAction) contracts.ActionResult {
	if action.ID == "install_file_search_service" {
		return s.InstallFileSearchService()
	}
	if action.Kind == contracts.ActionCopy {
		return contracts.ActionResult{OK: true, Message: "已复制"}
	}
	if action.Kind == contracts.ActionRemember {
		if s.rememberAction == nil {
			return contracts.ActionResult{OK: false, Message: "当前结果暂不支持加入工作记忆"}
		}
		return s.rememberAction(action)
	}
	if action.ID == "run_system" {
		return s.executeSystemCommandAction(action)
	}
	if action.Kind == contracts.ActionOpen {
		path := payloadString(action.Payload, "path")
		if path == "" {
			return contracts.ActionResult{OK: false, Message: "缺少打开路径"}
		}
		if err := openWithShell(path); err != nil {
			return contracts.ActionResult{OK: false, Message: err.Error()}
		}
		return contracts.ActionResult{OK: true, Message: actionSuccess(action, "已打开")}
	}
	if action.Kind == contracts.ActionOpenParent {
		path := payloadString(action.Payload, "path")
		if path == "" {
			return contracts.ActionResult{OK: false, Message: "缺少路径"}
		}
		if err := revealInExplorer(path); err != nil {
			return contracts.ActionResult{OK: false, Message: err.Error()}
		}
		return contracts.ActionResult{OK: true, Message: actionSuccess(action, "已打开所在文件夹")}
	}
	if action.Kind == contracts.ActionRun || action.Kind == contracts.ActionDanger {
		command := payloadString(action.Payload, "command")
		if command == "" {
			return contracts.ActionResult{OK: false, Message: "缺少命令"}
		}
		arguments, err := payloadArguments(action.Payload, "arguments")
		if err != nil {
			return contracts.ActionResult{OK: false, Message: err.Error()}
		}
		workingDir := payloadString(action.Payload, "workingDir")
		if payloadBool(action.Payload, "requiresConfirmation") || action.Kind == contracts.ActionDanger {
			if !payloadBool(action.Payload, "confirmed") && !payloadBool(action.Payload, "confirm") {
				return contracts.ActionResult{
					OK:                   false,
					Message:              "再次点击确认：" + actionConfirmationLabel(action, command, arguments),
					RequiresConfirmation: true,
					RiskReasons:          []string{"命令类启动项会启动本机进程", "请确认目标、参数和工作目录可信"},
				}
			}
		}
		if err := s.runCommand(commandRunRequest{Command: command, Arguments: arguments, WorkingDir: workingDir, Wait: payloadBool(action.Payload, "waitForExit")}); err != nil {
			return contracts.ActionResult{OK: false, Message: err.Error()}
		}
		if payloadBool(action.Payload, "quitAfterStart") {
			s.requestApplicationQuit()
		}
		return contracts.ActionResult{OK: true, Message: actionSuccess(action, "命令已启动："+commandLabel(command, arguments))}
	}
	return contracts.ActionResult{OK: true, Message: action.Label + " 已发送"}
}

func actionConfirmationLabel(action contracts.PreviewAction, command string, arguments []string) string {
	if label := payloadString(action.Payload, "confirmationLabel"); label != "" {
		return label
	}
	if strings.TrimSpace(action.Label) != "" {
		return action.Label
	}
	return commandLabel(command, arguments)
}

func (s *Service) executeSystemCommandAction(action contracts.PreviewAction) contracts.ActionResult {
	command := payloadString(action.Payload, "command")
	definition, ok := systemCommandDefinition(command)
	if !ok {
		return contracts.ActionResult{OK: false, Message: "不支持的系统命令: " + command}
	}
	if !payloadBool(action.Payload, "confirmed") && !payloadBool(action.Payload, "confirm") {
		return contracts.ActionResult{
			OK:                   false,
			Message:              "再次点击确认执行：" + definition.Label,
			RequiresConfirmation: true,
			RiskReasons:          definition.RiskReasons,
		}
	}
	if err := s.runCommand(commandRunRequest{Command: definition.Command, Arguments: definition.Arguments}); err != nil {
		return contracts.ActionResult{OK: false, Message: err.Error()}
	}
	return contracts.ActionResult{OK: true, Message: actionSuccess(action, definition.SuccessLabel)}
}

func (s *Service) runCommand(request commandRunRequest) error {
	runner := s.commandRunner
	if runner == nil {
		runner = defaultCommandRunner
	}
	if err := runner(request); err != nil {
		return commandRunError(request, err)
	}
	return nil
}

func (s *Service) runElevated(file string, args []string) error {
	runner := s.elevatedRunner
	if runner == nil {
		runner = elevation.RunasWait
	}
	return runner(file, args)
}

func (s *Service) requestApplicationQuit() {
	if s.applicationQuit == nil {
		return
	}
	go func() {
		time.Sleep(250 * time.Millisecond)
		s.applicationQuit()
	}()
}

func (s *Service) performLegacyHandoff(request LegacyHandoffRequest, before LegacyRuntimeStatus) legacyHandoffOutcome {
	if s.legacyHandoff != nil {
		return s.legacyHandoff(request, before)
	}
	return closeLegacyProcess(request, before)
}

func runtimeDiagnostics() RuntimeDiagnostics {
	workingDir, _ := os.Getwd()
	executablePath, _ := os.Executable()
	executableBytes := int64(0)
	if info, err := os.Stat(executablePath); err == nil {
		executableBytes = info.Size()
	}
	return RuntimeDiagnostics{
		OS:              runtime.GOOS,
		Arch:            runtime.GOARCH,
		GoVersion:       runtime.Version(),
		ProcessID:       os.Getpid(),
		WorkingDir:      workingDir,
		ExecutablePath:  executablePath,
		ExecutableBytes: executableBytes,
		AppDataEnv:      os.Getenv("APPDATA"),
		LocalAppDataEnv: os.Getenv("LOCALAPPDATA"),
		GoToolPath:      findTool("go"),
		WailsToolPath:   findTool("wails3"),
	}
}

func runtimeMetrics(diagnostics RuntimeDiagnostics, searchPerformance SearchPerformanceStatus, fileSearch FileSearchStatus, logs LogStatus) []RuntimeMetric {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	metrics := []RuntimeMetric{
		{ID: "go_heap_alloc", Label: "Go heap alloc", Value: int64(memory.HeapAlloc), Unit: "bytes"},
		{ID: "go_sys", Label: "Go runtime sys", Value: int64(memory.Sys), Unit: "bytes"},
		{ID: "executable_size", Label: "Executable size", Value: diagnostics.ExecutableBytes, Unit: "bytes"},
	}
	if searchPerformance.SampleCount > 0 {
		metrics = append(metrics,
			RuntimeMetric{ID: "search_p95", Label: "Search p95", Value: searchPerformance.P95Ms, Unit: "ms"},
			RuntimeMetric{ID: "search_average", Label: "Search average", Value: searchPerformance.AverageMs, Unit: "ms"},
			RuntimeMetric{ID: "search_last", Label: "Search last", Value: searchPerformance.LastElapsedMs, Unit: "ms"},
		)
	}
	if fileSearch.LastUpdatedAt > 0 {
		metrics = append(metrics, RuntimeMetric{ID: "file_index_last", Label: "File index last query", Value: fileSearch.LastElapsedMs, Unit: "ms"})
	}
	if logs.Exists {
		metrics = append(metrics, RuntimeMetric{ID: "log_file_size", Label: "Log file size", Value: logs.Bytes, Unit: "bytes"})
	}
	return metrics
}

func (s *Service) currentShellStatus() ShellStatus {
	if s.shellStatus == nil {
		return ShellStatus{}
	}
	return s.shellStatus()
}

func (s *Service) currentLegacyRuntime(shell ShellStatus, diagnostics RuntimeDiagnostics) LegacyRuntimeStatus {
	if s.legacyRuntime != nil {
		return s.legacyRuntime(shell, diagnostics)
	}
	return detectLegacyRuntime(shell, diagnostics)
}

func (s *Service) currentSearchPerformance() SearchPerformanceStatus {
	if s.searchPerformance == nil {
		return SearchPerformanceStatus{TargetP95Ms: 100, WithinTarget: true}
	}
	status := s.searchPerformance()
	if status.TargetP95Ms == 0 {
		status.TargetP95Ms = 100
	}
	if status.SampleCount == 0 {
		status.WithinTarget = true
	}
	return status
}

func (s *Service) currentFileSearchStatus(diagnostics RuntimeDiagnostics) FileSearchStatus {
	if s.fileSearchStatus != nil {
		status := s.fileSearchStatus()
		status.Provider = firstNonEmpty(status.Provider, "Ariadne USN/MFT")
		return normalizeFileSearchStatus(s.currentFileSearchServiceStatus(status))
	}
	return normalizeFileSearchStatus(s.currentFileSearchServiceStatus(FileSearchStatus{
		Provider: "Ariadne USN/MFT",
	}))
}

func (s *Service) currentFileSearchServiceStatus(status FileSearchStatus) FileSearchStatus {
	if s.serviceFileSearch != nil {
		return s.serviceFileSearch(status)
	}
	return enrichFileSearchServiceStatus(status)
}

func normalizeFileSearchStatus(status FileSearchStatus) FileSearchStatus {
	if status.ServiceRunning && isSearchServiceStateMessage(status.CoverageHint) {
		status.CoverageHint = ""
	}
	return status
}

func (s *Service) currentLogStatus(diagnostics RuntimeDiagnostics) LogStatus {
	if s.logStatus != nil {
		status := s.logStatus()
		if status.Directory == "" && status.Path != "" {
			status.Directory = filepath.Dir(status.Path)
		}
		return status
	}
	path := defaultLogPath(diagnostics)
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

func detectLegacyRuntime(shell ShellStatus, diagnostics RuntimeDiagnostics) LegacyRuntimeStatus {
	process := findLegacyProcess()
	status := LegacyRuntimeStatus{
		ProcessRunning: process.Running,
		ProcessID:      process.ID,
		ProcessName:    process.Name,
		ProcessPath:    process.Path,
		ConfigPath:     legacyConfigPath(diagnostics),
	}
	if info, err := os.Stat(status.ConfigPath); err == nil && !info.IsDir() {
		status.ConfigExists = true
		status.ConfigBytes = info.Size()
	}
	status.HotkeyConflictLikely = legacyHotkeyConflictLikely(shell, status.ProcessRunning)
	status.Notes = legacyRuntimeNotes(status, shell)
	return status
}

func legacyHotkeyConflictLikely(shell ShellStatus, legacyProcessRunning bool) bool {
	if shell.GlobalHotkeyRegistered {
		return false
	}
	lastError := strings.ToLower(shell.LastError)
	if lastError == "" {
		return false
	}
	if strings.Contains(lastError, "1409") || strings.Contains(lastError, "already registered") || strings.Contains(lastError, "hotkey is already registered") {
		return true
	}
	return legacyProcessRunning && strings.Contains(lastError, "hotkey")
}

func legacyRuntimeNotes(status LegacyRuntimeStatus, shell ShellStatus) []string {
	notes := []string{}
	if status.ProcessRunning {
		processLabel := firstNonEmpty(status.ProcessName, "x-tools.exe")
		if status.ProcessID > 0 {
			processLabel += " pid " + intString(status.ProcessID)
		}
		notes = append(notes, "旧版进程正在运行："+processLabel)
	}
	if status.ConfigExists {
		notes = append(notes, "检测到旧版配置："+status.ConfigPath)
	}
	if status.HotkeyConflictLikely {
		notes = append(notes, "Alt+Q 可能已被旧版或其他进程占用："+shell.LastError)
	}
	if len(notes) == 0 {
		notes = append(notes, "未发现旧版运行时冲突。")
	}
	return notes
}

func legacyCoexistenceNote(status LegacyRuntimeStatus) string {
	if status.HotkeyConflictLikely {
		return "旧版运行时可能占用 Alt+Q；Ariadne 会在平台诊断中暴露该冲突。"
	}
	if status.ProcessRunning {
		return "检测到旧版 x-tools 正在运行；发布迁移前不应与 Ariadne 长期并行抢占快捷键。"
	}
	if status.ConfigExists {
		return "检测到旧版配置，可在设置中心执行安全导入；未发现旧版进程冲突。"
	}
	return "未发现旧版 x-tools 运行时冲突。"
}

func shellSingleInstanceNote(status ShellStatus) string {
	if status.SingleInstanceConfigured {
		return "Wails SingleInstance 已接入；二次启动会唤起现有 Ariadne 窗口。"
	}
	return "尚未在当前运行态配置单例运行。"
}

func shellHotkeyNote(status ShellStatus) string {
	if status.GlobalHotkeyRegistered {
		note := "已注册 " + firstNonEmpty(status.GlobalHotkey, "Alt+Q") + "，用于唤起纯搜索启动器。"
		if status.ScreenshotHotkeyRegistered {
			note += " 截图热键 " + firstNonEmpty(status.ScreenshotHotkey, "Alt+A") + " 已接管。"
		}
		if status.PinClipboardHotkeyRegistered {
			note += " 贴图热键 " + firstNonEmpty(status.PinClipboardHotkey, "Alt+V") + " 已接管。"
		}
		return note
	}
	if status.LastError != "" {
		return "全局热键未注册：" + status.LastError
	}
	return "全局热键尚未在当前运行态注册。"
}

func shellTrayNote(status ShellStatus) string {
	if status.TrayConfigured {
		return "托盘菜单已接入启动器、工作记忆、剪贴板、截图、Hosts、网络监控、JSON 对比、工作流、设置和退出入口。"
	}
	return "托盘菜单尚未在当前运行态创建。"
}

func shellAutostartNote(status ShellStatus) string {
	if !status.AutostartSupported {
		if status.LastError != "" {
			return "开机启动状态不可用：" + status.LastError
		}
		return "开机启动尚未接入当前运行态。"
	}
	if status.AutostartEnabled {
		if status.AutostartCommandValid {
			return "开机启动已注册并验证隐藏启动参数：" + status.AutostartPath
		}
		if len(status.AutostartNotes) > 0 {
			return "开机启动已注册但需检查：" + strings.Join(status.AutostartNotes, "；")
		}
		return "开机启动已注册：" + status.AutostartPath
	}
	return "支持开机启动；当前未启用，设置开关会写入用户级自启动项。"
}

func firstNonEmpty(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func fileSearchNote(status FileSearchStatus) string {
	if !status.ServiceInstalled && !status.ServiceRunning {
		if status.LastError != "" && !isPrivilegeFileSearchMessage(status.LastError) {
			return "文件索引最近查询失败：" + status.LastError
		}
		return "搜索服务未安装；安装后会自动维护本机文件索引。"
	}
	if status.ServiceRunning && status.RequiresAdmin && !status.Elevated {
		return "文件索引服务正在运行；Ariadne 可保持普通权限启动。"
	}
	if status.RequiresAdmin && !status.Elevated {
		if status.ServiceInstalled && !status.ServiceRunning {
			return "搜索服务已停止；启动服务后可搜索本机文件。"
		}
		return "搜索服务未安装；安装后会自动维护本机文件索引。"
	}
	if status.Indexing {
		return "文件索引正在建立，完成后会返回本机文件结果。"
	}
	if status.LastError != "" {
		return "文件索引最近查询失败：" + status.LastError
	}
	if strings.TrimSpace(status.CoverageHint) != "" {
		return status.CoverageHint
	}
	if status.LastUpdatedAt > 0 {
		return "文件索引可用；最近查询 " + intString(int(status.LastElapsedMs)) + "ms，返回 " + intString(status.LastResultCount) + " 项。"
	}
	if status.Ready {
		return "文件索引已就绪，已索引 " + intString(status.IndexedCount) + " 项。"
	}
	return "文件索引尚未建立；首次文件搜索会触发后台索引。"
}

func isPrivilegeFileSearchMessage(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(message, "管理员权限") || strings.Contains(message, "权限不足") || strings.Contains(lower, "access is denied")
}

func isSearchServiceStateMessage(message string) bool {
	return strings.Contains(message, "搜索服务未运行") || strings.Contains(message, "搜索服务未安装")
}

func searchPerformanceNote(status SearchPerformanceStatus) string {
	if status.SampleCount == 0 {
		return "已接入滚动搜索耗时统计；等待真实查询样本。目标 p95 小于 " + intString(int(status.TargetP95Ms)) + "ms。"
	}
	state := "达标"
	if !status.WithinTarget {
		state = "超过目标"
	}
	return "最近 " + intString(status.SampleCount) + " 次查询 p95 " + intString(int(status.P95Ms)) + "ms，平均 " + intString(int(status.AverageMs)) + "ms，目标 " + intString(int(status.TargetP95Ms)) + "ms：" + state
}

func diagnosticLogNote(status LogStatus) string {
	if status.LastError != "" {
		return "日志写入异常：" + status.LastError
	}
	if status.Exists {
		return "本地运行日志已接入：" + status.Path
	}
	if status.DirectoryExists {
		return "日志目录已创建，等待运行事件写入：" + status.Path
	}
	return "日志目录尚未创建：" + status.Directory
}

func normalizeLegacyHandoffRequest(request LegacyHandoffRequest) LegacyHandoffRequest {
	if request.TimeoutMs <= 0 {
		request.TimeoutMs = 3000
	}
	if request.TimeoutMs < 500 {
		request.TimeoutMs = 500
	}
	if request.TimeoutMs > 10000 {
		request.TimeoutMs = 10000
	}
	return request
}

func findTool(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return path
}

func defaultLogPath(diagnostics RuntimeDiagnostics) string {
	base := diagnostics.AppDataEnv
	if base == "" {
		base = os.Getenv("APPDATA")
	}
	if base == "" {
		base = diagnostics.WorkingDir
	}
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "Ariadne", "logs", "ariadne.log")
}

func diagnosticsExportDir(diagnostics RuntimeDiagnostics) string {
	base := diagnostics.AppDataEnv
	if base == "" {
		base = os.Getenv("APPDATA")
	}
	if base == "" {
		base = diagnostics.WorkingDir
	}
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "Ariadne", "diagnostics")
}

func legacyConfigPath(diagnostics RuntimeDiagnostics) string {
	base := diagnostics.AppDataEnv
	if base == "" {
		base = os.Getenv("APPDATA")
	}
	if base == "" {
		return filepath.Join(".", "x-tools", "config.json")
	}
	return filepath.Join(base, "x-tools", "config.json")
}

func diagnosticsReadme(createdAt time.Time) string {
	return fmt.Sprintf(`# Ariadne Diagnostics

Created at: %s

This package contains local Ariadne runtime diagnostics, metrics, and the local log file when available. It does not include work memory exports, screenshots, clipboard image bodies, or legacy x-tools data.
`, createdAt.Format(time.RFC3339))
}

func writeZipBytes(archive *zip.Writer, name string, data []byte) error {
	writer, err := archive.Create(name)
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}

func writeZipFile(archive *zip.Writer, name string, path string) error {
	source, err := os.Open(path)
	if err != nil {
		return err
	}
	defer source.Close()
	writer, err := archive.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, source)
	return err
}

func intString(value int) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	negative := value < 0
	if negative {
		value = -value
	}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	if negative {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

func payloadString(payload map[string]interface{}, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func payloadBool(payload map[string]interface{}, key string) bool {
	if payload == nil {
		return false
	}
	value, ok := payload[key]
	if !ok {
		return false
	}
	boolValue, ok := value.(bool)
	return ok && boolValue
}

func payloadArguments(payload map[string]interface{}, key string) ([]string, error) {
	if payload == nil {
		return nil, nil
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return nil, nil
	}
	switch typed := value.(type) {
	case string:
		return splitCommandArguments(typed)
	case []string:
		return cleanArguments(typed), nil
	case []interface{}:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("命令参数格式无效")
			}
			items = append(items, text)
		}
		return cleanArguments(items), nil
	default:
		return nil, fmt.Errorf("命令参数格式无效")
	}
}

func splitCommandArguments(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	args := []string{}
	var builder strings.Builder
	var quote rune
	runes := []rune(value)
	for index := 0; index < len(runes); index++ {
		char := runes[index]
		if char == '\\' {
			if index+1 < len(runes) && (runes[index+1] == '\\' || runes[index+1] == quote || runes[index+1] == '"' || runes[index+1] == '\'') {
				builder.WriteRune(runes[index+1])
				index++
			} else {
				builder.WriteRune(char)
			}
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
			} else {
				builder.WriteRune(char)
			}
			continue
		}
		if char == '"' || char == '\'' {
			quote = char
			continue
		}
		if char == ' ' || char == '\t' || char == '\r' || char == '\n' {
			if builder.Len() > 0 {
				args = append(args, builder.String())
				builder.Reset()
			}
			continue
		}
		builder.WriteRune(char)
	}
	if quote != 0 {
		return nil, fmt.Errorf("命令参数引号未闭合")
	}
	if builder.Len() > 0 {
		args = append(args, builder.String())
	}
	return args, nil
}

func cleanArguments(values []string) []string {
	args := make([]string, 0, len(values))
	for _, value := range values {
		text := strings.TrimSpace(value)
		if text != "" {
			args = append(args, text)
		}
	}
	return args
}

func actionSuccess(action contracts.PreviewAction, fallback string) string {
	if action.Feedback != nil && action.Feedback.SuccessLabel != "" {
		return action.Feedback.SuccessLabel
	}
	return fallback
}

func systemCommandDefinition(command string) (systemCommand, bool) {
	switch strings.ToLower(strings.TrimSpace(command)) {
	case "lock":
		return systemCommand{
			Label:        "锁定工作站",
			Command:      "rundll32.exe",
			Arguments:    []string{"user32.dll,LockWorkStation"},
			SuccessLabel: "已锁定工作站",
			RiskReasons:  []string{"会立即锁定当前 Windows 会话", "需要用户重新解锁后继续工作"},
		}, true
	case "sleep":
		return systemCommand{
			Label:        "进入睡眠模式",
			Command:      "rundll32.exe",
			Arguments:    []string{"powrprof.dll,SetSuspendState", "0,1,0"},
			SuccessLabel: "已请求进入睡眠",
			RiskReasons:  []string{"会让当前设备进入睡眠状态", "未保存的前台任务可能被中断"},
		}, true
	case "empty":
		return systemCommand{
			Label:        "清空回收站",
			Command:      "powershell.exe",
			Arguments:    []string{"-NoProfile", "-Command", "Clear-RecycleBin -Force -ErrorAction Stop"},
			SuccessLabel: "已清空回收站",
			RiskReasons:  []string{"会删除回收站中的文件", "该操作可能无法从 Ariadne 内撤销"},
		}, true
	case "shutdown":
		return systemCommand{
			Label:        "关闭系统",
			Command:      "shutdown.exe",
			Arguments:    []string{"/s", "/t", "0"},
			SuccessLabel: "已请求关机",
			RiskReasons:  []string{"会立即关闭系统", "未保存的工作可能丢失"},
		}, true
	case "restart":
		return systemCommand{
			Label:        "重启系统",
			Command:      "shutdown.exe",
			Arguments:    []string{"/r", "/t", "0"},
			SuccessLabel: "已请求重启",
			RiskReasons:  []string{"会立即重启系统", "未保存的工作可能丢失"},
		}, true
	default:
		return systemCommand{}, false
	}
}

func defaultCommandRunner(request commandRunRequest) error {
	command := strings.TrimSpace(request.Command)
	if command == "" {
		return fmt.Errorf("缺少命令")
	}
	if request.WorkingDir != "" {
		info, err := os.Stat(request.WorkingDir)
		if err != nil {
			return fmt.Errorf("工作目录不可用 %q: %w", request.WorkingDir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("工作目录不是文件夹 %q", request.WorkingDir)
		}
	}
	cmd := exec.Command(command, request.Arguments...)
	if request.WorkingDir != "" {
		cmd.Dir = request.WorkingDir
	}
	if request.Wait {
		output, err := cmd.CombinedOutput()
		if err != nil && len(output) > 0 {
			return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
		}
		return err
	}
	return cmd.Start()
}

func commandRunError(request commandRunRequest, err error) error {
	parts := []string{commandLabel(request.Command, request.Arguments)}
	if request.WorkingDir != "" {
		parts = append(parts, "工作目录 "+request.WorkingDir)
	}
	return fmt.Errorf("启动命令失败: %s: %w", strings.Join(parts, " · "), err)
}

func commandLabel(command string, args []string) string {
	parts := []string{strings.TrimSpace(command)}
	parts = append(parts, args...)
	return strings.TrimSpace(strings.Join(parts, " "))
}

func openWithShell(path string) error {
	return exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", path).Start()
}

func revealInExplorer(path string) error {
	return exec.Command("explorer.exe", "/select,"+path).Start()
}
