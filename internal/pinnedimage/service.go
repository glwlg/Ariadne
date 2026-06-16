package pinnedimage

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"net/url"
	"strings"
	"sync"
	"time"

	"ariadne/internal/capturehistory"
	"ariadne/internal/clipboardhistory"

	goqrcode "github.com/skip2/go-qrcode"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type CaptureSource interface {
	Entry(id string) capturehistory.Entry
	ImageDataURL(id string) string
}

type ClipboardSource interface {
	CollectCurrentEntry(source string) clipboardhistory.CollectCurrentResult
	Entry(id string) clipboardhistory.Entry
	ImageDataURL(id string) string
}

type PinnedImage struct {
	ID         string `json:"id"`
	Source     string `json:"source"`
	SourceID   string `json:"sourceId,omitempty"`
	Title      string `json:"title"`
	ImagePath  string `json:"imagePath,omitempty"`
	Text       string `json:"text,omitempty"`
	DataURL    string `json:"dataUrl"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Bytes      int64  `json:"bytes"`
	CreatedAt  int64  `json:"createdAt"`
	WindowW    int    `json:"windowWidth"`
	WindowH    int    `json:"windowHeight"`
	WindowX    int    `json:"windowX,omitempty"`
	WindowY    int    `json:"windowY,omitempty"`
	Positioned bool   `json:"positioned,omitempty"`
	CanCopy    bool   `json:"canCopy"`
	CopyAction string `json:"copyAction,omitempty"`
	CanOCR     bool   `json:"canOcr"`
}

type OpenResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	PinID   string `json:"pinId,omitempty"`
	Title   string `json:"title,omitempty"`
	Width   int    `json:"width,omitempty"`
	Height  int    `json:"height,omitempty"`
}

type Service struct {
	mu         sync.RWMutex
	app        *application.App
	captures   CaptureSource
	clipboards ClipboardSource
	items      map[string]PinnedImage
	openWindow func(PinnedImage) error
}

func NewService(captures CaptureSource, clipboards ClipboardSource) *Service {
	return &Service{
		captures:   captures,
		clipboards: clipboards,
		items:      map[string]PinnedImage{},
	}
}

func (s *Service) Attach(app *application.App) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.app = app
}

func (s *Service) OpenCapture(id string) OpenResult {
	return s.openCaptureAt(id, 0, 0, false)
}

func (s *Service) OpenCaptureAt(id string, x int, y int) OpenResult {
	return s.openCaptureAt(id, x, y, true)
}

func (s *Service) openCaptureAt(id string, x int, y int, positioned bool) OpenResult {
	if s.captures == nil {
		return OpenResult{OK: false, Message: "截图历史服务不可用"}
	}
	entry := s.captures.Entry(id)
	if entry.ID == "" {
		return OpenResult{OK: false, Message: "未找到截图"}
	}
	dataURL := s.captures.ImageDataURL(entry.ID)
	if dataURL == "" {
		return OpenResult{OK: false, Message: "截图预览不可用"}
	}
	pin := newPinnedImage("capture", entry.ID, captureTitle(entry), entry.ImagePath, dataURL, entry.Width, entry.Height, int64(entry.Bytes), false, "")
	if positioned {
		pin.WindowX = x
		pin.WindowY = y
		pin.Positioned = true
	}
	return s.openPinned(pin)
}

func (s *Service) OpenClipboardImage(id string) OpenResult {
	if s.clipboards == nil {
		return OpenResult{OK: false, Message: "剪贴板历史服务不可用"}
	}
	entry := s.clipboards.Entry(id)
	if entry.ID == "" {
		return OpenResult{OK: false, Message: "未找到剪贴板图片"}
	}
	if entry.Type != clipboardhistory.EntryImage {
		return OpenResult{OK: false, Message: "该剪贴板记录不是图片"}
	}
	dataURL := s.clipboards.ImageDataURL(entry.ID)
	if dataURL == "" {
		return OpenResult{OK: false, Message: "剪贴板图片预览不可用"}
	}
	title := fmt.Sprintf("剪贴板图片 %dx%d", entry.Width, entry.Height)
	pin := newPinnedImage("clipboard", entry.ID, title, entry.ImagePath, dataURL, entry.Width, entry.Height, int64(entry.Bytes), true, "copy_clipboard_image")
	return s.openPinned(pin)
}

func (s *Service) OpenCurrentClipboard() OpenResult {
	if s.clipboards == nil {
		return OpenResult{OK: false, Message: "剪贴板历史服务不可用"}
	}
	collected := s.clipboards.CollectCurrentEntry("pin_clipboard_hotkey")
	if !collected.OK {
		message := strings.TrimSpace(collected.Message)
		if message == "" {
			message = "当前剪贴板没有可贴图的内容"
		}
		return OpenResult{OK: false, Message: message}
	}
	entry := collected.Entry
	switch entry.Type {
	case clipboardhistory.EntryImage:
		return s.OpenClipboardImage(entry.ID)
	case clipboardhistory.EntryText:
		return s.openClipboardText(entry)
	default:
		return OpenResult{OK: false, Message: "当前剪贴板没有可贴图的内容"}
	}
}

func (s *Service) openClipboardText(entry clipboardhistory.Entry) OpenResult {
	text := strings.TrimSpace(entry.Text)
	if text == "" {
		return OpenResult{OK: false, Message: "当前剪贴板没有可贴图的文本"}
	}
	dataURL, width, height := textPinDataURL(text)
	pin := newPinnedImage("clipboard_text", entry.ID, "剪贴板文本贴图", "", dataURL, width, height, int64(len(text)), true, "copy_clipboard_text")
	pin.Text = text
	pin.CanOCR = false
	return s.openPinned(pin)
}

func (s *Service) OpenQRText(text string) OpenResult {
	text = strings.TrimSpace(text)
	if text == "" {
		return OpenResult{OK: false, Message: "缺少二维码内容"}
	}
	data, err := goqrcode.Encode(text, goqrcode.Medium, 320)
	if err != nil {
		return OpenResult{OK: false, Message: err.Error()}
	}
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
	pin := newPinnedImage("qr", "", "二维码贴图", "", dataURL, 320, 320, int64(len(data)), false, "")
	return s.openPinned(pin)
}

func (s *Service) GetPinned(id string) PinnedImage {
	id = strings.TrimSpace(id)
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.items[id]
}

func (s *Service) ClosePinned(id string) OpenResult {
	id = strings.TrimSpace(id)
	if id == "" {
		return OpenResult{OK: false, Message: "缺少贴图 ID"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, id)
	return OpenResult{OK: true, Message: "贴图已关闭", PinID: id}
}

func (s *Service) MovePinned(id string, deltaX int, deltaY int) OpenResult {
	id = strings.TrimSpace(id)
	if id == "" {
		return OpenResult{OK: false, Message: "缺少贴图 ID"}
	}
	if deltaX == 0 && deltaY == 0 {
		return OpenResult{OK: true, Message: "贴图位置未变化", PinID: id}
	}
	s.mu.RLock()
	app := s.app
	_, exists := s.items[id]
	s.mu.RUnlock()
	if !exists {
		return OpenResult{OK: false, Message: "贴图已失效", PinID: id}
	}
	if app == nil {
		return OpenResult{OK: false, Message: "贴图窗口服务尚未就绪", PinID: id}
	}
	window, ok := app.Window.Get("pinned-image-" + id)
	if !ok {
		return OpenResult{OK: false, Message: "贴图窗口不存在", PinID: id}
	}
	x, y := window.Position()
	nextX := x + deltaX
	nextY := y + deltaY
	window.SetPosition(nextX, nextY)
	s.mu.Lock()
	if pin, ok := s.items[id]; ok {
		pin.WindowX = nextX
		pin.WindowY = nextY
		pin.Positioned = true
		s.items[id] = pin
	}
	s.mu.Unlock()
	return OpenResult{OK: true, Message: "贴图已移动", PinID: id}
}

func (s *Service) SetPinnedPosition(id string, x int, y int) OpenResult {
	id = strings.TrimSpace(id)
	if id == "" {
		return OpenResult{OK: false, Message: "缺少贴图 ID"}
	}
	s.mu.RLock()
	_, exists := s.items[id]
	s.mu.RUnlock()
	if !exists {
		return OpenResult{OK: false, Message: "贴图已失效", PinID: id}
	}
	s.mu.Lock()
	if pin, ok := s.items[id]; ok {
		pin.WindowX = x
		pin.WindowY = y
		pin.Positioned = true
		s.items[id] = pin
	}
	s.mu.Unlock()
	return OpenResult{OK: true, Message: "贴图位置已同步", PinID: id}
}

func (s *Service) openPinned(pin PinnedImage) OpenResult {
	s.mu.Lock()
	s.items[pin.ID] = pin
	opener := s.openWindow
	app := s.app
	s.mu.Unlock()

	if opener == nil {
		opener = func(next PinnedImage) error {
			return s.openWindowWithApp(app, next)
		}
	}
	if err := opener(pin); err != nil {
		s.mu.Lock()
		delete(s.items, pin.ID)
		s.mu.Unlock()
		return OpenResult{OK: false, Message: err.Error()}
	}
	return OpenResult{
		OK:      true,
		Message: "已创建贴图",
		PinID:   pin.ID,
		Title:   pin.Title,
		Width:   pin.WindowW,
		Height:  pin.WindowH,
	}
}

func (s *Service) openWindowWithApp(app *application.App, pin PinnedImage) error {
	if app == nil {
		return errors.New("贴图窗口服务尚未就绪")
	}
	name := "pinned-image-" + pin.ID
	if existing, ok := app.Window.Get(name); ok {
		existing.Show().SetAlwaysOnTop(true)
		existing.Focus()
		return nil
	}
	options := application.WebviewWindowOptions{
		Name:             name,
		Title:            pin.Title,
		URL:              "/?view=pinned-image&pinId=" + url.QueryEscape(pin.ID),
		Width:            pin.WindowW,
		Height:           pin.WindowH,
		MinWidth:         1,
		MinHeight:        1,
		AlwaysOnTop:      true,
		Frameless:        true,
		DisableResize:    true,
		BackgroundType:   application.BackgroundTypeTransparent,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
		InitialPosition:  application.WindowCentered,
		Windows: application.WindowsWindow{
			Theme:                             application.Light,
			DisableIcon:                       true,
			DisableFramelessWindowDecorations: true,
		},
	}
	if pin.Positioned {
		options.X = pin.WindowX
		options.Y = pin.WindowY
		options.InitialPosition = application.WindowXY
	}
	app.Window.NewWithOptions(options)
	return nil
}

func newPinnedImage(source string, sourceID string, title string, imagePath string, dataURL string, width int, height int, bytes int64, canCopy bool, copyAction string) PinnedImage {
	windowW, windowH := fitWindowSize(width, height)
	return PinnedImage{
		ID:         newPinID(source),
		Source:     source,
		SourceID:   sourceID,
		Title:      title,
		ImagePath:  imagePath,
		DataURL:    dataURL,
		Width:      width,
		Height:     height,
		Bytes:      bytes,
		CreatedAt:  time.Now().Unix(),
		WindowW:    windowW,
		WindowH:    windowH,
		CanCopy:    canCopy,
		CopyAction: copyAction,
		CanOCR:     source == "capture" || source == "clipboard",
	}
}

func fitWindowSize(width int, height int) (int, int) {
	if width <= 0 || height <= 0 {
		return 1, 1
	}
	return width, height
}

func textPinDataURL(text string) (string, int, int) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) == 0 {
		lines = []string{text}
	}
	const (
		padding    = 24
		lineHeight = 24
		fontSize   = 14
		minWidth   = 220
		maxWidth   = 800
		maxHeight  = 900
	)
	maxLineWidth := 0
	for _, line := range lines {
		if width := visualTextWidth(line); width > maxLineWidth {
			maxLineWidth = width
		}
	}
	width := clampInt(maxLineWidth+padding*2, minWidth, maxWidth)
	height := clampInt(len(lines)*lineHeight+padding*2, 72, maxHeight)
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, width, height, width, height))
	builder.WriteString(`<rect width="100%" height="100%" rx="0" fill="rgba(36,38,48,0.92)"/>`)
	builder.WriteString(fmt.Sprintf(`<g font-family="Microsoft YaHei UI, Segoe UI, Arial, sans-serif" font-size="%d" fill="#f0f0f0">`, fontSize))
	for index, line := range lines {
		y := padding + fontSize + index*lineHeight
		if y > height-padding/2 {
			break
		}
		builder.WriteString(fmt.Sprintf(`<text x="%d" y="%d">%s</text>`, padding, y, html.EscapeString(line)))
	}
	builder.WriteString(`</g></svg>`)
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(builder.String())), width, height
}

func visualTextWidth(value string) int {
	width := 0
	for _, char := range value {
		switch {
		case char == '\t':
			width += 32
		case char < 128:
			width += 8
		default:
			width += 16
		}
	}
	return width
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

func newPinID(source string) string {
	var raw [6]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("%s-%d", source, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s-%d-%s", source, time.Now().UnixNano(), base64.RawURLEncoding.EncodeToString(raw[:]))
}

func captureTitle(entry capturehistory.Entry) string {
	title := "截图贴图"
	if entry.Width > 0 && entry.Height > 0 {
		title = fmt.Sprintf("%s %dx%d", title, entry.Width, entry.Height)
	}
	return title
}
