//go:build windows

package nativecapture

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Microsoft/go-winio"
)

// UpdateNetworkMini uses a separate pipe so hiding the mini window never waits
// for an interactive screenshot request or its capture mutex.
func (m *Manager) UpdateNetworkMini(ctx context.Context, state NetworkMiniState) (NetworkMiniReply, error) {
	reply, err := m.dialAndUpdateNetworkMini(ctx, state)
	if err == nil || (ctx != nil && ctx.Err() != nil) {
		return reply, err
	}
	if m.ensureRunningForNetworkMini() {
		return m.dialAndUpdateNetworkMini(ctx, state)
	}
	return reply, err
}

func (m *Manager) dialAndUpdateNetworkMini(ctx context.Context, state NetworkMiniState) (NetworkMiniReply, error) {
	conn, err := winio.DialPipeContext(ctx, `\\.\pipe\`+m.pipeName+"-network-mini")
	if err != nil {
		return NetworkMiniReply{}, err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return NetworkMiniReply{}, err
		}
	}
	if err := json.NewEncoder(conn).Encode(state); err != nil {
		return NetworkMiniReply{}, err
	}
	var reply NetworkMiniReply
	if err := json.NewDecoder(conn).Decode(&reply); err != nil {
		return NetworkMiniReply{}, err
	}
	if !reply.OK {
		return reply, fmt.Errorf("原生网络小窗更新失败: %s", reply.Message)
	}
	return reply, nil
}

func (m *Manager) ensureRunningForNetworkMini() bool {
	if !m.mu.TryLock() {
		return false
	}
	defer m.mu.Unlock()
	if m.closed || m.isCommandAliveLocked() {
		return false
	}
	now := m.now()
	if !m.miniNextRestart.IsZero() && now.Before(m.miniNextRestart) {
		return false
	}
	delay := networkMiniRestartBackoff(m.miniRestartAttempt)
	m.miniRestartAttempt++
	m.miniNextRestart = now.Add(delay)

	start := m.startFn
	if start == nil {
		start = m.startLocked
	}
	if err := start(); err != nil {
		return false
	}
	m.miniRestartAttempt = 0
	m.miniNextRestart = time.Time{}
	return true
}

func (m *Manager) now() time.Time {
	if m.nowFn != nil {
		return m.nowFn()
	}
	return time.Now()
}

func networkMiniRestartBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return time.Second
	}
	shift := min(attempt, 5)
	return min(time.Second<<shift, 30*time.Second)
}
