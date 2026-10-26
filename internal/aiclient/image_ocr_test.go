package aiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"

	"path/filepath"

	"strings"
	"testing"

	"ariadne/internal/ocr"
)

func writeTestPNG(t *testing.T, width int, height int) string {
	t.Helper()
	imagePath := filepath.Join(t.TempDir(), "screen.png")
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x % 255), G: uint8(y % 255), B: 180, A: 255})
		}
	}
	file, err := os.Create(imagePath)
	if err != nil {
		t.Fatalf("create image: %v", err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode image: %v", err)
	}
	return imagePath
}

func TestOpenAICompatibleImageOCRPostsVisionChatCompletion(t *testing.T) {
	t.Setenv("ARIADNE_OCR_API_KEY", "ocr-key")
	imagePath := filepath.Join(t.TempDir(), "screen.png")
	if err := os.WriteFile(imagePath, []byte("fake png bytes"), 0o600); err != nil {
		t.Fatalf("write image: %v", err)
	}
	var captured visionChatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer ocr-key" {
			t.Fatalf("unexpected authorization header: %s", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"第一行\n第二行"}}]}`))
	}))
	defer server.Close()

	client := &OpenAICompatibleImageOCR{HTTPClient: server.Client(), APIKeyEnv: []string{"ARIADNE_OCR_API_KEY"}}
	result, err := client.RecognizeImageOCR(context.Background(), ocr.AIOCRJob{
		Provider:  "openai-compatible",
		BaseURL:   server.URL + "/v1",
		Model:     "vision-model",
		ImagePath: imagePath,
	})
	if err != nil {
		t.Fatalf("recognize image OCR: %v", err)
	}
	if captured.Model != "vision-model" || len(captured.Messages) != 2 {
		t.Fatalf("unexpected request payload: %#v", captured)
	}
	parts, ok := captured.Messages[1].Content.([]any)
	if !ok || len(parts) != 2 {
		t.Fatalf("expected multimodal content parts, got %#v", captured.Messages[1].Content)
	}
	imagePart, ok := parts[1].(map[string]any)
	if !ok || imagePart["type"] != "image_url" {
		t.Fatalf("expected image_url part, got %#v", parts[1])
	}
	imageURL, ok := imagePart["image_url"].(map[string]any)
	if !ok || !strings.HasPrefix(fmt.Sprint(imageURL["url"]), "data:image/png;base64,") {
		t.Fatalf("expected data URL image payload, got %#v", imagePart)
	}
	if result.Provider != "vision:vision-model" || result.Text != "第一行\n第二行" || len(result.Lines) != 2 {
		t.Fatalf("unexpected OCR result: %#v", result)
	}
}

func TestOCRImagePayloadDownscalesOversizedPNG(t *testing.T) {
	imagePath := writeTestPNG(t, 3840, 2099)
	payload, err := readOCRImagePayload(imagePath)
	if err != nil {
		t.Fatalf("read OCR image payload: %v", err)
	}
	if !payload.Resized {
		t.Fatalf("expected oversized OCR image to be resized")
	}
	if payload.MimeType != "image/png" {
		t.Fatalf("expected resized OCR payload to stay png, got %s", payload.MimeType)
	}
	config, err := png.DecodeConfig(bytes.NewReader(payload.Data))
	if err != nil {
		t.Fatalf("decode resized payload: %v", err)
	}
	if config.Width != ocrUploadMaxSide || config.Height > ocrUploadMaxSide {
		t.Fatalf("unexpected resized dimensions: %dx%d", config.Width, config.Height)
	}
	if config.Width != payload.Width || config.Height != payload.Height {
		t.Fatalf("payload dimensions not updated: payload=%dx%d config=%dx%d", payload.Width, payload.Height, config.Width, config.Height)
	}
}

func TestOpenAICompatibleImageOCRParsesStructuredOCRResponseWithoutAPIKey(t *testing.T) {
	imagePath := filepath.Join(t.TempDir(), "screen.png")
	if err := os.WriteFile(imagePath, []byte("fake png bytes"), 0o600); err != nil {
		t.Fatalf("write image: %v", err)
	}
	var captured visionChatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("local OCR-compatible endpoint should allow no authorization header, got %s", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		content := `{"text":"第一行\n第二行","images":[{"pages":[{"lines":[{"text":"第一行","score":0.98,"box":[10,20,110,40]},{"text":"第二行","confidence":0.87,"rect":{"x":12,"y":50,"width":128,"height":24}}]}]}]}`
		_ = json.NewEncoder(w).Encode(chatCompletionResponse{Choices: []struct {
			Message chatMessage `json:"message"`
		}{{Message: chatMessage{Role: "assistant", Content: content}}}})
	}))
	defer server.Close()

	client := &OpenAICompatibleImageOCR{HTTPClient: server.Client(), APIKeyEnv: []string{"ARIADNE_TEST_EMPTY_OCR_KEY"}}
	result, err := client.RecognizeImageOCR(context.Background(), ocr.AIOCRJob{
		Provider:  "openai-compatible",
		BaseURL:   server.URL + "/v1",
		Model:     "ppocrv6-medium-ocr",
		ImagePath: imagePath,
	})
	if err != nil {
		t.Fatalf("recognize image OCR: %v", err)
	}
	if captured.Model != "ppocrv6-medium-ocr" {
		t.Fatalf("unexpected request payload: %#v", captured)
	}
	if len(captured.Messages) != 2 {
		t.Fatalf("expected system plus user messages, got %#v", captured.Messages)
	}
	parts, ok := captured.Messages[1].Content.([]any)
	if !ok || len(parts) != 2 {
		t.Fatalf("expected multimodal user content, got %#v", captured.Messages[1].Content)
	}
	textPart, ok := parts[0].(map[string]any)
	if !ok || textPart["text"] != "ocr this image" {
		t.Fatalf("expected OCR prompt for GPU endpoint, got %#v", parts[0])
	}
	if result.Provider != "vision:ppocrv6-medium-ocr" || result.Text != "第一行\n第二行" || len(result.Lines) != 2 {
		t.Fatalf("unexpected OCR result: %#v", result)
	}
	if result.Lines[0].Rect.Width != 100 || result.Lines[1].Rect.Height != 24 {
		t.Fatalf("structured OCR lines lost geometry: %#v", result.Lines)
	}
}

func TestOllamaGenerateImageOCRPostsBase64Images(t *testing.T) {
	imagePath := filepath.Join(t.TempDir(), "screen.png")
	if err := os.WriteFile(imagePath, []byte("fake png bytes"), 0o600); err != nil {
		t.Fatalf("write image: %v", err)
	}
	var captured ollamaGenerateRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("ollama generate should not require authorization header, got %s", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"model":"glm-ocr:latest","response":"第一行\n第二行","done":true}`))
	}))
	defer server.Close()

	client := &OpenAICompatibleImageOCR{HTTPClient: server.Client()}
	result, err := client.RecognizeImageOCR(context.Background(), ocr.AIOCRJob{
		Provider:  "ollama-generate",
		BaseURL:   server.URL,
		Model:     "glm-ocr:latest",
		ImagePath: imagePath,
	})
	if err != nil {
		t.Fatalf("recognize image OCR: %v", err)
	}
	if captured.Model != "glm-ocr:latest" || captured.Stream || len(captured.Images) != 1 {
		t.Fatalf("unexpected ollama request payload: %#v", captured)
	}
	if strings.HasPrefix(captured.Images[0], "data:") || !strings.Contains(captured.Prompt, "请识别图片中所有可见文字") {
		t.Fatalf("unexpected ollama image/prompt payload: %#v", captured)
	}
	if result.Provider != "ollama-generate:glm-ocr:latest" || result.Text != "第一行\n第二行" || len(result.Lines) != 2 {
		t.Fatalf("unexpected OCR result: %#v", result)
	}
}

func TestOllamaGenerateEndpointAcceptsRootAPIAndFullPath(t *testing.T) {
	cases := map[string]string{
		"":                                       "http://localhost:11434/api/generate",
		"http://192.168.1.11:11434":              "http://192.168.1.11:11434/api/generate",
		"http://192.168.1.11:11434/api":          "http://192.168.1.11:11434/api/generate",
		"http://192.168.1.11:11434/api/generate": "http://192.168.1.11:11434/api/generate",
	}
	for input, want := range cases {
		if got := ollamaGenerateEndpoint(input); got != want {
			t.Fatalf("ollama endpoint for %q = %q, want %q", input, got, want)
		}
	}
}

func TestCleanAPIKeyAcceptsEnvAssignmentBlocks(t *testing.T) {
	cases := map[string]string{
		`"test-key"`:       "test-key",
		"test\x00-key\r\n": "test-key",
		"OPENAI__API_KEY=\"test-key\"\r\nOPENAI__BASE_URL=\"http://host\"": "test-key",
		"EMBED__API_KEY='embed-key' # comment":                             "embed-key",
	}
	for input, expected := range cases {
		if got := cleanAPIKey(input); got != expected {
			t.Fatalf("cleanAPIKey(%q)=%q want %q", input, got, expected)
		}
	}
}

func TestAPIKeySourcePrefersAriadneCredentialOverGenericEnv(t *testing.T) {
	t.Setenv("ARIADNE_AI_API_KEY", "")
	t.Setenv("OPENAI__API_KEY", "generic-env-key")
	t.Setenv("OPENAI_API_KEY", "")
	read := func(target string) (string, bool, error) {
		if target == "Ariadne/OpenAI/APIKey" {
			return "stored-ai-key", true, nil
		}
		return "", false, nil
	}

	got := apiKeyFromSourcesWithReader(
		[]string{"ARIADNE_AI_API_KEY", "OPENAI__API_KEY", "OPENAI_API_KEY"},
		[]string{"Ariadne/OpenAI/APIKey"},
		read,
	)
	if got != "stored-ai-key" {
		t.Fatalf("generic OpenAI env should not override stored Ariadne credential, got %q", got)
	}
}

func TestAPIKeySourcePrefersAriadneEnvOverCredential(t *testing.T) {
	t.Setenv("ARIADNE_AI_API_KEY", "ariadne-env-key")
	t.Setenv("OPENAI__API_KEY", "generic-env-key")
	read := func(target string) (string, bool, error) {
		if target == "Ariadne/OpenAI/APIKey" {
			return "stored-ai-key", true, nil
		}
		return "", false, nil
	}

	got := apiKeyFromSourcesWithReader(
		[]string{"ARIADNE_AI_API_KEY", "OPENAI__API_KEY", "OPENAI_API_KEY"},
		[]string{"Ariadne/OpenAI/APIKey"},
		read,
	)
	if got != "ariadne-env-key" {
		t.Fatalf("app-specific Ariadne env should override stored credential, got %q", got)
	}
}

func TestAPIKeySourceFallsBackToGenericEnv(t *testing.T) {
	t.Setenv("ARIADNE_AI_API_KEY", "")
	t.Setenv("OPENAI__API_KEY", "generic-env-key")
	read := func(target string) (string, bool, error) {
		return "", false, nil
	}

	got := apiKeyFromSourcesWithReader(
		[]string{"ARIADNE_AI_API_KEY", "OPENAI__API_KEY", "OPENAI_API_KEY"},
		[]string{"Ariadne/OpenAI/APIKey"},
		read,
	)
	if got != "generic-env-key" {
		t.Fatalf("generic env should be used when no Ariadne env or credential exists, got %q", got)
	}
}
