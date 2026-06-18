package ocr

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ariadne/internal/capturehistory"
	"ariadne/internal/clipboardhistory"
	"ariadne/internal/workmemory"
)

func TestRecognizeCaptureUsesExplicitCaptureImage(t *testing.T) {
	imagePath := testImage(t)
	service := NewService(
		fakeCaptures{entry: capturehistory.Entry{ID: "cap-1", ImagePath: imagePath, Width: 120, Height: 80}},
		nil,
		nil,
	)
	service.runner = func(_ context.Context, path string) (bridgeOutput, Status) {
		if path != imagePath {
			t.Fatalf("expected image path %q, got %q", imagePath, path)
		}
		return bridgeOutput{OK: true, Provider: "test-ocr", Text: "hello OCR"}, Status{Available: true, Provider: "test-ocr"}
	}

	result := service.RecognizeCapture("cap-1")

	if !result.OK || result.Text != "hello OCR" || result.CaptureID != "cap-1" || result.Width != 120 || result.Height != 80 {
		t.Fatalf("unexpected OCR result: %#v", result)
	}
}

func TestRecognizeClipboardImageRejectsTextEntries(t *testing.T) {
	service := NewService(nil, fakeClipboard{entry: clipboardhistory.Entry{ID: "clip-1", Type: clipboardhistory.EntryText, Text: "not image"}}, nil)

	result := service.RecognizeClipboardImage("clip-1")

	if result.OK || result.Error == "" {
		t.Fatalf("expected text clipboard entry to be rejected: %#v", result)
	}
}

func TestRecognizeWorkMemoryWritesOCRText(t *testing.T) {
	imagePath := testImage(t)
	memory := &fakeMemory{entry: workmemory.Entry{
		ID:        "memory-1",
		ImagePath: imagePath,
		Width:     32,
		Height:    16,
		Tags:      []string{"截图"},
	}}
	service := NewService(nil, nil, memory)
	service.runner = func(_ context.Context, path string) (bridgeOutput, Status) {
		if path != imagePath {
			t.Fatalf("expected image path %q, got %q", imagePath, path)
		}
		return bridgeOutput{OK: true, Provider: "test-ocr", Text: "gateway error"}, Status{Available: true, Provider: "test-ocr"}
	}

	result := service.RecognizeWorkMemory("memory-1")

	if !result.OK || result.WorkMemory == nil {
		t.Fatalf("expected OCR result with updated memory entry: %#v", result)
	}
	if memory.entry.OCRText != "gateway error" || memory.entry.OCRStatus != "done:test-ocr" || memory.entry.ContentType != "ocr_text" {
		t.Fatalf("OCR was not written back to memory: %#v", memory.entry)
	}
}

func TestRecognizeImagePrefersConfiguredAIOCR(t *testing.T) {
	imagePath := testImage(t)
	service := NewService(nil, nil, nil)
	service.runner = func(_ context.Context, _ string) (bridgeOutput, Status) {
		t.Fatal("local OCR runner should not run when AI OCR succeeds")
		return bridgeOutput{}, Status{}
	}
	RegisterAIClient(service, &fakeAIClient{
		result: AIResult{Provider: "vision:test-model", Text: "视觉 OCR 文本", Lines: []Line{{Text: "视觉 OCR 文本", Confidence: 0.99}}},
	})
	service.ApplyAIOCRPolicy(AIOCRPolicy{Enabled: true, Provider: "openai-compatible", BaseURL: "http://127.0.0.1:4000/v1", Model: "vision-model"})

	result := service.RecognizeImagePath(imagePath)

	if !result.OK || result.Text != "视觉 OCR 文本" || result.Provider != "vision:test-model" {
		t.Fatalf("expected AI OCR result, got %#v", result)
	}
	if len(result.Lines) != 1 || result.Lines[0].Text != "视觉 OCR 文本" {
		t.Fatalf("expected AI OCR lines, got %#v", result.Lines)
	}
}

func TestRecognizeImageFallsBackToLocalOCRWhenAIFails(t *testing.T) {
	imagePath := testImage(t)
	service := NewService(nil, nil, nil)
	localCalled := false
	service.runner = func(_ context.Context, path string) (bridgeOutput, Status) {
		localCalled = true
		if path != imagePath {
			t.Fatalf("expected image path %q, got %q", imagePath, path)
		}
		return bridgeOutput{OK: true, Provider: localOCRProvider, Text: "本地 OCR 文本"}, Status{Available: true, Provider: localOCRProvider}
	}
	RegisterAIClient(service, &fakeAIClient{err: errors.New("vision provider unavailable")})
	service.ApplyAIOCRPolicy(AIOCRPolicy{Enabled: true, Provider: "openai-compatible", Model: "vision-model"})

	result := service.RecognizeImagePath(imagePath)

	if !localCalled || !result.OK || result.Text != "本地 OCR 文本" || result.Provider != localOCRProvider {
		t.Fatalf("expected local OCR fallback, got result=%#v localCalled=%v", result, localCalled)
	}
}

func TestRecognizeImageUsesLocalOCRWhenAIOCRIsNotConfigured(t *testing.T) {
	imagePath := testImage(t)
	service := NewService(nil, nil, nil)
	localCalled := false
	service.runner = func(_ context.Context, _ string) (bridgeOutput, Status) {
		localCalled = true
		return bridgeOutput{OK: true, Provider: localOCRProvider, Text: "本地 OCR 文本"}, Status{Available: true, Provider: localOCRProvider}
	}
	RegisterAIClient(service, &fakeAIClient{
		result: AIResult{Provider: "vision:test-model", Text: "不应该使用"},
	})
	service.ApplyAIOCRPolicy(AIOCRPolicy{Enabled: true, Provider: "openai-compatible"})

	result := service.RecognizeImagePath(imagePath)

	if !localCalled || !result.OK || result.Text != "本地 OCR 文本" {
		t.Fatalf("expected local OCR when AI model is missing, got result=%#v localCalled=%v", result, localCalled)
	}
}

func TestRecognizeImageSensitiveFlagRequiresCredentialShape(t *testing.T) {
	imagePath := testImage(t)
	service := NewService(nil, nil, nil)
	service.runner = func(_ context.Context, _ string) (bridgeOutput, Status) {
		return bridgeOutput{OK: true, Provider: "test-ocr", Text: "登录页包含密码输入框、验证码按钮和 token 配置说明"}, Status{Available: true, Provider: "test-ocr"}
	}

	normal := service.RecognizeImagePath(imagePath)
	if !normal.OK || normal.Sensitive {
		t.Fatalf("plain security wording should not mark OCR result sensitive: %#v", normal)
	}

	service.runner = func(_ context.Context, _ string) (bridgeOutput, Status) {
		return bridgeOutput{OK: true, Provider: "test-ocr", Text: "Authorization: Bearer abcdefghijklmnop"}, Status{Available: true, Provider: "test-ocr"}
	}
	secret := service.RecognizeImagePath(imagePath)
	if !secret.OK || !secret.Sensitive {
		t.Fatalf("credential-shaped OCR text should be sensitive: %#v", secret)
	}
}

func TestRecognizeWorkMemoryBlocksSensitiveEntries(t *testing.T) {
	imagePath := testImage(t)
	memory := &fakeMemory{entry: workmemory.Entry{ID: "memory-secret", ImagePath: imagePath, Sensitive: true}}
	service := NewService(nil, nil, memory)
	service.runner = func(_ context.Context, _ string) (bridgeOutput, Status) {
		t.Fatal("sensitive entries must not run OCR")
		return bridgeOutput{}, Status{}
	}

	result := service.RecognizeWorkMemory("memory-secret")

	if result.OK || result.Error != "敏感条目默认不执行 OCR" {
		t.Fatalf("expected sensitive entry to be blocked: %#v", result)
	}
	if memory.entry.OCRStatus != "blocked_sensitive" {
		t.Fatalf("expected blocked status, got %#v", memory.entry)
	}
}

func TestOCRCommandEnvForcesUTF8(t *testing.T) {
	env := ocrCommandEnv([]string{
		"PATH=C:\\Windows",
		"PYTHONIOENCODING=gbk",
		"PYTHONUTF8=0",
	})
	joined := "\n" + strings.Join(env, "\n") + "\n"
	if !strings.Contains(joined, "\nPYTHONIOENCODING=utf-8\n") || !strings.Contains(joined, "\nPYTHONUTF8=1\n") {
		t.Fatalf("OCR Python env should force UTF-8, got %#v", env)
	}
	if strings.Contains(joined, "\nPYTHONIOENCODING=gbk\n") || strings.Contains(joined, "\nPYTHONUTF8=0\n") {
		t.Fatalf("OCR Python env should replace inherited Python encoding settings, got %#v", env)
	}
}

func TestDetectStatusDoesNotUseRepoVenvUnlessOptedIn(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "pyproject.toml"), "[project]\nname='x-tools'\n")
	writeFile(t, filepath.Join(root, "uv.lock"), "")
	repoPython := repoPythonPath(root)
	writeFile(t, repoPython, "")
	chdir(t, root)
	t.Setenv("PATH", "")
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "local"))
	t.Setenv("APPDATA", filepath.Join(root, "roaming"))
	t.Setenv("ARIADNE_OCR_HOME", "")
	t.Setenv("ARIADNE_OCR_PYTHON", "")
	t.Setenv("ARIADNE_OCR_ALLOW_REPO_PYTHON", "")

	status := NewService(nil, nil, nil).detectStatus()
	if status.Available || strings.EqualFold(status.PythonPath, repoPython) {
		t.Fatalf("repo .venv should not be used by default, got %#v", status)
	}

	t.Setenv("ARIADNE_OCR_ALLOW_REPO_PYTHON", "1")
	status = NewService(nil, nil, nil).detectStatus()
	if !status.Available || !strings.EqualFold(status.PythonPath, repoPython) {
		t.Fatalf("repo .venv should be used only when explicitly enabled, got %#v want %s", status, repoPython)
	}
}

func TestDetectStatusPrefersAriadneOCRRuntime(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "pyproject.toml"), "[project]\nname='x-tools'\n")
	writeFile(t, filepath.Join(root, "uv.lock"), "")
	writeFile(t, repoPythonPath(root), "")
	localPython := filepath.Join(root, "local", "Ariadne", "ocr-python", "python.exe")
	writeFile(t, localPython, "")
	chdir(t, root)
	t.Setenv("PATH", "")
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "local"))
	t.Setenv("APPDATA", filepath.Join(root, "roaming"))
	t.Setenv("ARIADNE_OCR_HOME", "")
	t.Setenv("ARIADNE_OCR_PYTHON", "")
	t.Setenv("ARIADNE_OCR_ALLOW_REPO_PYTHON", "1")

	status := NewService(nil, nil, nil).detectStatus()
	if !status.Available || !strings.EqualFold(status.PythonPath, localPython) {
		t.Fatalf("Ariadne OCR runtime should be preferred, got %#v want %s", status, localPython)
	}
}

func testImage(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "image.png")
	if err := osWriteFile(path, []byte("not actually decoded by fake OCR")); err != nil {
		t.Fatal(err)
	}
	return path
}

var osWriteFile = func(name string, data []byte) error {
	return os.WriteFile(name, data, 0o600)
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(old)
	})
}

func repoPythonPath(root string) string {
	if os.PathSeparator == '\\' {
		return filepath.Join(root, ".venv", "Scripts", "python.exe")
	}
	return filepath.Join(root, ".venv", "bin", "python")
}

type fakeCaptures struct {
	entry capturehistory.Entry
}

func (f fakeCaptures) Entry(id string) capturehistory.Entry {
	if id == f.entry.ID {
		return f.entry
	}
	return capturehistory.Entry{}
}

func (f fakeCaptures) CaptureScreen(source string) capturehistory.Status {
	entry := f.entry
	entry.Source = source
	return capturehistory.Status{Entries: []capturehistory.Entry{entry}}
}

type fakeClipboard struct {
	entry clipboardhistory.Entry
}

func (f fakeClipboard) Entry(id string) clipboardhistory.Entry {
	if id == f.entry.ID {
		return f.entry
	}
	return clipboardhistory.Entry{}
}

type fakeMemory struct {
	entry workmemory.Entry
}

type fakeAIClient struct {
	result AIResult
	err    error
}

func (f *fakeAIClient) RecognizeImageOCR(_ context.Context, _ AIOCRJob) (AIResult, error) {
	return f.result, f.err
}

func (f *fakeMemory) Entry(id string) workmemory.Entry {
	if id == f.entry.ID {
		return f.entry
	}
	return workmemory.Entry{}
}

func (f *fakeMemory) ApplyOCRText(id string, text string, provider string) workmemory.Entry {
	if id != f.entry.ID {
		return workmemory.Entry{}
	}
	if provider == "blocked_sensitive" || strings.HasPrefix(provider, "failed:") {
		f.entry.OCRStatus = provider
		return f.entry
	}
	f.entry.OCRText = text
	if text == "" {
		f.entry.OCRStatus = "empty"
		return f.entry
	}
	f.entry.OCRStatus = "done:" + provider
	f.entry.ContentType = "ocr_text"
	f.entry.Summary = text
	return f.entry
}
