package aiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"ariadne/internal/securestore"
	"ariadne/internal/workmemory"
)

type OpenAICompatibleEmbedder struct {
	HTTPClient    *http.Client
	APIKeyEnv     []string
	SecretTargets []string
}

type OpenAICompatiblePolisher struct {
	HTTPClient    *http.Client
	APIKeyEnv     []string
	SecretTargets []string
}

type OpenAICompatibleExperienceDiscoverer struct {
	HTTPClient    *http.Client
	APIKeyEnv     []string
	SecretTargets []string
}

func NewOpenAICompatibleEmbedder() *OpenAICompatibleEmbedder {
	return &OpenAICompatibleEmbedder{
		HTTPClient:    &http.Client{Timeout: 45 * time.Second},
		APIKeyEnv:     []string{"ARIADNE_EMBED_API_KEY", "EMBED__API_KEY", "OPENAI__API_KEY", "OPENAI_API_KEY"},
		SecretTargets: []string{securestore.TargetEmbeddingAPIKey, securestore.TargetOpenAIAPIKey},
	}
}

func NewOpenAICompatiblePolisher() *OpenAICompatiblePolisher {
	return &OpenAICompatiblePolisher{
		HTTPClient:    &http.Client{Timeout: 45 * time.Second},
		APIKeyEnv:     []string{"ARIADNE_AI_API_KEY", "OPENAI__API_KEY", "OPENAI_API_KEY"},
		SecretTargets: []string{securestore.TargetOpenAIAPIKey},
	}
}

func NewOpenAICompatibleExperienceDiscoverer() *OpenAICompatibleExperienceDiscoverer {
	return &OpenAICompatibleExperienceDiscoverer{
		HTTPClient:    &http.Client{Timeout: 60 * time.Second},
		APIKeyEnv:     []string{"ARIADNE_AI_API_KEY", "OPENAI__API_KEY", "OPENAI_API_KEY"},
		SecretTargets: []string{securestore.TargetOpenAIAPIKey},
	}
}

func (e *OpenAICompatibleEmbedder) EmbedTexts(ctx context.Context, job workmemory.EmbeddingJob) ([][]float64, error) {
	provider := strings.TrimSpace(strings.ToLower(job.Provider))
	if provider != "openai-compatible" && provider != "openai" {
		return nil, fmt.Errorf("不支持的 embedding provider: %s", firstNonEmpty(job.Provider, "disabled"))
	}
	model := strings.TrimSpace(job.Model)
	if model == "" {
		return nil, errors.New("embedding model 未配置")
	}
	inputs := make([]string, 0, len(job.Inputs))
	for _, input := range job.Inputs {
		if text := strings.TrimSpace(input); text != "" {
			inputs = append(inputs, text)
		}
	}
	if len(inputs) == 0 {
		return nil, errors.New("embedding input 为空")
	}
	apiKey := e.apiKey()
	if apiKey == "" {
		return nil, errors.New("未检测到 ARIADNE_EMBED_API_KEY、EMBED__API_KEY 或 OPENAI__API_KEY")
	}
	endpoint := strings.TrimRight(strings.TrimSpace(job.BaseURL), "/")
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	endpoint += "/embeddings"

	raw, err := json.Marshal(embeddingRequest{Model: model, Input: inputs})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+apiKey)

	client := e.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("embedding provider 返回 HTTP %d: %s", response.StatusCode, truncate(string(body), 240))
	}

	var result embeddingResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if len(result.Data) != len(inputs) {
		return nil, fmt.Errorf("embedding 返回数量不匹配: got %d want %d", len(result.Data), len(inputs))
	}
	vectors := make([][]float64, len(result.Data))
	for _, item := range result.Data {
		if item.Index < 0 || item.Index >= len(vectors) {
			return nil, fmt.Errorf("embedding 返回非法 index: %d", item.Index)
		}
		if len(item.Embedding) == 0 {
			return nil, fmt.Errorf("embedding index %d 为空", item.Index)
		}
		vectors[item.Index] = item.Embedding
	}
	return vectors, nil
}

func (e *OpenAICompatibleEmbedder) apiKey() string {
	envs := e.APIKeyEnv
	if len(envs) == 0 {
		envs = []string{"ARIADNE_EMBED_API_KEY", "EMBED__API_KEY", "OPENAI__API_KEY", "OPENAI_API_KEY"}
	}
	for _, name := range envs {
		if value := cleanAPIKey(os.Getenv(name)); value != "" {
			return value
		}
	}
	return apiKeyFromCredentialManager(e.SecretTargets)
}

func (p *OpenAICompatiblePolisher) PolishDraft(ctx context.Context, job workmemory.DraftPolishJob) (workmemory.Draft, error) {
	provider := strings.TrimSpace(strings.ToLower(job.Provider))
	if provider != "openai-compatible" && provider != "openai" {
		return workmemory.Draft{}, fmt.Errorf("不支持的 AI provider: %s", firstNonEmpty(job.Provider, "disabled"))
	}
	model := strings.TrimSpace(job.Model)
	if model == "" {
		return workmemory.Draft{}, errors.New("AI model 未配置")
	}
	apiKey := p.apiKey()
	if apiKey == "" {
		return workmemory.Draft{}, errors.New("未检测到 ARIADNE_AI_API_KEY 或 OPENAI_API_KEY")
	}
	endpoint := strings.TrimRight(strings.TrimSpace(job.BaseURL), "/")
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	endpoint += "/chat/completions"

	payload := chatCompletionRequest{
		Model:       model,
		Temperature: 0.2,
		Messages: []chatMessage{
			{Role: "system", Content: "你是 Ariadne 工作记忆中心的中文草稿编辑器。只润色表达、结构和可读性，不新增事实，不删除 evidence ID，不输出解释。"},
			{Role: "user", Content: polishPrompt(job)},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return workmemory.Draft{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return workmemory.Draft{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+apiKey)

	client := p.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return workmemory.Draft{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024))
	if err != nil {
		return workmemory.Draft{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return workmemory.Draft{}, fmt.Errorf("AI provider 返回 HTTP %d: %s", response.StatusCode, truncate(string(body), 240))
	}

	var result chatCompletionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return workmemory.Draft{}, err
	}
	if len(result.Choices) == 0 {
		return workmemory.Draft{}, errors.New("AI provider 未返回 choices")
	}
	content := strings.TrimSpace(result.Choices[0].Message.Content)
	if content == "" {
		return workmemory.Draft{}, errors.New("AI provider 返回空内容")
	}
	return workmemory.Draft{
		Title:    "AI 润色：" + strings.TrimSpace(job.Draft.Title),
		Body:     content,
		Evidence: append([]string(nil), job.Draft.Evidence...),
	}, nil
}

func (p *OpenAICompatiblePolisher) apiKey() string {
	envs := p.APIKeyEnv
	if len(envs) == 0 {
		envs = []string{"ARIADNE_AI_API_KEY", "OPENAI__API_KEY", "OPENAI_API_KEY"}
	}
	for _, name := range envs {
		if value := cleanAPIKey(os.Getenv(name)); value != "" {
			return value
		}
	}
	return apiKeyFromCredentialManager(p.SecretTargets)
}

func (d *OpenAICompatibleExperienceDiscoverer) DiscoverExperiences(ctx context.Context, job workmemory.ExperienceDiscoveryJob) (workmemory.ExperienceReport, error) {
	provider := strings.TrimSpace(strings.ToLower(job.Provider))
	if provider != "openai-compatible" && provider != "openai" {
		return workmemory.ExperienceReport{}, fmt.Errorf("不支持的 AI provider: %s", firstNonEmpty(job.Provider, "disabled"))
	}
	model := strings.TrimSpace(job.Model)
	if model == "" {
		return workmemory.ExperienceReport{}, errors.New("AI model 未配置")
	}
	if len(job.Evidence) == 0 {
		return workmemory.ExperienceReport{}, errors.New("experience evidence 为空")
	}
	apiKey := d.apiKey()
	if apiKey == "" {
		return workmemory.ExperienceReport{}, errors.New("未检测到 ARIADNE_AI_API_KEY 或 OPENAI_API_KEY")
	}
	endpoint := strings.TrimRight(strings.TrimSpace(job.BaseURL), "/")
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	endpoint += "/chat/completions"

	payload := chatCompletionRequest{
		Model:       model,
		Temperature: 0.2,
		Messages: []chatMessage{
			{Role: "system", Content: "你是 Ariadne 工作记忆中心的经验发现器。只根据给定 evidence 归纳稳定模式，不新增事实。必须只输出 JSON，不要使用 Markdown。"},
			{Role: "user", Content: experienceDiscoveryPrompt(job)},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return workmemory.ExperienceReport{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return workmemory.ExperienceReport{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+apiKey)

	client := d.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return workmemory.ExperienceReport{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024))
	if err != nil {
		return workmemory.ExperienceReport{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return workmemory.ExperienceReport{}, fmt.Errorf("AI provider 返回 HTTP %d: %s", response.StatusCode, truncate(string(body), 240))
	}

	var result chatCompletionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return workmemory.ExperienceReport{}, err
	}
	if len(result.Choices) == 0 {
		return workmemory.ExperienceReport{}, errors.New("AI provider 未返回 choices")
	}
	content := strings.TrimSpace(result.Choices[0].Message.Content)
	if content == "" {
		return workmemory.ExperienceReport{}, errors.New("AI provider 返回空内容")
	}
	return parseExperienceDiscoveryReport(content, job)
}

func (d *OpenAICompatibleExperienceDiscoverer) apiKey() string {
	envs := d.APIKeyEnv
	if len(envs) == 0 {
		envs = []string{"ARIADNE_AI_API_KEY", "OPENAI__API_KEY", "OPENAI_API_KEY"}
	}
	for _, name := range envs {
		if value := cleanAPIKey(os.Getenv(name)); value != "" {
			return value
		}
	}
	return apiKeyFromCredentialManager(d.SecretTargets)
}

func apiKeyFromCredentialManager(targets []string) string {
	for _, target := range targets {
		value, ok, err := securestore.Read(target)
		if err == nil && ok {
			if token := cleanAPIKey(value); token != "" {
				return token
			}
		}
	}
	return ""
}

func cleanAPIKey(value string) string {
	value = strings.ReplaceAll(value, "\x00", "")
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if key, candidate, ok := strings.Cut(line, "="); ok {
			normalized := strings.ToUpper(strings.TrimSpace(key))
			if strings.Contains(normalized, "API_KEY") || strings.Contains(normalized, "TOKEN") {
				value = candidate
				break
			}
		}
	}
	if hash := strings.Index(value, "#"); hash >= 0 {
		value = value[:hash]
	}
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	value = strings.ReplaceAll(value, "\x00", "")
	return strings.TrimSpace(value)
}

func polishPrompt(job workmemory.DraftPolishJob) string {
	kindLabel := map[string]string{
		"daily":         "日报草稿",
		"retrospective": "复盘草稿",
		"knowledge":     "知识草稿",
	}[strings.TrimSpace(job.Kind)]
	if kindLabel == "" {
		kindLabel = "工作记忆草稿"
	}
	return fmt.Sprintf("请润色下面的 Ariadne %s。要求：\n1. 使用中文 Markdown。\n2. 保留并明确引用 evidence ID。\n3. 不新增事实、不做无法从草稿推导的判断。\n4. 不输出任何前言、解释或代码块。\n\n标题：%s\nEvidence IDs：%s\n\n原始草稿：\n%s",
		kindLabel,
		strings.TrimSpace(job.Draft.Title),
		strings.Join(job.Draft.Evidence, ", "),
		strings.TrimSpace(job.Draft.Body),
	)
}

func experienceDiscoveryPrompt(job workmemory.ExperienceDiscoveryJob) string {
	now := job.Now
	if now.IsZero() {
		now = time.Now()
	}
	raw, _ := json.MarshalIndent(job.Evidence, "", "  ")
	return fmt.Sprintf(`请从 Ariadne 工作记忆 evidence 中发现可复用经验。只允许引用输入里的 evidence id，不要编造不存在的 id。

要求：
1. 输出严格 JSON object，不要代码块，不要 Markdown。
2. insight 最多 6 条，优先选择可行动、可复用、可验证的模式。
3. 每条 insight 必须至少引用 1 个 evidence id；没有足够证据时返回空 insights。
4. kind 只能是 repeated_issue、automation_opportunity、knowledge_gap 或 ai_insight。
5. severity 只能是 high、medium 或 low；confidence 使用 0 到 1 的数字。
6. recommendation 必须是下一步可执行建议，且保留人工审核边界。

JSON schema:
{
  "title": "AI 经验发现报告",
  "summary": "一句中文摘要",
  "insights": [
    {
      "kind": "repeated_issue",
      "title": "短标题",
      "summary": "线索摘要",
      "reason": "为什么这些 evidence 支持该线索",
      "recommendation": "下一步建议",
      "evidence": ["evidence-id"],
      "confidence": 0.7,
      "severity": "medium"
    }
  ]
}

周期：最近 %d 天
生成时间：%s
Evidence JSON:
%s`, job.PeriodDays, now.Format(time.RFC3339), string(raw))
}

func parseExperienceDiscoveryReport(content string, job workmemory.ExperienceDiscoveryJob) (workmemory.ExperienceReport, error) {
	var payload experienceDiscoveryPayload
	if err := json.Unmarshal([]byte(extractJSONObject(content)), &payload); err != nil {
		return workmemory.ExperienceReport{}, err
	}
	now := job.Now
	if now.IsZero() {
		now = time.Now()
	}
	report := workmemory.ExperienceReport{
		Title:       strings.TrimSpace(firstNonEmpty(payload.Title, "AI 经验发现报告")),
		Summary:     strings.TrimSpace(payload.Summary),
		PeriodDays:  job.PeriodDays,
		EntryCount:  len(job.Evidence),
		GeneratedAt: now.Unix(),
	}
	for _, item := range payload.Insights {
		report.Insights = append(report.Insights, workmemory.ExperienceInsight{
			Kind:           strings.TrimSpace(item.Kind),
			Title:          strings.TrimSpace(item.Title),
			Summary:        strings.TrimSpace(item.Summary),
			Reason:         strings.TrimSpace(item.Reason),
			Recommendation: strings.TrimSpace(item.Recommendation),
			Evidence:       cleanStrings(item.Evidence),
			Confidence:     item.Confidence,
			Severity:       strings.TrimSpace(item.Severity),
			RequiresReview: true,
			CreatedAt:      now.Unix(),
		})
	}
	return report, nil
}

func extractJSONObject(content string) string {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end >= start {
		return content[start : end+1]
	}
	return content
}

func cleanStrings(items []string) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func truncate(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	return value[:max] + "..."
}

type chatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

type experienceDiscoveryPayload struct {
	Title    string `json:"title"`
	Summary  string `json:"summary"`
	Insights []struct {
		Kind           string   `json:"kind"`
		Title          string   `json:"title"`
		Summary        string   `json:"summary"`
		Reason         string   `json:"reason"`
		Recommendation string   `json:"recommendation"`
		Evidence       []string `json:"evidence"`
		Confidence     float64  `json:"confidence"`
		Severity       string   `json:"severity"`
	} `json:"insights"`
}

type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}
