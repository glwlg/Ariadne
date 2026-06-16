package aiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"ariadne/internal/ocr"
	"ariadne/internal/workmemory"
)

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
		Runner:      "openai-agent",
		Provider:    "openai-compatible",
		BaseURL:     server.URL + "/v1",
		Model:       "test-model",
		Now:         time.Unix(1781435400, 0),
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
	if !strings.Contains(captured.Messages[0].Content, "通用心流 Agent") || !strings.Contains(captured.Messages[1].Content, "memory-chat-ye") {
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
	if result.Mode != "agent:openai-agents-sdk" || !strings.Contains(result.Answer, "SDK") {
		t.Fatalf("unexpected SDK result: %#v", result)
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
echo {"ok":true,"answer":"SDK bridge answer for memory-chat-ye","mode":"agent:openai-agents-sdk","message":"fake sdk"}
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
printf '%s\n' '{"ok":true,"answer":"SDK bridge answer for memory-chat-ye","mode":"agent:openai-agents-sdk","message":"fake sdk"}'
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
