package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const restartReplacePIDArg = "--ariadne-replace-pid"

type replacementWaiter func(pid int, timeout time.Duration) error

func handleRestartReplacementArgs(args []string, currentPID int, wait replacementWaiter) (bool, error) {
	pid, ok, err := restartReplacementPID(args)
	if err != nil || !ok {
		return ok, err
	}
	if pid == currentPID {
		return true, nil
	}
	if wait == nil {
		return true, nil
	}
	return true, wait(pid, 6*time.Second)
}

func restartReplacementPID(args []string) (int, bool, error) {
	for index, arg := range args {
		if !strings.EqualFold(strings.TrimSpace(arg), restartReplacePIDArg) {
			continue
		}
		if index+1 >= len(args) {
			return 0, true, fmt.Errorf("%s 缺少进程 ID", restartReplacePIDArg)
		}
		pid, err := strconv.Atoi(strings.TrimSpace(args[index+1]))
		if err != nil || pid <= 0 {
			return 0, true, fmt.Errorf("%s 进程 ID 无效: %s", restartReplacePIDArg, args[index+1])
		}
		return pid, true, nil
	}
	return 0, false, nil
}
