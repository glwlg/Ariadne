package hosts

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fancun/internal/contracts"
)

func TestHostsPreviewMergesEnabledProfilesAndDetectsConflicts(t *testing.T) {
	root := t.TempDir()
	hostsPath := filepath.Join(root, "hosts")
	configPath := filepath.Join(root, "hosts_profiles.json")
	if err := os.WriteFile(hostsPath, []byte("127.0.0.1 localhost\n"+legacyStartMarker+"\n1.1.1.1 old.local\n"+legacyEndMarker+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithPaths(configPath, hostsPath, "")
	service.Upsert(Profile{ID: "alpha", Title: "Alpha", Content: "10.0.0.1 app.local\n10.0.0.2 app.local\n", Enabled: true, Type: localProfile})

	preview := service.PreviewApply()

	if !strings.Contains(preview.FinalContent, fancunStartMarker) || strings.Contains(preview.FinalContent, legacyStartMarker) {
		t.Fatalf("expected fancun marker and stripped legacy marker:\n%s", preview.FinalContent)
	}
	if len(preview.Conflicts) != 1 || preview.Conflicts[0].Host != "app.local" {
		t.Fatalf("expected app.local conflict, got %#v", preview.Conflicts)
	}
	if !preview.Changed || preview.AddedLines == 0 {
		t.Fatalf("expected changed preview, got %#v", preview)
	}
}

func TestHostsProfilesPersistAndSystemProfileIsNotRemoved(t *testing.T) {
	root := t.TempDir()
	hostsPath := filepath.Join(root, "hosts")
	configPath := filepath.Join(root, "hosts_profiles.json")
	if err := os.WriteFile(hostsPath, []byte("127.0.0.1 localhost\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithPaths(configPath, hostsPath, "")
	service.Upsert(Profile{ID: "dev", Title: "Dev", Content: "127.0.0.1 dev.local", Enabled: true, Type: localProfile})
	service.Remove(systemProfileID)

	reloaded := NewServiceWithPaths(configPath, hostsPath, "")
	status := reloaded.Status()
	if status.Count != 2 || status.EnabledCount != 1 {
		t.Fatalf("expected system + one enabled profile, got %#v", status)
	}
	if status.Profiles[0].ID != systemProfileID {
		t.Fatalf("expected system profile first, got %#v", status.Profiles)
	}
}

func TestHostsRemoteFetchUpdatesProfile(t *testing.T) {
	root := t.TempDir()
	hostsPath := filepath.Join(root, "hosts")
	configPath := filepath.Join(root, "hosts_profiles.json")
	if err := os.WriteFile(hostsPath, []byte("127.0.0.1 localhost\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("10.1.2.3 remote.local\n"))
	}))
	defer server.Close()

	service := NewServiceWithPaths(configPath, hostsPath, "")
	service.Upsert(Profile{ID: "remote", Title: "Remote", Type: remoteProfile, URL: server.URL})
	status := service.FetchRemote("remote")

	if status.LastRemoteError != "" {
		t.Fatalf("remote fetch failed: %s", status.LastRemoteError)
	}
	found := false
	for _, profile := range status.Profiles {
		if profile.ID == "remote" && strings.Contains(profile.Content, "remote.local") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected remote content in status: %#v", status.Profiles)
	}
}

func TestHostsApplyRequiresConfirmationBeforeWrite(t *testing.T) {
	root := t.TempDir()
	hostsPath := filepath.Join(root, "hosts")
	configPath := filepath.Join(root, "hosts_profiles.json")
	original := "127.0.0.1 localhost\n"
	if err := os.WriteFile(hostsPath, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithPaths(configPath, hostsPath, "")
	service.Upsert(Profile{ID: "dev", Title: "Dev", Content: "127.0.0.1 dev.local", Enabled: true, Type: localProfile})

	result := service.ApplyEnabledProfiles(false)
	raw, err := os.ReadFile(hostsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !result.RequiresConfirm || result.OK {
		t.Fatalf("expected confirmation requirement, got %#v", result)
	}
	if string(raw) != original {
		t.Fatalf("hosts file should not change before confirmation: %q", string(raw))
	}
}

func TestHostsTriggerActionSurface(t *testing.T) {
	result := contracts.SearchResult{
		ID:       "hosts-window",
		Type:     contracts.ResultCommand,
		Title:    "打开 Hosts 管理",
		Subtitle: "Hosts 管理",
		Icon:     "command",
		Preview:  contracts.PreviewDescriptor{Kind: contracts.PreviewText, Title: "Hosts"},
		Actions:  []contracts.PreviewAction{contracts.PluginAction("open_tool", "打开 Hosts 管理", "open_hosts")},
	}
	if err := contracts.ValidateActionSurface(result); err != nil {
		t.Fatalf("invalid action surface: %v", err)
	}
}

func TestHostsUpsertRejectsEmptyTitleWithExplicitError(t *testing.T) {
	root := t.TempDir()
	hostsPath := filepath.Join(root, "hosts")
	configPath := filepath.Join(root, "hosts_profiles.json")
	if err := os.WriteFile(hostsPath, []byte("127.0.0.1 localhost\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithPaths(configPath, hostsPath, "")

	status := service.Upsert(Profile{Title: "   ", Content: "127.0.0.1 empty.local"})
	if status.LastSaveError == "" {
		t.Fatal("expected explicit LastSaveError for empty title on new profile")
	}
	if status.Count != 1 {
		t.Fatalf("empty title profile must not be saved, got count=%d", status.Count)
	}

	status = service.Upsert(Profile{ID: "dev", Title: "Dev", Content: "127.0.0.1 dev.local"})
	if status.LastSaveError != "" || status.Count != 2 {
		t.Fatalf("valid upsert failed: %#v", status)
	}

	status = service.Upsert(Profile{ID: "dev", Title: "   ", Content: "127.0.0.1 wiped.local"})
	if status.LastSaveError == "" {
		t.Fatal("expected explicit LastSaveError when clearing title on existing profile")
	}
	reloaded := NewServiceWithPaths(configPath, hostsPath, "")
	for _, profile := range reloaded.List() {
		if profile.ID == "dev" && (profile.Title != "Dev" || profile.Content != "127.0.0.1 dev.local") {
			t.Fatalf("existing profile was overwritten despite empty title: %#v", profile)
		}
	}
}

func TestHostsApplyCreatesBackupAndRollbackRestoresOriginal(t *testing.T) {
	root := t.TempDir()
	hostsPath := filepath.Join(root, "hosts")
	configPath := filepath.Join(root, "hosts_profiles.json")
	original := "127.0.0.1 localhost\n# keep comment\n"
	if err := os.WriteFile(hostsPath, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithPaths(configPath, hostsPath, "")
	if status := service.Status(); status.CanRollback || status.LastBackupAt != "" {
		t.Fatalf("expected no rollback available before apply, got %#v", status)
	}
	if emptyRollback := service.RollbackHosts(); emptyRollback.OK || emptyRollback.LastApplyError == "" {
		t.Fatalf("rollback without backup should fail explicitly, got %#v", emptyRollback)
	}

	service.Upsert(Profile{ID: "dev", Title: "Dev", Content: "127.0.0.1 dev.local", Enabled: true, Type: localProfile})
	applied := service.ApplyEnabledProfiles(true)
	if !applied.OK {
		t.Fatalf("expected apply to succeed: %#v", applied)
	}
	raw, err := os.ReadFile(hostsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "127.0.0.1 dev.local") {
		t.Fatalf("expected applied rule in hosts file: %q", string(raw))
	}

	reloaded := NewServiceWithPaths(configPath, hostsPath, "")
	status := reloaded.Status()
	if !status.CanRollback || status.LastBackupAt == "" {
		t.Fatalf("expected persisted rollback availability after apply, got %#v", status)
	}
	if _, err := time.Parse(time.RFC3339, status.LastBackupAt); err != nil {
		t.Fatalf("expected RFC3339 LastBackupAt, got %q: %v", status.LastBackupAt, err)
	}

	rolledBack := reloaded.RollbackHosts()
	if !rolledBack.OK || rolledBack.Preview.FinalContent != original {
		t.Fatalf("rollback failed: %#v", rolledBack)
	}
	restoredRaw, err := os.ReadFile(hostsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(restoredRaw) != original {
		t.Fatalf("hosts file mismatch after rollback: got %q want %q", string(restoredRaw), original)
	}
}

func TestHostsFailedApplyDoesNotLeaveFalseRollbackBackup(t *testing.T) {
	root := t.TempDir()
	hostsPath := filepath.Join(root, "hosts")
	configPath := filepath.Join(root, "hosts_profiles.json")
	original := "127.0.0.1 localhost\n"
	if err := os.WriteFile(hostsPath, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithPaths(configPath, hostsPath, "")
	service.Upsert(Profile{ID: "dev", Title: "Dev", Content: "127.0.0.1 dev.local", Enabled: true, Type: localProfile})
	service.writeHosts = func(string, string) error {
		return errors.New("用户取消了管理员授权")
	}

	res := service.ApplyEnabledProfiles(true)
	if res.OK || res.LastApplyError == "" {
		t.Fatalf("expected failed apply result, got %#v", res)
	}
	if status := service.Status(); status.CanRollback || status.LastBackupAt != "" {
		t.Fatalf("cancelled/failed apply must not advertise rollback backup: %#v", status)
	}
}

func TestHostsElevatedWriteUsesRunasWaitWithSeparateArgs(t *testing.T) {
	tmpRoot := filepath.Join(t.TempDir(), "O'Brien & Co")
	if err := os.MkdirAll(tmpRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", tmpRoot)
	t.Setenv("TEMP", tmpRoot)
	t.Setenv("TMP", tmpRoot)

	origRunas := runasWaitFn
	t.Cleanup(func() { runasWaitFn = origRunas })

	var gotFile string
	var gotArgs []string
	var scriptBody string
	runasWaitFn = func(file string, args []string) error {
		gotFile = file
		gotArgs = append([]string{}, args...)
		// Extract scriptPath from -File arg and simulate PowerShell writing SUCCESS to resultPath.
		for i := 0; i+1 < len(args); i++ {
			if args[i] == "-File" {
				raw, err := os.ReadFile(args[i+1])
				if err != nil {
					return err
				}
				scriptBody = string(raw)
				// Locate resultPath inside script: Set-Content -LiteralPath '...'
				const marker = "Set-Content -LiteralPath '"
				idx := strings.Index(scriptBody, marker)
				if idx >= 0 {
					rest := scriptBody[idx+len(marker):]
					end := strings.Index(rest, "' -Value 'SUCCESS'")
					if end >= 0 {
						resPath := strings.ReplaceAll(rest[:end], "''", "'")
						return os.WriteFile(resPath, []byte("SUCCESS\n"), 0o600)
					}
				}
			}
		}
		return errors.New("script did not write result")
	}

	targetHosts := filepath.Join(tmpRoot, "hosts")
	if err := writeHostsContentElevated(targetHosts, "127.0.0.1 localhost\n"); err != nil {
		t.Fatalf("writeHostsContentElevated failed: %v", err)
	}
	if gotFile != "powershell.exe" {
		t.Fatalf("expected powershell.exe, got %q", gotFile)
	}
	joined := strings.Join(gotArgs, "|")
	if strings.Contains(joined, "-Command") || !strings.Contains(joined, "-WindowStyle|Hidden|-File|") {
		t.Fatalf("expected discrete -WindowStyle Hidden -File args without -Command interpolation, got %#v", gotArgs)
	}
	if !strings.Contains(scriptBody, "O''Brien & Co") {
		t.Fatalf("expected escaped single quote in PowerShell script body, got:\n%s", scriptBody)
	}
}
