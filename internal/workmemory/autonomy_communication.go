package workmemory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

type communicationAssistFlowExtension struct{}

func (communicationAssistFlowExtension) Manifest(policy FlowAutonomyPolicy) FlowAutonomyExtensionManifest {
	policy = normalizeFlowAutonomyPolicy(policy)
	return FlowAutonomyExtensionManifest{
		ID:          flowAutonomyExtensionCommunicationAssist,
		Name:        "沟通辅助",
		Description: "从沟通留痕、OCR 和剪贴板生成待确认动作。",
		Enabled:     policy.Enabled && policy.CommunicationAssistEnabled,
		EventSources: []string{
			"work_memory.entry_upserted",
			"clipboard.entry",
			"ocr.text_ready",
		},
		ReadScopes: []string{
			"communication_window_trace",
			"clipboard_text",
			"ocr_text",
		},
		ActionTypes: []string{
			flowCandidateActionPrepareReply,
			flowCandidateActionFollowUp,
			flowCandidateActionFactCheckWarning,
		},
		ConfirmationPolicy: "confirm_before_user_visible_change",
		TTLSeconds:         policy.CandidateTTLHours * 3600,
		CooldownSeconds:    policy.CandidateCooldownMinutes * 60,
	}
}

func (communicationAssistFlowExtension) BuildCandidates(extensionContext flowAutonomyExtensionContext) []FlowCandidateAction {
	policy := normalizeFlowAutonomyPolicy(extensionContext.Policy)
	agentPolicy := normalizeFlowAgentPolicy(extensionContext.AgentPolicy)
	now := extensionContext.Now
	recent := cloneEntries(extensionContext.Entries)
	sort.SliceStable(recent, func(i, j int) bool {
		return recent[i].CreatedAt > recent[j].CreatedAt
	})
	if len(recent) > 40 {
		recent = recent[:40]
	}
	if extensionContext.Analyzer == nil || !agentPolicy.Enabled {
		return nil
	}
	entriesByID := map[string]Entry{}
	evidenceEntries := []Entry{}
	for _, entry := range recent {
		if entry.CreatedAt > 0 && now.Sub(time.Unix(entry.CreatedAt, 0)) > 48*time.Hour {
			continue
		}
		if !isCommunicationAssistSource(entry) {
			continue
		}
		text := communicationAssistText(entry)
		if len([]rune(text)) < 6 {
			continue
		}
		entriesByID[entry.ID] = entry
		evidenceEntries = append(evidenceEntries, entry)
		if len(evidenceEntries) >= 16 {
			break
		}
	}
	if len(evidenceEntries) == 0 {
		return nil
	}
	agentCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	analysis, err := extensionContext.Analyzer.AnalyzeFlowAutonomy(agentCtx, FlowAutonomyAnalysisJob{
		ExtensionID:  flowAutonomyExtensionCommunicationAssist,
		Evidence:     flowAgentEvidenceFromEntries(evidenceEntries, 16),
		Runner:       agentPolicy.Runner,
		Provider:     agentPolicy.Provider,
		BaseURL:      agentPolicy.BaseURL,
		Model:        agentPolicy.Model,
		NativeSkills: agentPolicy.NativeSkills,
		Now:          now,
	})
	if err != nil {
		return nil
	}
	return communicationCandidatesFromSuggestions(analysis.Suggestions, entriesByID, policy, now)
}

func communicationCandidatesFromSuggestions(suggestions []FlowAutonomySuggestion, entriesByID map[string]Entry, policy FlowAutonomyPolicy, now time.Time) []FlowCandidateAction {
	result := []FlowCandidateAction{}
	for _, suggestion := range suggestions {
		actionType := normalizeFlowCandidateActionType(suggestion.ActionType)
		switch actionType {
		case flowCandidateActionFollowUp:
		default:
			continue
		}
		entry, ok := entriesByID[strings.TrimSpace(suggestion.EntryID)]
		if !ok {
			continue
		}
		result = append(result, communicationFollowUpCandidateFromSuggestion(entry, suggestion, policy, now))
		if len(result) >= 6 {
			break
		}
	}
	return result
}

func communicationFollowUpCandidateFromSuggestion(entry Entry, suggestion FlowAutonomySuggestion, policy FlowAutonomyPolicy, now time.Time) FlowCandidateAction {
	summary := trimTextRunes(strings.Join(strings.Fields(firstNonEmpty(suggestion.Summary, suggestion.Body, suggestion.Title)), " "), 96)
	title := trimTextRunes(strings.Join(strings.Fields(suggestion.Title), " "), 80)
	if title == "" {
		title = "跟进沟通承诺"
		if summary != "" {
			title = "跟进：" + trimTextRunes(summary, 26)
		}
	}
	body := strings.TrimSpace(firstNonEmpty(suggestion.Body, summary))
	payload := map[string]string{
		"entryId":   entry.ID,
		"todoTitle": title,
	}
	for key, value := range suggestion.Payload {
		if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
			payload[key] = value
		}
	}
	evidenceIDs := cleanStrings(append([]string{entry.ID}, suggestion.EvidenceIDs...))
	key := strings.Join([]string{flowAutonomyExtensionCommunicationAssist, flowCandidateActionFollowUp, entry.ID, summary, title}, ":")
	return FlowCandidateAction{
		ID:                  "flow-action-" + shortHash(key),
		ExtensionID:         flowAutonomyExtensionCommunicationAssist,
		ActionType:          flowCandidateActionFollowUp,
		Title:               title,
		Summary:             firstNonEmpty(summary, "发现一条可能需要跟进的沟通承诺。"),
		Body:                firstNonEmpty(body, summary),
		Target:              firstNonEmpty(suggestion.Target, communicationTarget(entry)),
		Status:              flowCandidateStatusPending,
		Priority:            firstNonEmpty(suggestion.Priority, "normal"),
		ConfirmationPolicy:  "confirm",
		NotificationActions: notificationActionsForType(flowCandidateActionFollowUp),
		Payload:             payload,
		Evidence:            evidenceIDs,
		DedupKey:            strings.ToLower(flowAutonomyExtensionCommunicationAssist + ":follow_up:" + shortHash(key)),
		Source:              firstNonEmpty(entry.Source, "work_memory"),
		Confidence:          suggestion.Confidence,
		CreatedAt:           now.Unix(),
		UpdatedAt:           now.Unix(),
		ExpiresAt:           now.Add(time.Duration(policy.CandidateTTLHours) * time.Hour).Unix(),
	}
}

func communicationPrepareReplyCandidate(entry Entry, text string, policy FlowAutonomyPolicy, now time.Time) FlowCandidateAction {
	summary := trimTextRunes(strings.Join(strings.Fields(text), " "), 120)
	title := "准备回复"
	if target := communicationTarget(entry); target != "" {
		title = "回复：" + trimTextRunes(target, 24)
	}
	body := renderPrepareReplyBody(entry, summary)
	key := strings.Join([]string{flowAutonomyExtensionCommunicationAssist, flowCandidateActionPrepareReply, entry.ID, summary}, ":")
	return FlowCandidateAction{
		ID:                  "flow-action-" + shortHash(key),
		ExtensionID:         flowAutonomyExtensionCommunicationAssist,
		ActionType:          flowCandidateActionPrepareReply,
		Title:               title,
		Summary:             firstNonEmpty(summary, "发现一条可能需要回复的沟通消息。"),
		Body:                body,
		Target:              communicationTarget(entry),
		Status:              flowCandidateStatusPending,
		Priority:            "normal",
		ConfirmationPolicy:  "confirm",
		NotificationActions: notificationActionsForType(flowCandidateActionPrepareReply),
		Payload: map[string]string{
			"entryId":   entry.ID,
			"copyText":  body,
			"sourceApp": entry.AppName,
		},
		Evidence:   []string{entry.ID},
		DedupKey:   strings.ToLower(flowAutonomyExtensionCommunicationAssist + ":prepare_reply:" + shortHash(key)),
		Source:     firstNonEmpty(entry.Source, "work_memory"),
		Confidence: 0.58,
		CreatedAt:  now.Unix(),
		UpdatedAt:  now.Unix(),
		ExpiresAt:  now.Add(2 * time.Hour).Unix(),
	}
}

func renderPrepareReplyBody(entry Entry, summary string) string {
	target := communicationTarget(entry)
	if target == "" {
		target = "对方"
	}
	if summary == "" {
		summary = firstNonEmpty(entry.Summary, entry.Title)
	}
	return strings.TrimSpace(fmt.Sprintf("%s，我看到了。%s", target, prepareReplyContext(summary)))
}

func prepareReplyContext(summary string) string {
	summary = trimTextRunes(strings.Join(strings.Fields(summary), " "), 80)
	if summary == "" {
		return "我确认后回复你。"
	}
	if strings.ContainsAny(summary, "?？") {
		return "我先确认一下，再给你准确答复。"
	}
	return "我会处理并同步进展。"
}

func isCommunicationAssistSource(entry Entry) bool {
	if strings.EqualFold(entry.Source, "clipboard") {
		return true
	}
	haystack := strings.ToLower(strings.Join([]string{entry.AppName, entry.WindowTitle, entry.Title, entry.Summary}, " "))
	keywords := []string{
		"wechat", "weixin", "wxwork", "wecom", "lark", "feishu", "飞书", "钉钉", "dingtalk",
		"slack", "teams", "telegram", "discord", "outlook", "mail", "邮件", "微信", "企业微信",
	}
	for _, keyword := range keywords {
		if strings.Contains(haystack, keyword) {
			return true
		}
	}
	return false
}

func communicationAssistText(entry Entry) string {
	return firstNonEmpty(entry.QualityOCRText, entry.OCRText, entry.Text, entry.Summary, entry.Title)
}

func communicationTarget(entry Entry) string {
	value := firstNonEmpty(entry.WindowTitle, entry.AppName)
	value = strings.TrimSpace(strings.Join(strings.Fields(value), " "))
	value = strings.Trim(value, "-—| ")
	return trimTextRunes(value, 32)
}
