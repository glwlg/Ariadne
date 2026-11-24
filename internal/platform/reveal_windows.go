//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	revealOle32DLL             = syscall.NewLazyDLL("ole32.dll")
	revealShell32DLL           = syscall.NewLazyDLL("shell32.dll")
	procRevealCoInitializeEx   = revealOle32DLL.NewProc("CoInitializeEx")
	procRevealCoUninitialize   = revealOle32DLL.NewProc("CoUninitialize")
	procRevealCoTaskMemFree    = revealOle32DLL.NewProc("CoTaskMemFree")
	procSHParseDisplayName     = revealShell32DLL.NewProc("SHParseDisplayName")
	procSHOpenFolderAndSelect  = revealShell32DLL.NewProc("SHOpenFolderAndSelectItems")
)

const (
	revealCOInitApartmentThreaded = 0x2
	revealRPCEChangedMode         = int32(-2147417850) // 0x80010106
)

func revealInExplorer(path string) error {
	target, err := resolveRevealTarget(path, os.Stat)
	if err != nil {
		return err
	}
	if target.Mode == revealModeSelect {
		if err := shOpenFolderAndSelectItem(target.Path); err == nil {
			return nil
		}
	}
	return startExplorerWithCmdLine(target)
}

func shOpenFolderAndSelectItem(targetPath string) error {
	pathPtr, err := syscall.UTF16PtrFromString(targetPath)
	if err != nil {
		return err
	}
	errCh := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		hr, _, _ := procRevealCoInitializeEx.Call(0, revealCOInitApartmentThreaded)
		hr32 := int32(hr)
		if hr32 >= 0 {
			defer procRevealCoUninitialize.Call()
		} else if hr32 != revealRPCEChangedMode {
			errCh <- fmt.Errorf("CoInitializeEx failed: 0x%08x", uint32(hr32))
			return
		}

		var pidl uintptr
		hr, _, _ = procSHParseDisplayName.Call(
			uintptr(unsafe.Pointer(pathPtr)),
			0,
			uintptr(unsafe.Pointer(&pidl)),
			0,
			0,
		)
		if int32(hr) < 0 || pidl == 0 {
			errCh <- fmt.Errorf("SHParseDisplayName failed: 0x%08x", uint32(int32(hr)))
			return
		}
		defer procRevealCoTaskMemFree.Call(pidl)

		hr, _, _ = procSHOpenFolderAndSelect.Call(pidl, 0, 0, 0)
		if int32(hr) < 0 {
			errCh <- fmt.Errorf("SHOpenFolderAndSelectItems failed: 0x%08x", uint32(int32(hr)))
			return
		}
		errCh <- nil
	}()
	return <-errCh
}

func startExplorerWithCmdLine(target revealTarget) error {
	cmd := exec.Command("explorer.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: buildExplorerCmdLine(target),
	}
	return cmd.Start()
}
