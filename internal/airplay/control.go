package airplay

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"ariadne/internal/contracts"
)

func (s *Service) controlExecutable() string {
	return filepath.Join(filepath.Dir(s.runtimeDir), "iphone-control", "Ariadne.iPhoneControl.exe")
}

func (s *Service) OpenControl() contracts.ActionResult {
	info, err := os.Stat(s.controlExecutable())
	if err != nil || info.IsDir() {
		return contracts.ActionResult{Message: "手机控制组件缺失，请重新安装 Ariadne。"}
	}
	cmd := s.command(s.controlExecutable(), "--owner-pid", strconv.Itoa(os.Getpid()))
	hideConsole(cmd)
	if err := cmd.Start(); err != nil {
		return contracts.ActionResult{Message: "打开手机控制失败：" + err.Error()}
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	// A second invocation activates the existing control window and exits normally.
	select {
	case err := <-done:
		if err != nil {
			return contracts.ActionResult{Message: fmt.Sprintf("手机控制组件已退出：%v，请查看 control.log。", err)}
		}
	case <-time.After(300 * time.Millisecond):
	}
	return contracts.ActionResult{OK: true, Message: "已打开手机控制窗；请在 iPhone 开启辅助触控并完成蓝牙配对。"}
}
