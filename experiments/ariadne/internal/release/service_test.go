package release

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStatusCountsDataRootsAndExistingBackups(t *testing.T) {
	base := t.TempDir()
	roamingRoot := filepath.Join(base, "roaming")
	virtualRoot := filepath.Join(base, "virtualized")
	missingRoot := filepath.Join(base, "missing")
	backupDir := filepath.Join(roamingRoot, "backups")

	writeTestFile(t, filepath.Join(roamingRoot, "config.json"), `{"theme":"light"}`)
	writeTestFile(t, filepath.Join(virtualRoot, "work_memory.json"), `[]`)
	writeTestFile(t, filepath.Join(backupDir, "old.zip"), "previous backup")

	service := NewServiceWithRoots([]DataRootStatus{
		{Kind: "roaming", Path: roamingRoot},
		{Kind: "virtualized", Path: virtualRoot},
		{Kind: "missing", Path: missingRoot},
	}, backupDir)

	status := service.Status()
	if status.BackupDir != backupDir || status.BackupCount != 1 || status.BackupBytes <= 0 || status.LatestBackup == "" {
		t.Fatalf("unexpected backup status: %#v", status)
	}
	if !strings.Contains(status.Notes[0], "不删除") {
		t.Fatalf("status should describe non-destructive backup behavior: %#v", status.Notes)
	}

	roaming := findRootStatus(t, status.DataRoots, "roaming")
	if !roaming.Exists || roaming.ArchiveName != "roaming" || roaming.FileCount != 1 || roaming.Bytes <= 0 {
		t.Fatalf("roaming root should exclude nested backups from data stats: %#v", roaming)
	}
	virtualized := findRootStatus(t, status.DataRoots, "virtualized")
	if !virtualized.Exists || virtualized.FileCount != 1 || virtualized.Bytes <= 0 {
		t.Fatalf("unexpected virtualized root: %#v", virtualized)
	}
	missing := findRootStatus(t, status.DataRoots, "missing")
	if missing.Exists || missing.FileCount != 0 || missing.Bytes != 0 {
		t.Fatalf("missing root should be reported without failing status: %#v", missing)
	}
}

func TestCreateRollbackCheckpointWritesManifestAndData(t *testing.T) {
	base := t.TempDir()
	roamingRoot := filepath.Join(base, "roaming")
	virtualRoot := filepath.Join(base, "virtualized")
	secondVirtualRoot := filepath.Join(base, "virtualized-two")
	backupDir := filepath.Join(roamingRoot, "backups")

	writeTestFile(t, filepath.Join(roamingRoot, "config.json"), `{"general":{"theme":"light"}}`)
	writeTestFile(t, filepath.Join(virtualRoot, "work_memory.json"), `[{"id":"wm-1"}]`)
	writeTestFile(t, filepath.Join(secondVirtualRoot, "config.json"), `{"virtualized":true}`)
	writeTestFile(t, filepath.Join(backupDir, "old.zip"), "do not pack this file")

	service := NewServiceWithRoots([]DataRootStatus{
		{Kind: "roaming", Path: roamingRoot},
		{Kind: "virtualized", Path: virtualRoot},
		{Kind: "virtualized", Path: secondVirtualRoot},
	}, backupDir)

	result := service.CreateRollbackCheckpoint(BackupRequest{Reason: "manual_settings_checkpoint"})
	if !result.OK || result.Path == "" || result.Bytes <= 0 || result.FileCount != 3 {
		t.Fatalf("unexpected checkpoint result: %#v", result)
	}

	entries := zipEntries(t, result.Path)
	for _, name := range []string{
		"manifest.json",
		"data/roaming/config.json",
		"data/virtualized/work_memory.json",
		"data/virtualized_2/config.json",
	} {
		if !entries[name] {
			t.Fatalf("checkpoint missing %s, entries=%#v", name, entries)
		}
	}
	if entries["data/roaming/backups/old.zip"] {
		t.Fatal("checkpoint should not include prior backups")
	}

	var manifest backupManifest
	readZipJSON(t, result.Path, "manifest.json", &manifest)
	if manifest.App != "Ariadne" || manifest.Kind != "rollback_checkpoint" || manifest.Reason != "manual_settings_checkpoint" {
		t.Fatalf("unexpected manifest identity: %#v", manifest)
	}
	if manifest.FileCount != 3 || manifest.Bytes <= 0 || len(manifest.RestoreNotes) == 0 {
		t.Fatalf("manifest should describe backed up data and restore notes: %#v", manifest)
	}
	if findRootStatus(t, manifest.Roots, "virtualized").ArchiveName == "" {
		t.Fatalf("manifest roots should include archive names: %#v", manifest.Roots)
	}
}

func TestCreateRollbackCheckpointAllowsEmptyRoots(t *testing.T) {
	base := t.TempDir()
	service := NewServiceWithRoots([]DataRootStatus{
		{Kind: "missing", Path: filepath.Join(base, "missing")},
	}, filepath.Join(base, "backups"))

	result := service.CreateRollbackCheckpoint(BackupRequest{})
	if !result.OK || result.Path == "" || result.FileCount != 0 || !strings.Contains(result.Message, "空回滚检查点") {
		t.Fatalf("unexpected empty checkpoint result: %#v", result)
	}

	var manifest backupManifest
	readZipJSON(t, result.Path, "manifest.json", &manifest)
	if manifest.FileCount != 0 || manifest.Kind != "rollback_checkpoint" || len(manifest.Roots) != 1 {
		t.Fatalf("unexpected empty manifest: %#v", manifest)
	}
}

func TestRestoreRollbackCheckpointRequiresConfirmation(t *testing.T) {
	base := t.TempDir()
	service := NewServiceWithRoots([]DataRootStatus{
		{Kind: "roaming", Path: filepath.Join(base, "roaming")},
	}, filepath.Join(base, "roaming", "backups"))

	result := service.RestoreRollbackCheckpoint(RestoreRequest{})
	if !result.RequiresConfirmation || result.OK {
		t.Fatalf("restore should require explicit confirmation: %#v", result)
	}
}

func TestRestoreRollbackCheckpointRestoresDataAndCreatesPreRestoreBackup(t *testing.T) {
	base := t.TempDir()
	roamingRoot := filepath.Join(base, "roaming")
	virtualRoot := filepath.Join(base, "virtualized")
	backupDir := filepath.Join(roamingRoot, "backups")

	writeTestFile(t, filepath.Join(roamingRoot, "config.json"), `{"general":{"theme":"light"}}`)
	writeTestFile(t, filepath.Join(virtualRoot, "work_memory.json"), `[{"id":"wm-1"}]`)
	service := NewServiceWithRoots([]DataRootStatus{
		{Kind: "roaming", ArchiveName: "roaming", Path: roamingRoot},
		{Kind: "virtualized", ArchiveName: "virtualized", Path: virtualRoot},
	}, backupDir)
	checkpoint := service.CreateRollbackCheckpoint(BackupRequest{Reason: "restore_test"})
	if !checkpoint.OK {
		t.Fatalf("create checkpoint: %#v", checkpoint)
	}

	writeTestFile(t, filepath.Join(roamingRoot, "config.json"), `{"general":{"theme":"dark"}}`)
	writeTestFile(t, filepath.Join(roamingRoot, "stale.json"), `{"stale":true}`)
	if err := os.Remove(filepath.Join(virtualRoot, "work_memory.json")); err != nil {
		t.Fatal(err)
	}

	restored := service.RestoreRollbackCheckpoint(RestoreRequest{
		Path:                   checkpoint.Path,
		Confirm:                true,
		CreatePreRestoreBackup: true,
	})
	if !restored.OK || restored.FileCount != 2 || restored.PreRestoreBackupPath == "" {
		t.Fatalf("unexpected restore result: %#v", restored)
	}
	if raw := readFile(t, filepath.Join(roamingRoot, "config.json")); !strings.Contains(raw, "light") {
		t.Fatalf("config should be restored from checkpoint, got %s", raw)
	}
	if _, err := os.Stat(filepath.Join(roamingRoot, "stale.json")); !os.IsNotExist(err) {
		t.Fatalf("stale file should be removed during restore, err=%v", err)
	}
	if raw := readFile(t, filepath.Join(virtualRoot, "work_memory.json")); !strings.Contains(raw, "wm-1") {
		t.Fatalf("virtualized data should be restored, got %s", raw)
	}
	if _, err := os.Stat(restored.PreRestoreBackupPath); err != nil {
		t.Fatalf("pre-restore backup should exist: %v", err)
	}
	if _, err := os.Stat(backupDir); err != nil {
		t.Fatalf("backup dir should be preserved during restore: %v", err)
	}
}

func TestRealUserRollbackSmoke(t *testing.T) {
	if os.Getenv("ARIADNE_TEST_REAL_ROLLBACK") != "1" {
		t.Skip("set ARIADNE_TEST_REAL_ROLLBACK=1 to run the real user data rollback smoke")
	}

	service := NewService()
	status := service.Status()
	roots := existingRealRoots(status.DataRoots)
	if len(roots) == 0 {
		t.Fatalf("real rollback smoke requires at least one existing Ariadne data root: %#v", status.DataRoots)
	}

	runID := fmt.Sprintf("real-%d-%d", time.Now().UTC().UnixNano(), os.Getpid())
	createdBackups := []string{}
	passed := false
	defer func() {
		for _, root := range roots {
			_ = os.RemoveAll(smokeDir(root, runID))
			_ = os.Remove(smokeRootDir(root))
		}
		if passed {
			for _, path := range createdBackups {
				if strings.TrimSpace(path) != "" {
					_ = os.Remove(path)
				}
			}
		}
	}()

	safety := service.CreateRollbackCheckpoint(BackupRequest{Reason: "real_user_rollback_smoke_safety:" + runID})
	if !safety.OK || safety.Path == "" {
		t.Fatalf("create pre-smoke safety checkpoint: %#v", safety)
	}
	createdBackups = append(createdBackups, safety.Path)

	baselineByRoot := map[string]string{}
	for _, root := range roots {
		baseline := fmt.Sprintf("baseline:%s:%s:%s", runID, root.Kind, root.ArchiveName)
		baselineByRoot[root.Path] = baseline
		writeTestFile(t, smokeSentinelPath(root, runID), baseline)
	}

	checkpoint := service.CreateRollbackCheckpoint(BackupRequest{Reason: "real_user_rollback_smoke:" + runID})
	if !checkpoint.OK || checkpoint.Path == "" {
		t.Fatalf("create smoke checkpoint: %#v", checkpoint)
	}
	createdBackups = append(createdBackups, checkpoint.Path)

	for _, root := range roots {
		writeTestFile(t, smokeSentinelPath(root, runID), "mutated:"+runID)
		writeTestFile(t, filepath.Join(smokeDir(root, runID), "stale.txt"), "stale:"+runID)
	}

	restored := service.RestoreRollbackCheckpoint(RestoreRequest{
		Path:                   checkpoint.Path,
		Confirm:                true,
		CreatePreRestoreBackup: true,
	})
	if restored.PreRestoreBackupPath != "" {
		createdBackups = append(createdBackups, restored.PreRestoreBackupPath)
	}
	if !restored.OK || restored.PreRestoreBackupPath == "" {
		t.Fatalf("restore real smoke checkpoint: %#v", restored)
	}

	for _, root := range roots {
		if raw := readFile(t, smokeSentinelPath(root, runID)); raw != baselineByRoot[root.Path] {
			t.Fatalf("sentinel was not restored for %s: got %q want %q", root.Path, raw, baselineByRoot[root.Path])
		}
		if _, err := os.Stat(filepath.Join(smokeDir(root, runID), "stale.txt")); !os.IsNotExist(err) {
			t.Fatalf("stale smoke file should be removed for %s, err=%v", root.Path, err)
		}
	}

	for _, root := range roots {
		if err := os.RemoveAll(smokeDir(root, runID)); err != nil {
			t.Fatalf("cleanup smoke dir %s: %v", root.Path, err)
		}
		_ = os.Remove(smokeRootDir(root))
	}
	passed = true
}

func existingRealRoots(roots []DataRootStatus) []DataRootStatus {
	items := []DataRootStatus{}
	for _, root := range roots {
		if root.Exists && strings.TrimSpace(root.Path) != "" {
			items = append(items, root)
		}
	}
	return items
}

func smokeDir(root DataRootStatus, runID string) string {
	return filepath.Join(smokeRootDir(root), runID)
}

func smokeRootDir(root DataRootStatus) string {
	return filepath.Join(root.Path, ".ariadne-rollback-smoke")
}

func smokeSentinelPath(root DataRootStatus, runID string) string {
	return filepath.Join(smokeDir(root, runID), "sentinel.txt")
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func findRootStatus(t *testing.T, roots []DataRootStatus, kind string) DataRootStatus {
	t.Helper()
	for _, root := range roots {
		if root.Kind == kind {
			return root
		}
	}
	t.Fatalf("missing root kind %q in %#v", kind, roots)
	return DataRootStatus{}
}

func zipEntries(t *testing.T, path string) map[string]bool {
	t.Helper()
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open zip %s: %v", path, err)
	}
	defer reader.Close()

	entries := map[string]bool{}
	for _, file := range reader.File {
		entries[file.Name] = true
	}
	return entries
}

func readZipJSON(t *testing.T, path string, name string, target interface{}) {
	t.Helper()
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open zip %s: %v", path, err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		if file.Name != name {
			continue
		}
		item, openErr := file.Open()
		if openErr != nil {
			t.Fatalf("open zip item %s: %v", name, openErr)
		}
		defer item.Close()
		if err := json.NewDecoder(item).Decode(target); err != nil {
			t.Fatalf("decode zip item %s: %v", name, err)
		}
		return
	}
	t.Fatalf("missing zip item %s", name)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}
