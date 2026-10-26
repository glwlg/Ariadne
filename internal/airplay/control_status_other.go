//go:build !windows

package airplay

import "os"

func queryControlStatus() (ControlStatus, error) {
	return ControlStatus{}, os.ErrNotExist
}
