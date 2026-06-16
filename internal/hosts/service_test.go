package hosts

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ariadne/internal/contracts"
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

	if !strings.Contains(preview.FinalContent, ariadneStartMarker) || strings.Contains(preview.FinalContent, legacyStartMarker) {
		t.Fatalf("expected ariadne marker and stripped legacy marker:\n%s", preview.FinalContent)
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
