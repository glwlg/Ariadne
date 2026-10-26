package ocr

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ariadne/internal/capturehistory"
	"ariadne/internal/clipboardhistory"
)

func TestRecognizeCaptureUsesExplicitCaptureImage(t *testing.T) {
	imagePath := testImage(t)
	service := NewService(
		fakeCaptures{entry: capturehistory.Entry{ID: "cap-1", ImagePath: imagePath, Width: 120, Height: 80}},
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
	service := NewService(nil, fakeClipboard{entry: clipboardhistory.Entry{ID: "clip-1", Type: clipboardhistory.EntryText, Text: "not image"}})

	result := service.RecognizeClipboardImage("clip-1")

	if result.OK || result.Error == "" {
		t.Fatalf("expected text clipboard entry to be rejected: %#v", result)
	}
}

func TestRecognizeImagePrefersConfiguredAIOCR(t *testing.T) {
	imagePath := testImage(t)
	service := NewService(nil, nil)
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

func TestRecognizeImageRegionsUsesLocalOCREvenWhenAIIsConfigured(t *testing.T) {
	for _, localOK := range []bool{true, false} {
		t.Run(fmt.Sprint(localOK), func(t *testing.T) {
			imagePath := testImage(t)
			service := NewService(nil, nil)
			rect := Rect{X: 10, Y: 20, Width: 100, Height: 30}
			service.runner = func(_ context.Context, path string) (bridgeOutput, Status) {
				if path != imagePath {
					t.Fatalf("unexpected image path %q", path)
				}
				return bridgeOutput{OK: localOK, Text: "local text", Lines: []Line{{Text: "local text", Rect: rect}}, Error: "local error"}, Status{Provider: localOCRProvider}
			}
			client := &fakeAIClient{result: AIResult{Text: "text without coordinates"}}
			RegisterAIClient(service, client)
			service.ApplyAIOCRPolicy(AIOCRPolicy{Enabled: true, Provider: "openai-compatible", Model: "vision-model"})

			result := service.RecognizeImageRegions(imagePath)

			if len(client.jobs) != 0 || result.OK != localOK || result.Provider != localOCRProvider {
				t.Fatalf("region recognition must use local OCR: %#v, AI calls=%d", result, len(client.jobs))
			}
			if localOK && (len(result.Lines) != 1 || result.Lines[0].Rect != rect) {
				t.Fatalf("OCR must preserve pixel coordinates: %#v", result)
			}
			if !localOK && result.Error != "local error" {
				t.Fatalf("local failure must remain visible: %#v", result)
			}
		})
	}
}

func TestRecognitionEntryPointsShareOCRPolicy(t *testing.T) {
	imagePath := testImage(t)
	service := NewService(
		fakeCaptures{entry: capturehistory.Entry{ID: "cap-1", ImagePath: imagePath, Width: 120, Height: 80}},
		fakeClipboard{entry: clipboardhistory.Entry{ID: "clip-1", Type: clipboardhistory.EntryImage, ImagePath: imagePath, Width: 120, Height: 80}},
	)
	client := &fakeAIClient{result: AIResult{Text: "共用模型文字"}}
	RegisterAIClient(service, client)
	service.runner = func(_ context.Context, _ string) (bridgeOutput, Status) {
		return bridgeOutput{OK: true, Text: "本地文字"}, Status{Provider: localOCRProvider}
	}
	for _, enabled := range []bool{true, false} {
		service.ApplyAIOCRPolicy(AIOCRPolicy{Enabled: enabled, Provider: "openai-compatible", BaseURL: "http://shared.test/v1", Model: "shared-model"})
		for _, entry := range []struct {
			source string
			call   func() Result
		}{
			{"capture_history", func() Result { return service.RecognizeCapture("cap-1") }},
			{"current_screen", service.RecognizeCurrentScreen},
			{"clipboard_history", func() Result { return service.RecognizeClipboardImage("clip-1") }},
			{"pinned_capture", func() Result { return service.RecognizePinnedCapture("cap-1") }},
			{"pinned_clipboard", func() Result { return service.RecognizePinnedClipboardImage("clip-1") }},
			{"image_path", func() Result { return service.RecognizeImagePath(imagePath) }},
		} {
			t.Run(fmt.Sprintf("%s/model=%t", entry.source, enabled), func(t *testing.T) {
				client.jobs = nil
				result := entry.call()
				if !result.OK || result.Source != entry.source {
					t.Fatalf("recognition failed: %#v", result)
				}
				if enabled {
					if result.Text != "共用模型文字" || len(client.jobs) != 1 || client.jobs[0] != (AIOCRJob{Provider: "openai-compatible", BaseURL: "http://shared.test/v1", Model: "shared-model", ImagePath: imagePath}) {
						t.Fatalf("entry point must use shared model: result=%#v jobs=%#v", result, client.jobs)
					}
				} else if result.Text != "本地文字" || len(client.jobs) != 0 {
					t.Fatalf("shared model switch must disable AI for all entry points: result=%#v jobs=%#v", result, client.jobs)
				}
				if strings.Contains(entry.source, "capture") && (result.CaptureID != "cap-1" || result.Width != 120 || result.Height != 80) {
					t.Fatalf("capture metadata changed: %#v", result)
				}
				if strings.Contains(entry.source, "clipboard") && (result.ClipboardID != "clip-1" || result.Width != 120 || result.Height != 80) {
					t.Fatalf("clipboard metadata changed: %#v", result)
				}
			})
		}
	}
}

func TestRecognizeImageFallsBackToLocalOCRWhenAIFails(t *testing.T) {
	imagePath := testImage(t)
	service := NewService(nil, nil)
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
	service := NewService(nil, nil)
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
	service := NewService(nil, nil)
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

	status := NewService(nil, nil).detectStatus()
	if status.Available || strings.EqualFold(status.PythonPath, repoPython) {
		t.Fatalf("repo .venv should not be used by default, got %#v", status)
	}

	t.Setenv("ARIADNE_OCR_ALLOW_REPO_PYTHON", "1")
	status = NewService(nil, nil).detectStatus()
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

	status := NewService(nil, nil).detectStatus()
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

type fakeAIClient struct {
	result AIResult
	err    error
	jobs   []AIOCRJob
}

func (f *fakeAIClient) RecognizeImageOCR(_ context.Context, job AIOCRJob) (AIResult, error) {
	f.jobs = append(f.jobs, job)
	return f.result, f.err
}
