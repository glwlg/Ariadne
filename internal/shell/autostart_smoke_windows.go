//go:build windows

package shell

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func RunAutostartSmoke(options AutostartSmokeOptions) AutostartSmokeReport {
	executable := strings.TrimSpace(options.Executable)
	if executable == "" {
		var err error
		executable, err = os.Executable()
		if err != nil {
			return AutostartSmokeReport{Platform: runtime.GOOS, Error: "读取当前可执行文件失败: " + err.Error()}
		}
	}
	executable, err := filepath.Abs(executable)
	if err != nil {
		return AutostartSmokeReport{Platform: runtime.GOOS, Executable: strings.TrimSpace(options.Executable), Error: "解析可执行文件绝对路径失败: " + err.Error()}
	}
	executable = filepath.Clean(executable)
	if info, err := os.Stat(executable); err != nil || info.IsDir() {
		message := "可执行文件不存在"
		if err != nil {
			message += ": " + err.Error()
		}
		return AutostartSmokeReport{Platform: runtime.GOOS, Executable: executable, Error: message}
	}

	valueName := autostartSmokeValueName(options.ValueName)
	command := autostartSmokeCommand(executable)
	report := AutostartSmokeReport{
		Platform:     runtime.GOOS,
		ValueName:    valueName,
		RegistryPath: `HKCU\` + autostartRegistrySubKey + `\` + valueName,
		Executable:   executable,
		Command:      command,
	}

	key, err := registry.OpenKey(registry.CURRENT_USER, autostartRegistrySubKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		report.Error = "打开 HKCU Run 注册表失败: " + err.Error()
		return report
	}
	defer key.Close()

	previousValue, _, previousErr := key.GetStringValue(valueName)
	report.ExistingValue = previousErr == nil
	if previousErr != nil && previousErr != registry.ErrNotExist {
		report.Notes = append(report.Notes, "读取临时开机启动值失败: "+previousErr.Error())
	}

	if err := key.SetStringValue(valueName, command); err != nil {
		report.Error = "写入临时开机启动值失败: " + err.Error()
		restoreAutostartSmokeValue(key, valueName, previousValue, report.ExistingValue, &report)
		return report
	}

	readBack, err := readAutostartRegistryValue(valueName)
	if err != nil {
		report.Error = "回读临时开机启动值失败: " + err.Error()
		restoreAutostartSmokeValue(key, valueName, previousValue, report.ExistingValue, &report)
		return report
	}
	audit := buildAutostartAudit(valueName, valueName, readBack, executable)
	report.AuditValid = audit.CommandValid
	report.HiddenArgPresent = audit.HiddenArgPresent
	report.CommandMatchesExe = audit.CommandMatchesExe
	report.Notes = append(report.Notes, audit.Notes...)

	restoreAutostartSmokeValue(key, valueName, previousValue, report.ExistingValue, &report)
	report.OK = report.AuditValid && report.HiddenArgPresent && report.CommandMatchesExe && report.CleanupOK && report.Error == ""
	if !report.OK && report.Error == "" {
		report.Error = "临时开机启动命令未通过本地审计"
	}
	return report
}

func restoreAutostartSmokeValue(key registry.Key, valueName string, previousValue string, hadPrevious bool, report *AutostartSmokeReport) {
	if hadPrevious {
		if err := key.SetStringValue(valueName, previousValue); err != nil {
			report.CleanupOK = false
			report.Notes = append(report.Notes, "恢复原临时值失败: "+err.Error())
			if report.Error == "" {
				report.Error = "恢复原临时值失败: " + err.Error()
			}
			return
		}
		report.RestoredPrevious = true
		report.CleanupOK = true
		return
	}
	if err := key.DeleteValue(valueName); err != nil && err != registry.ErrNotExist {
		report.CleanupOK = false
		report.Notes = append(report.Notes, "删除临时开机启动值失败: "+err.Error())
		if report.Error == "" {
			report.Error = "删除临时开机启动值失败: " + err.Error()
		}
		return
	}
	report.CleanupOK = true
}
