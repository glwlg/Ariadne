//go:build !windows

package brandmigration

func checkLegacyStopped() error { return nil }

func NotifyFailure(err error) {}
