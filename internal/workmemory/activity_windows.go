//go:build windows

package workmemory

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

var (
	activityUser32       = syscall.NewLazyDLL("user32.dll")
	activityKernel32     = syscall.NewLazyDLL("kernel32.dll")
	procGetLastInputInfo = activityUser32.NewProc("GetLastInputInfo")
	procOpenInputDesktop = activityUser32.NewProc("OpenInputDesktop")
	procSwitchDesktop    = activityUser32.NewProc("SwitchDesktop")
	procCloseDesktop     = activityUser32.NewProc("CloseDesktop")
	procGetTickCount64   = activityKernel32.NewProc("GetTickCount64")
)

const desktopSwitchDesktop = 0x0100

type lastInputInfo struct {
	Size uint32
	Time uint32
}

func defaultActivityProvider() activityProvider {
	return activityProviderFunc(func(now time.Time) activitySnapshot {
		snapshot := activitySnapshot{Available: true}
		idleSeconds, lastActivityAt, err := windowsIdleSeconds(now)
		if err != nil {
			snapshot.Error = err.Error()
		} else {
			snapshot.IdleSeconds = idleSeconds
			snapshot.LastActivityAt = lastActivityAt
		}
		locked, lockErr := windowsSessionLocked()
		if lockErr != nil && !locked && snapshot.Error == "" {
			snapshot.Error = lockErr.Error()
		}
		snapshot.SessionLocked = locked
		return snapshot
	})
}

func windowsIdleSeconds(now time.Time) (int, int64, error) {
	info := lastInputInfo{Size: uint32(unsafe.Sizeof(lastInputInfo{}))}
	ok, _, err := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		return 0, 0, fmt.Errorf("GetLastInputInfo 失败: %v", err)
	}
	tick, _, _ := procGetTickCount64.Call()
	if tick == 0 {
		return 0, 0, fmt.Errorf("GetTickCount64 失败")
	}
	idleMilliseconds := int64(tick) - int64(info.Time)
	if idleMilliseconds < 0 {
		idleMilliseconds = 0
	}
	idleSeconds := int(idleMilliseconds / 1000)
	return idleSeconds, now.Add(-time.Duration(idleMilliseconds) * time.Millisecond).Unix(), nil
}

func windowsSessionLocked() (bool, error) {
	desktop, _, err := procOpenInputDesktop.Call(0, 0, desktopSwitchDesktop)
	if desktop == 0 {
		return true, fmt.Errorf("OpenInputDesktop 失败: %v", err)
	}
	defer procCloseDesktop.Call(desktop)
	ok, _, _ := procSwitchDesktop.Call(desktop)
	return ok == 0, nil
}
