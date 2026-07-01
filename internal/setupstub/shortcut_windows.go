//go:build windows

package setupstub

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"golang.org/x/sys/windows"
)

var (
	shell32SetupStub            = windows.NewLazySystemDLL("shell32.dll")
	procSHChangeNotifySetupStub = shell32SetupStub.NewProc("SHChangeNotify")
)

func createShortcut(targetPath string, shortcutPath string, iconPath string, arguments string) error {
	if err := os.MkdirAll(filepath.Dir(shortcutPath), 0o755); err != nil {
		return err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		return fmt.Errorf("initialize COM for shortcut: %w", err)
	}
	defer ole.CoUninitialize()

	unknown, err := oleutil.CreateObject("WScript.Shell")
	if err != nil {
		return fmt.Errorf("create WScript.Shell: %w", err)
	}
	defer unknown.Release()

	shell, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("query WScript.Shell dispatch: %w", err)
	}
	defer shell.Release()

	rawShortcut, err := oleutil.CallMethod(shell, "CreateShortcut", shortcutPath)
	if err != nil {
		return fmt.Errorf("create shortcut %s: %w", shortcutPath, err)
	}

	shortcut := rawShortcut.ToIDispatch()
	if shortcut == nil {
		return fmt.Errorf("create shortcut dispatch failed")
	}
	defer shortcut.Release()

	if _, err := oleutil.PutProperty(shortcut, "TargetPath", targetPath); err != nil {
		return fmt.Errorf("set shortcut target: %w", err)
	}
	if _, err := oleutil.PutProperty(shortcut, "WorkingDirectory", filepath.Dir(targetPath)); err != nil {
		return fmt.Errorf("set shortcut working directory: %w", err)
	}
	if arguments != "" {
		if _, err := oleutil.PutProperty(shortcut, "Arguments", arguments); err != nil {
			return fmt.Errorf("set shortcut arguments: %w", err)
		}
	}
	if iconPath != "" {
		if _, err := oleutil.PutProperty(shortcut, "IconLocation", iconPath); err != nil {
			return fmt.Errorf("set shortcut icon: %w", err)
		}
	}
	_, err = oleutil.CallMethod(shortcut, "Save")
	if err != nil {
		return fmt.Errorf("save shortcut: %w", err)
	}
	return nil
}

func refreshShell() {
	procSHChangeNotifySetupStub.Call(0x08000000, 0, 0, 0)
}
