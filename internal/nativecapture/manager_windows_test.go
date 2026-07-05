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
	raw, _ := json.Marshal(e.response)
	return "unexpected pin action response: " + string(raw)
}
