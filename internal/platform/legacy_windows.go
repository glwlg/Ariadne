//go:build windows

package platform

import (
	"os"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmClose          = 0x0010
	statusStillAlive = 259
)

var (
	user32Platform               = windows.NewLazySystemDLL("user32.dll")
	procEnumWindowsPlatform      = user32Platform.NewProc("EnumWindows")
	procGetWindowThreadProcessID = user32Platform.NewProc("GetWindowThreadProcessId")
	procPostMessagePlatform      = user32Platform.NewProc("PostMessageW")
)

type legacyProcessStatus struct {
	Running bool
	ID      int
	Name    string
	Path    string
}

func findLegacyProcess() legacyProcessStatus {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return legacyProcessStatus{}
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	currentPID := os.Getpid()
	for err := windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		name := windows.UTF16ToString(entry.ExeFile[:])
		if !isLegacyProcessName(name) || int(entry.ProcessID) == currentPID {
			continue
		}
		pid := int(entry.ProcessID)
		return legacyProcessStatus{
			Running: true,
			ID:      pid,
			Name:    name,
			Path:    processImagePath(pid),
		}
	}
	return legacyProcessStatus{}
}

func isLegacyProcessName(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), "x-tools.exe")
}

func processImagePath(pid int) string {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(handle)

	buffer := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
		return ""
	}
	return windows.UTF16ToString(buffer[:size])
}

func closeLegacyProcess(request LegacyHandoffRequest, before LegacyRuntimeStatus) legacyHandoffOutcome {
	if before.ProcessID <= 0 {
		return legacyHandoffOutcome{Actions: []string{"无法确认旧版进程 PID"}, Error: "缺少旧版进程 PID"}
	}
	actions := []string{}
	windowCount := postCloseToProcessWindows(before.ProcessID)
	if windowCount > 0 {
		actions = append(actions, "已向旧版窗口发送关闭请求")
	} else {
		actions = append(actions, "未发现旧版可关闭窗口")
	}
	timeout := time.Duration(request.TimeoutMs) * time.Millisecond
	if waitForProcessExit(before.ProcessID, timeout) {
		return legacyHandoffOutcome{
			Actions:        append(actions, "旧版 x-tools 已退出"),
			ProcessExited:  true,
			WindowsReached: windowCount,
		}
	}
	if !request.Force {
		return legacyHandoffOutcome{
			Actions:        append(actions, "旧版 x-tools 未在超时时间内退出"),
			Error:          "旧版 x-tools 未退出",
			WindowsReached: windowCount,
		}
	}
	process, err := os.FindProcess(before.ProcessID)
	if err != nil {
		return legacyHandoffOutcome{
			Actions:        append(actions, "强制结束旧版失败：无法打开进程"),
			Error:          err.Error(),
			ForceUsed:      true,
			WindowsReached: windowCount,
		}
	}
	if err := process.Kill(); err != nil {
		return legacyHandoffOutcome{
			Actions:        append(actions, "强制结束旧版失败"),
			Error:          err.Error(),
			ForceUsed:      true,
			WindowsReached: windowCount,
		}
	}
	if waitForProcessExit(before.ProcessID, timeout) {
		return legacyHandoffOutcome{
			Actions:        append(actions, "已强制结束旧版 x-tools"),
			ProcessExited:  true,
			ForceUsed:      true,
			WindowsReached: windowCount,
		}
	}
	return legacyHandoffOutcome{
		Actions:        append(actions, "已发送强制结束请求，但旧版仍可见"),
		Error:          "旧版 x-tools 仍在运行",
		ForceUsed:      true,
		WindowsReached: windowCount,
	}
}

func postCloseToProcessWindows(pid int) int {
	count := 0
	callback := windows.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		var windowPID uint32
		procGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&windowPID)))
		if int(windowPID) == pid {
			procPostMessagePlatform.Call(hwnd, wmClose, 0, 0)
			count++
		}
		return 1
	})
	procEnumWindowsPlatform.Call(callback, 0)
	return count
}

func waitForProcessExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if !isProcessRunning(pid) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func isProcessRunning(pid int) bool {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)

	var exitCode uint32
	if err := windows.GetExitCodeProcess(handle, &exitCode); err != nil {
		return false
	}
	return exitCode == statusStillAlive
}
