package aiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ariadne/internal/securestore"
	"ariadne/internal/workmemory"
)

type OpenAICompatibleFlowAutonomyAnalyzer struct {
	HTTPClient    *http.Client
	APIKeyEnv     []string
	SecretTargets []string
}

type flowAutonomyPayload struct {
	Suggestions []flowAutonomySuggestionPayload `json:"suggestions"`
}

type flowAutonomySuggestionPayload struct {
	EntryID     string            `json:"entryId"`
	ActionType  string            `json:"actionType"`
	Title       string            `json:"title"`
	Summary     string            `json:"summary"`
	Body        string            `json:"body"`
	Target      string            `json:"target"`
	Priority    string            `json:"priority"`
	Confidence  float64           `json:"confidence"`
	Payload     map[string]string `json:"payload"`
	EvidenceIDs []string          `json:"evidenceIds"`
}

func NewOpenAICompatibleFlowAutonomyAnalyzer() *OpenAICompatibleFlowAutonomyAnalyzer {
	return &OpenAICompatibleFlowAutonomyAnalyzer{
		HTTPClient:    &http.Client{Timeout: 45 * time.Second},
		APIKeyEnv:     []string{"ARIADNE_AI_API_KEY", "OPENAI__API_KEY", "OPENAI_API_KEY"},
		SecretTargets: []string{securestore.TargetOpenAIAPIKey},
	}
}

func (a *OpenAICompatibleFlowAutonomyAnalyzer) AnalyzeFlowAutonomy(ctx context.Context, job workmemory.FlowAutonomyAnalysisJob) (workmemory.FlowAutonomyAnalysisResult, error) {
	provider := strings.TrimSpace(strings.ToLower(job.Provider))
	if provider == "" {
		provider = "openai-compatible"
	}
	if provider != "openai-compatible" && provider != "openai" {
		return workmemory.FlowAutonomyAnalysisResult{}, fmt.Errorf("不支持的 AI provider: %s", firstNonEmpty(job.Provider, "disabled"))
	}
	model := strings.TrimSpace(job.Model)
	if model == "" {
		return workmemory.FlowAutonomyAnalysisResult{}, errors.New("AI model 未配置")
	}
	apiKey := a.apiKey()
	if apiKey == "" {
		return workmemory.FlowAutonomyAnalysisResult{}, errors.New("未检测到 ARIADNE_AI_API_KEY 或 OPENAI_API_KEY")
	}
	endpoint := strings.TrimRight(strings.TrimSpace(job.BaseURL), "/")
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	endpoint += "/chat/completions"

	payload := chatCompletionRequest{
		Model:       model,
		Temperature: 0,
		Messages: []chatMessage{
			{Role: "system", Content: flowAutonomySystemPrompt()},
			{Role: "user", Content: flowAutonomyPrompt(job)},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return workmemory.FlowAutonomyAnalysisResult{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return workmemory.FlowAutonomyAnalysisResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+apiKey)

	client := a.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return workmemory.FlowAutonomyAnalysisResult{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024))
	if err != nil {
		return workmemory.FlowAutonomyAnalysisResult{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return workmemory.FlowAutonomyAnalysisResult{}, fmt.Errorf("AI provider 返回 HTTP %d: %s", response.StatusCode, truncate(string(body), 360))
	}

	var result chatCompletionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return workmemory.FlowAutonomyAnalysisResult{}, err
	}
	if len(result.Choices) == 0 {
		return workmemory.FlowAutonomyAnalysisResult{}, errors.New("AI provider 未返回 choices")
	}
	return parseFlowAutonomyAnalysis(result.Choices[0].Message.Content)
}

func (a *OpenAICompatibleFlowAutonomyAnalyzer) apiKey() string {
	envs := a.APIKeyEnv
	if len(envs) == 0 {
		envs = []string{"ARIADNE_AI_API_KEY", "OPENAI__API_KEY", "OPENAI_API_KEY"}
	}
	return apiKeyFromSources(envs, a.SecretTargets)
}

func flowAutonomySystemPrompt() string {
	return `你是 Ariadne Flow Autonomy 的本地上下文分析 agent。你的任务是从工作记忆、OCR 和剪贴板证据中判断是否存在需要 Ariadne 主动准备的低风险候选动作。

只返回 JSON，不要 Markdown，不要解释。
不要依赖固定关键词；按语义判断。
只在证据显示用户本人或当前聊天中的第一人称明确承诺稍后执行某件事时，返回 follow_up_candidate。
如果截图里只有别人催促、普通聊天、已经完成的动作、指代不清、风险高或置信度低于 0.6，返回空 suggestions。
不要自动发送消息，不要修改外部应用，不要创建正式待办；这里只生成待确认候选动作。

JSON schema:
{
  "suggestions": [
    {
      "entryId": "证据 ID",
      "actionType": "follow_up_candidate",
      "title": "简短动作标题",
      "summary": "为什么需要跟进",
      "body": "保留原始承诺或可执行描述",
      "target": "联系人、群聊或窗口",
      "priority": "low|normal|high",
      "confidence": 0.0,
      "payload": {"todoTitle": "可加入正式待办的标题"},
      "evidenceIds": ["证据 ID"]
    }
  ]
}`
}

func flowAutonomyPrompt(job workmemory.FlowAutonomyAnalysisJob) string {
	now := job.Now
	if now.IsZero() {
		now = time.Now()
	}
	evidence, _ := json.MarshalIndent(job.Evidence, "", "  ")
	return fmt.Sprintf(`扩展：%s
当前时间：%s

请分析下面 Evidence JSON，找出需要生成待确认跟进动作的承诺。
特别注意聊天截图 OCR：一张截图可能包含左侧会话列表、当前会话、文件卡片和输入框；只有当前会话中明确表达的待办承诺才可生成候选动作。

Evidence JSON:
%s`, firstNonEmpty(job.ExtensionID, "flow.communication_assist"), now.Format(time.RFC3339), string(evidence))
}

func parseFlowAutonomyAnalysis(content string) (workmemory.FlowAutonomyAnalysisResult, error) {
	var payload flowAutonomyPayload
	if err := json.Unmarshal([]byte(extractJSONObject(content)), &payload); err != nil {
		return workmemory.FlowAutonomyAnalysisResult{}, err
	}
	result := workmemory.FlowAutonomyAnalysisResult{Suggestions: []workmemory.FlowAutonomySuggestion{}}
	for _, item := range payload.Suggestions {
		result.Suggestions = append(result.Suggestions, workmemory.FlowAutonomySuggestion{
			EntryID:     strings.TrimSpace(item.EntryID),
			ActionType:  strings.TrimSpace(item.ActionType),
			Title:       strings.TrimSpace(item.Title),
			Summary:     strings.TrimSpace(item.Summary),
			Body:        strings.TrimSpace(item.Body),
			Target:      strings.TrimSpace(item.Target),
			Priority:    strings.TrimSpace(item.Priority),
			Confidence:  item.Confidence,
			Payload:     item.Payload,
			EvidenceIDs: cleanStrings(item.EvidenceIDs),
		})
	}
	return result, nil
}
