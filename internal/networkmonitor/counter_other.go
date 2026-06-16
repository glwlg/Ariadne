//go:build !windows

package networkmonitor

import "errors"

func readInterfaceCounters() ([]interfaceCounter, error) {
	return nil, errors.New("network monitor counters are currently implemented for Windows")
}
