package airplay

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ariadne/internal/contracts"
	"ariadne/internal/elevation"
)

type Status struct {
	Ready        bool          `json:"ready"`
	ControlReady bool          `json:"controlReady"`
	Running      bool          `json:"running"`
	ReceiverName string        `json:"receiverName"`
	PIN          string        `json:"pin"`
	ProcessID    int           `json:"processId"`
	LogPath      string        `json:"logPath"`
	LastError    string        `json:"lastError"`
	Phase        string        `json:"phase"`
	FrameRate    int           `json:"frameRate"`
	Width        int           `json:"width"`
	Height       int           `json:"height"`
	ContentTitle string        `json:"contentTitle"`
	ClientName   string        `json:"clientName"`
	Control      ControlStatus `json:"control"`
}

type receiverProcess struct {
	cmd      *exec.Cmd
	done     chan struct{}
	stopping bool
	pin      string
	stream   videoEvent
}

type Service struct {
	operations       sync.Mutex
	mu               sync.Mutex
	runtimeDir       string
	dataDir          string
	name             string
	initError        string
	preferencesError string
	lastError        string
	process          *receiverProcess
	frameRate        int
	command          func(string, ...string) *exec.Cmd
	elevate          func(string, []string) error
	queryControl     func() (ControlStatus, error)
	controlProcess   *controlProcess
	controlError     string
}

func NewService() *Service {
	executable, exeErr := os.Executable()
	configDir, configErr := os.UserConfigDir()
	s := newService(filepath.Join(filepath.Dir(executable), "uxplay"), filepath.Join(configDir, "Ariadne", "airplay"))
	if err := errors.Join(exeErr, configErr); err != nil {
		s.initError = err.Error()
	}
	return s
}

func newService(runtimeDir, dataDir string) *Service {
	hostname, err := os.Hostname()
	initError := ""
	if err != nil {
		initError = err.Error()
	}
	s := &Service{
		runtimeDir:   runtimeDir,
		dataDir:      dataDir,
		name:         "Ariadne · " + hostname,
		initError:    initError,
		command:      exec.Command,
		elevate:      elevation.RunasWait,
		frameRate:    30,
		queryControl: queryControlStatus,
	}
	if err := s.loadPreferences(); err != nil {
		s.preferencesError = err.Error()
	}
	return s
}

func (s *Service) Status() Status {
	s.mu.Lock()
	info, err := os.Stat(filepath.Join(s.runtimeDir, "uxplay.exe"))
	status := Status{
		Ready:        s.initError == "" && s.preferencesError == "" && err == nil && !info.IsDir(),
		ReceiverName: s.name,
		LogPath:      filepath.Join(s.dataDir, "uxplay.log"),
		LastError:    s.lastError,
		Phase:        "stopped",
		FrameRate:    s.frameRate,
	}
	controlInfo, controlErr := os.Stat(s.controlExecutable())
	status.ControlReady = controlErr == nil && !controlInfo.IsDir()
	if s.initError != "" {
		status.LastError = s.initError
	}
	if s.preferencesError != "" {
		status.LastError = s.preferencesError
	}
	if p := s.process; p != nil {
		status.Running = true
		status.PIN = p.pin
		status.ProcessID = p.cmd.Process.Pid
		status.Phase = p.stream.Phase
		status.Width, status.Height = p.stream.Width, p.stream.Height
		status.ContentTitle, status.ClientName = p.stream.Title, p.stream.Client
	}
	s.mu.Unlock()
	status.Control = s.controlStatus(status.ControlReady)
	return status
}

func (s *Service) Execute(command string) contracts.ActionResult {
	switch strings.ToLower(strings.TrimSpace(command)) {
	case "start":
		return s.Start()
	case "stop":
		return s.Stop()
	case "control":
		return s.OpenControl()
	case "fps 30":
		return s.SetFrameRate(30)
	case "fps 60":
		return s.SetFrameRate(60)
	case "status":
		status := s.Status()
		if status.Running {
			return contracts.ActionResult{OK: true, Message: fmt.Sprintf("%s：%s，配对码 %s", status.StateLabel(), status.ReceiverName, status.PIN)}
		}
		if status.LastError != "" {
			return contracts.ActionResult{Message: "投屏已停止：" + status.LastError}
		}
		if !status.Ready {
			return contracts.ActionResult{Message: "投屏组件缺失，请重新安装 Ariadne。"}
		}
		return contracts.ActionResult{OK: true, Message: "投屏接收服务未启动"}
	case "firewall":
		if !s.Status().Ready {
			return contracts.ActionResult{Message: "投屏组件缺失，请重新安装 Ariadne。"}
		}
		path := strings.ReplaceAll(filepath.Join(s.runtimeDir, "uxplay.exe"), "'", "''")
		script := "$ErrorActionPreference='Stop'; " +
			"$rule=Get-NetFirewallRule -Name 'Ariadne-UxPlay' -ErrorAction SilentlyContinue; " +
			"if ($rule) { $rule | Remove-NetFirewallRule }; " +
			"New-NetFirewallRule -Name 'Ariadne-UxPlay' -DisplayName 'Ariadne iPhone 投屏' " +
			"-Direction Inbound -Action Allow -Enabled True -Profile Private -RemoteAddress LocalSubnet -Program '" + path + "' | Out-Null"
		if err := s.elevate("powershell.exe", []string{"-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script}); err != nil {
			return contracts.ActionResult{Message: "允许局域网投屏失败：" + err.Error()}
		}
		return contracts.ActionResult{OK: true, Message: "已允许专用网络内的设备投屏"}
	default:
		return contracts.ActionResult{Message: "未知投屏命令"}
	}
}

func (s *Service) Start() contracts.ActionResult {
	s.operations.Lock()
	defer s.operations.Unlock()
	return s.startLocked()
}

func (s *Service) startLocked() contracts.ActionResult {
	status := s.Status()
	if status.Running {
		return contracts.ActionResult{OK: true, Message: "接收服务已在运行，配对码 " + status.PIN}
	}
	if !status.Ready {
		if status.LastError != "" {
			return contracts.ActionResult{Message: "启动投屏失败：" + status.LastError}
		}
		return contracts.ActionResult{Message: "投屏组件缺失，请重新安装 Ariadne。"}
	}
	if err := os.MkdirAll(s.dataDir, 0o700); err != nil {
		return s.startFailure(err)
	}
	// UTF-8 config avoids MinGW's Windows command-line code page for receiver names.
	configPath := filepath.Join(s.dataDir, "receiver.conf")
	if err := os.WriteFile(configPath, []byte(fmt.Sprintf("n \"%s\"\n", s.name)), 0o600); err != nil {
		return s.startFailure(err)
	}
	number, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		return s.startFailure(err)
	}
	pin := fmt.Sprintf("%04d", number.Int64()+1000)
	logFile, err := os.OpenFile(status.LogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return s.startFailure(err)
	}
	cmd := s.command(filepath.Join(s.runtimeDir, "uxplay.exe"),
		"-rc", "receiver.conf", "-nh", "-pin", pin,
		"-vs", "d3d11videosink fullscreen-toggle-mode=none", "-as", "wasapisink", "-nofreeze",
		"-fps", fmt.Sprint(status.FrameRate))
	cmd.Dir = s.dataDir
	cmd.Env = s.environment()
	p := &receiverProcess{cmd: cmd, done: make(chan struct{}), pin: pin, stream: videoEvent{Phase: "waiting"}}
	events := &videoEventWriter{accept: func(event videoEvent) {
		s.acceptVideoEvent(p, event)
	}}
	cmd.Stdout, cmd.Stderr = io.MultiWriter(logFile, events), logFile
	hideConsole(cmd)
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return s.startFailure(err)
	}
	s.mu.Lock()
	s.lastError = ""
	s.process = p
	s.mu.Unlock()
	go func() {
		err := cmd.Wait()
		logFile.Close()
		s.mu.Lock()
		if !p.stopping {
			s.lastError = "UxPlay 已退出"
			if err != nil {
				s.lastError += "：" + err.Error()
			}
		}
		s.process = nil
		close(p.done)
		s.mu.Unlock()
	}()
	// Catch missing DLLs and startup failures before reporting that the process is running.
	select {
	case <-p.done:
		return contracts.ActionResult{Message: s.Status().LastError + "，请查看投屏日志。"}
	case <-time.After(time.Second):
		return contracts.ActionResult{OK: true, Message: "接收服务已启动，配对码 " + pin + "；在 iPhone「屏幕镜像」中选择 " + s.name}
	}
}

func (s *Service) acceptVideoEvent(p *receiverProcess, event videoEvent) {
	s.mu.Lock()
	connected := event.Phase == "mirroring" && p.stream.Phase != "mirroring"
	p.stream = event
	s.mu.Unlock()
	if connected {
		// Do not block UxPlay's output pipe while starting Bluetooth.
		go func() {
			s.operations.Lock()
			defer s.operations.Unlock()
			s.mu.Lock()
			active := s.process == p && !p.stopping && p.stream.Phase == "mirroring"
			s.mu.Unlock()
			if active {
				s.startControlLocked(true)
			}
		}()
	}
}

func (s *Service) Stop() contracts.ActionResult {
	s.operations.Lock()
	defer s.operations.Unlock()
	return s.stopLocked()
}

func (s *Service) stopLocked() contracts.ActionResult {
	s.mu.Lock()
	p := s.process
	if p == nil {
		s.mu.Unlock()
		return contracts.ActionResult{OK: true, Message: "投屏接收服务已停止"}
	}
	p.stopping = true
	s.mu.Unlock()
	if err := p.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		s.mu.Lock()
		p.stopping = false
		s.mu.Unlock()
		return contracts.ActionResult{Message: "停止投屏失败：" + err.Error()}
	}
	select {
	case <-p.done:
		return contracts.ActionResult{OK: true, Message: "投屏接收服务已停止"}
	case <-time.After(5 * time.Second):
		return contracts.ActionResult{Message: "等待投屏进程退出超时"}
	}
}

func (s *Service) startFailure(err error) contracts.ActionResult {
	s.mu.Lock()
	s.lastError = err.Error()
	s.mu.Unlock()
	return contracts.ActionResult{Message: "启动投屏失败：" + err.Error()}
}

func (s *Service) environment() []string {
	environment := os.Environ()
	// Keep GStreamer discovery and its cache inside the packaged runtime and user data.
	for key, value := range map[string]string{
		"PATH":                       s.runtimeDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"GST_PLUGIN_SYSTEM_PATH_1_0": filepath.Join(s.runtimeDir, "lib", "gstreamer-1.0"),
		"GST_PLUGIN_PATH_1_0":        "",
		"GST_PLUGIN_PATH":            "",
		"GST_PLUGIN_SCANNER_1_0":     filepath.Join(s.runtimeDir, "libexec", "gstreamer-1.0", "gst-plugin-scanner.exe"),
		"GST_REGISTRY_1_0":           filepath.Join(s.dataDir, "gstreamer-registry.bin"),
	} {
		prefix := key + "="
		env := environment[:0]
		for _, item := range environment {
			if !strings.EqualFold(strings.SplitN(item, "=", 2)[0], key) {
				env = append(env, item)
			}
		}
		environment = append(env, prefix+value)
	}
	return environment
}

func (s *Service) Logs() (string, error) {
	text, err := ReadLog(s.Status().LogPath)
	if os.IsNotExist(err) {
		return "", nil
	}
	return text, err
}

func ReadLog(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	// Keep diagnostics bounded even during a long mirroring session.
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if info.Size() > 4096 {
		if _, err := file.Seek(-4096, io.SeekEnd); err != nil {
			return "", err
		}
	}
	raw, err := io.ReadAll(io.LimitReader(file, 4096))
	return strings.ToValidUTF8(string(raw), ""), err
}
