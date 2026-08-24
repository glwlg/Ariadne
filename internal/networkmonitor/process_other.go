//go:build !windows

package networkmonitor

import "errors"

func readProcessCounters() ([]processCounter, error) {
	return nil, errors.New("process network counters are currently implemented for Windows")
}
