package secrets

import "testing"

type fakeStore struct {
	available bool
	backend   string
	values    map[string]string
}

func (f *fakeStore) Available() bool { return f.available }
func (f *fakeStore) Backend() string { return f.backend }
func (f *fakeStore) Read(target string) (string, bool, error) {
	value, ok := f.values[target]
	return value, ok, nil
}
func (f *fakeStore) Write(target string, secret string) error {
	if f.values == nil {
		f.values = map[string]string{}
	}
	f.values[target] = secret
	return nil
}
func (f *fakeStore) Delete(target string) error {
	delete(f.values, target)
	return nil
}

func TestStatusReportsEnvironmentAndStoredSecrets(t *testing.T) {
	t.Setenv("OPENAI__API_KEY", "env-key")
	store := &fakeStore{available: true, backend: "test", values: map[string]string{"Ariadne/Embedding/APIKey": "stored"}}
	service := NewServiceWithStore(store)

	status := service.Status()
	if !status.Available || status.Backend != "test" {
		t.Fatalf("unexpected status backend: %#v", status)
	}
	ai := record(status, "ai_api_key")
	if !ai.EnvPresent || ai.Stored || ai.ActiveSource != "environment" {
		t.Fatalf("AI record should prefer env and report no stored fallback: %#v", ai)
	}
	embedding := record(status, "embedding_api_key")
	if !embedding.Stored || embedding.ActiveSource != "environment" {
		t.Fatalf("embedding record should see shared OPENAI env and stored credential: %#v", embedding)
	}
}

func TestSaveAndClearSecret(t *testing.T) {
	store := &fakeStore{available: true, backend: "test", values: map[string]string{}}
	service := NewServiceWithStore(store)

	targets := map[string]string{
		"ai_api_key":        "Ariadne/OpenAI/APIKey",
		"embedding_api_key": "Ariadne/Embedding/APIKey",
		"milvus_token":      "Ariadne/Milvus/Token",
	}
	for kind, target := range targets {
		t.Run(kind, func(t *testing.T) {
			save := service.SaveSecret(SaveSecretRequest{Kind: kind, Value: "  secret-value-" + kind + "  "})
			if !save.OK || !record(save.Status, kind).Stored {
				t.Fatalf("save should store secret: %#v", save)
			}
			if store.values[target] != "secret-value-"+kind {
				t.Fatalf("secret was not stored trimmed in backend: %#v", store.values)
			}

			preview := service.ClearSecret(ClearSecretRequest{Kind: kind})
			if !preview.RequiresConfirmation || preview.OK {
				t.Fatalf("clear should require confirmation first: %#v", preview)
			}
			if store.values[target] == "" {
				t.Fatalf("preview clear must not delete %s", kind)
			}
			clear := service.ClearSecret(ClearSecretRequest{Kind: kind, Confirm: true})
			if !clear.OK || record(clear.Status, kind).Stored {
				t.Fatalf("clear should remove stored secret: %#v", clear)
			}
			if _, ok := store.values[target]; ok {
				t.Fatalf("confirmed clear should delete backend value: %#v", store.values)
			}
		})
	}
}

func TestUnavailableStoreBlocksWrite(t *testing.T) {
	service := NewServiceWithStore(&fakeStore{available: false, backend: "test", values: map[string]string{}})
	result := service.SaveSecret(SaveSecretRequest{Kind: "ai_api_key", Value: "secret"})
	if result.OK {
		t.Fatalf("unavailable store should block writes: %#v", result)
	}
}

func record(status SecretStatus, kind string) SecretRecordStatus {
	for _, record := range status.Records {
		if record.Kind == kind {
			return record
		}
	}
	return SecretRecordStatus{}
}
