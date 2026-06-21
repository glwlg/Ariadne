package workmemory

import (
	"sort"
	"strings"
	"time"
)

type textQualityFlowExtension struct{}

type textQualitySuggestion struct {
	Original   string
	Suggested  string
	Reasons    []string
	Confidence float64
}

func (textQualityFlowExtension) Manifest(policy FlowAutonomyPolicy) FlowAutonomyExtensionManifest {
	policy = normalizeFlowAutonomyPolicy(policy)
	return FlowAutonomyExtensionManifest{
		ID:          flowAutonomyExtensionTextQuality,
		Name:        "表达检查",
		Description: "识别沟通文本里的明显重复和标点问题。",
		Enabled:     policy.Enabled && policy.TextQualityAssistEnabled,
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
			flowCandidateActionTextPolishHint,
		},
		ConfirmationPolicy: "confirm_before_user_visible_change",
		TTLSeconds:         int(textQualityTTL(policy).Seconds()),
		CooldownSeconds:    policy.CandidateCooldownMinutes * 60,
	}
}

func (textQualityFlowExtension) BuildCandidates(context flowAutonomyExtensionContext) []FlowCandidateAction {
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
		if entry.CreatedAt > 0 && now.Sub(time.Unix(entry.CreatedAt, 0)) > 24*time.Hour {
			continue
		}
		if !isCommunicationAssistSource(entry) {
			continue
		}
		text := communicationAssistText(entry)
		if len([]rune(text)) < 6 {
			continue
		}
		suggestion := buildTextQualitySuggestion(text)
		if suggestion.Suggested == "" {
			continue
		}
		result = append(result, textQualityCandidate(entry, suggestion, policy, now))
		if len(result) >= 4 {
			break
		}
	}
	return result
}

func textQualityCandidate(entry Entry, suggestion textQualitySuggestion, policy FlowAutonomyPolicy, now time.Time) FlowCandidateAction {
	original := trimTextRunes(strings.Join(strings.Fields(suggestion.Original), " "), 180)
	suggested := trimTextRunes(strings.Join(strings.Fields(suggestion.Suggested), " "), 180)
	summary := "发现明显重复或标点问题。"
	if len(suggestion.Reasons) > 0 {
		summary = strings.Join(suggestion.Reasons, "，")
	}
	key := strings.Join([]string{flowAutonomyExtensionTextQuality, flowCandidateActionTextPolishHint, entry.ID, original, suggested}, ":")
	return FlowCandidateAction{
		ID:                  "flow-action-" + shortHash(key),
		ExtensionID:         flowAutonomyExtensionTextQuality,
		ActionType:          flowCandidateActionTextPolishHint,
		Title:               "表达检查：建议修正",
		Summary:             trimTextRunes(summary, 120),
		Body:                "建议改为：" + suggested,
		Target:              communicationTarget(entry),
		Status:              flowCandidateStatusPending,
		Priority:            "low",
		ConfirmationPolicy:  "confirm",
		NotificationActions: notificationActionsForType(flowCandidateActionTextPolishHint),
		Payload: map[string]string{
			"entryId":       entry.ID,
			"originalText":  original,
			"suggestedText": suggested,
			"copyText":      suggested,
		},
		Evidence:   []string{entry.ID},
		DedupKey:   strings.ToLower(flowAutonomyExtensionTextQuality + ":polish:" + shortHash(key)),
		Source:     firstNonEmpty(entry.Source, "work_memory"),
		Confidence: suggestion.Confidence,
		CreatedAt:  now.Unix(),
		UpdatedAt:  now.Unix(),
		ExpiresAt:  now.Add(textQualityTTL(policy)).Unix(),
	}
}

func buildTextQualitySuggestion(text string) textQualitySuggestion {
	original := strings.TrimSpace(text)
	suggested := original
	reasons := []string{}
	confidence := 0.64

	before := suggested
	suggested = collapseRepeatedPunctuation(suggested)
	if suggested != before {
		reasons = append(reasons, "标点可简化")
		confidence = 0.72
	}

	before = suggested
	for _, replacement := range textQualityPhraseReplacements() {
		suggested = strings.ReplaceAll(suggested, replacement.old, replacement.new)
	}
	if suggested != before {
		reasons = append(reasons, "存在重复表达")
		confidence = 0.7
	}

	suggested = strings.TrimSpace(suggested)
	if suggested == "" || suggested == original {
		return textQualitySuggestion{}
	}
	return textQualitySuggestion{
		Original:   original,
		Suggested:  suggested,
		Reasons:    reasons,
		Confidence: confidence,
	}
}

func collapseRepeatedPunctuation(text string) string {
	replacer := strings.NewReplacer(
		"？？？", "？",
		"？？", "？",
		"???", "?",
		"??", "?",
		"！！！", "！",
		"！！", "！",
		"!!!", "!",
		"!!", "!",
		"。。", "。",
		"，，", "，",
		",,", ",",
	)
	return replacer.Replace(text)
}

func textQualityPhraseReplacements() []struct {
	old string
	new string
} {
	return []struct {
		old string
		new string
	}{
		{old: "看下下", new: "看下"},
		{old: "看一下下", new: "看一下"},
		{old: "确认确认一下", new: "确认一下"},
		{old: "同步同步一下", new: "同步一下"},
		{old: "处理处理一下", new: "处理一下"},
		{old: "检查检查一下", new: "检查一下"},
		{old: "马上马上", new: "马上"},
		{old: "稍后稍后", new: "稍后"},
		{old: "在吗在吗", new: "在吗"},
		{old: "的的", new: "的"},
	}
}

func textQualityTTL(policy FlowAutonomyPolicy) time.Duration {
	hours := policy.CandidateTTLHours
	if hours <= 0 || hours > 2 {
		hours = 2
	}
	return time.Duration(hours) * time.Hour
}
