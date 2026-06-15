//go:build !windows

package workmemory

import "time"

func defaultActivityProvider() activityProvider {
	return activityProviderFunc(func(now time.Time) activitySnapshot {
		return activitySnapshot{Available: false, LastActivityAt: now.Unix()}
	})
}
