package skills

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ariadne/internal/workmemory"
)

func TestSaveSkillDraftRequiresConfirmationAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills.json")
	service := NewServiceWithPath(path)
	draft := workmemory.Draft{
		ID:        "knowledge-20260614133500",
		Title:     "PostgreSQL 连接排查 Skill",
		Body:      "适用于 PostgreSQL connection refused：先确认监听端口，再检查网关、防火墙和连接串。",
		Evidence:  []string{"memory-db-a", "memory-db-b"},
		CreatedAt: 1710000000,
	}

	preview := service.SaveSkillDraft(DraftSaveRequest{Draft: draft})
	if preview.OK || !preview.RequiresConfirmation {
		t.Fatalf("expected preview confirmation gate, got %#v", preview)
	}
	if preview.Skill.ID != "memory-skill-20260614133500" {
		t.Fatalf("unexpected skill id: %s", preview.Skill.ID)
	}
	if preview.Status.Count != 0 {
		t.Fatalf("preview should not persist skill, got count %d", preview.Status.Count)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("preview should not create store file, stat err=%v", err)
	}

	result := service.SaveSkillDraft(DraftSaveRequest{Draft: draft, Confirmed: true})
	if !result.OK {
		t.Fatalf("expected confirmed save, got %#v", result)
	}
	if result.Status.Count != 1 {
		t.Fatalf("expected one skill, got %d", result.Status.Count)
	}
	if len(result.Skill.Evidence) != 2 || !strings.Contains(result.Skill.Body, "connection refused") {
		t.Fatalf("skill content was not preserved: %#v", result.Skill)
	}
	if !strings.Contains(result.Message, "正式资产") {
		t.Fatalf("expected formal asset message, got %q", result.Message)
	}

	reloaded := NewServiceWithPath(path)
	skills := reloaded.List()
	if len(skills) != 1 {
		t.Fatalf("expected one reloaded skill, got %d", len(skills))
	}
	if skills[0].Title != draft.Title || skills[0].Body != draft.Body {
		t.Fatalf("reloaded skill mismatch: %#v", skills[0])
	}
}

func TestSaveSkillDraftRejectsInvalidDraft(t *testing.T) {
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "skills.json"))
	result := service.SaveSkillDraft(DraftSaveRequest{
		Draft: workmemory.Draft{
			ID:    "knowledge-empty",
			Title: "空 Skill",
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

func TestExportSkillPackageRequiresConfirmationAndWritesCodexSkill(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills.json")
	service := NewServiceWithPath(path)
	save := service.SaveSkillDraft(DraftSaveRequest{
		Draft: workmemory.Draft{
			ID:        "knowledge-proxy-steps",
			Title:     "代理排查 Skill",
			Body:      "排查代理故障时，先确认当前出口，再检查 DNS、网关、订阅规则和最近改动。",
			Evidence:  []string{"memory-proxy-a"},
			CreatedAt: 1710000000,
		},
		Confirmed: true,
	})
	if !save.OK {
		t.Fatalf("expected skill save, got %#v", save)
	}

	preview := service.ExportSkillPackage(ExportRequest{SkillID: save.Skill.ID})
	if preview.OK || !preview.RequiresConfirmation {
		t.Fatalf("expected export confirmation gate, got %#v", preview)
	}
	if preview.ZipPath != "" || preview.Directory != "" {
		t.Fatalf("preview should not write export paths: %#v", preview)
	}

	result := service.ExportSkillPackage(ExportRequest{SkillID: save.Skill.ID, Confirmed: true})
	if !result.OK {
		t.Fatalf("expected export success, got %#v", result)
	}
	skillPath := filepath.Join(result.Directory, "SKILL.md")
	raw, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("expected SKILL.md to exist: %v", err)
	}
	text := string(raw)
	for _, expected := range []string{
		"name: memory-skill-proxy-steps",
		"description: ",
		"# 代理排查 Skill",
		"排查代理故障时",
		"memory-proxy-a",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected SKILL.md to contain %q, got:\n%s", expected, text)
		}
	}
	if result.Bytes == 0 {
		t.Fatalf("expected zip bytes to be recorded")
	}
	zipText := readZipFile(t, result.ZipPath, "memory-skill-proxy-steps/SKILL.md")
	if zipText != text {
		t.Fatalf("zip SKILL.md did not match exported file")
	}
}

func TestExportSkillPackageRejectsUnknownSkill(t *testing.T) {
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "skills.json"))
	result := service.ExportSkillPackage(ExportRequest{SkillID: "missing", Confirmed: true})
	if result.OK {
		t.Fatalf("expected missing skill export to fail")
	}
	if !strings.Contains(result.Message, "未找到") {
		t.Fatalf("unexpected message: %s", result.Message)
	}
}

func TestInstallSkillPackageRequiresConfirmationAndInstallsToTargetRoot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills.json")
	service := NewServiceWithPath(path)
	save := service.SaveSkillDraft(DraftSaveRequest{
		Draft: workmemory.Draft{
			ID:        "knowledge-codex-skill",
			Title:     "Codex Skill 安装 Skill",
			Body:      "将 Ariadne 工作记忆沉淀安装为 Codex Skill 时，先预览内容，再确认写入本机 skills 目录。",
			Evidence:  []string{"memory-skill-install-a"},
			CreatedAt: 1710000000,
		},
		Confirmed: true,
	})
	if !save.OK {
		t.Fatalf("expected skill save, got %#v", save)
	}
	targetRoot := filepath.Join(t.TempDir(), "codex-skills")

	preview := service.InstallSkillPackage(InstallRequest{SkillID: save.Skill.ID, TargetRoot: targetRoot})
	if preview.OK || !preview.RequiresConfirmation {
		t.Fatalf("expected install confirmation gate, got %#v", preview)
	}
	if _, err := os.Stat(targetRoot); !os.IsNotExist(err) {
		t.Fatalf("preview should not create target root, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(targetRoot, ".ariadne-refresh.json")); !os.IsNotExist(err) {
		t.Fatalf("preview should not create refresh manifest, stat err=%v", err)
	}

	result := service.InstallSkillPackage(InstallRequest{SkillID: save.Skill.ID, TargetRoot: targetRoot, Confirmed: true})
	if !result.OK {
		t.Fatalf("expected install success, got %#v", result)
	}
	if filepath.Base(result.InstalledDir) != save.Skill.ID {
		t.Fatalf("expected installed dir to end with skill id, got %s", result.InstalledDir)
	}
	for _, expected := range []string{"SKILL.md", ".ariadne-refresh.json", ".ariadne-refresh.touch"} {
		if !containsString(result.Files, expected) {
			t.Fatalf("expected installed files to include %s, got %#v", expected, result.Files)
		}
	}
	raw, err := os.ReadFile(filepath.Join(result.InstalledDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("expected installed SKILL.md: %v", err)
	}
	text := string(raw)
	for _, expected := range []string{
		"name: memory-skill-codex-skill",
		"# Codex Skill 安装 Skill",
		"memory-skill-install-a",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected installed SKILL.md to contain %q, got:\n%s", expected, text)
		}
	}
	if !result.RefreshRequested || result.RefreshManifest == "" || result.RefreshMarker == "" {
		t.Fatalf("expected refresh marker metadata, got %#v", result)
	}
	manifestRaw, err := os.ReadFile(result.RefreshManifest)
	if err != nil {
		t.Fatalf("expected refresh manifest: %v", err)
	}
	var manifest codexRefreshManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatalf("decode refresh manifest: %v", err)
	}
	if manifest.Action != "skills.refresh" || manifest.Source != "ariadne" || manifest.SkillID != save.Skill.ID || manifest.InstalledDir != result.InstalledDir {
		t.Fatalf("unexpected refresh manifest: %#v", manifest)
	}
	markerRaw, err := os.ReadFile(result.RefreshMarker)
	if err != nil {
		t.Fatalf("expected refresh marker: %v", err)
	}
	if !strings.Contains(string(markerRaw), save.Skill.ID) {
		t.Fatalf("expected marker to include skill id, got %q", string(markerRaw))
	}
}

func TestInstallDiagnosticsVerifiesInstalledSkillAndRefreshHandshake(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills.json")
	service := NewServiceWithPath(path)
	save := service.SaveSkillDraft(DraftSaveRequest{
		Draft: workmemory.Draft{
			ID:        "knowledge-diagnostics-skill",
			Title:     "安装诊断 Skill",
			Body:      "安装到 Codex skills 目录后，需要检查 SKILL.md、发现目录和刷新握手是否一致。",
			Evidence:  []string{"memory-install-diagnostics-a"},
			CreatedAt: 1710000000,
		},
		Confirmed: true,
	})
	if !save.OK {
		t.Fatalf("expected skill save, got %#v", save)
	}
	targetRoot := filepath.Join(t.TempDir(), "codex-skills")
	install := service.InstallSkillPackage(InstallRequest{SkillID: save.Skill.ID, TargetRoot: targetRoot, Confirmed: true})
	if !install.OK {
		t.Fatalf("expected install success, got %#v", install)
	}

	diagnostics := service.InstallDiagnostics(InstallDiagnosticsRequest{SkillID: save.Skill.ID, TargetRoot: targetRoot})
	if !diagnostics.OK {
		t.Fatalf("expected diagnostics success, got %#v", diagnostics)
	}
	if !diagnostics.TargetRootExists || !diagnostics.TargetRootReadable || !diagnostics.Installed || !diagnostics.SkillFileExists {
		t.Fatalf("expected installed skill to be visible: %#v", diagnostics)
	}
	if diagnostics.SkillPath != filepath.Join(install.InstalledDir, "SKILL.md") || diagnostics.SkillFileBytes == 0 {
		t.Fatalf("expected skill file metadata, got %#v", diagnostics)
	}
	if diagnostics.DiscoveredCount != 1 || diagnostics.AriadneManagedCount != 1 || len(diagnostics.Skills) != 1 {
		t.Fatalf("expected one Ariadne-managed skill, got %#v", diagnostics)
	}
	if diagnostics.Skills[0].ID != save.Skill.ID || diagnostics.Skills[0].Title != save.Skill.Title || !diagnostics.Skills[0].AriadneManaged {
		t.Fatalf("unexpected discovered skill: %#v", diagnostics.Skills[0])
	}
	if !diagnostics.Refresh.Valid ||
		!diagnostics.Refresh.MarkerMatchesManifest ||
		!diagnostics.Refresh.SkillMatchesRequest ||
		!diagnostics.Refresh.InstalledDirExists ||
		!diagnostics.Refresh.InstalledSkillFileFound {
		t.Fatalf("expected valid refresh diagnostics, got %#v", diagnostics.Refresh)
	}
	if diagnostics.Refresh.SkillID != save.Skill.ID || diagnostics.Refresh.InstalledDir != install.InstalledDir {
		t.Fatalf("unexpected refresh target: %#v", diagnostics.Refresh)
	}
}

func TestInstallDiagnosticsReportsMissingTargetRoot(t *testing.T) {
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "skills.json"))
	targetRoot := filepath.Join(t.TempDir(), "missing-codex-skills")
	diagnostics := service.InstallDiagnostics(InstallDiagnosticsRequest{SkillID: "memory-skill-missing", TargetRoot: targetRoot})
	if diagnostics.OK {
		t.Fatalf("expected missing root diagnostics to fail")
	}
	if diagnostics.TargetRootExists || diagnostics.TargetRootReadable || diagnostics.Installed {
		t.Fatalf("missing root should not look available: %#v", diagnostics)
	}
	if !strings.Contains(diagnostics.Message, "不存在") {
		t.Fatalf("unexpected message: %s", diagnostics.Message)
	}
}

func TestInstallDiagnosticsDetectsRefreshMarkerMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills.json")
	service := NewServiceWithPath(path)
	save := service.SaveSkillDraft(DraftSaveRequest{
		Draft: workmemory.Draft{
			ID:        "knowledge-marker-mismatch",
			Title:     "握手不一致 Skill",
			Body:      "刷新 marker 和 manifest 不一致时，诊断应该保留已安装结论但不能通过握手核验。",
			CreatedAt: 1710000000,
		},
		Confirmed: true,
	})
	if !save.OK {
		t.Fatalf("expected skill save, got %#v", save)
	}
	targetRoot := filepath.Join(t.TempDir(), "codex-skills")
	install := service.InstallSkillPackage(InstallRequest{SkillID: save.Skill.ID, TargetRoot: targetRoot, Confirmed: true})
	if !install.OK {
		t.Fatalf("expected install success, got %#v", install)
	}
	if err := os.WriteFile(install.RefreshMarker, []byte("different-marker\n"), 0o600); err != nil {
		t.Fatalf("corrupt refresh marker: %v", err)
	}

	diagnostics := service.InstallDiagnostics(InstallDiagnosticsRequest{SkillID: save.Skill.ID, TargetRoot: targetRoot})
	if diagnostics.OK {
		t.Fatalf("expected mismatched refresh marker to fail diagnostics")
	}
	if !diagnostics.Installed || !diagnostics.SkillFileExists {
		t.Fatalf("installed skill should still be visible: %#v", diagnostics)
	}
	if diagnostics.Refresh.Valid || diagnostics.Refresh.MarkerMatchesManifest {
		t.Fatalf("refresh mismatch should be reported: %#v", diagnostics.Refresh)
	}
	if !strings.Contains(diagnostics.Message, "刷新握手") {
		t.Fatalf("unexpected message: %s", diagnostics.Message)
	}
}

func TestInstallSkillPackageRequiresOverwriteForExistingSkill(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills.json")
	service := NewServiceWithPath(path)
	save := service.SaveSkillDraft(DraftSaveRequest{
		Draft: workmemory.Draft{
			ID:        "knowledge-overwrite-skill",
			Title:     "覆盖保护 Skill",
			Body:      "安装到 Codex skills 目录时，如果同名目录已经存在，默认不能覆盖旧内容。",
			CreatedAt: 1710000000,
		},
		Confirmed: true,
	})
	if !save.OK {
		t.Fatalf("expected skill save, got %#v", save)
	}
	targetRoot := filepath.Join(t.TempDir(), "codex-skills")
	first := service.InstallSkillPackage(InstallRequest{SkillID: save.Skill.ID, TargetRoot: targetRoot, Confirmed: true})
	if !first.OK {
		t.Fatalf("expected first install success, got %#v", first)
	}
	skillPath := filepath.Join(first.InstalledDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("manual edit"), 0o600); err != nil {
		t.Fatalf("seed manual file: %v", err)
	}

	blocked := service.InstallSkillPackage(InstallRequest{SkillID: save.Skill.ID, TargetRoot: targetRoot, Confirmed: true})
	if blocked.OK || !blocked.RequiresConfirmation {
		t.Fatalf("expected existing install to require overwrite, got %#v", blocked)
	}
	raw, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read manual file: %v", err)
	}
	if string(raw) != "manual edit" {
		t.Fatalf("install without overwrite should not rewrite file, got %q", string(raw))
	}

	overwritten := service.InstallSkillPackage(InstallRequest{SkillID: save.Skill.ID, TargetRoot: targetRoot, Confirmed: true, Overwrite: true})
	if !overwritten.OK {
		t.Fatalf("expected overwrite install success, got %#v", overwritten)
	}
	raw, err = os.ReadFile(filepath.Join(overwritten.InstalledDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read overwritten file: %v", err)
	}
	if !strings.Contains(string(raw), "# 覆盖保护 Skill") {
		t.Fatalf("expected overwritten skill markdown, got %q", string(raw))
	}
}

func TestInstallSkillPackageRejectsUnknownSkill(t *testing.T) {
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "skills.json"))
	result := service.InstallSkillPackage(InstallRequest{
		SkillID:    "missing",
		TargetRoot: filepath.Join(t.TempDir(), "codex-skills"),
		Confirmed:  true,
	})
	if result.OK {
		t.Fatalf("expected missing skill install to fail")
	}
	if !strings.Contains(result.Message, "未找到") {
		t.Fatalf("unexpected message: %s", result.Message)
	}
}

func readZipFile(t *testing.T, zipPath string, name string) string {
	t.Helper()
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		if file.Name != name {
			continue
		}
		handle, err := file.Open()
		if err != nil {
			t.Fatalf("open zip entry: %v", err)
		}
		defer handle.Close()
		raw, err := io.ReadAll(handle)
		if err != nil {
			t.Fatalf("read zip entry: %v", err)
		}
		return string(raw)
	}
	t.Fatalf("zip entry %s not found", name)
	return ""
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
