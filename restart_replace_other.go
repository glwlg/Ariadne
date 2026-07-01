//go:build !windows

package main

import "time"

func waitForRestartReplacementProcess(pid int, timeout time.Duration) error {
	return nil
}
