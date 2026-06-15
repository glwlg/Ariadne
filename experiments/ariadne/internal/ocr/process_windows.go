package ocr

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

func configureOCRCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
