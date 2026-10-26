package airplay

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"ariadne/internal/contracts"
)

type ControlStatus struct {
	Phase            string `json:"phase"`
	Running          bool   `json:"running"`
	ProcessID        int    `json:"processId"`
	Advertising      bool   `json:"advertising"`
	ConnectedDevices int    `json:"connectedDevices"`
	TargetName       string `json:"targetName"`
	Readiness        int    `json:"readiness"`
	LastError        string `json:"lastError"`
	Diagnostics      string `json:"diagnostics"`
}

type controlProcess struct {
	cmd     *exec.Cmd
	done    chan struct{}
	started time.Time
	err     error
}

func (s *Service) controlStatus(installed bool) ControlStatus {
	if !installed {
		return ControlStatus{Phase: "not-installed"}
	}
	status, err := s.queryControl()
	if err == nil {
		return status
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.controlProcess != nil {
		if time.Since(s.controlProcess.started) < 5*time.Second && errors.Is(err, os.ErrNotExist) {
			return ControlStatus{Phase: "starting", Running: true, ProcessID: s.controlProcess.cmd.Process.Pid}
		}
		return ControlStatus{Phase: "unavailable", Running: true, ProcessID: s.controlProcess.cmd.Process.Pid,
			LastError: "无法读取手机控制状态：" + err.Error()}
	}
	if s.controlError != "" {
		return ControlStatus{Phase: "exited", LastError: s.controlError}
	}
	if !errors.Is(err, os.ErrNotExist) {
		return ControlStatus{Phase: "unavailable", LastError: "无法读取手机控制状态：" + err.Error()}
	}
	return ControlStatus{Phase: "stopped"}
}

func (s *Service) controlExecutable() string {
	return filepath.Join(filepath.Dir(s.runtimeDir), "iphone-control", "Ariadne.iPhoneControl.exe")
}

func (s *Service) OpenControl() contracts.ActionResult {
	s.operations.Lock()
	defer s.operations.Unlock()
	return s.startControlLocked(false)
}

func (s *Service) startControlLocked(background bool) contracts.ActionResult {
	if background {
		s.mu.Lock()
		running := s.controlProcess != nil
		s.mu.Unlock()
		if running {
			return contracts.ActionResult{OK: true}
		}
		if status, err := s.queryControl(); err == nil && status.Running {
			return contracts.ActionResult{OK: true}
		}
	}
	info, err := os.Stat(s.controlExecutable())
	if err != nil || info.IsDir() {
		return contracts.ActionResult{Message: "手机控制组件缺失，请重新安装 Ariadne。"}
	}
	args := []string{"--owner-pid", strconv.Itoa(os.Getpid())}
	if background {
		args = append(args, "--background")
	}
	cmd := s.command(s.controlExecutable(), args...)
	hideConsole(cmd)
	if err := cmd.Start(); err != nil {
		s.mu.Lock()
		s.controlError = "打开手机控制失败：" + err.Error()
		s.mu.Unlock()
		return contracts.ActionResult{Message: "打开手机控制失败：" + err.Error()}
	}
	p := &controlProcess{cmd: cmd, done: make(chan struct{}), started: time.Now()}
	s.mu.Lock()
	if s.controlProcess == nil {
		s.controlProcess = p
		s.controlError = ""
	}
	s.mu.Unlock()
	go func() {
		err := cmd.Wait()
		s.mu.Lock()
		p.err = err
		if s.controlProcess == p {
			s.controlProcess = nil
			if err != nil {
				s.controlError = "手机控制组件已退出：" + err.Error()
			}
		}
		close(p.done)
		s.mu.Unlock()
	}()
	// A second invocation activates the existing control window and exits normally.
	select {
	case <-p.done:
		if p.err != nil {
			return contracts.ActionResult{Message: fmt.Sprintf("手机控制组件已退出：%v，请查看 control.log。", p.err)}
		}
	case <-time.After(300 * time.Millisecond):
	}
	return contracts.ActionResult{OK: true, Message: "手机控制已启动，已连接的手机会自动恢复控制；关闭控制窗口后仍可操作投屏。"}
}

func (s *Service) Diagnostics() (string, error) {
	status := s.Status()
	status.PIN = ""
	raw, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return "", err
	}
	text := "Ariadne · iPhone 投屏诊断\n" + string(raw)
	for _, log := range []struct{ name, path string }{
		{"接收日志", status.LogPath},
		{"控制日志", filepath.Join(s.dataDir, "control.log")},
	} {
		content, err := ReadLog(log.path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("读取%s：%w", log.name, err)
		}
		text += "\n\n" + log.name + "\n" + content
	}
	return text, nil
}
