package contracts

import (
	"fmt"
	"strings"
)

func CopyAction(id string, label string, text string, shortcut string) PreviewAction {
	return PreviewAction{
		ID:       id,
		Label:    label,
		Icon:     "copy",
		Kind:     ActionCopy,
		Shortcut: shortcut,
		Payload: map[string]interface{}{
			"text": text,
		},
		Feedback: &ActionFeedback{SuccessLabel: "已复制", DurationMS: 1400},
	}
}

func RunAction(id string, label string, command string, shortcut string) PreviewAction {
	return PreviewAction{
		ID:       id,
		Label:    label,
		Icon:     "run",
		Kind:     ActionRun,
		Shortcut: shortcut,
		Payload: map[string]interface{}{
			"command": command,
		},
	}
}

func PluginAction(id string, label string, command string) PreviewAction {
	return PreviewAction{
		ID:    id,
		Label: label,
		Icon:  "plugin",
		Kind:  ActionPlugin,
		Payload: map[string]interface{}{
			"command": command,
		},
	}
}

func RememberAction(id string, label string, targetID string) PreviewAction {
	return PreviewAction{
		ID:    id,
		Label: label,
		Icon:  "remember",
		Kind:  ActionRemember,
		Payload: map[string]interface{}{
			"targetId": targetID,
		},
		Feedback: &ActionFeedback{SuccessLabel: "已加入", DurationMS: 1400},
	}
}

func ValidateActionSurface(result SearchResult) error {
	if result.ID == "" {
		return fmt.Errorf("search result has empty id")
	}
	if len(result.Actions) == 0 {
		return fmt.Errorf("%s has no explicit actions", result.ID)
	}

	for _, action := range result.Actions {
		if action.ID == "" || action.Label == "" || action.Kind == "" {
			return fmt.Errorf("%s has an incomplete action", result.ID)
		}
		if action.Kind == ActionCopy && action.Feedback == nil {
			return fmt.Errorf("%s copy action %s must declare inline feedback", result.ID, action.ID)
		}
		if !isFileLike(result.Type) && isFileOnlyLabel(action.Label) {
			return fmt.Errorf("%s non-file result exposes file-only action %q", result.ID, action.Label)
		}
		if action.Kind == ActionOpenParent && !isFileLike(result.Type) {
			return fmt.Errorf("%s non-file result exposes open_parent", result.ID)
		}
	}
	return nil
}

func ValidateActionSurfaces(results []SearchResult) error {
	for _, result := range results {
		if err := ValidateActionSurface(result); err != nil {
			return err
		}
	}
	return nil
}

func isFileLike(resultType SearchResultType) bool {
	return resultType == ResultFile || resultType == ResultCapture
}

func isFileOnlyLabel(label string) bool {
	normalized := strings.TrimSpace(label)
	return normalized == "打开文件" || normalized == "打开所在文件夹"
}
