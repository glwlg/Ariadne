package toolwindows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ariadne/internal/launcherwindow"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	launcherWindowName       = "tool-launcher"
	networkMiniView          = "network-mini"
	networkMiniDefaultAnchor = "taskbar-left"
	networkMiniWidth         = 156
	networkMiniHeight        = 40
	networkMiniMargin        = 6
	networkMiniFillRatio     = 82
	networkMiniLayerRefresh  = 30 * time.Second
)

type OpenResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	View    string `json:"view,omitempty"`
}

type NetworkMiniStatus struct {
	Anchor             string                    `json:"anchor"`
	ScreenMode         string                    `json:"screenMode"`
	ScreenID           string                    `json:"screenId,omitempty"`
	ActiveScreenID     string                    `json:"activeScreenId,omitempty"`
	ScreenName         string                    `json:"screenName,omitempty"`
	ScreenLabel        string                    `json:"screenLabel,omitempty"`
	ScreenCount        int                       `json:"screenCount"`
	Screens            []NetworkMiniScreenStatus `json:"screens,omitempty"`
	AutoHideFullscreen bool                      `json:"autoHideFullscreen"`
	FullscreenActive   bool                      `json:"fullscreenActive"`
	AutoHidden         bool                      `json:"autoHidden"`
	Visible            bool                      `json:"visible"`
	Locked             bool                      `json:"locked"`
	ConfigPath         string                    `json:"configPath,omitempty"`
	LastError          string                    `json:"lastError,omitempty"`
}

type NetworkMiniScreenStatus struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Primary    bool   `json:"primary"`
	X          int    `json:"x"`
	Y          int    `json:"y"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	WorkX      int    `json:"workX"`
	WorkY      int    `json:"workY"`
	WorkWidth  int    `json:"workWidth"`
	WorkHeight int    `json:"workHeight"`
}

type networkMiniConfig struct {
	Anchor             string `json:"anchor"`
	ScreenMode         string `json:"screenMode,omitempty"`
	ScreenID           string `json:"screenId,omitempty"`
	AutoHideFullscreen bool   `json:"autoHideFullscreen"`
	Visible            bool   `json:"visible,omitempty"`
}

type FullscreenDetector func() (bool, error)

type Service struct {
	mu                 sync.RWMutex
	app                *application.App
	windowIcon         []byte
	networkMiniPath    string
	networkMiniConfig  networkMiniConfig
	fullscreenDetector FullscreenDetector
	fullscreenActive   bool
	networkMiniHidden  bool
	networkMiniError   string
	monitorStop        chan struct{}
	networkMiniLayerAt time.Time
}

func NewService() *Service {
	return NewServiceWithOptions(defaultNetworkMiniConfigPath(), defaultFullscreenDetector)
}

func NewServiceWithOptions(networkMiniPath string, detector FullscreenDetector) *Service {
	if detector == nil {
		detector = func() (bool, error) { return false, nil }
	}
	service := &Service{
		networkMiniPath:    networkMiniPath,
		networkMiniConfig:  defaultNetworkMiniConfig(),
		fullscreenDetector: detector,
	}
	service.loadNetworkMiniConfig()
	return service
}

func (s *Service) Attach(app *application.App) {
	s.mu.Lock()
	s.app = app
	s.startNetworkMiniMonitorLocked()
	restoreNetworkMini := s.networkMiniConfig.Visible
	s.mu.Unlock()
	if restoreNetworkMini {
		s.restoreVisibleNetworkMiniAfterStartup()
	}
}

func (s *Service) SetWindowIcon(icon []byte) {
	s.mu.Lock()
	if len(icon) == 0 {
		s.windowIcon = nil
	} else {
		s.windowIcon = append([]byte(nil), icon...)
	}
	s.mu.Unlock()
}

func (s *Service) Open(view string) OpenResult {
	view = normalizeView(view)
	if view == "" {
		return OpenResult{OK: false, Message: "未知工具窗口"}
	}
	if view == "work-memory" {
		if err := s.showMainWorkMemory(); err != nil {
			return OpenResult{OK: false, Message: err.Error(), View: view}
		}
		return OpenResult{OK: true, Message: "心流已打开", View: view}
	}
	if err := s.open(view); err != nil {
		return OpenResult{OK: false, Message: err.Error(), View: view}
	}
	return OpenResult{OK: true, Message: toolTitle(view) + " 已打开", View: view}
}

func (s *Service) ShowLauncher() OpenResult {
	s.mu.RLock()
	app := s.app
	s.mu.RUnlock()
	if app == nil {
		return OpenResult{OK: false, Message: "Ariadne 窗口服务尚未就绪", View: "launcher"}
	}
	if existing, ok := app.Window.Get(launcherWindowName); ok {
		existing.Restore()
		existing.SetAlwaysOnTop(false)
		launcherwindow.ApplyCollapsed(existing, app.Screen.GetPrimary())
		existing.Show().Focus()
		focusLauncher(existing)
		return OpenResult{OK: true, Message: "启动器已打开", View: "launcher"}
	}

	launcherPosition, launcherX, launcherY, launcherScreen := launcherwindow.InitialPlacement(app.Screen.GetPrimary())
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             launcherWindowName,
		Title:            "Ariadne",
		URL:              "/?view=launcher",
		Width:            launcherwindow.Width,
		Height:           launcherwindow.CollapsedHeight,
		X:                launcherX,
		Y:                launcherY,
		AlwaysOnTop:      false,
		Frameless:        true,
		DisableResize:    false,
		BackgroundType:   application.BackgroundTypeTransparent,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
		InitialPosition:  launcherPosition,
		Screen:           launcherScreen,
		Windows: application.WindowsWindow{
			Theme:                             application.Light,
			DisableIcon:                       true,
			DisableFramelessWindowDecorations: true,
			HiddenOnTaskbar:                   true,
		},
	})
	launcherwindow.ApplyCollapsed(window, app.Screen.GetPrimary())
	window.Show().Focus()
	focusLauncher(window)
	return OpenResult{OK: true, Message: "启动器已打开", View: "launcher"}
}

func (s *Service) OpenFromShell(view string) bool {
	if strings.EqualFold(strings.TrimSpace(view), "launcher") {
		return s.ShowLauncher().OK
	}
	return s.Open(view).OK
}

func (s *Service) ApplyMainWindowPolicy() OpenResult {
	s.mu.RLock()
	app := s.app
	s.mu.RUnlock()
	if app == nil {
		return OpenResult{OK: false, Message: "Ariadne 窗口服务尚未就绪", View: "work-memory"}
	}
	main, ok := app.Window.Get("main")
	if !ok {
		return OpenResult{OK: false, Message: "心流主窗口不存在", View: "work-memory"}
	}
	s.applyOrdinaryWindowPolicy(main)
	return OpenResult{OK: true, Message: "心流主窗口策略已应用", View: "work-memory"}
}

func (s *Service) EnableTaskbarToggle(ctx context.Context, view string) OpenResult {
	view = normalizeView(view)
	if !ordinaryTaskbarToggleAllowed(view) {
		return OpenResult{OK: false, Message: "该窗口不使用任务栏最小化样式", View: view}
	}

	var window application.Window
	if ctx != nil {
		window, _ = ctx.Value(application.WindowKey).(application.Window)
	}
	if window == nil {
		s.mu.RLock()
		app := s.app
		s.mu.RUnlock()
		if app != nil {
			if existing, ok := app.Window.Get("tool-" + view); ok {
				window = existing
			} else if existing, ok := app.Window.Get("main"); ok {
				window = existing
			}
		}
	}
	if window == nil {
		return OpenResult{OK: false, Message: "当前窗口不可用", View: view}
	}
	s.applyOrdinaryWindowPolicy(window)
	return OpenResult{OK: true, Message: "窗口任务栏最小化样式已启用", View: view}
}

func (s *Service) NetworkMiniStatus() NetworkMiniStatus {
	s.mu.RLock()
	status := s.networkMiniStatusLocked()
	app := s.app
	config := s.networkMiniConfig
	s.mu.RUnlock()
	enrichNetworkMiniStatus(&status, app, config)
	return status
}

func (s *Service) SetNetworkMiniAnchor(anchor string) NetworkMiniStatus {
	anchor = normalizeNetworkMiniAnchor(anchor)
	s.mu.Lock()
	if anchor == "" {
		s.networkMiniError = "未知小窗贴边位置"
		s.mu.Unlock()
		return s.NetworkMiniStatus()
	}
	s.networkMiniConfig.Anchor = anchor
	s.networkMiniError = ""
	if err := s.saveNetworkMiniConfigLocked(); err != nil {
		s.networkMiniError = err.Error()
	}
	s.mu.Unlock()

	s.applyNetworkMiniPlacementToExisting()
	return s.NetworkMiniStatus()
}

func (s *Service) SetNetworkMiniScreenMode(mode string, screenID string) NetworkMiniStatus {
	mode = normalizeNetworkMiniScreenMode(mode)
	screenID = strings.TrimSpace(screenID)
	s.mu.Lock()
	if mode == "" {
		s.networkMiniError = "未知小窗屏幕模式"
		s.mu.Unlock()
		return s.NetworkMiniStatus()
	}
	if mode == "screen" && screenID == "" {
		s.networkMiniError = "指定屏幕模式需要屏幕 ID"
		s.mu.Unlock()
		return s.NetworkMiniStatus()
	}
	if mode != "screen" {
		screenID = ""
	}
	s.networkMiniConfig.ScreenMode = mode
	s.networkMiniConfig.ScreenID = screenID
	s.networkMiniError = ""
	if err := s.saveNetworkMiniConfigLocked(); err != nil {
		s.networkMiniError = err.Error()
	}
	s.mu.Unlock()

	s.applyNetworkMiniPlacementToExisting()
	return s.NetworkMiniStatus()
}

func (s *Service) SetNetworkMiniAutoHideFullscreen(enabled bool) NetworkMiniStatus {
	s.mu.Lock()
	s.networkMiniConfig.AutoHideFullscreen = enabled
	s.networkMiniError = ""
	if err := s.saveNetworkMiniConfigLocked(); err != nil {
		s.networkMiniError = err.Error()
	}
	s.mu.Unlock()

	if !enabled {
		s.restoreAutoHiddenNetworkMini()
	}
	return s.NetworkMiniStatus()
}

func (s *Service) ResetNetworkMiniPlacement() NetworkMiniStatus {
	s.mu.Lock()
	visible := s.networkMiniConfig.Visible
	s.networkMiniConfig = defaultNetworkMiniConfig()
	s.networkMiniConfig.Visible = visible
	s.networkMiniError = ""
	if err := s.saveNetworkMiniConfigLocked(); err != nil {
		s.networkMiniError = err.Error()
	}
	s.mu.Unlock()

	s.applyNetworkMiniPlacementToExisting()
	return s.NetworkMiniStatus()
}

func (s *Service) Stop() {
	s.mu.Lock()
	stop := s.monitorStop
	s.monitorStop = nil
	s.mu.Unlock()
	if stop != nil {
		close(stop)
	}
}

func (s *Service) open(view string) error {
	s.mu.RLock()
	app := s.app
	s.mu.RUnlock()
	if app == nil {
		return errors.New("Ariadne 工具窗口服务尚未就绪")
	}

	name := "tool-" + view
	if existing, ok := app.Window.Get(name); ok {
		if view == networkMiniView {
			s.applyNetworkMiniPlacement(existing, app)
			existing.SetAlwaysOnTop(true).Show()
			s.markNetworkMiniVisible()
		} else {
			existing.Show().Focus()
			s.applyOrdinaryWindowPolicy(existing)
		}
		return nil
	}

	width, height := toolSize(view)
	config := s.networkMiniConfigSnapshot()
	placementScreen := screenForNetworkMini(app, config)
	if view == networkMiniView && placementScreen == nil {
		return errors.New("网速小窗屏幕信息尚未就绪")
	}
	position, x, y, screen := toolPlacement(view, width, height, placementScreen, config.Anchor)
	if view == networkMiniView {
		frame := networkMiniFrame(screen, config.Anchor, width, height)
		width = frame.Width
		height = frame.Height
		x = frame.X
		y = frame.Y
	}
	background := application.NewRGB(244, 244, 245)
	if view == networkMiniView {
		background = application.NewRGBA(0, 0, 0, 0)
	}
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             name,
		Title:            toolTitle(view),
		URL:              "/?view=" + url.QueryEscape(view),
		Width:            width,
		Height:           height,
		MinWidth:         minWidth(view),
		MinHeight:        minHeight(view),
		MaxWidth:         maxWidth(view),
		MaxHeight:        maxHeight(view),
		AlwaysOnTop:      alwaysOnTop(view),
		Frameless:        frameless(view),
		DisableResize:    disableResize(view),
		BackgroundColour: background,
		InitialPosition:  position,
		X:                x,
		Y:                y,
		Screen:           screen,
		Windows: application.WindowsWindow{
			Theme:                             application.Light,
			DisableIcon:                       disableIcon(view),
			DisableFramelessWindowDecorations: frameless(view),
			HiddenOnTaskbar:                   hiddenOnTaskbar(view),
		},
	})

	if view == networkMiniView {
		s.applyNetworkMiniPlacement(window, app)
	} else {
		s.applyOrdinaryWindowPolicy(window)
	}

	if view == networkMiniView {
		s.markNetworkMiniVisible()
	}
	return nil
}

func (s *Service) showMainWorkMemory() error {
	s.mu.RLock()
	app := s.app
	s.mu.RUnlock()
	if app == nil {
		return errors.New("Ariadne 工具窗口服务尚未就绪")
	}
	main, ok := app.Window.Get("main")
	if !ok {
		return errors.New("心流主窗口不存在")
	}
	main.Restore()
	main.SetAlwaysOnTop(false)
	s.applyOrdinaryWindowPolicy(main)
	main.Show().Focus()
	main.EmitEvent("ariadne:navigate", "work-memory")
	main.ExecJS(`window.dispatchEvent(new CustomEvent("ariadne:navigate", { detail: "work-memory" }));`)
	return nil
}

func focusLauncher(window application.Window) {
	if window == nil {
		return
	}
	window.EmitEvent("ariadne:focus-launcher", map[string]any{"reset": true})
	window.ExecJS(`window.dispatchEvent(new CustomEvent("ariadne:focus-launcher", { detail: { reset: true } }));`)
}

func normalizeView(view string) string {
	switch strings.ToLower(strings.TrimSpace(view)) {
	case "work-memory", "clipboard", "capture", "hosts", "workflow", "json-compare", "network-monitor", "network-mini", "settings":
		return strings.ToLower(strings.TrimSpace(view))
	default:
		return ""
	}
}

func toolTitle(view string) string {
	switch view {
	case "work-memory":
		return "Ariadne - 心流"
	case "clipboard":
		return "Ariadne - 剪贴板历史"
	case "capture":
		return "Ariadne - 截图历史"
	case "hosts":
		return "Ariadne - Hosts"
	case "workflow":
		return "Ariadne - 工作流"
	case "json-compare":
		return "Ariadne - JSON 对比"
	case "network-monitor":
		return "Ariadne - 网络监控"
	case networkMiniView:
		return "Ariadne - 网速小窗"
	case "settings":
		return "Ariadne - 设置"
	default:
		return "Ariadne"
	}
}

func toolSize(view string) (int, int) {
	switch view {
	case "json-compare":
		return 1180, 760
	case "network-monitor":
		return 980, 640
	case networkMiniView:
		return networkMiniWidth, networkMiniHeight
	default:
		return 1120, 720
	}
}

func minWidth(view string) int {
	if view == networkMiniView {
		return networkMiniWidth
	}
	if view == "network-monitor" {
		return 760
	}
	return 900
}

func minHeight(view string) int {
	if view == networkMiniView {
		return networkMiniHeight
	}
	if view == "network-monitor" {
		return 520
	}
	return 620
}

func maxWidth(view string) int {
	if view == networkMiniView {
		return networkMiniWidth
	}
	return 0
}

func maxHeight(view string) int {
	if view == networkMiniView {
		return networkMiniHeight
	}
	return 0
}

func disableResize(view string) bool {
	return view == networkMiniView
}

func disableIcon(view string) bool {
	return view == networkMiniView
}

func hiddenOnTaskbar(view string) bool {
	return view == networkMiniView
}

func frameless(view string) bool {
	return view == networkMiniView
}

func alwaysOnTop(view string) bool {
	return view == networkMiniView
}

func ordinaryTaskbarToggleAllowed(view string) bool {
	view = normalizeView(view)
	return view != "" && view != networkMiniView
}

func (s *Service) applyOrdinaryWindowPolicy(window application.Window) {
	enableOrdinaryWindowTaskbarToggle(window)
	icon := s.windowIconSnapshot()
	setOrdinaryWindowIcon(window, icon)
	refreshOrdinaryWindowIcon(window, icon)
}

func (s *Service) windowIconSnapshot() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.windowIcon) == 0 {
		return nil
	}
	return append([]byte(nil), s.windowIcon...)
}

func refreshOrdinaryWindowIcon(window application.Window, icon []byte) {
	if window == nil || len(icon) == 0 {
		return
	}
	icon = append([]byte(nil), icon...)
	go func() {
		for _, delay := range []time.Duration{80 * time.Millisecond, 300 * time.Millisecond, 900 * time.Millisecond} {
			time.Sleep(delay)
			setOrdinaryWindowIcon(window, icon)
		}
	}()
}

func toolPlacement(view string, width int, height int, screen *application.Screen, anchor string) (application.WindowStartPosition, int, int, *application.Screen) {
	if view != networkMiniView || screen == nil {
		return application.WindowCentered, 0, 0, nil
	}
	if normalizeNetworkMiniAnchor(anchor) == "taskbar-left" {
		frame := networkMiniTaskbarFrame(screen, width, height)
		return application.WindowXY, frame.X, frame.Y, screen
	}
	x, y := networkMiniAnchorPosition(anchor, screen.WorkArea.Width, screen.WorkArea.Height, width, height)
	return application.WindowXY, x, y, screen
}

func networkMiniAnchorPosition(anchor string, workWidth int, workHeight int, width int, height int) (int, int) {
	right := workWidth - width - networkMiniMargin
	bottom := workHeight - height - networkMiniMargin
	if right < networkMiniMargin {
		right = networkMiniMargin
	}
	if bottom < networkMiniMargin {
		bottom = networkMiniMargin
	}
	switch normalizeNetworkMiniAnchor(anchor) {
	case "top-left":
		return networkMiniMargin, networkMiniMargin
	case "top-right":
		return right, networkMiniMargin
	case "bottom-left":
		return networkMiniMargin, bottom
	default:
		return right, bottom
	}
}

type networkMiniWindowFrame struct {
	X      int
	Y      int
	Width  int
	Height int
}

func networkMiniFrame(screen *application.Screen, anchor string, width int, height int) networkMiniWindowFrame {
	if normalizeNetworkMiniAnchor(anchor) == "taskbar-left" {
		return networkMiniTaskbarFrame(screen, width, height)
	}
	x, y := networkMiniAnchorPosition(anchor, screen.WorkArea.Width, screen.WorkArea.Height, width, height)
	return networkMiniWindowFrame{X: x, Y: y, Width: width, Height: height}
}

func networkMiniTaskbarFrame(screen *application.Screen, width int, fallbackHeight int) networkMiniWindowFrame {
	bounds := usableScreenBounds(screen)
	work := usableWorkArea(screen, bounds)
	taskbar := inferNetworkMiniTaskbarRect(bounds, work, fallbackHeight)
	horizontal := taskbar.Width >= taskbar.Height
	thickness := taskbar.Height
	if !horizontal {
		thickness = taskbar.Width
	}
	height := maxInt(24, (thickness*networkMiniFillRatio+50)/100)
	if fallbackHeight > 0 {
		height = minInt(maxInt(height, 24), maxInt(fallbackHeight+8, height))
	}
	if width <= 0 {
		width = networkMiniWidth
	}

	x := taskbar.X + networkMiniMargin
	y := taskbar.Y
	if horizontal {
		y += maxInt(0, (taskbar.Height-height)/2)
	} else {
		x = taskbar.X + maxInt(0, (taskbar.Width-width)/2)
		y += networkMiniMargin
	}
	x = clampInt(x, bounds.X, maxInt(bounds.X, rectRight(bounds)-width))
	y = clampInt(y, bounds.Y, maxInt(bounds.Y, rectBottom(bounds)-height))
	return networkMiniWindowFrame{
		X:      x - work.X,
		Y:      y - work.Y,
		Width:  width,
		Height: height,
	}
}

func usableScreenBounds(screen *application.Screen) application.Rect {
	if screen == nil {
		return application.Rect{Width: networkMiniWidth + networkMiniMargin*2, Height: networkMiniHeight + networkMiniMargin*2}
	}
	if screen.Bounds.Width > 0 && screen.Bounds.Height > 0 {
		return screen.Bounds
	}
	if screen.PhysicalBounds.Width > 0 && screen.PhysicalBounds.Height > 0 {
		return screen.PhysicalBounds
	}
	if screen.WorkArea.Width > 0 && screen.WorkArea.Height > 0 {
		return application.Rect{X: screen.WorkArea.X, Y: screen.WorkArea.Y, Width: screen.WorkArea.Width, Height: screen.WorkArea.Height + networkMiniHeight}
	}
	if screen.Size.Width > 0 && screen.Size.Height > 0 {
		return application.Rect{Width: screen.Size.Width, Height: screen.Size.Height}
	}
	return application.Rect{Width: networkMiniWidth + networkMiniMargin*2, Height: networkMiniHeight + networkMiniMargin*2}
}

func usableWorkArea(screen *application.Screen, bounds application.Rect) application.Rect {
	if screen != nil && screen.WorkArea.Width > 0 && screen.WorkArea.Height > 0 {
		return screen.WorkArea
	}
	return bounds
}

func inferNetworkMiniTaskbarRect(bounds application.Rect, work application.Rect, fallbackHeight int) application.Rect {
	topBand := maxInt(0, work.Y-bounds.Y)
	bottomBand := maxInt(0, rectBottom(bounds)-rectBottom(work))
	leftBand := maxInt(0, work.X-bounds.X)
	rightBand := maxInt(0, rectRight(bounds)-rectRight(work))
	largest := maxInt(maxInt(topBand, bottomBand), maxInt(leftBand, rightBand))
	if largest <= 0 {
		height := minInt(maxInt(fallbackHeight, 1), maxInt(bounds.Height, 1))
		return application.Rect{X: bounds.X, Y: rectBottom(bounds) - height, Width: bounds.Width, Height: height}
	}
	switch largest {
	case bottomBand:
		return application.Rect{X: bounds.X, Y: rectBottom(work), Width: bounds.Width, Height: bottomBand}
	case topBand:
		return application.Rect{X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: topBand}
	case leftBand:
		return application.Rect{X: bounds.X, Y: bounds.Y, Width: leftBand, Height: bounds.Height}
	default:
		return application.Rect{X: rectRight(work), Y: bounds.Y, Width: rightBand, Height: bounds.Height}
	}
}

func rectRight(rect application.Rect) int {
	return rect.X + rect.Width
}

func rectBottom(rect application.Rect) int {
	return rect.Y + rect.Height
}

func clampInt(value int, minValue int, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

func normalizeNetworkMiniAnchor(anchor string) string {
	switch strings.ToLower(strings.TrimSpace(anchor)) {
	case "", "taskbar", "taskbar-left", "任务栏", "任务栏左侧":
		return "taskbar-left"
	case "top-left", "top-right", "bottom-left", "bottom-right":
		return strings.ToLower(strings.TrimSpace(anchor))
	default:
		return ""
	}
}

func defaultNetworkMiniConfig() networkMiniConfig {
	return networkMiniConfig{
		Anchor:             networkMiniDefaultAnchor,
		ScreenMode:         "cursor",
		AutoHideFullscreen: true,
	}
}

func (s *Service) networkMiniConfigSnapshot() networkMiniConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.networkMiniConfig
}

func (s *Service) networkMiniStatusLocked() NetworkMiniStatus {
	anchor := normalizeNetworkMiniAnchor(s.networkMiniConfig.Anchor)
	if anchor == "" {
		anchor = networkMiniDefaultAnchor
	}
	mode := normalizeNetworkMiniScreenMode(s.networkMiniConfig.ScreenMode)
	if mode == "" {
		mode = "cursor"
	}
	return NetworkMiniStatus{
		Anchor:             anchor,
		ScreenMode:         mode,
		ScreenID:           strings.TrimSpace(s.networkMiniConfig.ScreenID),
		AutoHideFullscreen: s.networkMiniConfig.AutoHideFullscreen,
		FullscreenActive:   s.fullscreenActive,
		AutoHidden:         s.networkMiniHidden,
		Visible:            s.networkMiniConfig.Visible,
		Locked:             true,
		ConfigPath:         s.networkMiniPath,
		LastError:          s.networkMiniError,
	}
}

func (s *Service) loadNetworkMiniConfig() {
	if strings.TrimSpace(s.networkMiniPath) == "" {
		return
	}
	raw, err := os.ReadFile(s.networkMiniPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			s.networkMiniError = "读取网速小窗配置失败: " + err.Error()
		}
		return
	}
	var config struct {
		Anchor             string `json:"anchor"`
		ScreenMode         string `json:"screenMode,omitempty"`
		ScreenID           string `json:"screenId,omitempty"`
		AutoHideFullscreen *bool  `json:"autoHideFullscreen"`
		Visible            *bool  `json:"visible"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		s.networkMiniError = "解析网速小窗配置失败: " + err.Error()
		return
	}
	if anchor := normalizeNetworkMiniAnchor(config.Anchor); anchor != "" {
		s.networkMiniConfig.Anchor = anchor
	}
	s.networkMiniConfig.ScreenID = strings.TrimSpace(config.ScreenID)
	if strings.TrimSpace(config.ScreenMode) != "" {
		if mode := normalizeNetworkMiniScreenMode(config.ScreenMode); mode != "" {
			s.networkMiniConfig.ScreenMode = mode
		}
	} else if strings.TrimSpace(config.ScreenID) != "" {
		s.networkMiniConfig.ScreenMode = "screen"
	}
	if config.AutoHideFullscreen != nil {
		s.networkMiniConfig.AutoHideFullscreen = *config.AutoHideFullscreen
	}
	if config.Visible != nil {
		s.networkMiniConfig.Visible = *config.Visible
	}
}

func (s *Service) saveNetworkMiniConfigLocked() error {
	if strings.TrimSpace(s.networkMiniPath) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.networkMiniPath), 0o755); err != nil {
		return fmt.Errorf("创建网速小窗配置目录失败: %w", err)
	}
	raw, err := json.MarshalIndent(s.networkMiniConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("编码网速小窗配置失败: %w", err)
	}
	tmp := s.networkMiniPath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("写入网速小窗配置失败: %w", err)
	}
	if err := os.Rename(tmp, s.networkMiniPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("替换网速小窗配置失败: %w", err)
	}
	return nil
}

func (s *Service) startNetworkMiniMonitorLocked() {
	if s.monitorStop != nil {
		return
	}
	stop := make(chan struct{})
	s.monitorStop = stop
	go s.runNetworkMiniMonitor(stop)
}

func (s *Service) runNetworkMiniMonitor(stop <-chan struct{}) {
	ticker := time.NewTicker(900 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			s.tickNetworkMiniAutoHide()
		}
	}
}

func (s *Service) tickNetworkMiniAutoHide() {
	s.mu.RLock()
	app := s.app
	config := s.networkMiniConfig
	detector := s.fullscreenDetector
	s.mu.RUnlock()

	if app == nil {
		return
	}
	fullscreen, err := detector()
	s.mu.Lock()
	s.fullscreenActive = fullscreen
	if err != nil {
		s.networkMiniError = "检测全屏窗口失败: " + err.Error()
	} else if strings.HasPrefix(s.networkMiniError, "检测全屏窗口失败:") {
		s.networkMiniError = ""
	}
	enabled := config.AutoHideFullscreen
	wasAutoHidden := s.networkMiniHidden
	s.mu.Unlock()

	if err != nil {
		return
	}

	window, ok := app.Window.Get("tool-" + networkMiniView)
	if !ok {
		s.mu.Lock()
		s.networkMiniHidden = false
		s.mu.Unlock()
		return
	}
	if !enabled {
		if wasAutoHidden || !window.IsVisible() {
			s.applyNetworkMiniPlacement(window, app)
			window.SetAlwaysOnTop(true).Show()
		} else if s.shouldRefreshNetworkMiniLayer() {
			refreshNetworkMiniTaskbarLayer(window)
		}
		s.mu.Lock()
		s.networkMiniHidden = false
		s.mu.Unlock()
		return
	}
	if fullscreen {
		if window.IsVisible() {
			window.Hide()
		}
		s.mu.Lock()
		s.networkMiniHidden = true
		s.mu.Unlock()
		return
	}
	if wasAutoHidden {
		s.applyNetworkMiniPlacement(window, app)
		window.SetAlwaysOnTop(true).Show()
		s.mu.Lock()
		s.networkMiniHidden = false
		s.mu.Unlock()
		return
	}
	if window.IsVisible() && s.shouldRefreshNetworkMiniLayer() {
		refreshNetworkMiniTaskbarLayer(window)
	}
}

func (s *Service) restoreAutoHiddenNetworkMini() {
	s.mu.RLock()
	app := s.app
	wasAutoHidden := s.networkMiniHidden
	s.mu.RUnlock()
	if app == nil || !wasAutoHidden {
		return
	}
	if window, ok := app.Window.Get("tool-" + networkMiniView); ok {
		s.applyNetworkMiniPlacement(window, app)
		window.SetAlwaysOnTop(true).Show()
	}
	s.mu.Lock()
	s.networkMiniHidden = false
	s.mu.Unlock()
}

func (s *Service) applyNetworkMiniPlacementToExisting() {
	s.mu.RLock()
	app := s.app
	s.mu.RUnlock()
	if app == nil {
		return
	}
	if window, ok := app.Window.Get("tool-" + networkMiniView); ok {
		s.applyNetworkMiniPlacement(window, app)
	}
}

func (s *Service) applyNetworkMiniPlacement(window application.Window, app *application.App) {
	if window == nil || app == nil {
		return
	}
	config := s.networkMiniConfigSnapshot()
	screen := screenForNetworkMini(app, config)
	if screen == nil {
		return
	}
	frame := networkMiniFrame(screen, config.Anchor, networkMiniWidth, networkMiniHeight)
	window.SetSize(frame.Width, frame.Height)
	x, y := networkMiniAbsolutePosition(screen, frame)
	window.SetPosition(x, y)
	applyNetworkMiniTaskbarOwner(window)
	s.markNetworkMiniLayerRefreshed()
}

func (s *Service) shouldRefreshNetworkMiniLayer() bool {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.networkMiniLayerAt.IsZero() && now.Sub(s.networkMiniLayerAt) < networkMiniLayerRefresh {
		return false
	}
	s.networkMiniLayerAt = now
	return true
}

func (s *Service) markNetworkMiniLayerRefreshed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.networkMiniLayerAt = time.Now()
}

func (s *Service) restoreVisibleNetworkMiniAfterStartup() {
	go func() {
		var lastErr error
		for attempt := 0; attempt < 12; attempt++ {
			time.Sleep(250 * time.Millisecond)
			s.mu.RLock()
			visible := s.networkMiniConfig.Visible
			s.mu.RUnlock()
			if !visible {
				return
			}
			if err := s.open(networkMiniView); err != nil {
				lastErr = err
				continue
			}
			return
		}
		if lastErr != nil {
			s.mu.Lock()
			s.networkMiniError = "恢复网速小窗失败: " + lastErr.Error()
			s.mu.Unlock()
		}
	}()
}

func networkMiniAbsolutePosition(screen *application.Screen, frame networkMiniWindowFrame) (int, int) {
	bounds := usableScreenBounds(screen)
	work := usableWorkArea(screen, bounds)
	return work.X + frame.X, work.Y + frame.Y
}

func (s *Service) markNetworkMiniVisible() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.networkMiniHidden = false
	if !s.networkMiniConfig.Visible {
		s.networkMiniConfig.Visible = true
		s.networkMiniError = ""
		if err := s.saveNetworkMiniConfigLocked(); err != nil {
			s.networkMiniError = err.Error()
		}
	}
}

func screenForNetworkMini(app *application.App, config networkMiniConfig) *application.Screen {
	if app == nil {
		return nil
	}
	cursor, cursorOK := networkMiniCursorPoint()
	return selectNetworkMiniScreen(config, app.Screen.GetAll(), app.Screen.GetPrimary(), cursor, cursorOK)
}

func defaultNetworkMiniConfigPath() string {
	if dir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(dir) != "" {
		return filepath.Join(dir, "Ariadne", "network_mini_window.json")
	}
	return filepath.Join(".", "network_mini_window.json")
}

func normalizeNetworkMiniScreenMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "cursor", "current":
		return "cursor"
	case "primary":
		return "primary"
	case "screen", "specific":
		return "screen"
	default:
		return ""
	}
}

func selectNetworkMiniScreen(config networkMiniConfig, screens []*application.Screen, primary *application.Screen, cursor application.Point, cursorOK bool) *application.Screen {
	mode := normalizeNetworkMiniScreenMode(config.ScreenMode)
	if mode == "" {
		mode = "cursor"
	}
	if mode == "screen" {
		if screen := screenByID(screens, strings.TrimSpace(config.ScreenID)); screen != nil {
			return screen
		}
	}
	if mode == "cursor" && cursorOK {
		if screen := screenNearestPhysicalPoint(screens, cursor); screen != nil {
			return screen
		}
	}
	if primary != nil {
		return primary
	}
	if len(screens) > 0 {
		return screens[0]
	}
	return nil
}

func screenByID(screens []*application.Screen, id string) *application.Screen {
	if id == "" {
		return nil
	}
	for _, screen := range screens {
		if screen != nil && screen.ID == id {
			return screen
		}
	}
	return nil
}

func screenNearestPhysicalPoint(screens []*application.Screen, point application.Point) *application.Screen {
	var nearest *application.Screen
	bestDistance := 0
	for _, screen := range screens {
		if screen == nil {
			continue
		}
		rect := screen.PhysicalBounds
		if rect.Width <= 0 || rect.Height <= 0 {
			rect = screen.Bounds
		}
		distance := pointDistanceToRectSquared(point, rect)
		if nearest == nil || distance < bestDistance {
			nearest = screen
			bestDistance = distance
		}
		if distance < 0 {
			return screen
		}
	}
	return nearest
}

func pointDistanceToRectSquared(point application.Point, rect application.Rect) int {
	if rect.Width <= 0 || rect.Height <= 0 {
		return 1<<31 - 1
	}
	if point.X >= rect.X && point.X < rect.X+rect.Width && point.Y >= rect.Y && point.Y < rect.Y+rect.Height {
		return -rect.Width * rect.Height
	}
	dx := 0
	if point.X < rect.X {
		dx = rect.X - point.X
	} else if point.X >= rect.X+rect.Width {
		dx = point.X - (rect.X + rect.Width - 1)
	}
	dy := 0
	if point.Y < rect.Y {
		dy = rect.Y - point.Y
	} else if point.Y >= rect.Y+rect.Height {
		dy = point.Y - (rect.Y + rect.Height - 1)
	}
	return dx*dx + dy*dy
}

func enrichNetworkMiniStatus(status *NetworkMiniStatus, app *application.App, config networkMiniConfig) {
	if status == nil || app == nil {
		return
	}
	screens := app.Screen.GetAll()
	primary := app.Screen.GetPrimary()
	cursor, cursorOK := networkMiniCursorPoint()
	selected := selectNetworkMiniScreen(config, screens, primary, cursor, cursorOK)
	status.ScreenCount = len(screens)
	status.Screens = make([]NetworkMiniScreenStatus, 0, len(screens))
	for _, screen := range screens {
		if screen == nil {
			continue
		}
		status.Screens = append(status.Screens, networkMiniScreenStatus(screen))
	}
	if selected != nil {
		status.ActiveScreenID = selected.ID
		status.ScreenName = networkMiniScreenName(selected)
		status.ScreenLabel = networkMiniScreenLabel(status.ScreenMode, selected)
	}
}

func networkMiniScreenStatus(screen *application.Screen) NetworkMiniScreenStatus {
	return NetworkMiniScreenStatus{
		ID:         screen.ID,
		Name:       networkMiniScreenName(screen),
		Primary:    screen.IsPrimary,
		X:          screen.Bounds.X,
		Y:          screen.Bounds.Y,
		Width:      screen.Bounds.Width,
		Height:     screen.Bounds.Height,
		WorkX:      screen.WorkArea.X,
		WorkY:      screen.WorkArea.Y,
		WorkWidth:  screen.WorkArea.Width,
		WorkHeight: screen.WorkArea.Height,
	}
}

func networkMiniScreenName(screen *application.Screen) string {
	if screen == nil {
		return ""
	}
	if name := strings.TrimSpace(screen.Name); name != "" {
		return name
	}
	if screen.IsPrimary {
		return "主屏"
	}
	if strings.TrimSpace(screen.ID) != "" {
		return "屏幕 " + screen.ID
	}
	return "屏幕"
}

func networkMiniScreenLabel(mode string, screen *application.Screen) string {
	name := networkMiniScreenName(screen)
	switch normalizeNetworkMiniScreenMode(mode) {
	case "primary":
		return "主屏 · " + name
	case "screen":
		return "指定屏 · " + name
	default:
		return "跟随当前屏幕 · " + name
	}
}
