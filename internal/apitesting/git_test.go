package apitesting

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitFileCollectionRedactsSecretVariablesWithoutMutatingOriginal(t *testing.T) {
	original := Collection{
		ID:   "col-secret",
		Name: "secret-test",
		Variables: []Variable{
			{ID: "var-public", Name: "baseUrl", Value: "https://api.example.com", Enabled: true, Secret: false},
			{ID: "var-secret", Name: "apiKey", Value: "super-secret-token-123", Enabled: true, Secret: true},
		},
		Environments: []Environment{
			{
				ID:   "env-prod",
				Name: "prod",
				Variables: []Variable{
					{ID: "env-public", Name: "region", Value: "cn-east", Enabled: true, Secret: false},
					{ID: "env-secret", Name: "dbPassword", Value: "p@ssw0rd!", Enabled: false, Secret: true},
				},
			},
		},
	}

	payload := gitFileCollection(original)

	if len(payload.Variables) != 2 {
		t.Fatalf("expected 2 collection variables, got %#v", payload.Variables)
	}
	if payload.Variables[0].Value != "https://api.example.com" || payload.Variables[0].Secret {
		t.Fatalf("non-secret collection variable should be preserved: %#v", payload.Variables[0])
	}
	if payload.Variables[1].ID != "var-secret" || payload.Variables[1].Name != "apiKey" || !payload.Variables[1].Enabled || !payload.Variables[1].Secret {
		t.Fatalf("secret collection variable metadata should be preserved: %#v", payload.Variables[1])
	}
	if payload.Variables[1].Value != "" {
		t.Fatalf("secret collection variable value should be redacted, got %q", payload.Variables[1].Value)
	}

	if len(payload.Environments) != 1 || len(payload.Environments[0].Variables) != 2 {
		t.Fatalf("expected 2 environment variables, got %#v", payload.Environments)
	}
	if payload.Environments[0].Variables[0].Value != "cn-east" {
		t.Fatalf("non-secret env variable should be preserved: %#v", payload.Environments[0].Variables[0])
	}
	envSecret := payload.Environments[0].Variables[1]
	if envSecret.ID != "env-secret" || envSecret.Name != "dbPassword" || envSecret.Enabled || !envSecret.Secret {
		t.Fatalf("secret env variable metadata should be preserved: %#v", envSecret)
	}
	if envSecret.Value != "" {
		t.Fatalf("secret env variable value should be redacted, got %q", envSecret.Value)
	}

	if original.Variables[1].Value != "super-secret-token-123" || original.Environments[0].Variables[1].Value != "p@ssw0rd!" {
		t.Fatal("gitFileCollection mutated original collection secret values")
	}
}

func TestGitSyncDoesNotWriteSecretValuesToCollectionFile(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "api_testing.json"), nil)
	status := service.UpsertCollection(Collection{
		ID:   "col-git-secrets",
		Name: "secret-sync",
		Variables: []Variable{
			{ID: "v1", Name: "host", Value: "https://example.test", Enabled: true, Secret: false},
			{ID: "v2", Name: "masterToken", Value: "top-secret-collection-value", Enabled: true, Secret: true},
		},
		Environments: []Environment{{
			ID:   "env-main",
			Name: "prod",
			Variables: []Variable{
				{ID: "ev1", Name: "envName", Value: "production", Enabled: true, Secret: false},
				{ID: "ev2", Name: "clientSecret", Value: "top-secret-env-value", Enabled: true, Secret: true},
			},
		}},
		Requests: []Request{{
			ID:     "req-1",
			Name:   "ping",
			Method: http.MethodGet,
			URL:    "{{host}}/ping",
		}},
	})
	if status.LastSaveError != "" {
		t.Fatalf("unexpected save error: %s", status.LastSaveError)
	}

	repoPath := filepath.Join(t.TempDir(), "api-repo")
	gitStatus := service.ConfigureGit("col-git-secrets", repoPath, "")
	if !gitStatus.OK {
		t.Fatalf("expected git config ok, got %#v", gitStatus)
	}

	raw, err := os.ReadFile(filepath.Join(repoPath, gitCollectionFileName))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Contains(text, "top-secret-collection-value") || strings.Contains(text, "top-secret-env-value") {
		t.Fatalf("secret value leaked into %s: %s", gitCollectionFileName, text)
	}

	var synced Collection
	if err := json.Unmarshal(raw, &synced); err != nil {
		t.Fatal(err)
	}
	if synced.Variables[0].Value != "https://example.test" || synced.Variables[1].Value != "" || !synced.Variables[1].Secret {
		t.Fatalf("unexpected synced collection variables: %#v", synced.Variables)
	}
	if synced.Environments[0].Variables[0].Value != "production" || synced.Environments[0].Variables[1].Value != "" || !synced.Environments[0].Variables[1].Secret {
		t.Fatalf("unexpected synced env variables: %#v", synced.Environments[0].Variables)
	}

	// Ensure local service state still retains the secret values.
	inMemory := service.Status().Collections
	var found *Collection
	for i := range inMemory {
		if inMemory[i].ID == "col-git-secrets" {
			found = &inMemory[i]
			break
		}
	}
	if found == nil || found.Variables[1].Value != "top-secret-collection-value" || found.Environments[0].Variables[1].Value != "top-secret-env-value" {
		t.Fatalf("local secret values should remain intact in service state: %#v", found)
	}
}

func TestValidateGitRemoteAllowlistAndRejection(t *testing.T) {
	valid := []string{
		"",
		"https://github.com/glwlg/Ariadne.git",
		"http://git.internal:8080/team/api.git",
		"ssh://git@example.com:2222/team/api.git",
		"git@github.com:glwlg/Ariadne.git",
		"git@10.0.0.5:org/sub/repo.git",
	}
	for _, remote := range valid {
		if err := validateGitRemote(remote); err != nil {
			t.Fatalf("expected valid remote %q, got error: %v", remote, err)
		}
	}

	invalid := []string{
		"-oProxyCommand=evil",
		"--upload-pack=evil",
		"ext::sh -c touch",
		"ext::touch",
		"fd::3",
		"file:///etc/passwd",
		"ftp://example.com/repo.git",
		"git://example.com/repo.git",
		"/tmp/local-repo",
		"https:///missing-host",
		"https://example.com/repo.git\n--evil",
		"https://example.com/repo with spaces",
		"git@",
		"git@:repo.git",
		"git@github.com:",
		"git@-badhost:repo.git",
		"git@github.com:-badpath",
		"git@github.com:ext::evil",
	}
	for _, remote := range invalid {
		if err := validateGitRemote(remote); err == nil {
			t.Fatalf("expected invalid remote %q to be rejected", remote)
		}
	}
}

func TestEnsureGitRepositoryRejectsUnsafeRemoteAndSetsValidOrigin(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	badDir := filepath.Join(t.TempDir(), "bad-repo")
	for _, badRemote := range []string{"--upload-pack=evil", "ext::sh -c id", "file:///tmp/repo"} {
		if err := ensureGitRepository(badDir, badRemote); err == nil {
			t.Fatalf("expected ensureGitRepository to reject %q", badRemote)
		}
		if _, err := os.Stat(filepath.Join(badDir, ".git")); !os.IsNotExist(err) {
			t.Fatalf("rejected remote %q should not initialize git repository", badRemote)
		}
	}

	goodDir := filepath.Join(t.TempDir(), "good-repo")
	if err := ensureGitRepository(goodDir, "https://github.com/glwlg/Ariadne.git"); err != nil {
		t.Fatalf("ensureGitRepository add remote failed: %v", err)
	}
	got, err := runGit(goodDir, "remote", "get-url", "--", "origin")
	if err != nil || got != "https://github.com/glwlg/Ariadne.git" {
		t.Fatalf("unexpected origin url: %q err=%v", got, err)
	}
	if err := ensureGitRepository(goodDir, "git@github.com:glwlg/Ariadne.git"); err != nil {
		t.Fatalf("ensureGitRepository set-url failed: %v", err)
	}
	got, err = runGit(goodDir, "remote", "get-url", "--", "origin")
	if err != nil || got != "git@github.com:glwlg/Ariadne.git" {
		t.Fatalf("unexpected updated origin url: %q err=%v", got, err)
	}
}
