package aiclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ariadne/internal/ocr"
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

type OpenAICompatibleOCRSummarizer struct {
	HTTPClient    *http.Client
	APIKeyEnv     []string
	SecretTargets []string
}

type OpenAICompatibleImageOCR struct {
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

func NewOpenAICompatibleOCRSummarizer() *OpenAICompatibleOCRSummarizer {
	return &OpenAICompatibleOCRSummarizer{
		HTTPClient:    &http.Client{Timeout: 45 * time.Second},
		APIKeyEnv:     []string{"ARIADNE_AI_API_KEY", "OPENAI__API_KEY", "OPENAI_API_KEY"},
		SecretTargets: []string{securestore.TargetOpenAIAPIKey},
	}
}

func NewOpenAICompatibleImageOCR() *OpenAICompatibleImageOCR {
	return &OpenAICompatibleImageOCR{
		HTTPClient:    &http.Client{Timeout: 60 * time.Second},
		APIKeyEnv:     []string{"ARIADNE_OCR_API_KEY", "ARIADNE_AI_API_KEY", "OPENAI__API_KEY", "OPENAI_API_KEY"},
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

func (o *OpenAICompatibleImageOCR) RecognizeImageOCR(ctx context.Context, job ocr.AIOCRJob) (ocr.AIResult, error) {
	provider := strings.TrimSpace(strings.ToLower(job.Provider))
	if provider != "openai-compatible" && provider != "openai" {
		return ocr.AIResult{}, fmt.Errorf("不支持的大模型 OCR provider: %s", firstNonEmpty(job.Provider, "disabled"))
	}
	model := strings.TrimSpace(job.Model)
	if model == "" {
		return ocr.AIResult{}, errors.New("大模型 OCR model 未配置")
	}
	imagePath := strings.TrimSpace(job.ImagePath)
	if imagePath == "" {
		return ocr.AIResult{}, errors.New("大模型 OCR 图片路径为空")
	}
	rawImage, err := os.ReadFile(imagePath)
	if err != nil {
		return ocr.AIResult{}, fmt.Errorf("读取 OCR 图片失败: %w", err)
	}
	if len(rawImage) == 0 {
		return ocr.AIResult{}, errors.New("OCR 图片为空")
	}
	apiKey := o.apiKey()
	if apiKey == "" {
		return ocr.AIResult{}, errors.New("未检测到 ARIADNE_OCR_API_KEY、ARIADNE_AI_API_KEY 或 OPENAI_API_KEY")
	}
	endpoint := strings.TrimRight(strings.TrimSpace(job.BaseURL), "/")
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	endpoint += "/chat/completions"

	payload := visionChatCompletionRequest{
		Model:       model,
		Temperature: 0,
		Messages: []visionChatMessage{
			{
				Role:    "system",
				Content: "你是 Ariadne 的视觉 OCR 引擎。只识别图片中实际可见的文字，保持原文语言，按阅读顺序输出纯文本；不要解释、不要 Markdown、不要编造不可见内容，密钥和 token 只输出占位符 [REDACTED]。",
			},
			{
				Role: "user",
				Content: []visionContentPart{
					{Type: "text", Text: imageOCRPrompt()},
					{
						Type: "image_url",
						ImageURL: &visionImageURL{
							URL:    "data:" + imageMimeType(imagePath) + ";base64," + base64.StdEncoding.EncodeToString(rawImage),
							Detail: "high",
						},
					},
				},
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ocr.AIResult{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return ocr.AIResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+apiKey)

	client := o.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	started := time.Now()
	response, err := client.Do(request)
	if err != nil {
		return ocr.AIResult{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
	if err != nil {
		return ocr.AIResult{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ocr.AIResult{}, fmt.Errorf("大模型 OCR provider 返回 HTTP %d: %s", response.StatusCode, truncate(string(body), 240))
	}

	var result chatCompletionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return ocr.AIResult{}, err
	}
	if len(result.Choices) == 0 {
		return ocr.AIResult{}, errors.New("大模型 OCR provider 未返回 choices")
	}
	text := strings.TrimSpace(result.Choices[0].Message.Content)
	if text == "" {
		return ocr.AIResult{}, errors.New("大模型 OCR provider 返回空内容")
	}
	return ocr.AIResult{
		Provider:  "vision:" + model,
		Text:      text,
		Lines:     textToOCRLines(text),
		ElapsedMs: int(time.Since(started) / time.Millisecond),
	}, nil
}

func (o *OpenAICompatibleImageOCR) apiKey() string {
	envs := o.APIKeyEnv
	if len(envs) == 0 {
		envs = []string{"ARIADNE_OCR_API_KEY", "ARIADNE_AI_API_KEY", "OPENAI__API_KEY", "OPENAI_API_KEY"}
	}
	for _, name := range envs {
		if value := cleanAPIKey(os.Getenv(name)); value != "" {
			return value
		}
	}
	return apiKeyFromCredentialManager(o.SecretTargets)
}

func (s *OpenAICompatibleOCRSummarizer) SummarizeOCR(ctx context.Context, job workmemory.OCRSummaryJob) (workmemory.OCRSummaryResult, error) {
	provider := strings.TrimSpace(strings.ToLower(job.Provider))
	if provider != "openai-compatible" && provider != "openai" {
		return workmemory.OCRSummaryResult{}, fmt.Errorf("不支持的 AI provider: %s", firstNonEmpty(job.Provider, "disabled"))
	}
	model := strings.TrimSpace(job.Model)
	if model == "" {
		return workmemory.OCRSummaryResult{}, errors.New("AI model 未配置")
	}
	if strings.TrimSpace(job.OCRText) == "" {
		return workmemory.OCRSummaryResult{}, errors.New("OCR 文本为空")
	}
	apiKey := s.apiKey()
	if apiKey == "" {
		return workmemory.OCRSummaryResult{}, errors.New("未检测到 ARIADNE_AI_API_KEY 或 OPENAI_API_KEY")
	}
	endpoint := strings.TrimRight(strings.TrimSpace(job.BaseURL), "/")
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	endpoint += "/chat/completions"

	payload := chatCompletionRequest{
		Model:       model,
		Temperature: 0.1,
		Messages: []chatMessage{
			{Role: "system", Content: "你是 Ariadne 心流时间线的中文 OCR 整理器。只根据给定 OCR 和上下文生成标题、摘要和整理正文，不新增事实，不暴露敏感密钥。必须只输出 JSON。"},
			{Role: "user", Content: ocrSummaryPrompt(job)},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return workmemory.OCRSummaryResult{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return workmemory.OCRSummaryResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+apiKey)

	client := s.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return workmemory.OCRSummaryResult{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024))
	if err != nil {
		return workmemory.OCRSummaryResult{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return workmemory.OCRSummaryResult{}, fmt.Errorf("AI provider 返回 HTTP %d: %s", response.StatusCode, truncate(string(body), 240))
	}

	var result chatCompletionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return workmemory.OCRSummaryResult{}, err
	}
	if len(result.Choices) == 0 {
		return workmemory.OCRSummaryResult{}, errors.New("AI provider 未返回 choices")
	}
	content := strings.TrimSpace(result.Choices[0].Message.Content)
	if content == "" {
		return workmemory.OCRSummaryResult{}, errors.New("AI provider 返回空内容")
	}
	return parseOCRSummary(content)
}

func (s *OpenAICompatibleOCRSummarizer) apiKey() string {
	envs := s.APIKeyEnv
	if len(envs) == 0 {
		envs = []string{"ARIADNE_AI_API_KEY", "OPENAI__API_KEY", "OPENAI_API_KEY"}
	}
	for _, name := range envs {
		if value := cleanAPIKey(os.Getenv(name)); value != "" {
			return value
		}
	}
	return apiKeyFromCredentialManager(s.SecretTargets)
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

func imageOCRPrompt() string {
	return `请识别图片中所有可见文字。

要求：
1. 按屏幕阅读顺序输出纯文本，尽量保持换行。
2. 中文、英文、路径、命令和数字都要保留。
3. 删除明显的装饰噪声，不要输出图像描述。
4. 不要输出 Markdown、JSON、前言或解释。
5. 如果看不清或没有文字，只输出“未识别到文字”。`
}

func ocrSummaryPrompt(job workmemory.OCRSummaryJob) string {
	now := job.Now
	if now.IsZero() {
		now = time.Now()
	}
	entry := job.Entry
	return fmt.Sprintf(`请把下面的截图 OCR 结果整理成 Ariadne 心流时间线可以直接展示的内容。

要求：
1. 输出严格 JSON object，不要代码块，不要 Markdown 前言。
2. title 使用中文或原文关键短语，8 到 24 个中文字符左右，避免使用“work”“截图”“当前屏幕”等泛标题。
3. summary 用一句话概括这张截图主要在做什么。
4. text 使用简洁 Markdown，优先整理成“要点”或“可见内容”，不要逐字堆叠原始 OCR 噪声。
5. 只能使用输入里可见的信息，不要补充推测；信息不足时明确写“可见内容不足”。
6. 删除明显重复、乱码、路径噪声和 UI 装饰词；不要输出密钥、token、密码等敏感值。
7. 如果截图来自聊天软件，必须区分“当前会话正文”“左侧会话列表/联系人列表”“后台窗口/侧边栏”。不要把左侧列表里出现的人名、群名或服务号当成当前聊天对象；不要把右侧绿色气泡/本机发送者当成对方联系人；无法确认时只写“界面可见某某”，不要写成“与某某沟通”。
8. 群聊场景只在 OCR 明确显示当前群名或发言人紧邻消息时记录；否则不要从侧栏预览、消息列表或背景窗口推断人物关系。

JSON schema:
{
  "title": "可读标题",
  "summary": "一句摘要",
  "text": "## 可见内容\n- 整理后的要点"
}

时间：%s
应用：%s
窗口：%s
来源：%s
当前标题：%s
当前摘要：%s

OCR 文本：
%s`,
		now.Format(time.RFC3339),
		strings.TrimSpace(entry.AppName),
		strings.TrimSpace(entry.WindowTitle),
		strings.TrimSpace(entry.Source),
		strings.TrimSpace(entry.Title),
		strings.TrimSpace(entry.Summary),
		truncate(strings.TrimSpace(job.OCRText), 8000),
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

func parseOCRSummary(content string) (workmemory.OCRSummaryResult, error) {
	var payload ocrSummaryPayload
	if err := json.Unmarshal([]byte(extractJSONObject(content)), &payload); err != nil {
		return workmemory.OCRSummaryResult{}, err
	}
	return workmemory.OCRSummaryResult{
		Title:   strings.TrimSpace(payload.Title),
		Summary: strings.TrimSpace(payload.Summary),
		Text:    strings.TrimSpace(payload.Text),
	}, nil
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

type visionChatCompletionRequest struct {
	Model       string              `json:"model"`
	Messages    []visionChatMessage `json:"messages"`
	Temperature float64             `json:"temperature"`
}

type visionChatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type visionContentPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *visionImageURL `json:"image_url,omitempty"`
}

type visionImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
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

type ocrSummaryPayload struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Text    string `json:"text"`
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

func imageMimeType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	default:
		return "image/png"
	}
}

func textToOCRLines(text string) []ocr.Line {
	lines := []ocr.Line{}
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, ocr.Line{Text: line, Confidence: 1})
	}
	return lines
}
