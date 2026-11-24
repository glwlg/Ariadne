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

func TestNetworkMiniRestartsDeadHostWithBackoff(t *testing.T) {
	now := time.Unix(1700000000, 0)
	manager := NewManager(Options{PipeName: "fancun-mini-restart-test"})
	manager.nowFn = func() time.Time { return now }

	starts := 0
	manager.startFn = func() error {
		starts++
		if starts == 1 {
			return errors.New("start failed")
		}
		return nil
	}

	// First failed attempt sets 1s backoff.
	if manager.ensureRunningForNetworkMini() {
		t.Fatal("first failing start should return false")
	}
	if starts != 1 {
		t.Fatalf("expected 1 start attempt, got %d", starts)
	}

	// Immediate retry within backoff must be skipped.
	now = now.Add(500 * time.Millisecond)
	if manager.ensureRunningForNetworkMini() {
		t.Fatal("retry inside backoff window should be skipped")
	}
	if starts != 1 {
		t.Fatalf("expected backoff to suppress second attempt, got %d", starts)
	}

	// After backoff elapses, retry succeeds and resets attempt counter.
	now = now.Add(600 * time.Millisecond)
	if !manager.ensureRunningForNetworkMini() {
		t.Fatal("expected retry after backoff to succeed")
	}
	if starts != 2 || manager.miniRestartAttempt != 0 {
		t.Fatalf("expected backoff reset after success, starts=%d attempts=%d", starts, manager.miniRestartAttempt)
	}

	// Stop blocks further restarts.
	_ = manager.Stop()
	if manager.ensureRunningForNetworkMini() {
		t.Fatal("stopped manager must not restart host")
	}
}
