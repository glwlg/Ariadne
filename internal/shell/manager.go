package shell

import (
	"fmt"
	"strings"
	"sync"

	"ariadne/internal/launcherwindow"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const (
	navigateEvent = "ariadne:navigate"
	autostartID   = "com.glwlg.ariadne"
)

type Status struct {
	SingleInstanceConfigured     bool
	TrayConfigured               bool
	GlobalHotkeyRegistered       bool
	GlobalHotkey                 string
	ScreenshotHotkeyRegistered   bool
	ScreenshotHotkey             string
	PinClipboardHotkeyRegistered bool
	PinClipboardHotkey           string
	AutostartSupported           bool
	AutostartEnabled             bool
	AutostartPath                string
	AutostartIdentifier          string
	AutostartValueName           string
	AutostartCommand             string
	AutostartCommandValid        bool
	AutostartHiddenArgPresent    bool
	AutostartNotes               []string
	LastError                    string
}

type Manager struct {
	mu                           sync.RWMutex
	app                          *application.App
	window                       application.Window
	tray                         *application.SystemTray
	toggleHotkey                 *HotkeyRegistration
	screenshotHotkey             *HotkeyRegistration
	pinClipboardHotkey           *HotkeyRegistration
	toggleHotkeyText             string
	screenshotHotkeyText         string
	pinClipboardHotkeyText       string
	singleInstanceConfigured     bool
	trayConfigured               bool
	globalHotkeyRegistered       bool
	screenshotHotkeyRegistered   bool
	pinClipboardHotkeyRegistered bool
	quitting                     bool
	lastError                    string
	toolOpener                   func(string) bool
	screenshotOpener             func() bool
	pinClipboardOpener           func() bool
}

func NewManager(toggleHotkeyText string, screenshotHotkeyText string, pinClipboardHotkeyText string, toolOpener func(string) bool, screenshotOpener func() bool, pinClipboardOpener func() bool) *Manager {
	return &Manager{
		toggleHotkeyText:       normalizeHotkeyText(toggleHotkeyText, "alt+q"),
		screenshotHotkeyText:   normalizeHotkeyText(screenshotHotkeyText, "alt+a"),
		pinClipboardHotkeyText: normalizeHotkeyText(pinClipboardHotkeyText, "alt+v"),
		toolOpener:             toolOpener,
		screenshotOpener:       screenshotOpener,
		pinClipboardOpener:     pinClipboardOpener,
	}
}

func (m *Manager) SingleInstanceOptions() *application.SingleInstanceOptions {
	m.mu.Lock()
	m.singleInstanceConfigured = true
	m.mu.Unlock()

	return &application.SingleInstanceOptions{
		UniqueID:               "com.glwlg.ariadne",
		OnSecondInstanceLaunch: func(application.SecondInstanceData) {},
	}
}

func (m *Manager) Attach(app *application.App, window application.Window, icon []byte) {
	m.mu.Lock()
	m.app = app
	m.window = window
	m.mu.Unlock()

	if window != nil {
		window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
			m.mu.RLock()
			quitting := m.quitting
			m.mu.RUnlock()
			if quitting {
				return
			}
			event.Cancel()
			window.Hide()
		})
	}

	m.configureTray(icon)
	m.registerHotkeys()
}

func (m *Manager) ShowLauncher() {
	m.openView("launcher")
}

func (m *Manager) OpenWorkMemory() {
	m.openView("work-memory")
}

func (m *Manager) OpenSettings() {
	m.openView("settings")
}

func (m *Manager) Stop() error {
	m.mu.Lock()
	toggleHotkey := m.toggleHotkey
	screenshotHotkey := m.screenshotHotkey
	pinClipboardHotkey := m.pinClipboardHotkey
	m.toggleHotkey = nil
	m.screenshotHotkey = nil
	m.pinClipboardHotkey = nil
	m.globalHotkeyRegistered = false
	m.screenshotHotkeyRegistered = false
	m.pinClipboardHotkeyRegistered = false
	m.mu.Unlock()

	return stopHotkeys(toggleHotkey, screenshotHotkey, pinClipboardHotkey)
}

func stopHotkeys(registrations ...*HotkeyRegistration) error {
	var firstErr error
	for _, registration := range registrations {
		if registration == nil {
			continue
		}
		if err := registration.Stop(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *Manager) RetryHotkeyRegistration() Status {
	m.mu.Lock()
	retryToggle := !m.globalHotkeyRegistered
	retryScreenshot := !m.screenshotHotkeyRegistered
	retryPinClipboard := !m.pinClipboardHotkeyRegistered
	if !retryToggle && !retryScreenshot && !retryPinClipboard {
		m.mu.Unlock()
		return m.Status()
	}
	var toggleHotkey *HotkeyRegistration
	var screenshotHotkey *HotkeyRegistration
	var pinClipboardHotkey *HotkeyRegistration
	if retryToggle {
		toggleHotkey = m.toggleHotkey
		m.toggleHotkey = nil
		m.globalHotkeyRegistered = false
	}
	if retryScreenshot {
		screenshotHotkey = m.screenshotHotkey
		m.screenshotHotkey = nil
		m.screenshotHotkeyRegistered = false
	}
	if retryPinClipboard {
		pinClipboardHotkey = m.pinClipboardHotkey
		m.pinClipboardHotkey = nil
		m.pinClipboardHotkeyRegistered = false
	}
	m.mu.Unlock()

	if retryToggle {
		_ = stopHotkeys(toggleHotkey)
		m.registerToggleHotkey()
	}
	if retryScreenshot {
		_ = stopHotkeys(screenshotHotkey)
		m.registerScreenshotHotkey()
	}
	if retryPinClipboard {
		_ = stopHotkeys(pinClipboardHotkey)
		m.registerPinClipboardHotkey()
	}
	return m.Status()
}

func (m *Manager) ApplyHotkeys(toggleHotkeyText string, screenshotHotkeyText string, pinClipboardHotkeyText string) Status {
	toggleHotkeyText = normalizeHotkeyText(toggleHotkeyText, "alt+q")
	screenshotHotkeyText = normalizeHotkeyText(screenshotHotkeyText, "alt+a")
	pinClipboardHotkeyText = normalizeHotkeyText(pinClipboardHotkeyText, "alt+v")

	m.mu.Lock()
	if strings.EqualFold(m.toggleHotkeyText, toggleHotkeyText) && strings.EqualFold(m.screenshotHotkeyText, screenshotHotkeyText) && strings.EqualFold(m.pinClipboardHotkeyText, pinClipboardHotkeyText) {
		m.mu.Unlock()
		return m.Status()
	}
	toggleHotkey := m.toggleHotkey
	screenshotHotkey := m.screenshotHotkey
	pinClipboardHotkey := m.pinClipboardHotkey
	m.toggleHotkey = nil
	m.screenshotHotkey = nil
	m.pinClipboardHotkey = nil
	m.toggleHotkeyText = toggleHotkeyText
	m.screenshotHotkeyText = screenshotHotkeyText
	m.pinClipboardHotkeyText = pinClipboardHotkeyText
	m.globalHotkeyRegistered = false
	m.screenshotHotkeyRegistered = false
	m.pinClipboardHotkeyRegistered = false
	m.mu.Unlock()

	_ = stopHotkeys(toggleHotkey, screenshotHotkey, pinClipboardHotkey)
	m.registerHotkeys()
	return m.Status()
}

func (m *Manager) Quit() {
	m.mu.Lock()
	app := m.app
	m.quitting = true
	m.mu.Unlock()
	_ = m.Stop()
	if app != nil {
		app.Quit()
	}
}

func (m *Manager) ApplyAutostart(enabled bool) error {
	m.mu.RLock()
	app := m.app
	m.mu.RUnlock()
	if app == nil || app.Autostart == nil {
		return nil
	}

	var err error
	if enabled {
		err = app.Autostart.EnableWithOptions(application.AutostartOptions{
			Identifier: autostartID,
			Arguments:  []string{"--hidden"},
		})
	} else {
		err = app.Autostart.Disable()
	}
	if err != nil {
		m.setError(fmt.Sprintf("autostart: %v", err))
	}
	return err
}

func (m *Manager) Status() Status {
	m.mu.RLock()
	status := Status{
		SingleInstanceConfigured:     m.singleInstanceConfigured,
		TrayConfigured:               m.trayConfigured,
		GlobalHotkeyRegistered:       m.globalHotkeyRegistered,
		GlobalHotkey:                 m.toggleHotkeyText,
		ScreenshotHotkeyRegistered:   m.screenshotHotkeyRegistered,
		ScreenshotHotkey:             m.screenshotHotkeyText,
		PinClipboardHotkeyRegistered: m.pinClipboardHotkeyRegistered,
		PinClipboardHotkey:           m.pinClipboardHotkeyText,
		AutostartIdentifier:          autostartID,
		LastError:                    m.lastError,
	}
	app := m.app
	m.mu.RUnlock()

	if app != nil && app.Autostart != nil {
		autostart, err := app.Autostart.Status()
		if err == nil {
			status.AutostartSupported = true
			status.AutostartEnabled = autostart.Enabled
			status.AutostartPath = autostart.Path
			audit := auditAutostartRegistration(autostartID, autostart.Path)
			status.AutostartValueName = audit.ValueName
			status.AutostartCommand = audit.Command
			status.AutostartCommandValid = audit.CommandValid
			status.AutostartHiddenArgPresent = audit.HiddenArgPresent
			status.AutostartNotes = audit.Notes
		} else if status.LastError == "" {
			status.LastError = "autostart status: " + err.Error()
		}
	}
	return status
}

func (m *Manager) configureTray(icon []byte) {
	m.mu.RLock()
	app := m.app
	m.mu.RUnlock()
	if app == nil {
		return
	}

	menu := application.NewMenu()
	menu.Add("打开启动器").OnClick(func(*application.Context) { m.ShowLauncher() })
	menu.Add("心流").OnClick(func(*application.Context) { m.OpenWorkMemory() })
	menu.Add("剪贴板历史").OnClick(func(*application.Context) { m.openView("clipboard") })
	menu.Add("截图历史").OnClick(func(*application.Context) { m.openView("capture") })
	menu.Add("Hosts 管理").OnClick(func(*application.Context) { m.openView("hosts") })
	menu.Add("网络监控").OnClick(func(*application.Context) { m.openView("network-monitor") })
	menu.Add("网速小窗").OnClick(func(*application.Context) { m.openView("network-mini") })
	menu.Add("JSON 对比").OnClick(func(*application.Context) { m.openView("json-compare") })
	menu.Add("工作流").OnClick(func(*application.Context) { m.openView("workflow") })
	menu.Add("设置").OnClick(func(*application.Context) { m.OpenSettings() })
	menu.AddSeparator()
	menu.Add("退出 Ariadne").OnClick(func(*application.Context) { m.Quit() })

	tray := app.SystemTray.New()
	tray.SetMenu(menu)
	tray.SetTooltip("Ariadne")
	if len(icon) > 0 {
		tray.SetIcon(icon)
	}
	tray.OnClick(func() { m.OpenWorkMemory() })
	tray.OnRightClick(func() { tray.OpenMenu() })

	m.mu.Lock()
	m.tray = tray
	m.trayConfigured = true
	m.mu.Unlock()
}

func (m *Manager) registerHotkeys() {
	m.registerToggleHotkey()
	m.registerScreenshotHotkey()
	m.registerPinClipboardHotkey()
}

func (m *Manager) registerToggleHotkey() {
	spec, err := ParseHotkey(m.currentToggleHotkeyText())
	if err != nil {
		m.setError(err.Error())
		return
	}
	registration, err := RegisterGlobalHotkey(spec, func() {
		m.ShowLauncher()
	})
	if err != nil {
		m.mu.Lock()
		m.globalHotkeyRegistered = false
		m.lastError = err.Error()
		m.mu.Unlock()
		return
	}

	m.mu.Lock()
	m.toggleHotkey = registration
	m.globalHotkeyRegistered = true
	if m.screenshotHotkeyRegistered && m.pinClipboardHotkeyRegistered {
		m.lastError = ""
	}
	m.mu.Unlock()
}

func (m *Manager) registerScreenshotHotkey() {
	spec, err := ParseHotkey(m.currentScreenshotHotkeyText())
	if err != nil {
		m.setError(err.Error())
		return
	}
	registration, err := RegisterGlobalHotkey(spec, func() {
		m.openScreenshot()
	})
	if err != nil {
		m.mu.Lock()
		m.screenshotHotkeyRegistered = false
		m.lastError = err.Error()
		m.mu.Unlock()
		return
	}

	m.mu.Lock()
	m.screenshotHotkey = registration
	m.screenshotHotkeyRegistered = true
	if m.globalHotkeyRegistered && m.pinClipboardHotkeyRegistered {
		m.lastError = ""
	}
	m.mu.Unlock()
}

func (m *Manager) registerPinClipboardHotkey() {
	spec, err := ParseHotkey(m.currentPinClipboardHotkeyText())
	if err != nil {
		m.setError(err.Error())
		return
	}
	registration, err := RegisterGlobalHotkey(spec, func() {
		m.openPinClipboard()
	})
	if err != nil {
		m.mu.Lock()
		m.pinClipboardHotkeyRegistered = false
		m.lastError = err.Error()
		m.mu.Unlock()
		return
	}

	m.mu.Lock()
	m.pinClipboardHotkey = registration
	m.pinClipboardHotkeyRegistered = true
	if m.globalHotkeyRegistered && m.screenshotHotkeyRegistered {
		m.lastError = ""
	}
	m.mu.Unlock()
}

func (m *Manager) currentToggleHotkeyText() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.toggleHotkeyText
}

func (m *Manager) currentScreenshotHotkeyText() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.screenshotHotkeyText
}

func (m *Manager) currentPinClipboardHotkeyText() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pinClipboardHotkeyText
}

func (m *Manager) openScreenshot() {
	m.mu.RLock()
	opener := m.screenshotOpener
	m.mu.RUnlock()
	if opener == nil {
		return
	}
	if !opener() {
		m.setError("screenshot overlay did not open")
	}
}

func (m *Manager) openPinClipboard() {
	m.mu.RLock()
	opener := m.pinClipboardOpener
	m.mu.RUnlock()
	if opener == nil {
		return
	}
	if !opener() {
		m.setError("pin clipboard did not open")
	}
}

func (m *Manager) openView(view string) {
	if m.toolOpener != nil && m.toolOpener(view) {
		return
	}
	m.mu.RLock()
	app := m.app
	window := m.window
	m.mu.RUnlock()
	if window == nil {
		return
	}

	window.Restore()
	window.SetAlwaysOnTop(false)
	if view == "launcher" {
		launcherwindow.ApplyCollapsed(window, primaryScreen(app))
	} else if view == "work-memory" {
		window.SetSize(1280, 820)
		window.Center()
	} else {
		width, height := viewSize(view)
		window.SetSize(width, height)
		window.Center()
	}
	window.Show().Focus()
	window.EmitEvent(navigateEvent, view)
	window.ExecJS(fmt.Sprintf("window.dispatchEvent(new CustomEvent(%q, { detail: %q }));", navigateEvent, view))
}

func (m *Manager) setError(message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastError = message
}

func normalizeHotkeyText(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func viewSize(view string) (int, int) {
	switch view {
	case "launcher":
		return launcherwindow.Width, launcherwindow.CollapsedHeight
	case "json-compare":
		return 1180, 760
	case "network-monitor":
		return 980, 640
	case "network-mini":
		return 318, 168
	default:
		return 1120, 720
	}
}

func primaryScreen(app *application.App) *application.Screen {
	if app == nil {
		return nil
	}
	return app.Screen.GetPrimary()
}
