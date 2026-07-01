//go:build windows

package elevation

import (
	"fmt"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	seeMaskNoCloseProcess = 0x00000040
	swShowNormal          = 1
	waitTimeout           = 0x00000102
)

const elevatedProcessTimeout = 2 * time.Minute

var shell32Elevation = windows.NewLazySystemDLL("shell32.dll")
var procShellExecuteExW = shell32Elevation.NewProc("ShellExecuteExW")

type shellExecuteInfo struct {
	cbSize       uint32
	fMask        uint32
	hwnd         windows.Handle
	lpVerb       *uint16
	lpFile       *uint16
	lpParameters *uint16
	lpDirectory  *uint16
	nShow        int32
	hInstApp     windows.Handle
	lpIDList     uintptr
	lpClass      *uint16
	hkeyClass    windows.Handle
	dwHotKey     uint32
	hIconMonitor windows.Handle
	hProcess     windows.Handle
}

func RunasWait(file string, args []string) error {
	file = strings.TrimSpace(file)
	if file == "" {
		return fmt.Errorf("缺少程序路径")
	}
	verbPtr, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	filePtr, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return err
	}
	paramsPtr, err := windows.UTF16PtrFromString(commandLine(args))
	if err != nil {
		return err
	}
	info := shellExecuteInfo{
		cbSize:       uint32(unsafe.Sizeof(shellExecuteInfo{})),
		fMask:        shellExecuteMask(),
		lpVerb:       verbPtr,
		lpFile:       filePtr,
		lpParameters: paramsPtr,
		nShow:        shellExecuteShowCommand(),
	}
	ret, _, callErr := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		if callErr == windows.ERROR_CANCELLED {
			return fmt.Errorf("用户取消了管理员授权")
		}
		if callErr != windows.ERROR_SUCCESS {
			return callErr
		}
		return fmt.Errorf("请求管理员授权失败")
	}
	if info.hProcess == 0 {
		return nil
	}
	defer windows.CloseHandle(info.hProcess)
	waitResult, err := windows.WaitForSingleObject(info.hProcess, uint32(elevatedProcessTimeout/time.Millisecond))
	if err != nil {
		return err
	}
	if waitResult == waitTimeout {
		return fmt.Errorf("等待管理员授权进程超时")
	}
	var exitCode uint32
	if err := windows.GetExitCodeProcess(info.hProcess, &exitCode); err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("提权进程退出码 %d", exitCode)
	}
	return nil
}

func shellExecuteMask() uint32 {
	return seeMaskNoCloseProcess
}

func shellExecuteShowCommand() int32 {
	return swShowNormal
}

func commandLine(args []string) string {
	escaped := make([]string, 0, len(args))
	for _, arg := range args {
		escaped = append(escaped, windows.EscapeArg(arg))
	}
	return strings.Join(escaped, " ")
}
