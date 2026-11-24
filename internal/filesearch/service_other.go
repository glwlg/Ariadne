//go:build !windows

package filesearch

import "fmt"

const WindowsServiceName = "FancunFileSearch"

func RunWindowsService(args []string) error {
	return fmt.Errorf("Fancun 文件索引服务仅支持 Windows")
}

func RunWindowsServiceWithPolicy(args []string, policy FileSearchPolicy) error {
	return RunWindowsService(args)
}

func RunWindowsServiceWithPolicyProvider(args []string, policyProvider PolicyProvider) error {
	return RunWindowsService(args)
}
