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
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"ariadne/internal/ocr"
	"ariadne/internal/workmemory"
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

func TestOpenAICompatiblePolisherPostsChatCompletion(t *testing.T) {
	t.Setenv("ARIADNE_AI_API_KEY", "test-key")
	var captured chatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected authorization header: %s", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"## 润色日报\n- 保留 evidence-a"}}]}`))
	}))
	defer server.Close()

	polisher := &OpenAICompatiblePolisher{HTTPClient: server.Client(), APIKeyEnv: []string{"ARIADNE_AI_API_KEY"}}
	result, err := polisher.PolishDraft(context.Background(), workmemory.DraftPolishJob{
		Provider: "openai-compatible",
		BaseURL:  server.URL + "/v1",
		Model:    "test-model",
		Kind:     "daily",
		Draft: workmemory.Draft{
			Title:    "今日工作日报草稿",
			Body:     "原始日报内容",
			Evidence: []string{"evidence-a"},
		},
	})
	if err != nil {
		t.Fatalf("polish draft: %v", err)
	}
	if captured.Model != "test-model" || len(captured.Messages) != 2 {
		t.Fatalf("unexpected request payload: %#v", captured)
	}
	if !strings.Contains(captured.Messages[1].Content, "evidence-a") || !strings.Contains(captured.Messages[1].Content, "原始日报内容") {
		t.Fatalf("prompt lost draft context: %s", captured.Messages[1].Content)
	}
	if !strings.Contains(result.Body, "润色日报") || result.Evidence[0] != "evidence-a" {
		t.Fatalf("unexpected polished draft: %#v", result)
	}
}

func TestOpenAICompatiblePolisherRequiresKeyAndSupportedProvider(t *testing.T) {
	polisher := &OpenAICompatiblePolisher{APIKeyEnv: []string{"ARIADNE_AI_API_KEY"}}
	_, err := polisher.PolishDraft(context.Background(), workmemory.DraftPolishJob{
		Provider: "openai-compatible",
		Model:    "test-model",
		Draft:    workmemory.Draft{Title: "t", Body: "b"},
	})
	if err == nil || !strings.Contains(err.Error(), "API_KEY") {
		t.Fatalf("expected missing key error, got %v", err)
	}

	t.Setenv("ARIADNE_AI_API_KEY", "test-key")
	_, err = polisher.PolishDraft(context.Background(), workmemory.DraftPolishJob{
		Provider: "disabled",
		Model:    "test-model",
		Draft:    workmemory.Draft{Title: "t", Body: "b"},
	})
	if err == nil || !strings.Contains(err.Error(), "不支持") {
		t.Fatalf("expected unsupported provider error, got %v", err)
	}
}

func TestOpenAICompatibleOCRSummarizerPostsChatCompletion(t *testing.T) {
	t.Setenv("ARIADNE_AI_API_KEY", "test-key")
	var captured chatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected authorization header: %s", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		response := `{"choices":[{"message":{"role":"assistant","content":"{\"title\":\"时间线标题优化\",\"summary\":\"正在整理截图后的 OCR 内容。\",\"text\":\"## 可见内容\\n- 时间线标题需要更有意义\"}"}}]}`
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	summarizer := &OpenAICompatibleOCRSummarizer{HTTPClient: server.Client(), APIKeyEnv: []string{"ARIADNE_AI_API_KEY"}}
	result, err := summarizer.SummarizeOCR(context.Background(), workmemory.OCRSummaryJob{
		Provider: "openai-compatible",
		BaseURL:  server.URL + "/v1",
		Model:    "test-model",
		Now:      time.Unix(1781404200, 0),
		Entry: workmemory.Entry{
			ID:          "memory-a",
			Title:       "work",
			AppName:     "msedge.exe",
			WindowTitle: "work",
			Source:      "work_memory_time_machine",
		},
		OCRText: "时间线里的标题还是没什么意义\n截图之后应该自动 OCR",
	})
	if err != nil {
		t.Fatalf("summarize OCR: %v", err)
	}
	if captured.Model != "test-model" || len(captured.Messages) != 2 {
		t.Fatalf("unexpected request payload: %#v", captured)
	}
	if !strings.Contains(captured.Messages[1].Content, "截图之后应该自动 OCR") || !strings.Contains(captured.Messages[1].Content, "JSON schema") {
		t.Fatalf("prompt lost OCR context: %s", captured.Messages[1].Content)
	}
	if result.Title != "时间线标题优化" || !strings.Contains(result.Text, "时间线标题") {
		t.Fatalf("unexpected OCR summary: %#v", result)
	}
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

func TestOpenAICompatibleExperienceDiscovererPostsChatCompletion(t *testing.T) {
	t.Setenv("ARIADNE_AI_API_KEY", "test-key")
	var captured chatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected authorization header: %s", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		response := `{"choices":[{"message":{"role":"assistant","content":"{\"title\":\"AI 报告\",\"summary\":\"发现网络模式\",\"insights\":[{\"kind\":\"repeated_issue\",\"title\":\"代理排障\",\"summary\":\"超时重复出现\",\"reason\":\"两条证据都指向 proxy timeout\",\"recommendation\":\"沉淀为清单并人工审核\",\"evidence\":[\"memory-a\",\"memory-b\"],\"confidence\":0.82,\"severity\":\"high\"}]}"}}]}`
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	discoverer := &OpenAICompatibleExperienceDiscoverer{HTTPClient: server.Client(), APIKeyEnv: []string{"ARIADNE_AI_API_KEY"}}
	report, err := discoverer.DiscoverExperiences(context.Background(), workmemory.ExperienceDiscoveryJob{
		Provider:   "openai-compatible",
		BaseURL:    server.URL + "/v1",
		Model:      "test-model",
		PeriodDays: 7,
		Now:        time.Unix(1781404200, 0),
		Evidence: []workmemory.ExperienceDiscoveryEvidence{
			{ID: "memory-a", Title: "OpenWrt timeout", Summary: "proxy timeout", Text: "gateway proxy timeout", Tags: []string{"network"}},
			{ID: "memory-b", Title: "Cloudflare timeout", Summary: "gateway timeout", Text: "network timeout again", Tags: []string{"network"}},
		},
	})
	if err != nil {
		t.Fatalf("discover experiences: %v", err)
	}
	if captured.Model != "test-model" || len(captured.Messages) != 2 {
		t.Fatalf("unexpected request payload: %#v", captured)
	}
	if !strings.Contains(captured.Messages[1].Content, "memory-a") || !strings.Contains(captured.Messages[1].Content, "Evidence JSON") {
		t.Fatalf("prompt lost evidence context: %s", captured.Messages[1].Content)
	}
	if report.Title != "AI 报告" || len(report.Insights) != 1 || report.Insights[0].Evidence[0] != "memory-a" {
		t.Fatalf("unexpected report: %#v", report)
	}
}

func TestOpenAICompatibleExperienceDiscovererRequiresKey(t *testing.T) {
	discoverer := &OpenAICompatibleExperienceDiscoverer{APIKeyEnv: []string{"ARIADNE_AI_API_KEY"}}
	_, err := discoverer.DiscoverExperiences(context.Background(), workmemory.ExperienceDiscoveryJob{
		Provider: "openai-compatible",
		Model:    "test-model",
		Evidence: []workmemory.ExperienceDiscoveryEvidence{{ID: "memory-a", Title: "t"}},
	})
	if err == nil || !strings.Contains(err.Error(), "API_KEY") {
		t.Fatalf("expected missing key error, got %v", err)
	}
}

func TestOpenAICompatibleFlowAgentPostsChatCompletion(t *testing.T) {
	t.Setenv("ARIADNE_AI_API_KEY", "test-key")
	var captured chatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected authorization header: %s", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"你今天和叶志伟主要聊了心流问答要走动态检索。\n\n依据：memory-chat-ye"}}]}`))
	}))
	defer server.Close()

	agent := &OpenAICompatibleFlowAgent{HTTPClient: server.Client(), APIKeyEnv: []string{"ARIADNE_AI_API_KEY"}}
	result, err := agent.AnswerFlow(context.Background(), workmemory.FlowAgentJob{
		Question:    "今天跟叶志伟聊了什么",
		Intent:      "contacts",
		LocalAnswer: "本地摘要不应作为最终回答。",
		SelfModel:   "Identity - 姓名: luwei; 账号显示名: lw",
		Conversation: []workmemory.FlowConversationContextMessage{
			{Role: "user", Text: "端午值班保存待办", CreatedAt: 1781769600},
			{Role: "assistant", Text: "待办工具未成功执行，未保存待办。", CreatedAt: 1781769660},
		},
		Runner:   "openai-agent",
		Provider: "openai-compatible",
		BaseURL:  server.URL + "/v1",
		Model:    "test-model",
		Now:      time.Unix(1781435400, 0),
		Evidence: []workmemory.FlowAgentEvidence{
			{ID: "memory-chat-ye", Title: "微信 - 叶志伟", Summary: "讨论心流问答", Text: "叶志伟：心流要真的检索内容再回复。", AppName: "Weixin.exe"},
		},
	})
	if err != nil {
		t.Fatalf("answer flow: %v", err)
	}
	if captured.Model != "test-model" || len(captured.Messages) != 2 {
		t.Fatalf("unexpected request payload: %#v", captured)
	}
	if !strings.Contains(captured.Messages[0].Content, "通用心流 Agent") ||
		!strings.Contains(captured.Messages[0].Content, "聊天原文里的“你/我”") ||
		!strings.Contains(captured.Messages[1].Content, "memory-chat-ye") ||
		!strings.Contains(captured.Messages[1].Content, "Identity - 姓名: luwei") ||
		!strings.Contains(captured.Messages[1].Content, "Conversation Context") ||
		!strings.Contains(captured.Messages[1].Content, "端午值班保存待办") ||
		!strings.Contains(captured.Messages[1].Content, "群聊中有人提到") {
		t.Fatalf("prompt lost agent instructions or evidence: %#v", captured.Messages)
	}
	if result.Mode != "agent:openai-compatible-direct" || !strings.Contains(result.Answer, "叶志伟") {
		t.Fatalf("unexpected flow agent result: %#v", result)
	}
}

func TestFlowAgentRouterDefaultsToOpenAIAgentsSDK(t *testing.T) {
	router := NewFlowAgentRouter()
	if _, ok := router.OpenAI.(*OpenAIAgentsSDKFlowAgent); !ok {
		t.Fatalf("default openai flow runner should use OpenAI Agents SDK, got %T", router.OpenAI)
	}
}

func TestOpenAIAgentsSDKFlowAgentRunsBridgeProcess(t *testing.T) {
	t.Setenv("ARIADNE_AI_API_KEY", "test-key")
	pythonPath := writeFakeFlowAgentPython(t)
	agent := &OpenAIAgentsSDKFlowAgent{
		PythonPath: pythonPath,
		APIKeyEnv:  []string{"ARIADNE_AI_API_KEY"},
		Timeout:    5 * time.Second,
	}
	result, err := agent.AnswerFlow(context.Background(), workmemory.FlowAgentJob{
		Question:    "今天跟叶志伟聊了什么",
		Intent:      "contacts",
		LocalAnswer: "本地摘要不应作为最终回答。",
		Runner:      "openai-agent",
		Provider:    "openai-compatible",
		BaseURL:     "http://127.0.0.1:4000/v1",
		Model:       "glm-5.1",
		Now:         time.Unix(1781435400, 0),
		Evidence: []workmemory.FlowAgentEvidence{
			{ID: "memory-chat-ye", Title: "微信 - 叶志伟", Summary: "讨论心流问答", Text: "叶志伟：心流要真的检索内容再回复。", AppName: "Weixin.exe"},
		},
	})
	if err != nil {
		t.Fatalf("answer flow through sdk bridge: %v", err)
	}
	if result.Mode != "agent:openai-agents-sdk-chat-tools" || !strings.Contains(result.Answer, "SDK") {
		t.Fatalf("unexpected SDK result: %#v", result)
	}
}

func TestFlowAgentsSDKBridgeNativeSkillRouting(t *testing.T) {
	pythonPath, err := exec.LookPath("python")
	pythonArgs := []string{}
	if err != nil && runtime.GOOS == "windows" {
		pythonPath, err = exec.LookPath("py")
		pythonArgs = []string{"-3"}
	}
	if err != nil {
		t.Skipf("python executable not found: %v", err)
	}

	tempDir := t.TempDir()
	bridgePath := filepath.Join(tempDir, "flow_agents_sdk_bridge.py")
	if err := os.WriteFile(bridgePath, flowAgentsSDKBridge, 0o600); err != nil {
		t.Fatalf("write bridge: %v", err)
	}
	script := `
import importlib.util
import os
import sys

for key in (
    "ARIADNE_FLOW_AGENT_FORCE_FUNCTION_TOOLS",
    "ARIADNE_FLOW_AGENT_NATIVE_SKILLS",
    "ARIADNE_FLOW_AGENT_NATIVE_SKILLS_STRICT",
    "ARIADNE_FLOW_AGENT_ALLOW_COMPAT_NATIVE_SKILLS",
):
    os.environ.pop(key, None)

spec = importlib.util.spec_from_file_location("bridge", sys.argv[1])
bridge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bridge)

cases = [
    ("openai-compatible", "http://ai.local/v1", {"nativeSkills": True}, False),
    ("openai-compatible", "http://ai.local/v1", {}, False),
    ("openai", "http://ai.local/v1", {"nativeSkills": True}, True),
    ("openai", "https://api.openai.com/v1", {}, True),
]
for provider, base_url, payload, want in cases:
    got = bridge._should_try_native_shell_skill(provider, base_url, payload)
    if got is not want:
        raise AssertionError(f"{provider} {base_url} {payload}: got {got}, want {want}")

os.environ["ARIADNE_FLOW_AGENT_ALLOW_COMPAT_NATIVE_SKILLS"] = "1"
if bridge._should_try_native_shell_skill("openai-compatible", "http://ai.local/v1", {"nativeSkills": True}) is not True:
    raise AssertionError("explicit compat native opt-in should enable native shell skill")

os.environ.pop("ARIADNE_FLOW_AGENT_ALLOW_COMPAT_NATIVE_SKILLS", None)
os.environ["ARIADNE_FLOW_AGENT_NATIVE_SKILLS_STRICT"] = "1"
if bridge._should_try_native_shell_skill("openai-compatible", "http://ai.local/v1", {}) is not True:
    raise AssertionError("strict native env should enable native shell skill")

os.environ["ARIADNE_FLOW_AGENT_FORCE_FUNCTION_TOOLS"] = "1"
if bridge._should_try_native_shell_skill("openai", "https://api.openai.com/v1", {"nativeSkills": True}) is not False:
    raise AssertionError("force function tools should disable native shell skill")

plain_args = bridge._normalize_tool_arguments("<tool_call>get_workmemory_status</tool_call>")
if plain_args != "{}":
    raise AssertionError(f"plain XML tool arguments should become empty JSON object, got {plain_args}")

search_args = bridge._normalize_tool_arguments("<tool_call>search_flow_memory<arg_key>query</arg_key><arg_value>ariadne</arg_value><arg_key>limit</arg_key><arg_value>3</arg_value><arg_key>exact</arg_key><arg_value>true</arg_value></tool_call>")
if search_args != '{"query":"ariadne","limit":3,"exact":true}':
    raise AssertionError(f"XML key/value tool arguments should become JSON, got {search_args}")
`
	scriptPath := filepath.Join(tempDir, "bridge_routing_smoke.py")
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatalf("write bridge script: %v", err)
	}
	args := append(pythonArgs, scriptPath, bridgePath)
	cmd := exec.Command(pythonPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bridge routing smoke failed: %v\n%s", err, output)
	}
}

func TestFlowAgentsSDKBridgeRejectsTodoSaveWithoutTool(t *testing.T) {
	pythonPath, err := exec.LookPath("python")
	pythonArgs := []string{}
	if err != nil && runtime.GOOS == "windows" {
		pythonPath, err = exec.LookPath("py")
		pythonArgs = []string{"-3"}
	}
	if err != nil {
		t.Skipf("python executable not found: %v", err)
	}

	tempDir := t.TempDir()
	bridgePath := filepath.Join(tempDir, "flow_agents_sdk_bridge.py")
	if err := os.WriteFile(bridgePath, flowAgentsSDKBridge, 0o600); err != nil {
		t.Fatalf("write bridge: %v", err)
	}
	script := `
import asyncio
import importlib.util
import sys

spec = importlib.util.spec_from_file_location("bridge", sys.argv[1])
bridge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bridge)

class FakeMessage:
    content = "已保存端午值班待办。"
    tool_calls = []

class FakeChoice:
    message = FakeMessage()

class FakeResponse:
    choices = [FakeChoice()]

seen_tool_choices = []

class FakeCompletions:
    async def create(self, **kwargs):
        seen_tool_choices.append(kwargs.get("tool_choice"))
        return FakeResponse()

class FakeChat:
    completions = FakeCompletions()

class FakeClient:
    chat = FakeChat()

result = asyncio.run(bridge._run_with_compatible_chat_tools(
    client=FakeClient(),
    model_name="glm-5.1",
    system_prompt="",
    user_prompt="用户问题：端午值班保存待办\n回答要求：先调用工具。",
    skill="",
    cli_command="ariadne",
))
if result.get("ok"):
    raise AssertionError(f"todo save claim without tool must be rejected, got {result}")
if "待办工具" not in result.get("error", ""):
    raise AssertionError(f"unexpected rejection reason: {result}")
if not seen_tool_choices or seen_tool_choices[0] != {"type": "function", "function": {"name": "add_flow_todo"}}:
    raise AssertionError(f"todo save should force add_flow_todo first, got {seen_tool_choices}")

seen_tool_choices.clear()
retry_result = asyncio.run(bridge._run_with_compatible_chat_tools(
    client=FakeClient(),
    model_name="glm-5.1",
    system_prompt="",
    user_prompt='用户问题：你刚才没加成功，再加一次\nConversation Context: [{"role":"user","text":"端午值班保存待办"},{"role":"assistant","text":"待办工具未成功执行，未保存待办。"}]',
    skill="",
    cli_command="ariadne",
))
if retry_result.get("ok"):
    raise AssertionError(f"retry todo save claim without tool must be rejected, got {retry_result}")
if "待办工具" not in retry_result.get("error", ""):
    raise AssertionError(f"unexpected retry rejection reason: {retry_result}")
if not seen_tool_choices or seen_tool_choices[0] != {"type": "function", "function": {"name": "add_flow_todo"}}:
    raise AssertionError(f"retry reference should force add_flow_todo first, got {seen_tool_choices}")
`
	scriptPath := filepath.Join(tempDir, "bridge_todo_guard.py")
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatalf("write bridge script: %v", err)
	}
	args := append(pythonArgs, scriptPath, bridgePath)
	cmd := exec.Command(pythonPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bridge todo save guard failed: %v\n%s", err, output)
	}
}

func TestFlowAgentsSDKBridgeRetriesFinalAnswerAfterEmptyToolTurn(t *testing.T) {
	pythonPath, err := exec.LookPath("python")
	pythonArgs := []string{}
	if err != nil && runtime.GOOS == "windows" {
		pythonPath, err = exec.LookPath("py")
		pythonArgs = []string{"-3"}
	}
	if err != nil {
		t.Skipf("python executable not found: %v", err)
	}

	tempDir := t.TempDir()
	bridgePath := filepath.Join(tempDir, "flow_agents_sdk_bridge.py")
	if err := os.WriteFile(bridgePath, flowAgentsSDKBridge, 0o600); err != nil {
		t.Fatalf("write bridge: %v", err)
	}
	script := `
import asyncio
import importlib.util
import sys

spec = importlib.util.spec_from_file_location("bridge", sys.argv[1])
bridge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bridge)

class FakeFunction:
    def __init__(self):
        self.name = "unexpected_probe_tool"
        self.arguments = "{}"

class FakeToolCall:
    id = "call_probe"
    function = FakeFunction()

class FakeMessage:
    def __init__(self, content="", tool_calls=None):
        self.content = content
        self.tool_calls = tool_calls or []

class FakeChoice:
    def __init__(self, message):
        self.message = message

class FakeResponse:
    def __init__(self, message):
        self.choices = [FakeChoice(message)]

calls = []

class FakeCompletions:
    async def create(self, **kwargs):
        calls.append(kwargs)
        if len(calls) == 1:
            return FakeResponse(FakeMessage("", [FakeToolCall()]))
        if len(calls) == 2:
            return FakeResponse(FakeMessage(""))
        return FakeResponse(FakeMessage("最终回答：今天杨阳找过你。"))

class FakeChat:
    completions = FakeCompletions()

class FakeClient:
    chat = FakeChat()

result = asyncio.run(bridge._run_with_compatible_chat_tools(
    client=FakeClient(),
    model_name="glm-5.1",
    system_prompt="",
    user_prompt="用户问题：今天有哪些人找过我？\n回答要求：必须先查工具。",
    skill="",
    cli_command="ariadne",
))
if not result.get("ok"):
    raise AssertionError(f"empty post-tool turn should retry final answer, got {result}")
if "杨阳" not in result.get("answer", ""):
    raise AssertionError(f"unexpected answer: {result}")
if len(calls) != 3:
    raise AssertionError(f"expected two tool-loop calls plus one final retry, got {len(calls)}")
if "tools" in calls[2]:
    raise AssertionError(f"final retry should omit tools, got {calls[2]}")
if "不要再调用工具" not in calls[2]["messages"][-1]["content"]:
    raise AssertionError(f"final retry instruction missing: {calls[2]['messages'][-1]}")
`
	scriptPath := filepath.Join(tempDir, "bridge_empty_final_retry.py")
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatalf("write bridge script: %v", err)
	}
	args := append(pythonArgs, scriptPath, bridgePath)
	cmd := exec.Command(pythonPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bridge empty final retry failed: %v\n%s", err, output)
	}
}

func TestFlowAgentsSDKBridgeRetriesFinalAnswerWithPlainToolResults(t *testing.T) {
	pythonPath, err := exec.LookPath("python")
	pythonArgs := []string{}
	if err != nil && runtime.GOOS == "windows" {
		pythonPath, err = exec.LookPath("py")
		pythonArgs = []string{"-3"}
	}
	if err != nil {
		t.Skipf("python executable not found: %v", err)
	}

	tempDir := t.TempDir()
	bridgePath := filepath.Join(tempDir, "flow_agents_sdk_bridge.py")
	if err := os.WriteFile(bridgePath, flowAgentsSDKBridge, 0o600); err != nil {
		t.Fatalf("write bridge: %v", err)
	}
	script := `
import asyncio
import importlib.util
import sys

spec = importlib.util.spec_from_file_location("bridge", sys.argv[1])
bridge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bridge)

class FakeFunction:
    def __init__(self):
        self.name = "get_flow_memory_entry"
        self.arguments = '{"entry_id":"memory-a"}'

class FakeToolCall:
    id = "call_get"
    function = FakeFunction()

class FakeMessage:
    def __init__(self, content="", tool_calls=None):
        self.content = content
        self.tool_calls = tool_calls or []

class FakeChoice:
    def __init__(self, message):
        self.message = message

class FakeResponse:
    def __init__(self, message):
        self.choices = [FakeChoice(message)]

calls = []

class FakeCompletions:
    async def create(self, **kwargs):
        calls.append(kwargs)
        if len(calls) == 1:
            return FakeResponse(FakeMessage("", [FakeToolCall()]))
        if len(calls) in (2, 3):
            return FakeResponse(FakeMessage(""))
        return FakeResponse(FakeMessage("最终回答：杨阳找过你，依据 memory-a。"))

class FakeChat:
    completions = FakeCompletions()

class FakeClient:
    chat = FakeChat()

result = asyncio.run(bridge._run_with_compatible_chat_tools(
    client=FakeClient(),
    model_name="glm-5.1",
    system_prompt="",
    user_prompt="用户问题：今天有哪些人找过我？\n回答要求：必须先查工具。",
    skill="",
    cli_command="ariadne",
))
if not result.get("ok"):
    raise AssertionError(f"plain tool-result retry should recover, got {result}")
if "memory-a" not in result.get("answer", ""):
    raise AssertionError(f"unexpected answer: {result}")
if len(calls) != 4:
    raise AssertionError(f"expected tool loop, original retry, and plain retry, got {len(calls)}")
if "tools" in calls[3]:
    raise AssertionError(f"plain retry should omit tools, got {calls[3]}")
plain_messages = calls[3]["messages"]
if len(plain_messages) != 2 or plain_messages[0]["role"] != "system" or plain_messages[1]["role"] != "user":
    raise AssertionError(f"plain retry should use simple system/user messages, got {plain_messages}")
if "已执行的 Ariadne 工具结果" not in plain_messages[1]["content"]:
    raise AssertionError(f"plain retry prompt missing tool results: {plain_messages[1]}")
`
	scriptPath := filepath.Join(tempDir, "bridge_plain_retry.py")
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatalf("write bridge script: %v", err)
	}
	args := append(pythonArgs, scriptPath, bridgePath)
	cmd := exec.Command(pythonPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bridge plain final retry failed: %v\n%s", err, output)
	}
}

func TestFlowAgentsSDKBridgeRetriesTransientChatCompletionError(t *testing.T) {
	pythonPath, err := exec.LookPath("python")
	pythonArgs := []string{}
	if err != nil && runtime.GOOS == "windows" {
		pythonPath, err = exec.LookPath("py")
		pythonArgs = []string{"-3"}
	}
	if err != nil {
		t.Skipf("python executable not found: %v", err)
	}

	tempDir := t.TempDir()
	bridgePath := filepath.Join(tempDir, "flow_agents_sdk_bridge.py")
	if err := os.WriteFile(bridgePath, flowAgentsSDKBridge, 0o600); err != nil {
		t.Fatalf("write bridge: %v", err)
	}
	script := `
import asyncio
import importlib.util
import sys

spec = importlib.util.spec_from_file_location("bridge", sys.argv[1])
bridge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bridge)

class FakeMessage:
    content = "最终回答：请求已恢复。"
    tool_calls = []

class FakeChoice:
    message = FakeMessage()

class FakeResponse:
    choices = [FakeChoice()]

calls = 0

class FakeCompletions:
    async def create(self, **kwargs):
        global calls
        calls += 1
        if calls == 1:
            raise RuntimeError("Upstream error: 400")
        return FakeResponse()

class FakeChat:
    completions = FakeCompletions()

class FakeClient:
    chat = FakeChat()

result = asyncio.run(bridge._run_with_compatible_chat_tools(
    client=FakeClient(),
    model_name="glm-5.1",
    system_prompt="",
    user_prompt="用户问题：你好",
    skill="",
    cli_command="ariadne",
))
if not result.get("ok"):
    raise AssertionError(f"transient upstream error should retry, got {result}")
if calls != 2:
    raise AssertionError(f"expected exactly one retry, got {calls}")

class NonRetryCompletions:
    async def create(self, **kwargs):
        raise RuntimeError("Invalid API key")

class NonRetryChat:
    completions = NonRetryCompletions()

class NonRetryClient:
    chat = NonRetryChat()

result = asyncio.run(bridge._run_with_compatible_chat_tools(
    client=NonRetryClient(),
    model_name="glm-5.1",
    system_prompt="",
    user_prompt="用户问题：你好",
    skill="",
    cli_command="ariadne",
))
if result.get("ok"):
    raise AssertionError(f"non-retryable errors should still fail, got {result}")
if "Invalid API key" not in result.get("error", ""):
    raise AssertionError(f"unexpected non-retry error: {result}")
`
	scriptPath := filepath.Join(tempDir, "bridge_transient_retry.py")
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatalf("write bridge script: %v", err)
	}
	args := append(pythonArgs, scriptPath, bridgePath)
	cmd := exec.Command(pythonPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bridge transient retry failed: %v\n%s", err, output)
	}
}

func TestFlowAgentsSDKBridgeFallsBackToLocalRetrievalWhenChatToolsFail(t *testing.T) {
	pythonPath, err := exec.LookPath("python")
	pythonArgs := []string{}
	if err != nil && runtime.GOOS == "windows" {
		pythonPath, err = exec.LookPath("py")
		pythonArgs = []string{"-3"}
	}
	if err != nil {
		t.Skipf("python executable not found: %v", err)
	}

	tempDir := t.TempDir()
	bridgePath := filepath.Join(tempDir, "flow_agents_sdk_bridge.py")
	if err := os.WriteFile(bridgePath, flowAgentsSDKBridge, 0o600); err != nil {
		t.Fatalf("write bridge: %v", err)
	}
	script := `
import asyncio
import importlib.util
import json
import sys

spec = importlib.util.spec_from_file_location("bridge", sys.argv[1])
bridge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bridge)

async def always_upstream_error(*args, **kwargs):
    raise RuntimeError("Upstream error: 400")

def fake_cli(command, action, args, timeout_ms=None):
    if action == "search":
        return json.dumps({
            "ok": True,
            "results": [
                {
                    "id": "memory-a",
                    "title": "微信：杨阳找你",
                    "summary": "杨阳让你匹配好了发给他。",
                    "appName": "Weixin.exe",
                    "preview": "杨阳：匹配好了发我。"
                }
            ]
        }, ensure_ascii=False)
    if action == "recent":
        return json.dumps({"ok": True, "results": []}, ensure_ascii=False)
    if action == "get":
        return json.dumps({
            "ok": True,
            "id": "memory-a",
            "title": "微信：杨阳找你",
            "text": "杨阳：匹配好了发我。"
        }, ensure_ascii=False)
    return json.dumps({"ok": False}, ensure_ascii=False)

class FakeClient:
    pass

bridge._create_chat_completion_with_retries = always_upstream_error
bridge._run_workmemory_cli = fake_cli

result = asyncio.run(bridge._run_with_compatible_chat_tools(
    client=FakeClient(),
    model_name="glm-5.1",
    system_prompt="",
    user_prompt="用户问题：今天有哪些人找过我？",
    skill="",
    cli_command="ariadne",
))
if not result.get("ok"):
    raise AssertionError(f"local retrieval fallback should return an answer, got {result}")
if "memory-a" not in result.get("answer", ""):
    raise AssertionError(f"fallback answer should cite local evidence, got {result}")
if result.get("mode") != "agent:openai-compatible-chat-tools":
    raise AssertionError(f"unexpected mode: {result}")
`
	scriptPath := filepath.Join(tempDir, "bridge_local_retrieval_fallback.py")
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatalf("write bridge script: %v", err)
	}
	args := append(pythonArgs, scriptPath, bridgePath)
	cmd := exec.Command(pythonPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bridge local retrieval fallback failed: %v\n%s", err, output)
	}
}

func TestFlowAgentsSDKBridgePreloadsReadonlyTodoInsteadOfForcingToolChoice(t *testing.T) {
	pythonPath, err := exec.LookPath("python")
	pythonArgs := []string{}
	if err != nil && runtime.GOOS == "windows" {
		pythonPath, err = exec.LookPath("py")
		pythonArgs = []string{"-3"}
	}
	if err != nil {
		t.Skipf("python executable not found: %v", err)
	}

	tempDir := t.TempDir()
	bridgePath := filepath.Join(tempDir, "flow_agents_sdk_bridge.py")
	if err := os.WriteFile(bridgePath, flowAgentsSDKBridge, 0o600); err != nil {
		t.Fatalf("write bridge: %v", err)
	}
	script := `
import asyncio
import importlib.util
import json
import sys

spec = importlib.util.spec_from_file_location("bridge", sys.argv[1])
bridge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bridge)

class FakeMessage:
    content = "当前没有未完成的 Ariadne 待办。"
    tool_calls = []

class FakeChoice:
    message = FakeMessage()

class FakeResponse:
    choices = [FakeChoice()]

calls = []

class FakeCompletions:
    async def create(self, **kwargs):
        calls.append(kwargs)
        return FakeResponse()

class FakeChat:
    completions = FakeCompletions()

class FakeClient:
    chat = FakeChat()

def fake_tool(cli_command, user_prompt, tool_name, args):
    if tool_name != "list_flow_todos":
        raise AssertionError(f"unexpected preload tool: {tool_name}")
    return json.dumps({"ok": True, "action": "todos", "message": "returned 0 todos", "items": []}, ensure_ascii=False)

bridge._run_compatible_tool = fake_tool

result = asyncio.run(bridge._run_with_compatible_chat_tools(
    client=FakeClient(),
    model_name="glm-5.1",
    system_prompt="",
    user_prompt="用户问题：今天有什么待办吗",
    skill="",
    cli_command="ariadne",
))
if not result.get("ok"):
    raise AssertionError(f"readonly todo preload should answer, got {result}")
if calls[0].get("tool_choice") != "auto":
    raise AssertionError(f"readonly todo query must not force tool_choice, got {calls[0].get('tool_choice')}")
if "Ariadne Todo 工具已查询结果" not in calls[0]["messages"][2]["content"]:
    raise AssertionError(f"preloaded todo result missing from prompt: {calls[0]['messages']}")
if "未调用待办工具" in result.get("error", ""):
    raise AssertionError(f"preloaded list_flow_todos should satisfy required todo guard: {result}")
`
	scriptPath := filepath.Join(tempDir, "bridge_readonly_todo_preload.py")
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatalf("write bridge script: %v", err)
	}
	args := append(pythonArgs, scriptPath, bridgePath)
	cmd := exec.Command(pythonPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bridge readonly todo preload failed: %v\n%s", err, output)
	}
}

func TestFlowAgentsSDKBridgePayloadEnablesNativeSkills(t *testing.T) {
	payload := flowAgentsSDKBridgePayload(workmemory.FlowAgentJob{
		Question:     "今天干了什么",
		Intent:       "daily",
		LocalAnswer:  "请通过 skill 查询。",
		Provider:     "openai-compatible",
		BaseURL:      "http://ai.local/v1",
		Model:        "test-model",
		NativeSkills: true,
	}, "openai-compatible", "http://ai.local/v1", "test-model")

	if payload["nativeSkills"] != true {
		t.Fatalf("responses/native skill support should be passed to bridge: %#v", payload)
	}
	if payload["provider"] != "openai-compatible" || payload["baseURL"] != "http://ai.local/v1" || payload["model"] != "test-model" {
		t.Fatalf("unexpected bridge payload identity: %#v", payload)
	}
}

func TestLooksLikeUnexecutedAgentToolCall(t *testing.T) {
	raw := `<tool_call>shell<arg_key>command</arg_key><arg_value>cat "C:\Users\luwei\AppData\Local\Temp\ariadne-agent-skills\ariadne-flow-memory\SKILL.md"</arg_value></tool_call>`
	if !looksLikeUnexecutedAgentToolCall(raw) {
		t.Fatalf("raw shell tool call should be rejected")
	}
	jsonTool := `{"tool_calls":[{"name":"shell","arguments":{"command":"ariadne workmemory recent"}}]}`
	if !looksLikeUnexecutedAgentToolCall(jsonTool) {
		t.Fatalf("raw JSON tool call should be rejected")
	}
	normal := "今天主要处理了心流 OCR 和 Agent 兼容问题。"
	if looksLikeUnexecutedAgentToolCall(normal) {
		t.Fatalf("normal answer should not be rejected")
	}
}

func writeFakeFlowAgentPython(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, "python.cmd")
		body := `@echo off
more > nul
if "%OPENAI_API_KEY%"=="" (
  echo {"ok":false,"error":"missing key"}
  exit /b 0
)
echo {"ok":true,"answer":"SDK bridge answer for memory-chat-ye","mode":"agent:openai-agents-sdk-chat-tools","message":"fake sdk"}
`
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatalf("write fake python: %v", err)
		}
		return path
	}
	path := filepath.Join(dir, "python")
	body := `#!/bin/sh
cat >/dev/null
if [ -z "$OPENAI_API_KEY" ]; then
  printf '%s\n' '{"ok":false,"error":"missing key"}'
  exit 0
fi
printf '%s\n' '{"ok":true,"answer":"SDK bridge answer for memory-chat-ye","mode":"agent:openai-agents-sdk-chat-tools","message":"fake sdk"}'
`
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake python: %v", err)
	}
	return path
}

func TestOpenAICompatibleEmbedderPostsEmbeddingRequest(t *testing.T) {
	t.Setenv("EMBED__API_KEY", "embed-key")
	var captured embeddingRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer embed-key" {
			t.Fatalf("unexpected authorization header: %s", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[1,0,0]},{"index":1,"embedding":[0,1,0]}]}`))
	}))
	defer server.Close()

	embedder := &OpenAICompatibleEmbedder{HTTPClient: server.Client(), APIKeyEnv: []string{"EMBED__API_KEY"}}
	vectors, err := embedder.EmbedTexts(context.Background(), workmemory.EmbeddingJob{
		Provider: "openai-compatible",
		BaseURL:  server.URL + "/v1",
		Model:    "/model/qwen_eb",
		Inputs:   []string{"gateway failure", "postgres timeout"},
	})
	if err != nil {
		t.Fatalf("embed texts: %v", err)
	}
	if captured.Model != "/model/qwen_eb" || len(captured.Input) != 2 || captured.Input[0] != "gateway failure" {
		t.Fatalf("unexpected request payload: %#v", captured)
	}
	if len(vectors) != 2 || vectors[0][0] != 1 || vectors[1][1] != 1 {
		t.Fatalf("unexpected vectors: %#v", vectors)
	}
}

func TestOpenAICompatibleEmbedderRequiresKey(t *testing.T) {
	t.Setenv("EMBED__API_KEY", "")
	embedder := &OpenAICompatibleEmbedder{APIKeyEnv: []string{"EMBED__API_KEY"}}
	_, err := embedder.EmbedTexts(context.Background(), workmemory.EmbeddingJob{
		Provider: "openai-compatible",
		Model:    "/model/qwen_eb",
		Inputs:   []string{"gateway failure"},
	})
	if err == nil || !strings.Contains(err.Error(), "API_KEY") {
		t.Fatalf("expected missing key error, got %v", err)
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
