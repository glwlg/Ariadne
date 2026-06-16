package checklists

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ariadne/internal/workmemory"
)

func TestSaveChecklistDraftRequiresConfirmationAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checklists.json")
	service := NewServiceWithPath(path)
	draft := workmemory.ChecklistDraft{
		ID:             "checklist-draft-network-proxy",
		Title:          "网络代理排查清单",
		Context:        "代理、网关和 DNS 异常的重复排查流程",
		Items:          []string{"确认当前网络出口", "检查 DNS 解析", "记录可复用处理结论"},
		Evidence:       []string{"memory-a", "memory-b"},
		RequiresReview: true,
		CreatedAt:      1710000000,
	}

	preview := service.SaveChecklistDraft(DraftSaveRequest{Draft: draft})
	if preview.OK || !preview.RequiresConfirmation {
		t.Fatalf("expected preview confirmation gate, got %#v", preview)
	}
	if preview.Checklist.ID != "memory-checklist-network-proxy" {
		t.Fatalf("unexpected checklist id: %s", preview.Checklist.ID)
	}
	if preview.Status.Count != 0 {
		t.Fatalf("preview should not persist checklist, got count %d", preview.Status.Count)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("preview should not create store file, stat err=%v", err)
	}

	result := service.SaveChecklistDraft(DraftSaveRequest{Draft: draft, Confirmed: true})
	if !result.OK {
		t.Fatalf("expected confirmed save, got %#v", result)
	}
	if result.Status.Count != 1 {
		t.Fatalf("expected one checklist, got %d", result.Status.Count)
	}
	if len(result.Checklist.Items) != 3 || len(result.Checklist.Evidence) != 2 {
		t.Fatalf("checklist content was not preserved: %#v", result.Checklist)
	}
	if !strings.Contains(result.Message, "正式资产") {
		t.Fatalf("expected formal asset message, got %q", result.Message)
	}

	reloaded := NewServiceWithPath(path)
	checklists := reloaded.List()
	if len(checklists) != 1 {
		t.Fatalf("expected one reloaded checklist, got %d", len(checklists))
	}
	if checklists[0].Title != draft.Title || checklists[0].Context != draft.Context {
		t.Fatalf("reloaded checklist mismatch: %#v", checklists[0])
	}
}

func TestSaveChecklistDraftRejectsInvalidDraft(t *testing.T) {
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "checklists.json"))
	result := service.SaveChecklistDraft(DraftSaveRequest{
		Draft: workmemory.ChecklistDraft{
			ID:    "checklist-draft-empty",
			Title: "空清单",
		},
		Confirmed: true,
	})
	if result.OK {
		t.Fatalf("expected invalid draft to be rejected")
	}
	if !strings.Contains(result.Message, "无效") {
		t.Fatalf("unexpected message: %s", result.Message)
	}
	if result.Status.Count != 0 {
		t.Fatalf("invalid draft should not persist, got count %d", result.Status.Count)
	}
}
