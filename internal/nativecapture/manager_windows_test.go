//go:build windows

package nativecapture

import (
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestHandlePinActionDoesNotWaitForCaptureMutex(t *testing.T) {
	manager := NewManager(Options{ExePath: "unused"})
	manager.SetPinActionHandler(func(request PinActionRequest) PinActionResponse {
		return PinActionResponse{OK: true, Message: "ok", Text: request.Action}
	})

	server, client := net.Pipe()
	defer client.Close()

	manager.mu.Lock()
	defer manager.mu.Unlock()

	go manager.handlePinAction(server)

	done := make(chan error, 1)
	go func() {
		if err := json.NewEncoder(client).Encode(PinActionRequest{Action: "ocr_text"}); err != nil {
			done <- err
			return
		}
		var response PinActionResponse
		if err := json.NewDecoder(client).Decode(&response); err != nil {
			done <- err
			return
		}
		if !response.OK || response.Text != "ocr_text" {
			done <- &unexpectedPinActionResponse{response: response}
			return
		}
		done <- nil
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("pin action callback waited for capture mutex")
	}
}

type unexpectedPinActionResponse struct {
	response PinActionResponse
}

func (e *unexpectedPinActionResponse) Error() string {
	if raw, err := json.Marshal(e.response); err == nil {
		return "unexpected pin action response: " + string(raw)
	}
	return "unexpected pin action response"
}

func TestDefaultCaptureTimeoutSupportsExtendedAnnotations(t *testing.T) {
	if defaultCaptureTimeout < 10*time.Minute {
		t.Fatalf("expected generous capture timeout for interactive annotations, got %s", defaultCaptureTimeout)
	}
}

func TestCreateKillOnCloseJobObjectSucceeds(t *testing.T) {
	job, err := createKillOnCloseJobObject()
	if err != nil {
		t.Fatalf("createKillOnCloseJobObject failed: %v", err)
	}
	if job == 0 {
		t.Fatal("expected valid Windows Job Object handle")
	}
	// CloseHandle via Stop
	manager := NewManager(Options{ExePath: "unused"})
	manager.job = job
	if err := manager.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if manager.job != 0 {
		t.Fatal("expected Stop to clear and close job handle")
	}
}
