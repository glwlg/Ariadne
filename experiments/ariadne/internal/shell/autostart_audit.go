package shell

import (
	"path/filepath"
	"strings"
)

type autostartAudit struct {
	Identifier        string
	ValueName         string
	Command           string
	CommandMatchesExe bool
	HiddenArgPresent  bool
	CommandValid      bool
	Notes             []string
}

func buildAutostartAudit(identifier string, valueName string, command string, executable string) autostartAudit {
	identifier = strings.TrimSpace(identifier)
	valueName = strings.TrimSpace(valueName)
	command = strings.TrimSpace(command)
	executable = strings.TrimSpace(executable)
	audit := autostartAudit{
		Identifier: identifier,
		ValueName:  valueName,
		Command:    command,
	}
	if valueName == "" {
		valueName = identifier
		audit.ValueName = valueName
	}
	if !strings.EqualFold(valueName, identifier) {
		audit.Notes = append(audit.Notes, "开机启动值名不是 Ariadne 标准 identifier: "+valueName)
	}
	tokens := windowsCommandLineTokens(command)
	if len(tokens) == 0 {
		audit.Notes = append(audit.Notes, "开机启动命令为空")
		return audit
	}
	audit.CommandMatchesExe = sameWindowsPath(tokens[0], executable)
	if !audit.CommandMatchesExe {
		audit.Notes = append(audit.Notes, "开机启动命令未指向当前 Ariadne 可执行文件")
	}
	for _, token := range tokens[1:] {
		if strings.EqualFold(token, "--hidden") || strings.EqualFold(token, "/hidden") {
			audit.HiddenArgPresent = true
			break
		}
	}
	if !audit.HiddenArgPresent {
		audit.Notes = append(audit.Notes, "开机启动命令缺少 --hidden，登录后可能直接弹出启动器")
	}
	audit.CommandValid = audit.CommandMatchesExe && audit.HiddenArgPresent && strings.EqualFold(valueName, identifier)
	return audit
}

func windowsCommandLineTokens(command string) []string {
	command = strings.TrimSpace(command)
	tokens := []string{}
	var current strings.Builder
	inQuotes := false
	escaping := false
	for i := 0; i < len(command); i++ {
		ch := command[i]
		if escaping {
			current.WriteByte(ch)
			escaping = false
			continue
		}
		if ch == '\\' && inQuotes && i+1 < len(command) && command[i+1] == '"' {
			escaping = true
			continue
		}
		if ch == '"' {
			inQuotes = !inQuotes
			continue
		}
		if (ch == ' ' || ch == '\t') && !inQuotes {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteByte(ch)
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func sameWindowsPath(left string, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}
