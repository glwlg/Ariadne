//go:build windows

package nativecapture

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
)

func TestNetworkMiniUpdateDoesNotWaitForCaptureMutex(t *testing.T) {
	manager := NewManager(Options{PipeName: "fancun-mini-test-" + strconv.Itoa(os.Getpid())})
	listener, err := winio.ListenPipe(`\\.\pipe\`+manager.pipeName+"-network-mini", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(time.Second))
		var state NetworkMiniState
		if err := json.NewDecoder(conn).Decode(&state); err != nil {
			done <- err
			return
		}
		if state.Visible || state.UploadBytesPerSecond != 123 {
			done <- errors.New("unexpected mini state")
			return
		}
		done <- json.NewEncoder(conn).Encode(NetworkMiniReply{OK: true, Action: "close"})
	}()
	manager.mu.Lock()
	defer manager.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	reply, err := manager.UpdateNetworkMini(ctx, NetworkMiniState{UploadBytesPerSecond: 123})
	if err != nil || !reply.OK || reply.Action != "close" {
		t.Fatalf("independent native update failed: %#v, %v", reply, err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
