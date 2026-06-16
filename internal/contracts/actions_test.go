package contracts

import "testing"

func TestValidateActionSurfaceRejectsFileActionsForNonFileResults(t *testing.T) {
	result := SearchResult{
		ID:    "copy-result",
		Type:  ResultPluginResult,
		Title: "copy",
		Preview: PreviewDescriptor{
			Kind:  PreviewText,
			Title: "copy",
		},
		Actions: []PreviewAction{
			{ID: "open_parent", Label: "打开所在文件夹", Kind: ActionOpenParent},
		},
	}

	if err := ValidateActionSurface(result); err == nil {
		t.Fatal("expected non-file result with file action to be rejected")
	}
}

func TestValidateActionSurfaceRequiresCopyFeedback(t *testing.T) {
	result := SearchResult{
		ID:    "copy-result",
		Type:  ResultPluginResult,
		Title: "copy",
		Preview: PreviewDescriptor{
			Kind:  PreviewText,
			Title: "copy",
		},
		Actions: []PreviewAction{
			{ID: "copy", Label: "复制结果", Kind: ActionCopy},
		},
	}

	if err := ValidateActionSurface(result); err == nil {
		t.Fatal("expected copy action without inline feedback to be rejected")
	}
}

func TestValidateActionSurfaceAllowsFileDefaults(t *testing.T) {
	result := SearchResult{
		ID:    "file-readme",
		Type:  ResultFile,
		Title: "README.md",
		Preview: PreviewDescriptor{
			Kind:  PreviewText,
			Title: "README.md",
		},
		Actions: []PreviewAction{
			{ID: "open", Label: "打开文件", Kind: ActionOpen},
			{ID: "open_parent", Label: "打开所在文件夹", Kind: ActionOpenParent},
			CopyAction("copy_path", "复制路径", "P:\\workspace\\glwlg\\app\\x-tools\\README.md", ""),
		},
	}

	if err := ValidateActionSurface(result); err != nil {
		t.Fatalf("expected file defaults to validate: %v", err)
	}
}
