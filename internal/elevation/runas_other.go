//go:build !windows

package elevation

import "fmt"

func RunasWait(file string, args []string) error {
	return fmt.Errorf("管理员授权仅支持 Windows")
}
