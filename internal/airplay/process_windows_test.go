//go:build windows

package airplay

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestReceiverWindowHelper(t *testing.T) {
	if len(os.Args) == 0 || os.Args[len(os.Args)-1] != "--airplay-window-helper" {
		return
	}
	user32 := windows.NewLazySystemDLL("user32.dll")
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	class, _ := windows.UTF16PtrFromString("STATIC")
	title, _ := windows.UTF16PtrFromString("Ariadne receiver window test")
	hwnd, _, err := user32.NewProc("CreateWindowExW").Call(
		0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)),
		0x00cf0000, 0, 0, 160, 120, 0, 0, 0, 0,
	)
	if hwnd == 0 {
		fmt.Printf("create window failed: %v\n", err)
		os.Exit(1)
	}
	user32.NewProc("ShowWindow").Call(hwnd, 5)
	visible, _, _ := user32.NewProc("IsWindowVisible").Call(hwnd)
	console, _, _ := kernel32.NewProc("GetConsoleWindow").Call()
	user32.NewProc("DestroyWindow").Call(hwnd)
	fmt.Printf("visible=%d console=%d\n", visible, console)
	os.Exit(0)
}

func TestReceiverHidesConsoleWithoutHidingVideoWindow(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestReceiverWindowHelper$", "--", "--airplay-window-helper")
	hideConsole(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "visible=1 console=0") {
		t.Fatalf("receiver must show its first window without a console: %s (%v)", output, err)
	}
}
