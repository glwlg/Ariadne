//go:build !windows

package nativecapture

import (
	"context"
	"errors"
)

func (m *Manager) UpdateNetworkMini(ctx context.Context, state NetworkMiniState) (NetworkMiniReply, error) {
	return NetworkMiniReply{}, errors.New("native network mini is only available on Windows")
}
