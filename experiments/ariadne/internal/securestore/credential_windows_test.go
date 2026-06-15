//go:build windows

package securestore

import (
	"os"
	"testing"
)

func TestWindowsCredentialManagerRoundTrip(t *testing.T) {
	if os.Getenv("ARIADNE_TEST_CREDENTIAL_MANAGER") != "1" {
		t.Skip("set ARIADNE_TEST_CREDENTIAL_MANAGER=1 to test the real Windows Credential Manager")
	}
	target := "Ariadne/Test/CredentialManagerRoundTrip"
	_ = Delete(target)
	t.Cleanup(func() { _ = Delete(target) })

	if err := Write(target, "dummy-secret"); err != nil {
		t.Fatalf("write credential: %v", err)
	}
	value, ok, err := Read(target)
	if err != nil {
		t.Fatalf("read credential: %v", err)
	}
	if !ok || value != "dummy-secret" {
		t.Fatalf("unexpected credential value ok=%v value=%q", ok, value)
	}
	if err := Delete(target); err != nil {
		t.Fatalf("delete credential: %v", err)
	}
	_, ok, err = Read(target)
	if err != nil {
		t.Fatalf("read deleted credential: %v", err)
	}
	if ok {
		t.Fatal("credential should be removed")
	}
}

func TestConfiguredAriadneCredentialsReadable(t *testing.T) {
	if os.Getenv("ARIADNE_TEST_CONFIGURED_CREDENTIALS") != "1" {
		t.Skip("set ARIADNE_TEST_CONFIGURED_CREDENTIALS=1 to verify configured Ariadne credentials")
	}
	for _, target := range []string{TargetOpenAIAPIKey, TargetEmbeddingAPIKey} {
		value, ok, err := Read(target)
		if err != nil {
			t.Fatalf("read %s: %v", target, err)
		}
		if !ok || value == "" {
			t.Fatalf("%s should be present and non-empty", target)
		}
	}
}
