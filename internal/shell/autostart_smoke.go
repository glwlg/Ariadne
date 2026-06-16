package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type AutostartSmokeOptions struct {
	Executable string
	ValueName  string
}

type AutostartSmokeReport struct {
	OK                bool     `json:"ok"`
	Platform          string   `json:"platform"`
	ValueName         string   `json:"valueName"`
	RegistryPath      string   `json:"registryPath,omitempty"`
	Executable        string   `json:"executable,omitempty"`
	Command           string   `json:"command,omitempty"`
	ExistingValue     bool     `json:"existingValue"`
	RestoredPrevious  bool     `json:"restoredPrevious"`
	CleanupOK         bool     `json:"cleanupOk"`
	AuditValid        bool     `json:"auditValid"`
	HiddenArgPresent  bool     `json:"hiddenArgPresent"`
	CommandMatchesExe bool     `json:"commandMatchesExe"`
	Notes             []string `json:"notes,omitempty"`
	Error             string   `json:"error,omitempty"`
}

func autostartSmokeValueName(value string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return fmt.Sprintf("%s.smoke.%d.%d", autostartID, os.Getpid(), time.Now().UnixNano())
}

func autostartSmokeCommand(executable string) string {
	escaped := strings.ReplaceAll(filepath.Clean(executable), `"`, `\"`)
	return `"` + escaped + `" --hidden`
}
