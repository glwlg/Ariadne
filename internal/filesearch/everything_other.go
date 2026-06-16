//go:build !windows

package filesearch

import "fmt"

func newEverythingClient(dllPath string) (everythingClient, error) {
	return nil, fmt.Errorf("Everything SDK is only available on Windows")
}
