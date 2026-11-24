package platform

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type revealMode int

const (
	revealModeSelect revealMode = iota
	revealModeOpenDir
)

type revealTarget struct {
	Path string
	Mode revealMode
}

func normalizeRevealPath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) >= 2 && trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"' {
		trimmed = strings.TrimSpace(trimmed[1 : len(trimmed)-1])
	}
	if trimmed == "" {
		return ""
	}

	normalized := strings.ReplaceAll(trimmed, "/", `\`)
	isUNC := strings.HasPrefix(normalized, `\\`)
	prefix := ""
	rest := normalized
	if isUNC {
		prefix = `\\`
		rest = strings.TrimLeft(normalized, `\`)
	} else if len(normalized) >= 2 && isASCIIAlpha(normalized[0]) && normalized[1] == ':' {
		prefix = strings.ToUpper(normalized[:1]) + ":"
		rest = normalized[2:]
		if rest == "" {
			return prefix + `\`
		}
		if strings.HasPrefix(rest, `\`) {
			prefix += `\`
			rest = strings.TrimLeft(rest, `\`)
		}
	}

	rawParts := strings.Split(rest, `\`)
	stack := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			if isUNC {
				if len(stack) > 2 {
					stack = stack[:len(stack)-1]
				}
			} else if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			continue
		}
		stack = append(stack, part)
	}

	if isUNC {
		if len(stack) == 0 {
			return ""
		}
		return prefix + strings.Join(stack, `\`)
	}
	if prefix != "" {
		if len(stack) == 0 {
			if strings.HasSuffix(prefix, `\`) {
				return prefix
			}
			return prefix + `\`
		}
		if strings.HasSuffix(prefix, `\`) {
			return prefix + strings.Join(stack, `\`)
		}
		return prefix + `\` + strings.Join(stack, `\`)
	}
	return strings.Join(stack, `\`)
}

func isASCIIAlpha(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isRevealRoot(path string) bool {
	if len(path) == 3 && isASCIIAlpha(path[0]) && path[1] == ':' && path[2] == '\\' {
		return true
	}
	if strings.HasPrefix(path, `\\`) {
		parts := strings.Split(strings.TrimPrefix(path, `\\`), `\`)
		return len(parts) <= 2
	}
	return false
}

func windowsParentDir(path string) string {
	if path == "" || isRevealRoot(path) {
		return ""
	}
	idx := strings.LastIndex(path, `\`)
	if idx < 0 {
		return ""
	}
	if strings.HasPrefix(path, `\\`) {
		parent := path[:idx]
		if !strings.HasPrefix(parent, `\\`) || len(strings.Split(strings.TrimPrefix(parent, `\\`), `\`)) < 2 {
			return ""
		}
		return parent
	}
	if idx == 2 && len(path) > 3 && isASCIIAlpha(path[0]) && path[1] == ':' {
		return path[:3]
	}
	return path[:idx]
}

func resolveRevealTarget(raw string, statFn func(string) (os.FileInfo, error)) (revealTarget, error) {
	cleaned := normalizeRevealPath(raw)
	if cleaned == "" {
		return revealTarget{}, errors.New("缺少路径")
	}
	if statFn == nil {
		statFn = os.Stat
	}
	if _, err := statFn(cleaned); err == nil {
		if isRevealRoot(cleaned) {
			return revealTarget{Path: cleaned, Mode: revealModeOpenDir}, nil
		}
		return revealTarget{Path: cleaned, Mode: revealModeSelect}, nil
	}
	for parent := windowsParentDir(cleaned); parent != ""; parent = windowsParentDir(parent) {
		if info, err := statFn(parent); err == nil && info != nil && info.IsDir() {
			return revealTarget{Path: parent, Mode: revealModeOpenDir}, nil
		}
	}
	return revealTarget{}, fmt.Errorf("目标路径不存在: %s", cleaned)
}

func buildExplorerCmdLine(target revealTarget) string {
	sanitized := strings.ReplaceAll(target.Path, `"`, "")
	if target.Mode == revealModeOpenDir {
		return fmt.Sprintf(`explorer.exe "%s"`, sanitized)
	}
	return fmt.Sprintf(`explorer.exe /select,"%s"`, sanitized)
}
