//go:build !windows

package shell

func auditAutostartRegistration(identifier string, statusPath string) autostartAudit {
	return autostartAudit{
		Identifier: identifier,
		ValueName:  identifier,
		Notes:      []string{"当前平台未实现 Ariadne 开机启动命令审计: " + statusPath},
	}
}
