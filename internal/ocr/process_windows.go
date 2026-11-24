package ocr

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const createNoWindow = 0x08000000

func configureOCRCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
	cmd.WaitDelay = 2 * time.Second
	cmd.Cancel = func() error {
		return killOCRProcessTree(cmd.Process)
	}
}

func killOCRProcessTree(process *os.Process) error {
	if process == nil || process.Pid <= 0 {
		return nil
	}
	kill := exec.Command("taskkill.exe", "/PID", strconv.Itoa(process.Pid), "/T", "/F")
	kill.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
	if err := kill.Run(); err != nil {
		return process.Kill()
	}
	return nil
}

func attachOCRProcessTree(cmd *exec.Cmd) func() {
	if cmd == nil || cmd.Process == nil || cmd.Process.Pid <= 0 {
		return func() {}
	}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return func() {}
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(job)
		return func() {}
	}
	procHandle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		_ = windows.CloseHandle(job)
		return func() {}
	}
	defer windows.CloseHandle(procHandle)
	if err := windows.AssignProcessToJobObject(job, procHandle); err != nil {
		_ = windows.CloseHandle(job)
		return func() {}
	}
	return func() {
		_ = windows.CloseHandle(job)
	}
}
