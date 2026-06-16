package captureoverlay

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ariadne/internal/capturehistory"
	"ariadne/internal/clipboardhistory"
	"ariadne/internal/pinnedimage"
	"ariadne/internal/qrscan"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type CaptureSink interface {
	AddPNG(data []byte, width int, height int, source string, savedPath string, actions []string) capturehistory.Status
}

type PinService interface {
	OpenCapture(id string) pinnedimage.OpenResult
}

type PositionedPinService interface {
	OpenCaptureAt(id string, x int, y int) pinnedimage.OpenResult
}

type ScreenshotPolicy struct {
	AutoCopy         bool
	AutoPin          bool
	AutoSave         bool
	SaveDir          string
	FilenameTemplate string
}

type OpenResult struct {
	OK        bool                        `json:"ok"`
	Message   string                      `json:"message"`
	SessionID string                      `json:"sessionId,omitempty"`
	Bounds    capturehistory.ScreenBounds `json:"bounds,omitempty"`
	Native    capturehistory.ScreenBounds `json:"nativeBounds,omitempty"`
}

type Session struct {
	ID        string                      `json:"id"`
	Bounds    capturehistory.ScreenBounds `json:"bounds"`
	Native    capturehistory.ScreenBounds `json:"nativeBounds,omitempty"`
	ImageURL  string                      `json:"imageUrl"`
	CreatedAt int64                       `json:"createdAt"`
}

type SelectionRequest struct {
	SessionID       string                `json:"sessionId"`
	X               int                   `json:"x"`
	Y               int                   `json:"y"`
	Width           int                   `json:"width"`
	Height          int                   `json:"height"`
	CoordinateSpace string                `json:"coordinateSpace,omitempty"`
	DisplayWidth    int                   `json:"displayWidth,omitempty"`
	DisplayHeight   int                   `json:"displayHeight,omitempty"`
	Action          string                `json:"action"`
	SavedPath       string                `json:"savedPath,omitempty"`
	PinPositioned   bool                  `json:"pinPositioned,omitempty"`
	PinX            int                   `json:"pinX,omitempty"`
	PinY            int                   `json:"pinY,omitempty"`
	Operations      []AnnotationOperation `json:"operations,omitempty"`
	RenderedImage   string                `json:"renderedImage,omitempty"`
}

type AnnotationPoint struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type AnnotationOperation struct {
	Kind        string            `json:"kind"`
	X           int               `json:"x"`
	Y           int               `json:"y"`
	Width       int               `json:"width,omitempty"`
	Height      int               `json:"height,omitempty"`
	EndX        int               `json:"endX,omitempty"`
	EndY        int               `json:"endY,omitempty"`
	Color       string            `json:"color,omitempty"`
	StrokeWidth int               `json:"strokeWidth,omitempty"`
	PixelSize   int               `json:"pixelSize,omitempty"`
	Points      []AnnotationPoint `json:"points,omitempty"`
	Text        string            `json:"text,omitempty"`
	FontSize    int               `json:"fontSize,omitempty"`
	Number      int               `json:"number,omitempty"`
}

type CaptureResult struct {
	OK        bool                    `json:"ok"`
	Message   string                  `json:"message"`
	CaptureID string                  `json:"captureId,omitempty"`
	ImagePath string                  `json:"imagePath,omitempty"`
	SavedPath string                  `json:"savedPath,omitempty"`
	Width     int                     `json:"width,omitempty"`
	Height    int                     `json:"height,omitempty"`
	QR        *qrscan.Result          `json:"qr,omitempty"`
	Pin       *pinnedimage.OpenResult `json:"pin,omitempty"`
}

type overlaySession struct {
	Session
	pngBytes           []byte
	groupID            string
	windowName         string
	restoreMain        bool
	restoreWindowNames []string
}

var writeImageToClipboard = clipboardhistory.WriteImageToSystemClipboard

type Service struct {
	mu       sync.RWMutex
	app      *application.App
	captures CaptureSink
	pins     PinService
	sessions map[string]overlaySession
	policy   ScreenshotPolicy
}

func NewService(captures CaptureSink, pins PinService) *Service {
	return &Service{
		captures: captures,
		pins:     pins,
		sessions: map[string]overlaySession{},
	}
}

func (s *Service) Attach(app *application.App) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.app = app
}

func (s *Service) ApplyScreenshotPolicy(policy ScreenshotPolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policy = normalizeScreenshotPolicy(policy)
}

func (s *Service) Open() OpenResult {
	s.mu.RLock()
	app := s.app
	s.mu.RUnlock()
	if app == nil {
		return OpenResult{OK: false, Message: "截图覆盖层服务尚未就绪"}
	}

	data, bounds, err := capturehistory.CaptureScreenPNG()
	if err != nil {
		return OpenResult{OK: false, Message: err.Error()}
	}
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return OpenResult{OK: false, Message: "虚拟屏幕尺寸无效"}
	}

	sessions, err := s.overlaySessionsForDisplays(app, data, bounds, false)
	if err != nil {
		return OpenResult{OK: false, Message: err.Error()}
	}
	if len(sessions) == 0 {
		return OpenResult{OK: false, Message: "未找到可用显示器"}
	}

	s.mu.Lock()
	for _, session := range sessions {
		s.sessions[session.ID] = session
	}
	s.trimSessionsLocked(16)
	s.mu.Unlock()

	for _, session := range sessions {
		if err := s.openOverlayWindow(app, session.Session); err != nil {
			s.finishSession(session.ID)
			return OpenResult{OK: false, Message: err.Error()}
		}
	}
	first := sessions[0]
	message := "已打开截图覆盖层"
	if len(sessions) > 1 {
		message = fmt.Sprintf("已打开截图覆盖层（%d 个显示器）", len(sessions))
	}
	return OpenResult{OK: true, Message: message, SessionID: first.ID, Bounds: first.Bounds, Native: first.Native}
}

func (s *Service) GetSession(id string) Session {
	id = strings.TrimSpace(id)
	s.mu.RLock()
	defer s.mu.RUnlock()
	session := s.sessions[id]
	return session.Session
}

func (s *Service) Cancel(id string) CaptureResult {
	id = strings.TrimSpace(id)
	if id == "" {
		return CaptureResult{OK: false, Message: "缺少截图会话 ID"}
	}
	s.finishSession(id)
	return CaptureResult{OK: true, Message: "已取消截图"}
}

func (s *Service) CaptureSelection(request SelectionRequest) CaptureResult {
	session, ok := s.session(request.SessionID)
	if !ok {
		return CaptureResult{OK: false, Message: "截图覆盖层会话已失效"}
	}
	if s.captures == nil {
		s.finishSession(request.SessionID)
		return CaptureResult{OK: false, Message: "截图历史服务不可用"}
	}
	cropSelection, cropBounds, pinSelection, err := resolveSelection(request, session)
	if err != nil {
		return CaptureResult{OK: false, Message: err.Error()}
	}
	if cropSelection.Empty() || cropSelection.Dx() < 2 || cropSelection.Dy() < 2 {
		return CaptureResult{OK: false, Message: "截图区域太小"}
	}

	pngBytes, err := renderSelectionPNG(session.pngBytes, cropSelection, cropBounds, request.Operations, request.RenderedImage)
	if err != nil {
		return CaptureResult{OK: false, Message: err.Error()}
	}
	action := normalizeAction(request.Action)
	policy := s.screenshotPolicy()
	savedPath := ""
	sideEffects := []string{}
	autoSaveError := ""
	if action == "save_as" {
		var err error
		savedPath, err = writeExternalPNG(request.SavedPath, pngBytes)
		if err != nil {
			return CaptureResult{OK: false, Message: err.Error()}
		}
		sideEffects = append(sideEffects, "save")
	} else if action != "qr" && policy.AutoSave {
		autoSavePath, err := autoSavePath(policy, time.Now())
		if err != nil {
			autoSaveError = err.Error()
		} else {
			var writeErr error
			savedPath, writeErr = writeExternalPNG(autoSavePath, pngBytes)
			if writeErr != nil {
				autoSaveError = writeErr.Error()
				savedPath = ""
			} else {
				sideEffects = append(sideEffects, "save")
			}
		}
	}
	shouldCopy := action != "qr" && (action == "copy" || policy.AutoCopy)
	shouldPin := action != "qr" && (action == "pin" || policy.AutoPin)
	if shouldCopy {
		sideEffects = append(sideEffects, "copy")
	}
	if shouldPin {
		sideEffects = append(sideEffects, "pin")
	}

	status := s.captures.AddPNG(pngBytes, cropSelection.Dx(), cropSelection.Dy(), "overlay_selection", savedPath, actionTags(action, request.Operations, sideEffects))
	if status.LastCaptureError != "" || status.LastSaveError != "" || len(status.Entries) == 0 {
		return CaptureResult{OK: false, Message: firstNonEmpty(status.LastCaptureError, status.LastSaveError, "截图保存失败")}
	}
	entry := status.Entries[0]
	message := selectionResultMessage(action, len(request.Operations), autoSaveError)
	result := CaptureResult{
		OK:        true,
		Message:   message,
		CaptureID: entry.ID,
		ImagePath: entry.ImagePath,
		SavedPath: entry.SavedPath,
		Width:     entry.Width,
		Height:    entry.Height,
	}

	if shouldCopy {
		if err := writeImageToClipboard(entry.ImagePath); err != nil {
			result.OK = false
			result.Message = "已保存截图，复制失败: " + err.Error()
		} else if action == "copy" {
			result.Message = appendResultMessage(result.Message, "已复制截图")
		} else if policy.AutoCopy {
			result.Message = appendResultMessage(result.Message, "已复制")
		}
	}

	if shouldPin {
		if s.pins == nil {
			result.OK = false
			result.Message = "已保存截图，但贴图服务不可用"
		} else {
			pin := s.openPinnedSelection(entry.ID, pinSelection, request)
			result.Pin = &pin
			if pin.OK {
				if action == "pin" {
					result.Message = appendResultMessage(result.Message, "已创建贴图")
				} else if policy.AutoPin {
					result.Message = appendResultMessage(result.Message, "已贴图")
				}
			} else {
				result.OK = false
				result.Message = "已保存截图，贴图失败: " + pin.Message
			}
		}
	}

	switch action {
	case "qr":
		qr := qrscan.DecodeImagePath(entry.ImagePath)
		qr.Source = "capture_overlay"
		qr.CaptureID = entry.ID
		qr.ImagePath = entry.ImagePath
		qr.Width = entry.Width
		qr.Height = entry.Height
		result.QR = &qr
		if qr.OK {
			result.Message = "已识别二维码"
		} else {
			result.OK = false
			result.Message = qr.Error
		}
	}

	if result.OK || action == "pin" || action == "qr" {
		s.finishSession(request.SessionID)
	}
	return result
}

func (s *Service) openPinnedSelection(captureID string, selection image.Rectangle, request SelectionRequest) pinnedimage.OpenResult {
	if positioned, ok := s.pins.(PositionedPinService); ok {
		if request.PinPositioned {
			return positioned.OpenCaptureAt(captureID, request.PinX, request.PinY)
		}
		x, y := selection.Min.X, selection.Min.Y
		s.mu.RLock()
		app := s.app
		s.mu.RUnlock()
		if app != nil {
			point := displayPointForNative(app, application.Point{X: x, Y: y})
			x, y = point.X, point.Y
		}
		return positioned.OpenCaptureAt(captureID, x, y)
	}
	return s.pins.OpenCapture(captureID)
}

func (s *Service) session(id string) (overlaySession, bool) {
	id = strings.TrimSpace(id)
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[id]
	return session, ok
}

func (s *Service) screenshotPolicy() ScreenshotPolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return normalizeScreenshotPolicy(s.policy)
}

func (s *Service) finishSession(id string) {
	id = strings.TrimSpace(id)
	s.mu.Lock()
	session, ok := s.sessions[id]
	groupID := session.groupID
	toClose := []string{}
	restoreMain := ok && session.restoreMain
	restoreWindowNames := []string{}
	if ok {
		for _, name := range session.restoreWindowNames {
			restoreWindowNames = appendUnique(restoreWindowNames, name)
		}
	}
	if ok && groupID != "" {
		for candidateID, candidate := range s.sessions {
			if candidate.groupID != groupID {
				continue
			}
			delete(s.sessions, candidateID)
			if candidateID != id && candidate.windowName != "" {
				toClose = append(toClose, candidate.windowName)
			}
			restoreMain = restoreMain || candidate.restoreMain
			for _, name := range candidate.restoreWindowNames {
				restoreWindowNames = appendUnique(restoreWindowNames, name)
			}
		}
	} else {
		delete(s.sessions, id)
	}
	app := s.app
	s.mu.Unlock()
	if app != nil {
		for _, name := range toClose {
			if window, exists := app.Window.Get(name); exists {
				window.Close()
			}
		}
	}
	if ok && app != nil {
		if restoreMain {
			restoreWindowNames = appendUnique(restoreWindowNames, "main")
		}
		restoreCaptureWindows(app, restoreWindowNames)
	}
}

func (s *Service) openOverlayWindow(app *application.App, session Session) error {
	if app == nil {
		return errors.New("截图覆盖层服务尚未就绪")
	}
	name := "capture-overlay-" + session.ID
	bounds := screenBoundsToApplicationRect(session.Bounds)
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             name,
		Title:            "Ariadne - 截图覆盖层",
		URL:              "/?view=capture-overlay&sessionId=" + url.QueryEscape(session.ID),
		Width:            bounds.Width,
		Height:           bounds.Height,
		X:                bounds.X,
		Y:                bounds.Y,
		AlwaysOnTop:      true,
		Frameless:        true,
		DisableResize:    true,
		BackgroundColour: application.NewRGB(244, 244, 245),
		InitialPosition:  application.WindowXY,
		Windows: application.WindowsWindow{
			Theme:                             application.Light,
			DisableIcon:                       true,
			DisableFramelessWindowDecorations: true,
		},
	})
	if window != nil {
		window.SetBounds(bounds)
	}
	return nil
}

func (s *Service) overlaySessionsForDisplays(app *application.App, data []byte, bounds capturehistory.ScreenBounds, restoreMain bool) ([]overlaySession, error) {
	return s.overlaySessionsForDisplayBounds(app, data, bounds, displayNativeBounds(bounds, capturehistory.MonitorBounds()), restoreMain)
}

func (s *Service) overlaySessionsForDisplayBounds(app *application.App, data []byte, bounds capturehistory.ScreenBounds, displays []capturehistory.ScreenBounds, restoreMain bool) ([]overlaySession, error) {
	groupID := newSessionID()
	createdAt := time.Now().Unix()
	sessions := make([]overlaySession, 0, len(displays))
	for index, display := range displays {
		cropped, err := cropPNG(data, image.Rect(display.X, display.Y, display.X+display.Width, display.Y+display.Height), bounds)
		if err != nil {
			return nil, err
		}
		id := newSessionID()
		session := overlaySession{
			Session: Session{
				ID:        id,
				Bounds:    displayBoundsForNative(app, display),
				Native:    display,
				ImageURL:  "data:image/png;base64," + base64.StdEncoding.EncodeToString(cropped),
				CreatedAt: createdAt,
			},
			pngBytes:    cropped,
			groupID:     groupID,
			windowName:  "capture-overlay-" + id,
			restoreMain: restoreMain && index == 0,
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func displayNativeBounds(bounds capturehistory.ScreenBounds, monitors []capturehistory.ScreenBounds) []capturehistory.ScreenBounds {
	virtualRect := image.Rect(bounds.X, bounds.Y, bounds.X+bounds.Width, bounds.Y+bounds.Height)
	if virtualRect.Empty() {
		return nil
	}
	displays := make([]capturehistory.ScreenBounds, 0, len(monitors))
	for _, monitor := range monitors {
		rect := image.Rect(monitor.X, monitor.Y, monitor.X+monitor.Width, monitor.Y+monitor.Height).Intersect(virtualRect)
		if rect.Empty() {
			continue
		}
		displays = append(displays, capturehistory.ScreenBounds{
			X:      rect.Min.X,
			Y:      rect.Min.Y,
			Width:  rect.Dx(),
			Height: rect.Dy(),
		})
	}
	if len(displays) == 0 {
		displays = append(displays, bounds)
	}
	sort.SliceStable(displays, func(i int, j int) bool {
		if displays[i].Y == displays[j].Y {
			return displays[i].X < displays[j].X
		}
		return displays[i].Y < displays[j].Y
	})
	return displays
}

func cropPNG(source []byte, selection image.Rectangle, bounds capturehistory.ScreenBounds) ([]byte, error) {
	cropped, err := cropImage(source, selection, bounds)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := png.Encode(&out, cropped); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func hideCaptureWindows(app *application.App) []string {
	if app == nil {
		return nil
	}
	names := []string{}
	for _, window := range app.Window.GetAll() {
		if window == nil || !captureWindowShouldHide(window.Name()) || !window.IsVisible() {
			continue
		}
		window.Hide()
		names = appendUnique(names, window.Name())
	}
	if len(names) > 0 {
		time.Sleep(120 * time.Millisecond)
	}
	return names
}

func restoreCaptureWindows(app *application.App, names []string) {
	if app == nil || len(names) == 0 {
		return
	}
	var mainWindow application.Window
	for _, name := range names {
		window, ok := app.Window.Get(name)
		if !ok || window == nil {
			continue
		}
		window.Show()
		if name == "main" {
			mainWindow = window
		}
	}
	if mainWindow != nil {
		mainWindow.Focus()
	}
}

func captureWindowShouldHide(name string) bool {
	return false
}

func stringSliceContains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func (s *Service) trimSessionsLocked(limit int) {
	if len(s.sessions) <= limit {
		return
	}
	oldestID := ""
	oldestAt := time.Now().Unix() + 1
	for id, session := range s.sessions {
		if session.CreatedAt < oldestAt {
			oldestID = id
			oldestAt = session.CreatedAt
		}
	}
	if oldestID != "" {
		delete(s.sessions, oldestID)
	}
}

func normalizedSelection(request SelectionRequest, bounds capturehistory.ScreenBounds) image.Rectangle {
	x1 := min(request.X, request.X+request.Width)
	x2 := max(request.X, request.X+request.Width)
	y1 := min(request.Y, request.Y+request.Height)
	y2 := max(request.Y, request.Y+request.Height)
	return image.Rect(x1, y1, x2, y2).Intersect(image.Rect(bounds.X, bounds.Y, bounds.X+bounds.Width, bounds.Y+bounds.Height))
}

func resolveSelection(request SelectionRequest, session overlaySession) (image.Rectangle, capturehistory.ScreenBounds, image.Rectangle, error) {
	nativeBounds := sessionNativeBounds(session)
	switch strings.ToLower(strings.TrimSpace(request.CoordinateSpace)) {
	case "visual":
		imageBounds, err := pngImageBounds(session.pngBytes)
		if err != nil {
			return image.Rectangle{}, capturehistory.ScreenBounds{}, image.Rectangle{}, err
		}
		localBounds := capturehistory.ScreenBounds{
			X:      imageBounds.Min.X,
			Y:      imageBounds.Min.Y,
			Width:  imageBounds.Dx(),
			Height: imageBounds.Dy(),
		}
		displayWidth := firstPositiveInt(request.DisplayWidth, session.Bounds.Width, imageBounds.Dx())
		displayHeight := firstPositiveInt(request.DisplayHeight, session.Bounds.Height, imageBounds.Dy())
		localSelection := visualSelectionToImageRect(request, imageBounds, displayWidth, displayHeight)
		pinSelection := localSelection.Add(image.Pt(nativeBounds.X, nativeBounds.Y))
		return localSelection, localBounds, pinSelection, nil
	case "session":
		imageBounds, err := pngImageBounds(session.pngBytes)
		if err != nil {
			return image.Rectangle{}, capturehistory.ScreenBounds{}, image.Rectangle{}, err
		}
		localBounds := capturehistory.ScreenBounds{
			X:      imageBounds.Min.X,
			Y:      imageBounds.Min.Y,
			Width:  imageBounds.Dx(),
			Height: imageBounds.Dy(),
		}
		localSelection := normalizedSelection(request, localBounds)
		pinSelection := localSelection.Add(image.Pt(nativeBounds.X, nativeBounds.Y))
		return localSelection, localBounds, pinSelection, nil
	}
	selection := normalizedSelection(request, nativeBounds)
	return selection, nativeBounds, selection, nil
}

func visualSelectionToImageRect(request SelectionRequest, imageBounds image.Rectangle, displayWidth int, displayHeight int) image.Rectangle {
	displayWidth = max(displayWidth, 1)
	displayHeight = max(displayHeight, 1)
	x1 := min(request.X, request.X+request.Width)
	x2 := max(request.X, request.X+request.Width)
	y1 := min(request.Y, request.Y+request.Height)
	y2 := max(request.Y, request.Y+request.Height)
	scaleX := float64(imageBounds.Dx()) / float64(displayWidth)
	scaleY := float64(imageBounds.Dy()) / float64(displayHeight)
	left := imageBounds.Min.X + int(math.Floor(float64(x1)*scaleX))
	top := imageBounds.Min.Y + int(math.Floor(float64(y1)*scaleY))
	right := imageBounds.Min.X + int(math.Ceil(float64(x2)*scaleX))
	bottom := imageBounds.Min.Y + int(math.Ceil(float64(y2)*scaleY))
	rect := image.Rect(left, top, right, bottom).Intersect(imageBounds)
	if rect.Empty() {
		return rect
	}
	if rect.Dx() < 1 && rect.Min.X < imageBounds.Max.X {
		rect.Max.X = rect.Min.X + 1
	}
	if rect.Dy() < 1 && rect.Min.Y < imageBounds.Max.Y {
		rect.Max.Y = rect.Min.Y + 1
	}
	return rect
}

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 1
}

func pngImageBounds(data []byte) (image.Rectangle, error) {
	img, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return image.Rectangle{}, fmt.Errorf("截图背景解码失败: %w", err)
	}
	return image.Rect(0, 0, img.Width, img.Height), nil
}

func sessionNativeBounds(session overlaySession) capturehistory.ScreenBounds {
	if session.Native.Width > 0 && session.Native.Height > 0 {
		return session.Native
	}
	return session.Bounds
}

func displayBoundsForNative(app *application.App, bounds capturehistory.ScreenBounds) capturehistory.ScreenBounds {
	if app == nil || app.Screen == nil || len(app.Screen.GetAll()) == 0 {
		return bounds
	}
	rect := app.Screen.PhysicalToDipRect(screenBoundsToApplicationRect(bounds))
	if rect.Width <= 0 || rect.Height <= 0 {
		return bounds
	}
	return screenBoundsFromApplicationRect(rect)
}

func displayPointForNative(app *application.App, point application.Point) application.Point {
	if app == nil || app.Screen == nil || len(app.Screen.GetAll()) == 0 {
		return point
	}
	return app.Screen.PhysicalToDipPoint(point)
}

func screenBoundsToApplicationRect(bounds capturehistory.ScreenBounds) application.Rect {
	return application.Rect{
		X:      bounds.X,
		Y:      bounds.Y,
		Width:  bounds.Width,
		Height: bounds.Height,
	}
}

func screenBoundsFromApplicationRect(rect application.Rect) capturehistory.ScreenBounds {
	return capturehistory.ScreenBounds{
		X:      rect.X,
		Y:      rect.Y,
		Width:  rect.Width,
		Height: rect.Height,
	}
}

func normalizeAction(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "copy", "clipboard":
		return "copy"
	case "pin":
		return "pin"
	case "qr":
		return "qr"
	case "save_as":
		return "save_as"
	default:
		return "capture"
	}
}

func actionTags(action string, operations []AnnotationOperation, sideEffects []string) []string {
	tags := []string{"overlay", "selection"}
	if action != "capture" {
		tags = appendUnique(tags, action)
	}
	for _, sideEffect := range sideEffects {
		tags = appendUnique(tags, sideEffect)
	}
	if len(operations) > 0 {
		tags = appendUnique(tags, "annotated")
	}
	for _, operation := range operations {
		tags = appendUnique(tags, normalizeOperationKind(operation.Kind))
	}
	return tags
}

func appendUnique(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func normalizeScreenshotPolicy(policy ScreenshotPolicy) ScreenshotPolicy {
	policy.SaveDir = strings.TrimSpace(policy.SaveDir)
	policy.FilenameTemplate = strings.TrimSpace(policy.FilenameTemplate)
	if policy.FilenameTemplate == "" {
		policy.FilenameTemplate = "ariadne_{date}_{time}"
	}
	return policy
}

func autoSavePath(policy ScreenshotPolicy, now time.Time) (string, error) {
	policy = normalizeScreenshotPolicy(policy)
	if policy.SaveDir == "" {
		return "", errors.New("未配置截图自动保存目录")
	}
	return filepath.Join(policy.SaveDir, screenshotFilename(policy.FilenameTemplate, now)), nil
}

func screenshotFilename(template string, now time.Time) string {
	name := strings.TrimSpace(template)
	if name == "" {
		name = "ariadne_{date}_{time}"
	}
	replacements := map[string]string{
		"{date}":     now.Format("20060102"),
		"{time}":     now.Format("150405"),
		"{datetime}": now.Format("20060102_150405"),
	}
	for token, value := range replacements {
		name = strings.ReplaceAll(name, token, value)
	}
	for _, ch := range `<>:"/\|?*` {
		name = strings.ReplaceAll(name, string(ch), "_")
	}
	name = strings.Trim(strings.TrimSpace(name), ".")
	if name == "" {
		name = "ariadne_" + now.Format("20060102_150405")
	}
	if !strings.EqualFold(filepath.Ext(name), ".png") {
		name += ".png"
	}
	return name
}

func selectionResultMessage(action string, operationCount int, autoSaveError string) string {
	message := "已保存选区截图"
	if operationCount > 0 {
		message = "已保存标注截图"
	}
	switch action {
	case "copy":
		message = "已复制截图"
	case "pin":
		message = "已保存选区截图"
	case "save_as":
		message = "已另存截图"
	}
	if strings.TrimSpace(autoSaveError) != "" {
		message += "，自动保存失败: " + autoSaveError
	}
	return message
}

func appendResultMessage(message string, addition string) string {
	message = strings.TrimSpace(message)
	addition = strings.TrimSpace(addition)
	if addition == "" || strings.Contains(message, addition) {
		return message
	}
	if message == "" {
		return addition
	}
	return message + "，" + addition
}

func renderSelectionPNG(source []byte, selection image.Rectangle, bounds capturehistory.ScreenBounds, operations []AnnotationOperation, renderedImage string) ([]byte, error) {
	if strings.TrimSpace(renderedImage) != "" {
		return decodeRenderedPNG(renderedImage, selection.Dx(), selection.Dy())
	}
	cropped, err := cropImage(source, selection, bounds)
	if err != nil {
		return nil, err
	}
	applyAnnotationOperations(cropped, operations)
	var out bytes.Buffer
	if err := png.Encode(&out, cropped); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func decodeRenderedPNG(value string, expectedWidth int, expectedHeight int) ([]byte, error) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "data:image/png;base64,")
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("标注截图解码失败: %w", err)
	}
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("标注截图无效: %w", err)
	}
	if format != "png" {
		return nil, errors.New("标注截图必须是 PNG")
	}
	if img.Bounds().Dx() != expectedWidth || img.Bounds().Dy() != expectedHeight {
		return nil, fmt.Errorf("标注截图尺寸不匹配: %dx%d != %dx%d", img.Bounds().Dx(), img.Bounds().Dy(), expectedWidth, expectedHeight)
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func cropImage(source []byte, selection image.Rectangle, bounds capturehistory.ScreenBounds) (*image.RGBA, error) {
	img, _, err := image.Decode(bytes.NewReader(source))
	if err != nil {
		return nil, fmt.Errorf("截图背景解码失败: %w", err)
	}
	offset := image.Rect(selection.Min.X-bounds.X, selection.Min.Y-bounds.Y, selection.Max.X-bounds.X, selection.Max.Y-bounds.Y)
	if offset.Empty() || !offset.In(img.Bounds()) {
		return nil, fmt.Errorf("截图区域超出背景范围")
	}
	cropped := image.NewRGBA(image.Rect(0, 0, offset.Dx(), offset.Dy()))
	draw.Draw(cropped, cropped.Bounds(), img, offset.Min, draw.Src)
	return cropped, nil
}

func applyAnnotationOperations(img *image.RGBA, operations []AnnotationOperation) {
	base := cloneRGBA(img)
	for _, operation := range operations {
		switch normalizeOperationKind(operation.Kind) {
		case "rect":
			drawRect(img, normalizeRect(operation, img.Bounds()), annotationColor(operation.Color), clampInt(operation.StrokeWidth, 2, 12, 3))
		case "line":
			drawSimpleLine(img, operation, annotationColor(operation.Color), clampInt(operation.StrokeWidth, 1, 24, 3))
		case "arrow":
			drawArrow(img, operation, annotationColor(operation.Color), clampInt(operation.StrokeWidth, 2, 24, 4))
		case "pen":
			drawPolyline(img, operation.Points, annotationColor(operation.Color), clampInt(operation.StrokeWidth, 1, 24, 3))
		case "mosaic":
			if len(operation.Points) > 1 {
				applyMosaicPath(img, operation.Points, clampInt(operation.PixelSize, 6, 48, 12))
			} else {
				applyMosaic(img, normalizeRect(operation, img.Bounds()), clampInt(operation.PixelSize, 6, 48, 12))
			}
		case "eraser":
			applyEraserPath(img, base, operation.Points, clampInt(operation.StrokeWidth, 1, 24, 3))
		case "number":
			drawNumberMarker(img, operation, annotationColor(operation.Color), clampInt(operation.StrokeWidth, 1, 24, 3))
		}
	}
}

func normalizeOperationKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "rect", "rectangle":
		return "rect"
	case "line":
		return "line"
	case "arrow":
		return "arrow"
	case "pen", "brush":
		return "pen"
	case "mosaic", "pixelate":
		return "mosaic"
	case "text":
		return "text"
	case "number":
		return "number"
	case "eraser":
		return "eraser"
	default:
		return ""
	}
}

func normalizeRect(operation AnnotationOperation, bounds image.Rectangle) image.Rectangle {
	x1 := operation.X
	x2 := operation.X + operation.Width
	y1 := operation.Y
	y2 := operation.Y + operation.Height
	if operation.Width < 0 {
		x1, x2 = x2, x1
	}
	if operation.Height < 0 {
		y1, y2 = y2, y1
	}
	return image.Rect(x1, y1, x2, y2).Intersect(bounds)
}

func drawRect(img *image.RGBA, rect image.Rectangle, col color.RGBA, stroke int) {
	if rect.Empty() {
		return
	}
	for i := 0; i < stroke; i++ {
		drawLine(img, rect.Min.X, rect.Min.Y+i, rect.Max.X-1, rect.Min.Y+i, col, 1)
		drawLine(img, rect.Min.X, rect.Max.Y-1-i, rect.Max.X-1, rect.Max.Y-1-i, col, 1)
		drawLine(img, rect.Min.X+i, rect.Min.Y, rect.Min.X+i, rect.Max.Y-1, col, 1)
		drawLine(img, rect.Max.X-1-i, rect.Min.Y, rect.Max.X-1-i, rect.Max.Y-1, col, 1)
	}
}

func drawSimpleLine(img *image.RGBA, operation AnnotationOperation, col color.RGBA, stroke int) {
	startX := clampInt(operation.X, img.Bounds().Min.X, img.Bounds().Max.X-1, img.Bounds().Min.X)
	startY := clampInt(operation.Y, img.Bounds().Min.Y, img.Bounds().Max.Y-1, img.Bounds().Min.Y)
	endX := clampInt(operation.EndX, img.Bounds().Min.X, img.Bounds().Max.X-1, img.Bounds().Min.X)
	endY := clampInt(operation.EndY, img.Bounds().Min.Y, img.Bounds().Max.Y-1, img.Bounds().Min.Y)
	drawLine(img, startX, startY, endX, endY, col, stroke)
}

func drawArrow(img *image.RGBA, operation AnnotationOperation, col color.RGBA, stroke int) {
	startX := clampInt(operation.X, img.Bounds().Min.X, img.Bounds().Max.X-1, img.Bounds().Min.X)
	startY := clampInt(operation.Y, img.Bounds().Min.Y, img.Bounds().Max.Y-1, img.Bounds().Min.Y)
	endX := clampInt(operation.EndX, img.Bounds().Min.X, img.Bounds().Max.X-1, img.Bounds().Min.X)
	endY := clampInt(operation.EndY, img.Bounds().Min.Y, img.Bounds().Max.Y-1, img.Bounds().Min.Y)
	if abs(endX-startX)+abs(endY-startY) < 4 {
		return
	}
	drawLine(img, startX, startY, endX, endY, col, stroke)
	angle := math.Atan2(float64(endY-startY), float64(endX-startX))
	headLength := float64(max(12, stroke*4))
	for _, delta := range []float64{math.Pi * 0.82, -math.Pi * 0.82} {
		headX := endX + int(math.Cos(angle+delta)*headLength)
		headY := endY + int(math.Sin(angle+delta)*headLength)
		drawLine(img, endX, endY, headX, headY, col, stroke)
	}
}

func drawPolyline(img *image.RGBA, points []AnnotationPoint, col color.RGBA, stroke int) {
	if len(points) == 0 {
		return
	}
	if len(points) == 1 {
		paintBrush(img, points[0].X, points[0].Y, col, stroke)
		return
	}
	for i := 1; i < len(points); i++ {
		drawLine(img, points[i-1].X, points[i-1].Y, points[i].X, points[i].Y, col, stroke)
	}
}

func drawLine(img *image.RGBA, x1 int, y1 int, x2 int, y2 int, col color.RGBA, stroke int) {
	dx := x2 - x1
	dy := y2 - y1
	steps := max(abs(dx), abs(dy))
	if steps == 0 {
		paintBrush(img, x1, y1, col, stroke)
		return
	}
	for i := 0; i <= steps; i++ {
		x := x1 + int(math.Round(float64(dx)*float64(i)/float64(steps)))
		y := y1 + int(math.Round(float64(dy)*float64(i)/float64(steps)))
		paintBrush(img, x, y, col, stroke)
	}
}

func paintBrush(img *image.RGBA, x int, y int, col color.RGBA, stroke int) {
	radius := max(1, stroke/2)
	for py := y - radius; py <= y+radius; py++ {
		for px := x - radius; px <= x+radius; px++ {
			if !image.Pt(px, py).In(img.Bounds()) {
				continue
			}
			if (px-x)*(px-x)+(py-y)*(py-y) <= radius*radius {
				img.SetRGBA(px, py, col)
			}
		}
	}
}

func cloneRGBA(img *image.RGBA) *image.RGBA {
	clone := image.NewRGBA(img.Bounds())
	draw.Draw(clone, clone.Bounds(), img, img.Bounds().Min, draw.Src)
	return clone
}

func applyMosaic(img *image.RGBA, rect image.Rectangle, blockSize int) {
	if rect.Empty() {
		return
	}
	for y := rect.Min.Y; y < rect.Max.Y; y += blockSize {
		for x := rect.Min.X; x < rect.Max.X; x += blockSize {
			block := image.Rect(x, y, min(x+blockSize, rect.Max.X), min(y+blockSize, rect.Max.Y))
			col := averageColor(img, block)
			draw.Draw(img, block, &image.Uniform{C: col}, image.Point{}, draw.Src)
		}
	}
}

func applyMosaicPath(img *image.RGBA, points []AnnotationPoint, blockSize int) {
	if len(points) == 0 {
		return
	}
	radius := max(blockSize, 6)
	forEachPathPoint(points, max(2, blockSize/2), func(point AnnotationPoint) {
		applyMosaic(img, image.Rect(point.X-radius, point.Y-radius, point.X+radius, point.Y+radius).Intersect(img.Bounds()), blockSize)
	})
}

func applyEraserPath(img *image.RGBA, base *image.RGBA, points []AnnotationPoint, stroke int) {
	if base == nil || len(points) == 0 {
		return
	}
	radius := max(4, stroke*2)
	forEachPathPoint(points, max(2, radius/2), func(point AnnotationPoint) {
		restoreCircle(img, base, point.X, point.Y, radius)
	})
}

func restoreCircle(img *image.RGBA, base *image.RGBA, x int, y int, radius int) {
	for py := y - radius; py <= y+radius; py++ {
		for px := x - radius; px <= x+radius; px++ {
			if !image.Pt(px, py).In(img.Bounds()) || (px-x)*(px-x)+(py-y)*(py-y) > radius*radius {
				continue
			}
			img.SetRGBA(px, py, base.RGBAAt(px, py))
		}
	}
}

func forEachPathPoint(points []AnnotationPoint, step int, callback func(AnnotationPoint)) {
	if len(points) == 0 {
		return
	}
	callback(points[0])
	for i := 1; i < len(points); i++ {
		start := points[i-1]
		end := points[i]
		dx := end.X - start.X
		dy := end.Y - start.Y
		distance := max(abs(dx), abs(dy))
		samples := max(1, int(math.Ceil(float64(distance)/float64(max(1, step)))))
		for sample := 1; sample <= samples; sample++ {
			callback(AnnotationPoint{
				X: start.X + int(math.Round(float64(dx)*float64(sample)/float64(samples))),
				Y: start.Y + int(math.Round(float64(dy)*float64(sample)/float64(samples))),
			})
		}
	}
}

func drawNumberMarker(img *image.RGBA, operation AnnotationOperation, col color.RGBA, stroke int) {
	radius := max(10, clampInt(operation.FontSize, 10, 48, 18))
	centerX := clampInt(operation.X, img.Bounds().Min.X, img.Bounds().Max.X-1, img.Bounds().Min.X)
	centerY := clampInt(operation.Y, img.Bounds().Min.Y, img.Bounds().Max.Y-1, img.Bounds().Min.Y)
	fillCircle(img, centerX, centerY, radius, color.RGBA{R: 255, G: 255, B: 255, A: 235})
	for i := 0; i < max(1, stroke); i++ {
		drawCircleOutline(img, centerX, centerY, radius-i, col)
	}
}

func fillCircle(img *image.RGBA, x int, y int, radius int, col color.RGBA) {
	for py := y - radius; py <= y+radius; py++ {
		for px := x - radius; px <= x+radius; px++ {
			if !image.Pt(px, py).In(img.Bounds()) || (px-x)*(px-x)+(py-y)*(py-y) > radius*radius {
				continue
			}
			img.SetRGBA(px, py, col)
		}
	}
}

func drawCircleOutline(img *image.RGBA, x int, y int, radius int, col color.RGBA) {
	if radius <= 0 {
		return
	}
	steps := max(24, radius*8)
	for i := 0; i < steps; i++ {
		angle := 2 * math.Pi * float64(i) / float64(steps)
		paintBrush(img, x+int(math.Round(math.Cos(angle)*float64(radius))), y+int(math.Round(math.Sin(angle)*float64(radius))), col, 1)
	}
}

func averageColor(img *image.RGBA, rect image.Rectangle) color.RGBA {
	var r, g, b, a uint32
	count := uint32(0)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			col := img.RGBAAt(x, y)
			r += uint32(col.R)
			g += uint32(col.G)
			b += uint32(col.B)
			a += uint32(col.A)
			count++
		}
	}
	if count == 0 {
		return color.RGBA{}
	}
	return color.RGBA{R: uint8(r / count), G: uint8(g / count), B: uint8(b / count), A: uint8(a / count)}
}

func annotationColor(value string) color.RGBA {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) != 6 {
		return color.RGBA{R: 220, G: 38, B: 38, A: 255}
	}
	raw, err := strconv.ParseUint(value, 16, 32)
	if err != nil {
		return color.RGBA{R: 220, G: 38, B: 38, A: 255}
	}
	return color.RGBA{R: uint8(raw >> 16), G: uint8(raw >> 8), B: uint8(raw), A: 255}
}

func writeExternalPNG(path string, data []byte) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("未选择保存路径")
	}
	if strings.TrimSpace(filepath.Ext(path)) == "" {
		path += ".png"
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("创建保存目录失败: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("另存截图失败: %w", err)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return path, nil
	}
	return absolute, nil
}

func clampInt(value int, minValue int, maxValue int, fallback int) int {
	if value == 0 {
		value = fallback
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func newSessionID() string {
	var raw [6]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("overlay-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("overlay-%d-%s", time.Now().UnixNano(), base64.RawURLEncoding.EncodeToString(raw[:]))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
