//go:build windows

package nativecapture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Microsoft/go-winio"
)

const (
	defaultCaptureTimeout = 2 * time.Minute
	defaultStartTimeout   = 4 * time.Second
)

type Options struct {
	ExePath  string
	PipeName string
}

type Request struct {
	Command             string `json:"command"`
	AutoPin             bool   `json:"autoPin"`
	DirectClipboardCopy bool   `json:"directClipboardCopy"`
	CallbackPipeName    string `json:"callbackPipeName,omitempty"`
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

type Response struct {
	OK               bool                  `json:"ok"`
	Canceled         bool                  `json:"canceled"`
	Message          string                `json:"message"`
	Action           string                `json:"action"`
	PNGBase64        string                `json:"pngBase64"`
	X                int                   `json:"x"`
	Y                int                   `json:"y"`
	Width            int                   `json:"width"`
	Height           int                   `json:"height"`
	SavedPath        string                `json:"savedPath"`
	ClipboardWritten bool                  `json:"clipboardWritten"`
	Pinned           bool                  `json:"pinned"`
	PinPositioned    bool                  `json:"pinPositioned"`
	PinX             int                   `json:"pinX"`
	PinY             int                   `json:"pinY"`
	NativePinID      string                `json:"nativePinId"`
	RenderMS         int64                 `json:"renderMs"`
	ClipboardMS      int64                 `json:"clipboardMs"`
	TotalMS          int64                 `json:"totalMs"`
	Operations       []AnnotationOperation `json:"operations,omitempty"`
}

type PinActionRequest struct {
	Action      string `json:"action"`
	NativePinID string `json:"nativePinId"`
	ImagePath   string `json:"imagePath"`
}

type PinActionResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Text    string `json:"text,omitempty"`
}

type PinActionHandler func(PinActionRequest) PinActionResponse

type Manager struct {
	mu             sync.Mutex
	actionMu       sync.RWMutex
	exePath        string
	pipeName       string
	actionPipeName string
	actionHandler  PinActionHandler
	actionListener net.Listener
	command        *exec.Cmd
}

func NewManager(options Options) *Manager {
	exePath := strings.TrimSpace(options.ExePath)
	if exePath == "" {
		exePath = defaultExePath()
	}
	pipeName := strings.TrimSpace(options.PipeName)
	if pipeName == "" {
		pipeName = "ariadne-capture-" + strconv.Itoa(os.Getpid())
	}
	return &Manager{exePath: exePath, pipeName: pipeName, actionPipeName: pipeName + "-actions"}
}

func (m *Manager) SetPinActionHandler(handler PinActionHandler) {
	m.actionMu.Lock()
	m.actionHandler = handler
	m.actionMu.Unlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if handler == nil && m.actionListener != nil {
		_ = m.actionListener.Close()
		m.actionListener = nil
	}
}

func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startLocked()
}

func (m *Manager) Capture(ctx context.Context, request Request) (Response, error) {
	if request.Command == "" {
		request.Command = "capture"
	}
	if ctx == nil {
		ctx = context.Background()
	}
	captureCtx, cancel := context.WithTimeout(ctx, defaultCaptureTimeout)
	defer cancel()

	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.startLocked(); err != nil {
		return Response{}, err
	}
	if m.actionListener != nil && request.CallbackPipeName == "" {
		request.CallbackPipeName = m.actionPipeName
	}
	response, err := m.roundTrip(captureCtx, request)
	if err == nil {
		return response, nil
	}
	first := err
	if captureCtx.Err() != nil {
		return Response{}, first
	}
	m.discardCommandLocked()
	if err := m.startLocked(); err != nil {
		return Response{}, fmt.Errorf("%w; 重启原生截图进程失败: %v", first, err)
	}
	response, err = m.roundTrip(captureCtx, request)
	if err != nil {
		return Response{}, fmt.Errorf("%w; 重试原生截图请求失败: %v", first, err)
	}
	return response, nil
}

func (m *Manager) Stop() error {
	m.mu.Lock()
	command := m.command
	m.command = nil
	actionListener := m.actionListener
	m.actionListener = nil
	m.mu.Unlock()
	if actionListener != nil {
		_ = actionListener.Close()
	}
	if command == nil || command.Process == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	_, _ = m.roundTrip(ctx, Request{Command: "shutdown"})
	cancel()

	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(1200 * time.Millisecond):
		_ = command.Process.Kill()
		return <-done
	}
}

func (m *Manager) startLocked() error {
	if m.command != nil && m.command.Process != nil {
		if err := m.startActionServerLocked(); err != nil {
			return err
		}
		return nil
	}
	if strings.TrimSpace(m.exePath) == "" {
		return errors.New("原生截图进程路径为空")
	}
	if info, err := os.Stat(m.exePath); err != nil || info.IsDir() {
		if err == nil {
			err = errors.New("path is a directory")
		}
		return fmt.Errorf("原生截图进程不可用: %s: %w", m.exePath, err)
	}

	command := exec.Command(m.exePath, "--pipe", m.pipeName)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := command.Start(); err != nil {
		return fmt.Errorf("启动原生截图进程失败: %w", err)
	}
	m.command = command
	if err := m.startActionServerLocked(); err != nil {
		_ = command.Process.Kill()
		m.command = nil
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultStartTimeout)
	defer cancel()
	for {
		response, err := m.roundTrip(ctx, Request{Command: "ping"})
		if err == nil && response.OK {
			return nil
		}
		if ctx.Err() != nil {
			_ = command.Process.Kill()
			m.command = nil
			return fmt.Errorf("原生截图进程未就绪: %w", firstErr(err, ctx.Err()))
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (m *Manager) startActionServerLocked() error {
	if !m.hasActionHandler() || m.actionListener != nil {
		return nil
	}
	listener, err := winio.ListenPipe(`\\.\pipe\`+m.actionPipeName, nil)
	if err != nil {
		return fmt.Errorf("启动原生贴图动作管道失败: %w", err)
	}
	m.actionListener = listener
	go m.listenPinActions(listener)
	return nil
}

func (m *Manager) listenPinActions(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		go m.handlePinAction(conn)
	}
}

func (m *Manager) hasActionHandler() bool {
	m.actionMu.RLock()
	defer m.actionMu.RUnlock()
	return m.actionHandler != nil
}

func (m *Manager) handlePinAction(conn net.Conn) {
	defer conn.Close()
	var request PinActionRequest
	if err := json.NewDecoder(conn).Decode(&request); err != nil {
		_ = json.NewEncoder(conn).Encode(PinActionResponse{OK: false, Message: "贴图动作请求无效"})
		return
	}
	m.actionMu.RLock()
	handler := m.actionHandler
	m.actionMu.RUnlock()
	if handler == nil {
		_ = json.NewEncoder(conn).Encode(PinActionResponse{OK: false, Message: "贴图动作服务不可用"})
		return
	}
	response := handler(request)
	_ = json.NewEncoder(conn).Encode(response)
}

func (m *Manager) discardCommandLocked() {
	command := m.command
	m.command = nil
	if command == nil || command.Process == nil {
		return
	}
	_ = command.Process.Kill()
	done := make(chan struct{}, 1)
	go func() {
		_ = command.Wait()
		done <- struct{}{}
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
	}
}

func (m *Manager) roundTrip(ctx context.Context, request Request) (Response, error) {
	conn, err := winio.DialPipeContext(ctx, `\\.\pipe\`+m.pipeName)
	if err != nil {
		return Response{}, err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return Response{}, err
	}
	if closeWriter, ok := conn.(interface{ CloseWrite() error }); ok {
		_ = closeWriter.CloseWrite()
	}
	var response Response
	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return Response{}, ctx.Err()
		}
		return Response{}, err
	}
	return response, nil
}

func defaultExePath() string {
	executable, err := os.Executable()
	if err != nil || strings.TrimSpace(executable) == "" {
		return filepath.Join("bin", "native-capture", "Ariadne.CaptureHost.exe")
	}
	return filepath.Join(filepath.Dir(executable), "native-capture", "Ariadne.CaptureHost.exe")
}

func firstErr(err error, fallback error) error {
	if err != nil {
		return err
	}
	return fallback
}
