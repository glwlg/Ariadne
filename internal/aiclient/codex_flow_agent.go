package aiclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"ariadne/internal/workmemory"
)

type CodexFlowAgent struct {
	Command string
	Timeout time.Duration
	WorkDir string
}

func NewCodexFlowAgent() *CodexFlowAgent {
	return &CodexFlowAgent{
		Command: "codex",
		Timeout: 90 * time.Second,
	}
}

func (a *CodexFlowAgent) AnswerFlow(ctx context.Context, job workmemory.FlowAgentJob) (workmemory.FlowAgentResult, error) {
	runner := strings.TrimSpace(strings.ToLower(job.Runner))
	if runner != "codex" {
		return workmemory.FlowAgentResult{}, fmt.Errorf("不支持的 flow agent runner: %s", firstNonEmpty(job.Runner, "disabled"))
	}
	command := strings.TrimSpace(a.Command)
	if command == "" {
		command = "codex"
	}
	timeout := a.Timeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	workDir := firstNonEmpty(job.WorkDir, a.WorkDir, filepath.Join(os.TempDir(), "ariadne-flow-agent"))
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return workmemory.FlowAgentResult{}, fmt.Errorf("创建 Codex agent 工作目录失败: %w", err)
	}
	outputFile := filepath.Join(workDir, fmt.Sprintf("flow-answer-%d.md", time.Now().UnixNano()))
	defer os.Remove(outputFile)

	args := []string{
		"exec",
		"--ignore-user-config",
		"--ephemeral",
		"--skip-git-repo-check",
		"--sandbox", "read-only",
		"--config", "approval_policy=\"never\"",
		"--color", "never",
		"--output-last-message", outputFile,
	}
	if model := strings.TrimSpace(job.Model); model != "" {
		args = append(args, "--model", model)
	}
	args = append(args, "-")

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	cmd.Stdin = strings.NewReader(codexFlowPrompt(job))
	configureAgentCommand(cmd)

	rawOutput, err := cmd.CombinedOutput()
	answer := strings.TrimSpace(readTextFile(outputFile))
	if err != nil {
		if ctx.Err() != nil {
			return workmemory.FlowAgentResult{}, errors.New("Codex agent 执行超时")
		}
		detail := truncate(strings.TrimSpace(string(rawOutput)), 360)
		if detail == "" {
			detail = err.Error()
		}
		return workmemory.FlowAgentResult{}, fmt.Errorf("Codex agent 执行失败: %s", detail)
	}
	if answer == "" {
		return workmemory.FlowAgentResult{}, errors.New("Codex agent 未返回回答")
	}
	return workmemory.FlowAgentResult{
		Answer:  answer,
		Mode:    "agent:codex",
		Message: codexFlowAgentResultMessage(len(job.Evidence)),
	}, nil
}

func codexFlowPrompt(job workmemory.FlowAgentJob) string {
	now := job.Now
	if now.IsZero() {
		now = time.Now()
	}
	evidence, _ := json.MarshalIndent(job.Evidence, "", "  ")
	return fmt.Sprintf(`你是 Ariadne 的“心流”代理。你的任务是根据用户的本地工作记忆证据，生成一个有判断、有上下文的中文回答。

边界：
1. 只使用下面给出的 Evidence JSON 和本地兜底摘要，不要访问文件、运行命令、联网、读取系统环境或控制桌面。
2. Evidence 已经过 Ariadne 过滤；仍然不要复述 token、密码、密钥、隐私内容。如果证据为空或不足，普通寒暄可以正常回应，涉及记忆事实的问题要直接说明不足。
3. 不要把回答写成静态统计报表。你要像一个理解上下文的个人助理，归纳主线、解释原因、指出可能的下一步。
4. 可以主动发现“适合沉淀为工作流、Skill、清单、复盘”的机会，但不要让用户逐条审批；只有涉及执行或外部协作时才提示需要确认。
5. 输出纯中文 Markdown，不要代码块，不要 JSON，不要前言，不要提到你是 Codex。
6. 聊天原文里的“你/我”属于消息发言人的视角，不要自动换成当前用户。只有当前证据明确显示该消息由当前用户发出，才可写“你说/你提出”；只有证据明确显示当前用户被 @、点名或被当前会话上下文指向，才可写成用户待办。否则写“群聊中有人提到”，并标注指代不明。
7. 结尾用一行“依据：...”列出最多 6 个关键 evidence id。

用户问题：%s
问题意图：%s
生成时间：%s

本地兜底摘要：
%s

Self Model（只包含已确认且允许进入模型上下文的低敏断言；用于理解“我”的身份和偏好，不能覆盖当前证据）:
%s

Evidence JSON:
%s`,
		strings.TrimSpace(job.Question),
		strings.TrimSpace(job.Intent),
		now.Format(time.RFC3339),
		strings.TrimSpace(job.LocalAnswer),
		firstNonEmpty(strings.TrimSpace(job.SelfModel), "No confirmed low-risk Self Model assertions are available."),
		string(evidence),
	)
}

func codexFlowAgentResultMessage(evidenceCount int) string {
	if evidenceCount <= 0 {
		return "Codex agent 已通过 Ariadne Flow Memory skill 完成动态回答。"
	}
	return "Codex agent 已基于本地证据生成回答。"
}

func readTextFile(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 1024*1024))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}
