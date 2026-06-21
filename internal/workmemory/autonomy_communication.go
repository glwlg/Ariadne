package workmemory

import (
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

func (communicationAssistFlowExtension) BuildCandidates(context flowAutonomyExtensionContext) []FlowCandidateAction {
	policy := normalizeFlowAutonomyPolicy(context.Policy)
	now := context.Now
	recent := cloneEntries(context.Entries)
	sort.SliceStable(recent, func(i, j int) bool {
		return recent[i].CreatedAt > recent[j].CreatedAt
	})
	if len(recent) > 40 {
		recent = recent[:40]
	}
	result := []FlowCandidateAction{}
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
		if isLikelyFollowUpCommitment(text) {
			result = append(result, communicationFollowUpCandidate(entry, text, policy, now))
		}
		if isLikelyReplyRequest(text) {
			result = append(result, communicationPrepareReplyCandidate(entry, text, policy, now))
		}
		if len(result) >= 6 {
			break
		}
	}
	return result
}

func communicationFollowUpCandidate(entry Entry, text string, policy FlowAutonomyPolicy, now time.Time) FlowCandidateAction {
	summary := trimTextRunes(strings.Join(strings.Fields(text), " "), 96)
	title := "跟进沟通承诺"
	if summary != "" {
		title = "跟进：" + trimTextRunes(summary, 26)
	}
	key := strings.Join([]string{flowAutonomyExtensionCommunicationAssist, flowCandidateActionFollowUp, entry.ID, summary}, ":")
	return FlowCandidateAction{
		ID:                  "flow-action-" + shortHash(key),
		ExtensionID:         flowAutonomyExtensionCommunicationAssist,
		ActionType:          flowCandidateActionFollowUp,
		Title:               title,
		Summary:             firstNonEmpty(summary, "发现一条可能需要跟进的沟通承诺。"),
		Body:                firstNonEmpty(summary, text),
		Target:              communicationTarget(entry),
		Status:              flowCandidateStatusPending,
		Priority:            "normal",
		ConfirmationPolicy:  "confirm",
		NotificationActions: notificationActionsForType(flowCandidateActionFollowUp),
		Payload: map[string]string{
			"entryId":   entry.ID,
			"todoTitle": title,
		},
		Evidence:   []string{entry.ID},
		DedupKey:   strings.ToLower(flowAutonomyExtensionCommunicationAssist + ":follow_up:" + shortHash(key)),
		Source:     firstNonEmpty(entry.Source, "work_memory"),
		Confidence: 0.62,
		CreatedAt:  now.Unix(),
		UpdatedAt:  now.Unix(),
		ExpiresAt:  now.Add(time.Duration(policy.CandidateTTLHours) * time.Hour).Unix(),
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

func isLikelyFollowUpCommitment(text string) bool {
	compact := strings.ToLower(strings.Join(strings.Fields(text), ""))
	if compact == "" {
		return false
	}
	commitmentHints := []string{
		"我一会", "我待会", "我等下", "我稍后", "我晚点", "我回头", "我明天",
		"我来处理", "我处理", "我确认", "我看看", "我查一下", "我发你", "我给你",
		"等我", "稍后给", "晚点给", "一会给", "回头给",
	}
	for _, hint := range commitmentHints {
		if strings.Contains(compact, hint) {
			return true
		}
	}
	return false
}

func isLikelyReplyRequest(text string) bool {
	compact := strings.ToLower(strings.Join(strings.Fields(text), ""))
	if strings.ContainsAny(text, "?？") {
		return true
	}
	requestHints := []string{
		"请问", "麻烦", "帮忙", "能否", "可以帮", "看一下", "确认一下", "同步一下", "回复一下",
		"please", "couldyou", "canyou", "wouldyou",
	}
	for _, hint := range requestHints {
		if strings.Contains(compact, hint) {
			return true
		}
	}
	return false
}
