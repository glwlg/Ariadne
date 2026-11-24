package toolwindows

import (
	"context"
	"errors"
	"math"
	"runtime"
	"strings"
	"time"

	"fancun/internal/nativecapture"
	"fancun/internal/networkmonitor"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type nativeNetworkMini interface {
	UpdateNetworkMini(context.Context, nativecapture.NetworkMiniState) (nativecapture.NetworkMiniReply, error)
}

// ConfigureNativeNetworkMini must run before Attach restores persisted windows.
func ConfigureNativeNetworkMini(s *Service, renderer nativeNetworkMini, snapshot func() networkmonitor.TrafficSnapshot) {
	s.nativeMiniRenderer = renderer
	s.nativeMiniSnapshot = snapshot
}

func (s *Service) usesNativeNetworkMini() bool {
	return runtime.GOOS == "windows" || s.nativeMiniRenderer != nil
}

func (s *Service) syncNativeNetworkMini() error {
	s.nativeMiniMu.Lock()
	reply, err := s.updateNativeNetworkMini()
	s.nativeMiniMu.Unlock()
	s.mu.Lock()
	if err != nil {
		s.networkMiniError = "原生网络小窗: " + err.Error()
	} else if strings.HasPrefix(s.networkMiniError, "原生网络小窗:") {
		s.networkMiniError = ""
	}
	s.mu.Unlock()
	if err != nil {
		return err
	}
	switch reply.Action {
	case "open-center":
		s.Open("network-monitor")
	case "close":
		s.CloseNetworkMini()
	}
	return nil
}

// Called with nativeMiniMu held; this serializes capture hiding, close and
// telemetry updates so a stale visible frame cannot reopen a hidden window.
func (s *Service) updateNativeNetworkMini() (nativecapture.NetworkMiniReply, error) {
	s.mu.RLock()
	config := s.networkMiniConfig
	app := s.app
	visible := nativeNetworkMiniVisible(config, s.captureFocusPaused, s.fullscreenActive)
	s.mu.RUnlock()
	if !visible && !s.nativeMiniWasVisible {
		return nativecapture.NetworkMiniReply{}, nil
	}
	if s.nativeMiniRenderer == nil {
		return nativecapture.NetworkMiniReply{}, errors.New("原生组件尚未就绪")
	}
	state := nativecapture.NetworkMiniState{Visible: visible}
	if visible {
		screen := screenForNetworkMini(app, config)
		if screen == nil || screen.Bounds.Width <= 0 || screen.Bounds.Height <= 0 ||
			screen.PhysicalBounds.Width <= 0 || screen.PhysicalBounds.Height <= 0 {
			return nativecapture.NetworkMiniReply{}, errors.New("屏幕信息尚未就绪")
		}
		if s.nativeMiniSnapshot == nil {
			return nativecapture.NetworkMiniReply{}, errors.New("网络监控服务尚未就绪")
		}
		state = nativeNetworkMiniFrame(screen, config.Anchor)
		state.Visible = true
		if time.Since(s.nativeMiniSampleAt) >= time.Second {
			s.nativeMiniTraffic = s.nativeMiniSnapshot()
			s.nativeMiniSampleAt = time.Now()
		}
		state.UploadBytesPerSecond = s.nativeMiniTraffic.UploadBytesPerSecond
		state.DownloadBytesPerSecond = s.nativeMiniTraffic.DownloadBytesPerSecond
		state.Error = s.nativeMiniTraffic.LastError
		state.AdapterName = "未检测到网卡"
		for _, adapter := range s.nativeMiniTraffic.Adapters {
			state.AdapterName = firstNonEmpty(adapter.Alias, adapter.Name)
			if adapter.Operational {
				break
			}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	reply, err := s.nativeMiniRenderer.UpdateNetworkMini(ctx, state)
	if err == nil {
		s.nativeMiniWasVisible = visible
		s.mu.Lock()
		s.networkMiniHidden = config.Visible && !visible
		s.mu.Unlock()
	}
	return reply, err
}

func nativeNetworkMiniVisible(config networkMiniConfig, capturePaused bool, fullscreen bool) bool {
	return config.Visible && !capturePaused && !(config.AutoHideFullscreen && fullscreen)
}

// Wails screen layout is in logical coordinates; SetWindowPos in the per-monitor
// DPI-aware WPF host takes physical pixels. Scale relative to each monitor's
// origin, including monitors with negative origins and different DPI.
func nativeNetworkMiniFrame(screen *application.Screen, anchor string) nativecapture.NetworkMiniState {
	frame := networkMiniFrame(screen, anchor, networkMiniWidth, networkMiniHeight)
	x, y := networkMiniAbsolutePosition(screen, frame)
	logical := screen.Bounds
	physical := screen.PhysicalBounds
	scaleX := float64(physical.Width) / float64(logical.Width)
	scaleY := float64(physical.Height) / float64(logical.Height)
	return nativecapture.NetworkMiniState{
		X:      physical.X + int(math.Round(float64(x-logical.X)*scaleX)),
		Y:      physical.Y + int(math.Round(float64(y-logical.Y)*scaleY)),
		Width:  int(math.Round(float64(frame.Width) * scaleX)),
		Height: int(math.Round(float64(frame.Height) * scaleY)),
	}
}
