//go:build windows

package securestore

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

const (
	credTypeGeneric         = 1
	credPersistLocalMachine = 2
	errorNotFound           = syscall.Errno(1168)
)

var (
	advapi32       = syscall.NewLazyDLL("advapi32.dll")
	procCredRead   = advapi32.NewProc("CredReadW")
	procCredWrite  = advapi32.NewProc("CredWriteW")
	procCredDelete = advapi32.NewProc("CredDeleteW")
	procCredFree   = advapi32.NewProc("CredFree")
)

type credential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        syscall.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

func defaultAvailable() bool {
	return true
}

func defaultBackend() string {
	return "windows_credential_manager"
}

func Read(target string) (string, bool, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", false, fmt.Errorf("credential target is empty")
	}
	targetPtr, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return "", false, err
	}
	var raw uintptr
	result, _, callErr := procCredRead.Call(
		uintptr(unsafe.Pointer(targetPtr)),
		uintptr(credTypeGeneric),
		0,
		uintptr(unsafe.Pointer(&raw)),
	)
	if result == 0 {
		if errno, ok := callErr.(syscall.Errno); ok && errno == errorNotFound {
			return "", false, nil
		}
		return "", false, callErr
	}
	defer procCredFree.Call(raw)
	cred := (*credential)(unsafe.Pointer(raw))
	if cred.CredentialBlobSize == 0 || cred.CredentialBlob == nil {
		return "", true, nil
	}
	blob := unsafe.Slice(cred.CredentialBlob, int(cred.CredentialBlobSize))
	return string(blob), true, nil
}

func Write(target string, secret string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("credential target is empty")
	}
	if strings.TrimSpace(secret) == "" {
		return fmt.Errorf("credential secret is empty")
	}
	targetPtr, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	userPtr, err := syscall.UTF16PtrFromString("Ariadne")
	if err != nil {
		return err
	}
	blob := []byte(secret)
	cred := credential{
		Type:               credTypeGeneric,
		TargetName:         targetPtr,
		CredentialBlobSize: uint32(len(blob)),
		Persist:            credPersistLocalMachine,
		UserName:           userPtr,
	}
	if len(blob) > 0 {
		cred.CredentialBlob = &blob[0]
	}
	result, _, callErr := procCredWrite.Call(uintptr(unsafe.Pointer(&cred)), 0)
	if result == 0 {
		return callErr
	}
	return nil
}

func Delete(target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("credential target is empty")
	}
	targetPtr, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	result, _, callErr := procCredDelete.Call(
		uintptr(unsafe.Pointer(targetPtr)),
		uintptr(credTypeGeneric),
		0,
	)
	if result == 0 {
		if errno, ok := callErr.(syscall.Errno); ok && errno == errorNotFound {
			return nil
		}
		return callErr
	}
	return nil
}
