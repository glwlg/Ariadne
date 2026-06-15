package aiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
