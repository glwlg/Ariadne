//go:build windows

package shell

import (
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const autostartRegistrySubKey = `Software\Microsoft\Windows\CurrentVersion\Run`

func auditAutostartRegistration(identifier string, statusPath string) autostartAudit {
	valueName := autostartValueNameFromPath(statusPath)
	if valueName == "" {
		valueName = identifier
	}
	command, err := readAutostartRegistryValue(valueName)
	if err != nil {
		return autostartAudit{
			Identifier: identifier,
			ValueName:  valueName,
			Notes:      []string{"读取开机启动注册表失败: " + err.Error()},
		}
	}
	executable, err := os.Executable()
	if err != nil {
		return autostartAudit{
			Identifier: identifier,
			ValueName:  valueName,
			Command:    command,
			Notes:      []string{"读取当前 Ariadne 可执行文件失败: " + err.Error()},
		}
	}
	return buildAutostartAudit(identifier, valueName, command, executable)
}

func readAutostartRegistryValue(valueName string) (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, autostartRegistrySubKey, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer key.Close()
	value, _, err := key.GetStringValue(valueName)
	return value, err
}

func autostartValueNameFromPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if index := strings.LastIndex(path, `\`); index >= 0 && index < len(path)-1 {
		return path[index+1:]
	}
	return ""
}
