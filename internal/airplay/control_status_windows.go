//go:build windows

package airplay

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

func queryControlStatus() (ControlStatus, error) {
	var session uint32
	if err := windows.ProcessIdToSessionId(uint32(os.Getpid()), &session); err != nil {
		return ControlStatus{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	conn, err := winio.DialPipeContext(ctx, fmt.Sprintf(`\\.\pipe\ariadne-iphone-control-%d`, session))
	if err != nil {
		return ControlStatus{}, err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return ControlStatus{}, err
		}
	}
	var status ControlStatus
	if err := json.NewDecoder(io.LimitReader(conn, 32*1024)).Decode(&status); err != nil {
		return ControlStatus{}, err
	}
	if status.ProcessID <= 0 || !status.Running || status.Phase == "" {
		return ControlStatus{}, fmt.Errorf("手机控制组件返回了无效状态")
	}
	return status, nil
}
