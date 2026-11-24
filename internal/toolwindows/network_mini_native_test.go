package toolwindows

import (
	"context"
	"errors"
	"strings"
	"testing"

	"fancun/internal/nativecapture"
	"fancun/internal/networkmonitor"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestNativeNetworkMiniVisibilityRespectsCloseCaptureAndFullscreen(t *testing.T) {
	for _, visible := range []bool{false, true} {
		for _, capture := range []bool{false, true} {
			for _, fullscreen := range []bool{false, true} {
				for _, autoHide := range []bool{false, true} {
					config := networkMiniConfig{Visible: visible, AutoHideFullscreen: autoHide}
					got := nativeNetworkMiniVisible(config, capture, fullscreen)
					want := visible && !capture && (!fullscreen || !autoHide)
					if got != want {
						t.Fatalf("visible=%t capture=%t fullscreen=%t autoHide=%t: %t", visible, capture, fullscreen, autoHide, got)
					}
				}
			}
		}
	}
}

func TestNativeNetworkMiniPlacementUsesPhysicalPixelsOnScaledSecondaryMonitor(t *testing.T) {
	screen := &application.Screen{
		Bounds:         application.Rect{X: -1280, Y: -100, Width: 1280, Height: 720},
		WorkArea:       application.Rect{X: -1280, Y: -100, Width: 1280, Height: 680},
		PhysicalBounds: application.Rect{X: -1920, Y: -150, Width: 1920, Height: 1080},
	}
	frame := nativeNetworkMiniFrame(screen, "top-left")
	if frame.X != -1911 || frame.Y != -141 || frame.Width != networkMiniWidth*3/2 || frame.Height != networkMiniHeight*3/2 {
		t.Fatalf("scaled placement did not preserve the monitor origin: %#v", frame)
	}
	taskbar := nativeNetworkMiniFrame(screen, "taskbar-left")
	if taskbar.X != -1911 || taskbar.Y < 870 || taskbar.Y+taskbar.Height > 930 {
		t.Fatalf("mini must fit inside the secondary monitor taskbar: %#v", taskbar)
	}
}

func TestNativeNetworkMiniCaptureHideDoesNotSampleTrafficOrUseWebview(t *testing.T) {
	service := NewServiceWithOptions("", nil)
	renderer := &recordingNativeMini{}
	ConfigureNativeNetworkMini(service, renderer, func() networkmonitor.TrafficSnapshot {
		t.Fatal("hidden mini must not sample network traffic")
		return networkmonitor.TrafficSnapshot{}
	})
	service.networkMiniConfig.Visible = true
	service.nativeMiniWasVisible = true
	service.SetCaptureFocusSuspended(true)
	if len(renderer.states) != 1 || renderer.states[0].Visible || !service.NetworkMiniStatus().AutoHidden {
		t.Fatalf("capture did not hide native mini: %#v", renderer.states)
	}
	if !service.NetworkMiniStatus().Visible {
		t.Fatal("temporary capture hiding must preserve saved visibility")
	}
	service.CloseNetworkMini()
	service.SetCaptureFocusSuspended(false)
	if service.NetworkMiniStatus().Visible || len(renderer.states) != 1 {
		t.Fatal("capture restoration reopened a mini closed during capture")
	}
}

func TestNativeNetworkMiniFailedHideIsReportedAndDoesNotClaimSuccess(t *testing.T) {
	service := NewServiceWithOptions("", nil)
	renderer := &recordingNativeMini{err: errors.New("pipe unavailable")}
	ConfigureNativeNetworkMini(service, renderer, nil)
	service.nativeMiniWasVisible = true
	if result := service.CloseNetworkMini(); result.OK {
		t.Fatal("failed native close must be reported")
	}
	if !strings.Contains(service.NetworkMiniStatus().LastError, "pipe unavailable") || !service.nativeMiniWasVisible {
		t.Fatal("failed hide lost its error or was treated as acknowledged")
	}
	renderer.err = nil
	if err := service.syncNativeNetworkMini(); err != nil || service.NetworkMiniStatus().LastError != "" {
		t.Fatalf("successful native hide did not clear the transport error: %v", err)
	}
}

func TestHiddenNativeNetworkMiniDoesNotRequireAStartedHost(t *testing.T) {
	service := NewServiceWithOptions("", nil)
	if err := service.syncNativeNetworkMini(); err != nil {
		t.Fatalf("hidden mini should not contact a native host: %v", err)
	}
}

func TestNativeNetworkMiniCloseActionPersistsDuringCapture(t *testing.T) {
	path := t.TempDir() + "/network_mini_window.json"
	service := NewServiceWithOptions(path, nil)
	renderer := &recordingNativeMini{action: "close"}
	ConfigureNativeNetworkMini(service, renderer, nil)
	service.networkMiniConfig.Visible = true
	service.nativeMiniWasVisible = true
	service.SetCaptureFocusSuspended(true)
	if service.NetworkMiniStatus().Visible {
		t.Fatal("native close action did not clear saved visibility")
	}
	if NewServiceWithOptions(path, nil).NetworkMiniStatus().Visible {
		t.Fatal("native close action did not survive reloading settings")
	}
}

type recordingNativeMini struct {
	states []nativecapture.NetworkMiniState
	err    error
	action string
}

func (r *recordingNativeMini) UpdateNetworkMini(_ context.Context, state nativecapture.NetworkMiniState) (nativecapture.NetworkMiniReply, error) {
	r.states = append(r.states, state)
	action := r.action
	r.action = ""
	return nativecapture.NetworkMiniReply{OK: r.err == nil, Action: action}, r.err
}
