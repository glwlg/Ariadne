//go:build windows

package brandmigration

import (
	"errors"
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func checkLegacyStopped() error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		name := windows.UTF16ToString(entry.ExeFile[:])
		for _, legacy := range []string{"ariadne.exe", "Ariadne.CaptureHost.exe", "Ariadne.iPhoneControl.exe"} {
			if strings.EqualFold(name, legacy) {
				return fmt.Errorf("请先退出 Ariadne 并停止旧版文件索引服务，再启动 Fancun 迁移数据")
			}
		}
	}
	if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil
	}
	return err
}

// NotifyFailure is visible even when Fancun is built without a console window.
func NotifyFailure(err error) {
	title, _ := windows.UTF16PtrFromString("方寸 Fancun")
	message, convertErr := windows.UTF16PtrFromString("数据迁移未完成。\n\n" + err.Error())
	if convertErr != nil {
		return
	}
	windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW").Call(
		0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), 0x00010010)
}
