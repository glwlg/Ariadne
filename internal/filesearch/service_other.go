//go:build !windows

package filesearch

import "fmt"

const WindowsServiceName = "AriadneFileSearch"

func RunWindowsService(args []string) error {
	return fmt.Errorf("Ariadne 文件索引服务仅支持 Windows")
}
