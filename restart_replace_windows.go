//go:build windows

package main

import (
	"fmt"
	"time"

	"golang.org/x/sys/windows"
)

func waitForRestartReplacementProcess(pid int, timeout time.Duration) error {
	if pid <= 0 {
		return nil
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		if errno, ok := err.(windows.Errno); ok && errno == windows.ERROR_INVALID_PARAMETER {
			return nil
		}
		return err
	}
	defer windows.CloseHandle(handle)

	status, err := windows.WaitForSingleObject(handle, waitMilliseconds(timeout))
	if err != nil {
		return err
	}
	if status == uint32(windows.WAIT_OBJECT_0) {
		return nil
	}
	if status != uint32(windows.WAIT_TIMEOUT) {
		return fmt.Errorf("等待旧 Ariadne 进程退出失败: wait status %d", status)
	}
	if err := windows.TerminateProcess(handle, 0); err != nil {
		return err
	}
	_, _ = windows.WaitForSingleObject(handle, waitMilliseconds(3*time.Second))
	return nil
}

func waitMilliseconds(timeout time.Duration) uint32 {
	if timeout <= 0 {
		return 0
	}
	ms := timeout / time.Millisecond
	if ms > time.Duration(^uint32(0)) {
		return ^uint32(0)
	}
	return uint32(ms)
}
