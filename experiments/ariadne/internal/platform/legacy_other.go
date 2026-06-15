//go:build !windows

package platform

type legacyProcessStatus struct {
	Running bool
	ID      int
	Name    string
	Path    string
}

func findLegacyProcess() legacyProcessStatus {
	return legacyProcessStatus{}
}

func closeLegacyProcess(request LegacyHandoffRequest, before LegacyRuntimeStatus) legacyHandoffOutcome {
	return legacyHandoffOutcome{
		Actions: []string{"当前平台不支持自动关闭旧版 x-tools"},
		Error:   "unsupported platform",
	}
}
