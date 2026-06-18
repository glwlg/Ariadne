package aiclient

import (
	"bytes"
	"context"
	"crypto/sha1"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"ariadne/internal/securestore"
	"ariadne/internal/workmemory"
)

//go:embed flow_agents_sdk_bridge.py
var flowAgentsSDKBridge []byte

type FlowAgentRouter struct {
	OpenAI workmemory.FlowAgentRunner
	Codex  workmemory.FlowAgentRunner
}

type OpenAIAgentsSDKFlowAgent struct {
	PythonPath    string
	APIKeyEnv     []string
	SecretTargets []string
	Timeout       time.Duration
}

type OpenAICompatibleFlowAgent struct {
	HTTPClient    *http.Client
	APIKeyEnv     []string
	SecretTargets []string
}

func NewFlowAgentRouter() *FlowAgentRouter {
	return &FlowAgentRouter{
		OpenAI: NewOpenAIAgentsSDKFlowAgent(),
		Codex:  NewCodexFlowAgent(),
	}
}

func NewOpenAIAgentsSDKFlowAgent() *OpenAIAgentsSDKFlowAgent {
	return &OpenAIAgentsSDKFlowAgent{
		APIKeyEnv:     []string{"ARIADNE_AI_API_KEY", "OPENAI__API_KEY", "OPENAI_API_KEY"},
		SecretTargets: []string{securestore.TargetOpenAIAPIKey},
		Timeout:       120 * time.Second,
	}
}

func NewOpenAICompatibleFlowAgent() *OpenAICompatibleFlowAgent {
	return &OpenAICompatibleFlowAgent{
		HTTPClient:    &http.Client{Timeout: 90 * time.Second},
		APIKeyEnv:     []string{"ARIADNE_AI_API_KEY", "OPENAI__API_KEY", "OPENAI_API_KEY"},
		SecretTargets: []string{securestore.TargetOpenAIAPIKey},
	}
}

func (a *OpenAIAgentsSDKFlowAgent) AnswerFlow(ctx context.Context, job workmemory.FlowAgentJob) (workmemory.FlowAgentResult, error) {
	runner := strings.TrimSpace(strings.ToLower(job.Runner))
	if runner != "" && runner != "openai-agent" && runner != "agent-sdk" && runner != "agents-sdk" && runner != "openai-agents-sdk" {
		return workmemory.FlowAgentResult{}, fmt.Errorf("不支持的 flow agent runner: %s", firstNonEmpty(job.Runner, "disabled"))
	}
	provider := strings.TrimSpace(strings.ToLower(job.Provider))
	if provider == "" {
		provider = "openai-compatible"
	}
	if provider != "openai-compatible" && provider != "openai" {
		return workmemory.FlowAgentResult{}, fmt.Errorf("不支持的 AI provider: %s", firstNonEmpty(job.Provider, "disabled"))
	}
	model := strings.TrimSpace(job.Model)
	if model == "" {
		return workmemory.FlowAgentResult{}, errors.New("AI model 未配置")
	}
	apiKey := a.apiKey()
	if apiKey == "" {
		return workmemory.FlowAgentResult{}, errors.New("未检测到 ARIADNE_AI_API_KEY 或 OPENAI_API_KEY")
	}
	pythonPath := strings.TrimSpace(a.PythonPath)
	if pythonPath == "" {
		pythonPath = findFlowAgentPython()
	}
	if pythonPath == "" {
		return workmemory.FlowAgentResult{}, errors.New("未找到 Ariadne OpenAI Agents SDK Python runtime；请配置 ARIADNE_FLOW_AGENT_PYTHON，或安装到 %LOCALAPPDATA%\\Ariadne\\agent-python")
	}
	bridgePath, err := writeFlowAgentsSDKBridgeFile()
	if err != nil {
		return workmemory.FlowAgentResult{}, err
	}

	endpoint := strings.TrimRight(strings.TrimSpace(job.BaseURL), "/")
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	requestPayload := flowAgentsSDKBridgePayload(job, provider, endpoint, model)
	raw, err := json.Marshal(requestPayload)
	if err != nil {
		return workmemory.FlowAgentResult{}, err
	}
	runCtx := ctx
	cancel := func() {}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && a.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, a.Timeout)
	}
	defer cancel()

	cmd := exec.CommandContext(runCtx, pythonPath, bridgePath)
	cmd.Stdin = bytes.NewReader(raw)
	cmd.Env = flowAgentCommandEnv(os.Environ(), apiKey, endpoint)
	configureFlowAgentCommand(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if runCtx.Err() != nil {
			return workmemory.FlowAgentResult{}, errors.New("OpenAI Agents SDK 执行超时")
		}
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return workmemory.FlowAgentResult{}, fmt.Errorf("OpenAI Agents SDK 执行失败: %s", truncate(detail, 480))
	}
	var response struct {
		OK      bool   `json:"ok"`
		Answer  string `json:"answer"`
		Mode    string `json:"mode"`
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return workmemory.FlowAgentResult{}, fmt.Errorf("OpenAI Agents SDK 输出解析失败: %w", err)
	}
	if !response.OK {
		return workmemory.FlowAgentResult{}, errors.New(firstNonEmpty(strings.TrimSpace(response.Error), "OpenAI Agents SDK 未返回成功结果"))
	}
	answer := strings.TrimSpace(response.Answer)
	if answer == "" {
		return workmemory.FlowAgentResult{}, errors.New("OpenAI Agents SDK 返回空内容")
	}
	if looksLikeUnexecutedAgentToolCall(answer) {
		return workmemory.FlowAgentResult{}, errors.New("OpenAI Agents SDK 返回了未执行的工具调用文本，当前兼容接口未真正执行工具调用")
	}
	return workmemory.FlowAgentResult{
		Answer:  answer,
		Mode:    firstNonEmpty(strings.TrimSpace(response.Mode), "agent:openai-agents-sdk"),
		Message: firstNonEmpty(strings.TrimSpace(response.Message), flowAgentResultMessage("OpenAI Agents SDK", len(job.Evidence))),
	}, nil
}

func (r *FlowAgentRouter) AnswerFlow(ctx context.Context, job workmemory.FlowAgentJob) (workmemory.FlowAgentResult, error) {
	switch strings.TrimSpace(strings.ToLower(job.Runner)) {
	case "openai-agent", "agent-sdk", "agents-sdk", "openai-agents-sdk", "":
		if r.OpenAI == nil {
			return workmemory.FlowAgentResult{}, errors.New("内置心流 agent runner 未注册")
		}
		job.Runner = "openai-agent"
		return r.OpenAI.AnswerFlow(ctx, job)
	case "codex", "codex-cli":
		if r.Codex == nil {
			return workmemory.FlowAgentResult{}, errors.New("Codex runner 未注册")
		}
		job.Runner = "codex"
		return r.Codex.AnswerFlow(ctx, job)
	default:
		return workmemory.FlowAgentResult{}, fmt.Errorf("不支持的 flow agent runner: %s", firstNonEmpty(job.Runner, "disabled"))
	}
}

func flowAgentsSDKBridgePayload(job workmemory.FlowAgentJob, provider string, endpoint string, model string) map[string]any {
	return map[string]any{
		"provider":     provider,
		"baseURL":      endpoint,
		"model":        model,
		"systemPrompt": flowAgentSystemPrompt(),
		"userPrompt":   flowAgentPrompt(job),
		"skill":        flowMemorySkillContent(),
		"toolCommand":  flowMemoryToolCommand(job),
		"nativeSkills": job.NativeSkills,
	}
}

func (a *OpenAICompatibleFlowAgent) AnswerFlow(ctx context.Context, job workmemory.FlowAgentJob) (workmemory.FlowAgentResult, error) {
	runner := strings.TrimSpace(strings.ToLower(job.Runner))
	if runner != "" && runner != "openai-agent" && runner != "agent-sdk" && runner != "agents-sdk" && runner != "openai-agents-sdk" {
		return workmemory.FlowAgentResult{}, fmt.Errorf("不支持的 flow agent runner: %s", firstNonEmpty(job.Runner, "disabled"))
	}
	provider := strings.TrimSpace(strings.ToLower(job.Provider))
	if provider == "" {
		provider = "openai-compatible"
	}
	if provider != "openai-compatible" && provider != "openai" {
		return workmemory.FlowAgentResult{}, fmt.Errorf("不支持的 AI provider: %s", firstNonEmpty(job.Provider, "disabled"))
	}
	model := strings.TrimSpace(job.Model)
	if model == "" {
		return workmemory.FlowAgentResult{}, errors.New("AI model 未配置")
	}
	apiKey := a.apiKey()
	if apiKey == "" {
		return workmemory.FlowAgentResult{}, errors.New("未检测到 ARIADNE_AI_API_KEY 或 OPENAI_API_KEY")
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
			{Role: "system", Content: flowAgentSystemPrompt()},
			{Role: "user", Content: flowAgentPrompt(job)},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return workmemory.FlowAgentResult{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return workmemory.FlowAgentResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+apiKey)

	client := a.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return workmemory.FlowAgentResult{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024))
	if err != nil {
		return workmemory.FlowAgentResult{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return workmemory.FlowAgentResult{}, fmt.Errorf("AI provider 返回 HTTP %d: %s", response.StatusCode, truncate(string(body), 360))
	}

	var result chatCompletionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return workmemory.FlowAgentResult{}, err
	}
	if len(result.Choices) == 0 {
		return workmemory.FlowAgentResult{}, errors.New("AI provider 未返回 choices")
	}
	content := strings.TrimSpace(result.Choices[0].Message.Content)
	if content == "" {
		return workmemory.FlowAgentResult{}, errors.New("AI provider 返回空内容")
	}
	if looksLikeUnexecutedAgentToolCall(content) {
		return workmemory.FlowAgentResult{}, errors.New("AI provider 返回了未执行的工具调用文本")
	}
	return workmemory.FlowAgentResult{
		Answer:  content,
		Mode:    "agent:openai-compatible-direct",
		Message: flowAgentResultMessage("OpenAI-compatible 直连兼容 runner", len(job.Evidence)),
	}, nil
}

func (a *OpenAICompatibleFlowAgent) apiKey() string {
	envs := a.APIKeyEnv
	if len(envs) == 0 {
		envs = []string{"ARIADNE_AI_API_KEY", "OPENAI__API_KEY", "OPENAI_API_KEY"}
	}
	for _, name := range envs {
		if value := cleanAPIKey(os.Getenv(name)); value != "" {
			return value
		}
	}
	return apiKeyFromCredentialManager(a.SecretTargets)
}

func flowAgentSystemPrompt() string {
	return "你是 Ariadne 内置的通用心流 Agent。你可以通过 Ariadne Flow Memory skill 查询本地时间线、OCR、剪贴板、向量/关键词索引和证据详情；不要依赖硬编码分词或本地模板来替用户下结论。不要编造证据，不要输出静态统计报表，不要要求用户逐条审批。聊天截图常同时包含当前会话、左侧会话列表和后台窗口；回答人物关系时必须区分这些区域，不能把界面上出现过的人名都当成联系人。"
}

func flowAgentPrompt(job workmemory.FlowAgentJob) string {
	now := job.Now
	if now.IsZero() {
		now = time.Now()
	}
	evidence, _ := json.MarshalIndent(job.Evidence, "", "  ")
	return fmt.Sprintf(`用户问题：%s
问题意图：%s
生成时间：%s

本地兜底/种子信息（不能替代工具查询）：
%s

Seed Evidence JSON（可能为空，不能替代工具查询）:
%s

回答要求：
1. 对事实型记忆问题，必须先调用 Ariadne Flow Memory 工具查询；普通寒暄可以不查。
2. 先直接回答用户问题，再补充你依据了哪些线索。
3. 如果问题问“谁找过我”“跟谁聊过”“跟某某聊了什么”，recent/search 的摘要只能用于候选召回，必须再 get 相关 memory 读取 OCR/正文明细；不要只根据 title/summary 下结论。
4. 不要把“沉淀了多少条、跳过多少条”当成主体；那些只能作为辅助背景。
5. 对聊天软件截图，当前会话对象通常来自会话窗口标题或正文中明确的发言上下文；左侧会话列表、群聊列表、服务号、后台窗口和本机发送者只能作为“界面可见”，不能直接列为“我聊过的人”。无法确认时按“疑似/群聊中出现”分组，不要硬凑联系人表。
6. 输出中文 Markdown；有证据时结尾用一行“依据：...”列出最多 6 个 memory id，没有证据时结尾写“依据：本次未命中可引用证据”。`,
		strings.TrimSpace(job.Question),
		strings.TrimSpace(job.Intent),
		now.Format(time.RFC3339),
		strings.TrimSpace(job.LocalAnswer),
		string(evidence),
	)
}

func flowMemoryToolCommand(job workmemory.FlowAgentJob) string {
	if command := strings.TrimSpace(job.ToolCommand); command != "" {
		return command
	}
	if exe, err := os.Executable(); err == nil && strings.TrimSpace(exe) != "" {
		return exe
	}
	return "ariadne"
}

func flowMemorySkillContent() string {
	return `---
name: ariadne-flow-memory
description: Query Ariadne local flow memory, timeline, OCR, clipboard, window context, and evidence details.
---

Use this skill whenever a user asks about what happened, who contacted them, what they worked on, which workflow can be improved, or asks for supporting evidence from Ariadne.

Available CLI:
- ariadne.exe workmemory search --query "<text>" --limit 8 --since-hours 24
- ariadne.exe workmemory recent --limit 8 --since-hours 24
- ariadne.exe workmemory get --id "<memory-id>"

Rules:
- Query the CLI before answering factual questions about the user's day or memory.
- Search multiple times with different focused queries when the first query is too narrow.
- Use get after search when a result needs full text, OCR, frame, or image metadata.
- For contact/chat questions, search/recent summaries are only candidates. Always call get for the candidate memories before naming people.
- Chat screenshots may contain the active conversation, left conversation list, service accounts, group previews, and background apps at the same time. Only name a person as a chat contact when the active conversation title or adjacent message context supports it. Do not treat sidebar names, group member names, or the user's own right-side messages as confirmed contacts.
- Do not expose sensitive entries. The CLI filters sensitive and pending records by default.
- If no evidence is found, answer conversationally but clearly say that Ariadne did not find local evidence.
- Cite memory ids in the final answer when evidence was used.`
}

func flowAgentResultMessage(label string, evidenceCount int) string {
	if evidenceCount <= 0 {
		return label + " 已通过 Ariadne Flow Memory skill 完成动态回答。"
	}
	return label + " 已基于本地检索证据生成回答。"
}

func looksLikeUnexecutedAgentToolCall(answer string) bool {
	text := strings.ToLower(strings.TrimSpace(answer))
	if text == "" {
		return false
	}
	if strings.Contains(text, "<tool_call") || strings.Contains(text, "</tool_call>") {
		return true
	}
	if strings.Contains(text, "<arg_key>") && strings.Contains(text, "<arg_value>") {
		return true
	}
	if (strings.Contains(text, `"tool_call"`) || strings.Contains(text, `"tool_calls"`)) &&
		(strings.Contains(text, `"arguments"`) || strings.Contains(text, `"command"`) || strings.Contains(text, `"name"`)) {
		return true
	}
	return false
}

func (a *OpenAIAgentsSDKFlowAgent) apiKey() string {
	envs := a.APIKeyEnv
	if len(envs) == 0 {
		envs = []string{"ARIADNE_AI_API_KEY", "OPENAI__API_KEY", "OPENAI_API_KEY"}
	}
	for _, name := range envs {
		if value := cleanAPIKey(os.Getenv(name)); value != "" {
			return value
		}
	}
	return apiKeyFromCredentialManager(a.SecretTargets)
}

func writeFlowAgentsSDKBridgeFile() (string, error) {
	sum := sha1.Sum(flowAgentsSDKBridge)
	name := "ariadne-flow-agent-sdk-" + hex.EncodeToString(sum[:6]) + ".py"
	path := filepath.Join(os.TempDir(), name)
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, flowAgentsSDKBridge) {
		return path, nil
	}
	if err := os.WriteFile(path, flowAgentsSDKBridge, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func flowAgentCommandEnv(base []string, apiKey string, baseURL string) []string {
	env := make([]string, 0, len(base)+6)
	skip := map[string]bool{
		"OPENAI_API_KEY":  true,
		"OPENAI_BASE_URL": true,
		"OPENAI_AGENTS_TRACE_INCLUDE_SENSITIVE_DATA": true,
		"PYTHONIOENCODING":                           true,
		"PYTHONUTF8":                                 true,
	}
	for _, item := range base {
		key, _, ok := strings.Cut(item, "=")
		if !ok || skip[strings.ToUpper(key)] {
			continue
		}
		env = append(env, item)
	}
	env = append(env,
		"OPENAI_API_KEY="+apiKey,
		"OPENAI_BASE_URL="+baseURL,
		"OPENAI_AGENTS_TRACE_INCLUDE_SENSITIVE_DATA=0",
		"PYTHONIOENCODING=utf-8",
		"PYTHONUTF8=1",
		"ARIADNE_FLOW_AGENT_BRIDGE=1",
	)
	return env
}

func findFlowAgentPython() string {
	for _, name := range []string{"ARIADNE_FLOW_AGENT_PYTHON", "ARIADNE_AGENTS_PYTHON"} {
		if path := strings.TrimSpace(os.Getenv(name)); path != "" && fileExists(path) {
			return path
		}
	}
	candidates := []string{}
	for _, name := range []string{"ARIADNE_FLOW_AGENT_HOME", "ARIADNE_AGENTS_HOME"} {
		if root := strings.TrimSpace(os.Getenv(name)); root != "" {
			candidates = append(candidates, pythonCandidatesIn(root)...)
		}
	}
	if exe, err := os.Executable(); err == nil && exe != "" {
		exeDir := filepath.Dir(exe)
		for _, rel := range []string{
			filepath.Join("agent-python"),
			filepath.Join("runtime", "agent-python"),
			filepath.Join("runtime", "python"),
			filepath.Join("python"),
		} {
			candidates = append(candidates, pythonCandidatesIn(filepath.Join(exeDir, rel))...)
		}
	}
	if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
		candidates = append(candidates, pythonCandidatesIn(filepath.Join(local, "Ariadne", "agent-python"))...)
		candidates = append(candidates, pythonCandidatesIn(filepath.Join(local, "Ariadne", "runtime", "agent-python"))...)
	}
	if appdata := strings.TrimSpace(os.Getenv("APPDATA")); appdata != "" {
		candidates = append(candidates, pythonCandidatesIn(filepath.Join(appdata, "Ariadne", "agent-python"))...)
	}
	for _, candidate := range candidates {
		if fileExists(candidate) {
			return candidate
		}
	}
	if path, err := exec.LookPath("python"); err == nil && path != "" && !isRepoVenvPython(path) {
		return path
	}
	if allowRepoFlowAgentPython() {
		for _, candidate := range repoPythonCandidates() {
			if fileExists(candidate) {
				return candidate
			}
		}
	}
	return ""
}

func pythonCandidatesIn(root string) []string {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	return []string{
		filepath.Join(root, "python.exe"),
		filepath.Join(root, "Scripts", "python.exe"),
		filepath.Join(root, "bin", "python"),
	}
}

func repoPythonCandidates() []string {
	root := repoRoot()
	if root == "" {
		root, _ = os.Getwd()
	}
	if root == "" {
		return nil
	}
	parent := filepath.Dir(root)
	grandparent := filepath.Dir(parent)
	return []string{
		filepath.Join(root, ".venv", "Scripts", "python.exe"),
		filepath.Join(root, ".venv", "bin", "python"),
		filepath.Join(parent, ".venv", "Scripts", "python.exe"),
		filepath.Join(parent, ".venv", "bin", "python"),
		filepath.Join(grandparent, ".venv", "Scripts", "python.exe"),
		filepath.Join(grandparent, ".venv", "bin", "python"),
	}
}

func allowRepoFlowAgentPython() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv("ARIADNE_FLOW_AGENT_ALLOW_REPO_PYTHON")))
	return value == "1" || value == "true" || value == "yes"
}

func isRepoVenvPython(path string) bool {
	clean := strings.ToLower(filepath.ToSlash(filepath.Clean(path)))
	return strings.Contains(clean, "/.venv/") && (strings.Contains(clean, "/x-tools/") || strings.Contains(clean, "/experiments/ariadne/"))
}

func repoRoot() string {
	wd, err := os.Getwd()
	if err != nil || wd == "" {
		return ""
	}
	dir := wd
	for i := 0; i < 8; i++ {
		if fileExists(filepath.Join(dir, "go.mod")) && strings.EqualFold(filepath.Base(dir), "ariadne") {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
