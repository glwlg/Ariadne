//go:build windows

package setupstub

import (
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	messageBoxOK            = 0x00000000
	messageBoxIconError     = 0x00000010
	messageBoxIconInfo      = 0x00000040
	messageBoxSetForeground = 0x00010000
)

var (
	user32SetupStub         = windows.NewLazySystemDLL("user32.dll")
	procMessageBoxSetupStub = user32SetupStub.NewProc("MessageBoxW")
)

func ShowInfo(title string, message string) {
	showMessageBox(title, message, messageBoxIconInfo)
}

func ShowError(title string, message string) {
	showMessageBox(title, message, messageBoxIconError)
}

func showMessageBox(title string, message string, icon uintptr) {
	titlePtr, err := windows.UTF16PtrFromString(cleanMessage(title, 120))
	if err != nil {
		return
	}
	messagePtr, err := windows.UTF16PtrFromString(cleanMessage(message, 1800))
	if err != nil {
		return
	}
	procMessageBoxSetupStub.Call(
		0,
		uintptr(unsafe.Pointer(messagePtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		messageBoxOK|icon|messageBoxSetForeground,
	)
}

func cleanMessage(text string, limit int) string {
	text = strings.ReplaceAll(text, "\x00", " ")
	if len([]rune(text)) <= limit {
		return text
	}
	runes := []rune(text)
	return string(runes[:limit]) + "\n..."
}
