package airplay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"ariadne/internal/contracts"
)

type videoEvent struct {
	Phase  string `json:"phase"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Title  string `json:"title"`
	Client string `json:"client"`
}

type videoEventWriter struct {
	mu         sync.Mutex
	pending    []byte
	discarding bool
	accept     func(videoEvent)
}

func (w *videoEventWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	size := len(data)
	for len(data) > 0 {
		end := bytes.IndexByte(data, '\n')
		if end < 0 {
			end = len(data)
		}
		if len(w.pending)+end > 8192 {
			w.discarding = true
			w.pending = nil
		}
		if !w.discarding {
			w.pending = append(w.pending, data[:end]...)
		}
		if end == len(data) {
			break
		}
		if !w.discarding {
			w.parse(w.pending)
		}
		w.pending = nil
		w.discarding = false
		data = data[end+1:]
	}
	return size, nil
}

func (w *videoEventWriter) parse(line []byte) {
	payload, ok := bytes.CutPrefix(line, []byte("ARIADNE_VIDEO "))
	if !ok {
		return
	}
	var event videoEvent
	if json.Unmarshal(payload, &event) != nil ||
		(event.Phase != "mirroring" && event.Phase != "disconnected") ||
		event.Width < 0 || event.Height < 0 || event.Width > 32767 || event.Height > 32767 ||
		len(event.Title) > 4096 || len(event.Client) > 1024 {
		return
	}
	if event.Phase == "mirroring" && (event.Width == 0 || event.Height == 0) {
		return
	}
	if event.Phase == "disconnected" {
		event.Width, event.Height, event.Title, event.Client = 0, 0, "", ""
	}
	w.accept(event)
}

func (status Status) StateLabel() string {
	if !status.Ready {
		if status.LastError != "" {
			return "投屏不可用"
		}
		return "投屏组件缺失"
	}
	if !status.Running {
		return "未启动"
	}
	switch status.Phase {
	case "mirroring":
		return "正在投屏"
	case "disconnected":
		return "连接已断开，等待重新连接"
	default:
		return "等待 iPhone 连接"
	}
}

func (s *Service) loadPreferences() error {
	raw, err := os.ReadFile(filepath.Join(s.dataDir, "preferences.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取投屏设置失败：%w", err)
	}
	var preferences struct {
		FrameRate int `json:"frameRate"`
	}
	if err := json.Unmarshal(raw, &preferences); err != nil {
		return fmt.Errorf("读取投屏设置失败：%w", err)
	}
	if preferences.FrameRate != 30 && preferences.FrameRate != 60 {
		return fmt.Errorf("投屏帧率必须为 30 或 60")
	}
	s.frameRate = preferences.FrameRate
	return nil
}

func (s *Service) SetFrameRate(fps int) contracts.ActionResult {
	s.operations.Lock()
	defer s.operations.Unlock()
	if fps != 30 && fps != 60 {
		return contracts.ActionResult{Message: "投屏帧率必须为 30 或 60"}
	}
	status := s.Status()
	if status.FrameRate == fps && s.preferencesError == "" {
		return contracts.ActionResult{OK: true, Message: fmt.Sprintf("当前已设为 %d fps", fps)}
	}
	if status.Running {
		if result := s.stopLocked(); !result.OK {
			return result
		}
	}
	if err := os.MkdirAll(s.dataDir, 0o700); err != nil {
		return contracts.ActionResult{Message: "保存投屏设置失败：" + err.Error()}
	}
	raw := []byte(fmt.Sprintf("{\"frameRate\":%d}\n", fps))
	if err := os.WriteFile(filepath.Join(s.dataDir, "preferences.json"), raw, 0o600); err != nil {
		return contracts.ActionResult{Message: "保存投屏设置失败：" + err.Error()}
	}
	s.mu.Lock()
	s.frameRate = fps
	s.preferencesError = ""
	s.mu.Unlock()
	message := fmt.Sprintf("已设为 %d fps", fps)
	if status.Running {
		if result := s.startLocked(); !result.OK {
			return result
		}
		message += "，请在 iPhone 重新连接投屏"
	}
	return contracts.ActionResult{OK: true, Message: message}
}
