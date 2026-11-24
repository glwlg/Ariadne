//go:build windows

package nativecapture

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Microsoft/go-winio"
)

// UpdateNetworkMini uses a separate pipe so hiding the mini window never waits
// for an interactive screenshot request or its capture mutex.
func (m *Manager) UpdateNetworkMini(ctx context.Context, state NetworkMiniState) (NetworkMiniReply, error) {
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
