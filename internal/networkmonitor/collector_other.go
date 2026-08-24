//go:build !windows

package networkmonitor

import (
	"context"
	"errors"
)

func RunCollector(context.Context) error {
	return errors.New("process network collector is only available on Windows")
}

func readPrivilegedTraffic() (privilegedTrafficSnapshot, error) {
	return privilegedTrafficSnapshot{}, errors.New("process network collector is only available on Windows")
}
