package workmemory

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	flowAutonomyExtensionCommunicationAssist = "flow.communication_assist"
	flowAutonomyExtensionTextQuality         = "flow.text_quality"

	flowCandidateStatusPending         = "pending"
	flowCandidateStatusAccepted        = "accepted"
	flowCandidateStatusSnoozed         = "snoozed"
	flowCandidateStatusIgnored         = "ignored"
	flowCandidateStatusExpired         = "expired"
	flowCandidateStatusDismissedByRule = "dismissed_by_rule"
	flowCandidateStatusExecuted        = "executed"
	flowCandidateStatusFailed          = "failed"

	flowCandidateActionPrepareReply     = "prepare_reply"
	flowCandidateActionFollowUp         = "follow_up_candidate"
	flowCandidateActionFactCheckWarning = "fact_check_warning"
	flowCandidateActionTextPolishHint   = "text_polish_hint"
)

type FlowAutonomyPolicy struct {
	Enabled                      bool `json:"enabled"`
	CommunicationAssistEnabled   bool `json:"communicationAssistEnabled"`
	TextQualityAssistEnabled     bool `json:"textQualityAssistEnabled"`
	CandidateTTLHours            int  `json:"candidateTtlHours"`
	CandidateCooldownMinutes     int  `json:"candidateCooldownMinutes"`
	DefaultSnoozeMinutes         int  `json:"defaultSnoozeMinutes"`
	NotifyLowRiskAutomaticAction bool `json:"notifyLowRiskAutomaticAction"`
}

type FlowAutonomyStatus struct {
	Enabled                  bool                            `json:"enabled"`
	PrivacyMode              bool                            `json:"privacyMode"`
	LastRunAt                int64                           `json:"lastRunAt,omitempty"`
	LastMessage              string                          `json:"lastMessage,omitempty"`
	Pending                  int                             `json:"pending"`
	Snoozed                  int                             `json:"snoozed"`
	Expired                  int                             `json:"expired"`
	Executed                 int                             `json:"executed"`
	Extensions               []FlowAutonomyExtensionManifest `json:"extensions"`
	NotifyLowRiskAutomatic   bool                            `json:"notifyLowRiskAutomatic"`
	CandidateTTLHours        int                             `json:"candidateTtlHours"`
	CandidateCooldownMinutes int                             `json:"candidateCooldownMinutes"`
	DefaultSnoozeMinutes     int                             `json:"defaultSnoozeMinutes"`
	UpdatedAt                int64                           `json:"updatedAt"`
}

type FlowAutonomyExtensionManifest struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	Enabled            bool     `json:"enabled"`
	EventSources       []string `json:"eventSources"`
	ReadScopes         []string `json:"readScopes"`
	ActionTypes        []string `json:"actionTypes"`
	ConfirmationPolicy string   `json:"confirmationPolicy"`
	TTLSeconds         int      `json:"ttlSeconds"`
	CooldownSeconds    int      `json:"cooldownSeconds"`
}

type FlowNotificationAction struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
}

type FlowCandidateAction struct {
	ID                  string                   `json:"id"`
	ExtensionID         string                   `json:"extensionId"`
	ActionType          string                   `json:"actionType"`
	Title               string                   `json:"title"`
	Summary             string                   `json:"summary"`
	Body                string                   `json:"body"`
	Target              string                   `json:"target,omitempty"`
	Status              string                   `json:"status"`
	Priority            string                   `json:"priority"`
	ConfirmationPolicy  string                   `json:"confirmationPolicy"`
	NotificationActions []FlowNotificationAction `json:"notificationActions"`
	Payload             map[string]string        `json:"payload,omitempty"`
	Evidence            []string                 `json:"evidence"`
	DedupKey            string                   `json:"dedupKey,omitempty"`
	Source              string                   `json:"source,omitempty"`
	DecisionActionID    string                   `json:"decisionActionId,omitempty"`
	DecisionReason      string                   `json:"decisionReason,omitempty"`
	Confidence          float64                  `json:"confidence,omitempty"`
	CreatedAt           int64                    `json:"createdAt"`
	UpdatedAt           int64                    `json:"updatedAt,omitempty"`
	ExpiresAt           int64                    `json:"expiresAt,omitempty"`
	SnoozedUntil        int64                    `json:"snoozedUntil,omitempty"`
	DecidedAt           int64                    `json:"decidedAt,omitempty"`
	ExecutedAt          int64                    `json:"executedAt,omitempty"`
}

type FlowCandidateActionListRequest struct {
	Status         string `json:"status,omitempty"`
	IncludeExpired bool   `json:"includeExpired,omitempty"`
	Limit          int    `json:"limit,omitempty"`
}

type FlowCandidateActionList struct {
	Items     []FlowCandidateAction `json:"items"`
	Pending   int                   `json:"pending"`
	Snoozed   int                   `json:"snoozed"`
	Accepted  int                   `json:"accepted"`
	Ignored   int                   `json:"ignored"`
	Expired   int                   `json:"expired"`
	Executed  int                   `json:"executed"`
	Failed    int                   `json:"failed"`
	UpdatedAt int64                 `json:"updatedAt"`
}

type FlowCandidateActionDecisionRequest struct {
	ID            string `json:"id"`
	ActionID      string `json:"actionId,omitempty"`
	Decision      string `json:"decision,omitempty"`
	Reason        string `json:"reason,omitempty"`
	SnoozeMinutes int    `json:"snoozeMinutes,omitempty"`
}

type FlowCandidateActionDecisionResult struct {
	OK      bool                    `json:"ok"`
	Message string                  `json:"message"`
	Action  FlowCandidateAction     `json:"action,omitempty"`
	List    FlowCandidateActionList `json:"list"`
}

type FlowAutonomyRunResult struct {
	OK        bool                  `json:"ok"`
	Message   string                `json:"message"`
	Generated int                   `json:"generated"`
	Skipped   int                   `json:"skipped"`
	Expired   int                   `json:"expired"`
	Actions   []FlowCandidateAction `json:"actions"`
	Status    FlowAutonomyStatus    `json:"status"`
	CreatedAt int64                 `json:"createdAt"`
}

type flowAutonomyExtension interface {
	Manifest(FlowAutonomyPolicy) FlowAutonomyExtensionManifest
	BuildCandidates(flowAutonomyExtensionContext) []FlowCandidateAction
}

type flowAutonomyExtensionContext struct {
	Entries []Entry
	Policy  FlowAutonomyPolicy
	Now     time.Time
}

func defaultFlowAutonomyPolicy() FlowAutonomyPolicy {
	return FlowAutonomyPolicy{
		Enabled:                      true,
		CommunicationAssistEnabled:   true,
		TextQualityAssistEnabled:     true,
		CandidateTTLHours:            8,
		CandidateCooldownMinutes:     15,
		DefaultSnoozeMinutes:         30,
		NotifyLowRiskAutomaticAction: false,
	}
}

func (s *Service) ApplyFlowAutonomyPolicy(policy FlowAutonomyPolicy) FlowAutonomyStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flowAutonomyPolicy = normalizeFlowAutonomyPolicy(policy)
	s.settleFlowCandidateActionsLocked(s.now())
	return s.flowAutonomyStatusLocked()
}

func (s *Service) FlowAutonomyStatus() FlowAutonomyStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := s.settleFlowCandidateActionsLocked(s.now())
	if changed > 0 {
		s.saveLockedWithStatus()
	}
	return s.flowAutonomyStatusLocked()
}

func (s *Service) FlowCandidateActions(request FlowCandidateActionListRequest) FlowCandidateActionList {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := s.settleFlowCandidateActionsLocked(s.now())
	if changed > 0 {
		s.saveLockedWithStatus()
	}
	return buildFlowCandidateActionListLocked(s.flowCandidateActions, request, s.now().Unix())
}

func (s *Service) RunFlowAutonomyNow() FlowAutonomyRunResult {
	s.mu.Lock()
	result := s.runFlowAutonomyLocked(true)
	s.scheduledDrafts.AutonomousMessage = result.Message
	if result.OK || result.Expired > 0 {
		s.saveLockedWithStatus()
	}
	result.Status = s.flowAutonomyStatusLocked()
	generated := cloneFlowCandidateActions(result.Actions)
	s.mu.Unlock()
	s.notifyFlowCandidateActions("flow_candidate_created", generated)
	return result
}

func (s *Service) DecideFlowCandidateAction(request FlowCandidateActionDecisionRequest) FlowCandidateActionDecisionResult {
	id := strings.TrimSpace(request.ID)
	if id == "" {
		return FlowCandidateActionDecisionResult{OK: false, Message: "缺少候选动作 ID", List: s.FlowCandidateActions(FlowCandidateActionListRequest{})}
	}
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settleFlowCandidateActionsLocked(now)
	for index := range s.flowCandidateActions {
		if s.flowCandidateActions[index].ID != id {
			continue
		}
		action := s.flowCandidateActions[index]
		if !flowCandidateActionDecisionAllowed(action.Status) {
			return FlowCandidateActionDecisionResult{
				OK:      false,
				Message: "该动作已处理",
				Action:  action,
				List:    buildFlowCandidateActionListLocked(s.flowCandidateActions, FlowCandidateActionListRequest{}, now.Unix()),
			}
		}
		decision := normalizeFlowCandidateDecision(request.Decision, request.ActionID)
		action.DecisionActionID = strings.TrimSpace(request.ActionID)
		action.DecisionReason = strings.TrimSpace(request.Reason)
		action.DecidedAt = now.Unix()
		action.UpdatedAt = now.Unix()
		message := "已记录处理结果"
		switch decision {
		case flowCandidateStatusSnoozed:
			minutes := request.SnoozeMinutes
			if minutes <= 0 {
				minutes = normalizeFlowAutonomyPolicy(s.flowAutonomyPolicy).DefaultSnoozeMinutes
			}
			action.Status = flowCandidateStatusSnoozed
			action.SnoozedUntil = now.Add(time.Duration(minutes) * time.Minute).Unix()
			message = "已稍后提醒"
		case flowCandidateStatusIgnored:
			action.Status = flowCandidateStatusIgnored
			message = "已忽略"
		case flowCandidateStatusDismissedByRule:
			action.Status = flowCandidateStatusDismissedByRule
			message = "已按规则关闭"
		case flowCandidateStatusFailed:
			action.Status = flowCandidateStatusFailed
			message = "已标记失败"
		default:
			action.Status = flowCandidateStatusAccepted
			message = "已接受"
			if executed, executeMessage := s.executeFlowCandidateActionLocked(&action, now); executed {
				action.Status = flowCandidateStatusExecuted
				action.ExecutedAt = now.Unix()
				message = executeMessage
			}
		}
		s.flowCandidateActions[index] = normalizeFlowCandidateAction(action, now)
		if err := s.saveLocked(); err != nil {
			s.saveError = err.Error()
			return FlowCandidateActionDecisionResult{
				OK:      false,
				Message: err.Error(),
				Action:  s.flowCandidateActions[index],
				List:    buildFlowCandidateActionListLocked(s.flowCandidateActions, FlowCandidateActionListRequest{}, now.Unix()),
			}
		}
		s.notifyChangeObservers(s.changeEventForFlowCandidateLocked("flow_candidate_decided", s.flowCandidateActions[index]))
		return FlowCandidateActionDecisionResult{
			OK:      true,
			Message: message,
			Action:  s.flowCandidateActions[index],
			List:    buildFlowCandidateActionListLocked(s.flowCandidateActions, FlowCandidateActionListRequest{}, now.Unix()),
		}
	}
	return FlowCandidateActionDecisionResult{
		OK:      false,
		Message: "未找到候选动作",
		List:    buildFlowCandidateActionListLocked(s.flowCandidateActions, FlowCandidateActionListRequest{}, now.Unix()),
	}
}

func (s *Service) maybeRunFlowAutonomyForNewEntryLocked(entry Entry) FlowAutonomyRunResult {
	if !entryUsableForExtraction(entry) {
		return FlowAutonomyRunResult{}
	}
	result := s.runFlowAutonomyLocked(false)
	if result.Generated > 0 || result.Expired > 0 {
		s.scheduledDrafts.AutonomousMessage = result.Message
	}
	return result
}

func (s *Service) notifyFlowCandidateActions(kind string, actions []FlowCandidateAction) {
	if len(actions) == 0 {
		return
	}
	s.mu.RLock()
	events := make([]ChangeEvent, 0, len(actions))
	for _, action := range actions {
		events = append(events, s.changeEventForFlowCandidateLocked(kind, action))
	}
	s.mu.RUnlock()
	for _, event := range events {
		s.notifyChangeObservers(event)
	}
}

func (s *Service) runFlowAutonomyLocked(force bool) FlowAutonomyRunResult {
	now := s.now()
	result := FlowAutonomyRunResult{OK: false, CreatedAt: now.Unix()}
	result.Expired = s.settleFlowCandidateActionsLocked(now)
	policy := normalizeFlowAutonomyPolicy(s.flowAutonomyPolicy)
	if !s.status.Enabled {
		result.Message = "工作记忆已停用"
		return result
	}
	if s.status.PrivacyMode {
		result.Message = "隐私模式已开启，主动动作暂停"
		return result
	}
	if !policy.Enabled {
		result.Message = "主动动作已关闭"
		return result
	}
	if !force && s.lastFlowAutonomyRunAt > 0 {
		nextAllowed := time.Unix(s.lastFlowAutonomyRunAt, 0).Add(time.Duration(policy.CandidateCooldownMinutes) * time.Minute)
		if now.Before(nextAllowed) {
			result.Message = "主动动作扫描冷却中"
			return result
		}
	}
	usable := make([]Entry, 0, len(s.entries))
	for _, entry := range s.entries {
		if entryUsableForExtraction(entry) {
			usable = append(usable, entry)
		}
	}
	if len(usable) == 0 {
		result.Message = "没有可用于主动动作的本地留痕"
		return result
	}
	existingKeys := activeFlowCandidateKeys(s.flowCandidateActions, now)
	add := func(action FlowCandidateAction) {
		action = normalizeFlowCandidateAction(action, now)
		if action.ID == "" || action.DedupKey == "" {
			result.Skipped++
			return
		}
		if existingKeys[action.DedupKey] {
			result.Skipped++
			return
		}
		existingKeys[action.DedupKey] = true
		s.flowCandidateActions = append([]FlowCandidateAction{action}, s.flowCandidateActions...)
		result.Actions = append(result.Actions, action)
	}
	for _, extension := range flowAutonomyExtensions() {
		manifest := extension.Manifest(policy)
		if !manifest.Enabled {
			continue
		}
		for _, action := range extension.BuildCandidates(flowAutonomyExtensionContext{Entries: usable, Policy: policy, Now: now}) {
			add(action)
		}
	}
	s.lastFlowAutonomyRunAt = now.Unix()
	result.OK = true
	result.Generated = len(result.Actions)
	if result.Generated == 0 {
		result.Message = "已检查主动动作，暂无待确认项"
	} else {
		result.Message = fmt.Sprintf("生成 %d 个待确认动作", result.Generated)
	}
	return result
}

func (s *Service) executeFlowCandidateActionLocked(action *FlowCandidateAction, now time.Time) (bool, string) {
	if action == nil {
		return false, ""
	}
	switch action.ActionType {
	case flowCandidateActionFollowUp:
		if action.DecisionActionID != "add" {
			return false, ""
		}
		title := firstNonEmpty(action.Payload["todoTitle"], action.Title)
		if title == "" {
			return false, ""
		}
		for _, existing := range s.todoItems {
			if existing.Source == "flow_autonomy" && strings.EqualFold(existing.Title, title) && !isTodoClosed(existing.Status) {
				return true, "已存在待办"
			}
		}
		item := normalizeTodoItem(TodoItem{
			ID:        fmt.Sprintf("todo-%d-%s", now.UnixNano(), shortHash(action.DedupKey)),
			Title:     title,
			Note:      firstNonEmpty(action.Body, action.Summary),
			Status:    todoStatusOpen,
			Priority:  todoPriorityNormal,
			Scope:     firstNonEmpty(action.Target, "沟通跟进"),
			Source:    "flow_autonomy",
			Evidence:  action.Evidence,
			CreatedAt: now.Unix(),
			UpdatedAt: now.Unix(),
		}, now.Unix())
		s.todoItems = append(s.todoItems, item)
		sortTodoItems(s.todoItems)
		return true, "已添加待办"
	default:
		return false, ""
	}
}

func (s *Service) flowAutonomyStatusLocked() FlowAutonomyStatus {
	now := s.now().Unix()
	policy := normalizeFlowAutonomyPolicy(s.flowAutonomyPolicy)
	list := buildFlowCandidateActionListLocked(s.flowCandidateActions, FlowCandidateActionListRequest{IncludeExpired: true}, now)
	return FlowAutonomyStatus{
		Enabled:                  policy.Enabled && s.status.Enabled && !s.status.PrivacyMode,
		PrivacyMode:              s.status.PrivacyMode,
		LastRunAt:                s.lastFlowAutonomyRunAt,
		LastMessage:              s.scheduledDrafts.AutonomousMessage,
		Pending:                  list.Pending,
		Snoozed:                  list.Snoozed,
		Expired:                  list.Expired,
		Executed:                 list.Executed,
		Extensions:               flowAutonomyExtensionManifests(policy),
		NotifyLowRiskAutomatic:   policy.NotifyLowRiskAutomaticAction,
		CandidateTTLHours:        policy.CandidateTTLHours,
		CandidateCooldownMinutes: policy.CandidateCooldownMinutes,
		DefaultSnoozeMinutes:     policy.DefaultSnoozeMinutes,
		UpdatedAt:                now,
	}
}

func normalizeFlowAutonomyPolicy(policy FlowAutonomyPolicy) FlowAutonomyPolicy {
	defaults := defaultFlowAutonomyPolicy()
	policy.CandidateTTLHours = clampInt(policy.CandidateTTLHours, 1, 168, defaults.CandidateTTLHours)
	policy.CandidateCooldownMinutes = clampInt(policy.CandidateCooldownMinutes, 1, 1440, defaults.CandidateCooldownMinutes)
	policy.DefaultSnoozeMinutes = clampInt(policy.DefaultSnoozeMinutes, 5, 1440, defaults.DefaultSnoozeMinutes)
	return policy
}

func flowAutonomyExtensionManifests(policy FlowAutonomyPolicy) []FlowAutonomyExtensionManifest {
	policy = normalizeFlowAutonomyPolicy(policy)
	extensions := flowAutonomyExtensions()
	manifests := make([]FlowAutonomyExtensionManifest, 0, len(extensions))
	for _, extension := range extensions {
		manifests = append(manifests, extension.Manifest(policy))
	}
	return manifests
}

func notificationActionsForType(actionType string) []FlowNotificationAction {
	switch actionType {
	case flowCandidateActionPrepareReply:
		return []FlowNotificationAction{
			{ID: "copy", Label: "复制", Kind: "primary"},
			{ID: "open", Label: "打开", Kind: "secondary"},
			{ID: "ignore", Label: "忽略", Kind: "quiet"},
		}
	case flowCandidateActionFollowUp:
		return []FlowNotificationAction{
			{ID: "add", Label: "添加", Kind: "primary"},
			{ID: "later", Label: "稍后", Kind: "secondary"},
			{ID: "ignore", Label: "忽略", Kind: "quiet"},
		}
	case flowCandidateActionFactCheckWarning:
		return []FlowNotificationAction{
			{ID: "view", Label: "查看", Kind: "primary"},
			{ID: "ignore", Label: "仍然忽略", Kind: "quiet"},
			{ID: "dismiss_rule", Label: "不再提示此类", Kind: "quiet"},
		}
	case flowCandidateActionTextPolishHint:
		return []FlowNotificationAction{
			{ID: "copy_revision", Label: "复制修正版", Kind: "primary"},
			{ID: "view", Label: "查看", Kind: "secondary"},
			{ID: "ignore", Label: "忽略", Kind: "quiet"},
		}
	default:
		return []FlowNotificationAction{{ID: "open_flow", Label: "打开心流", Kind: "secondary"}}
	}
}

func normalizeFlowCandidateAction(action FlowCandidateAction, now time.Time) FlowCandidateAction {
	action.ID = strings.TrimSpace(action.ID)
	action.ExtensionID = strings.TrimSpace(strings.ToLower(action.ExtensionID))
	action.ActionType = normalizeFlowCandidateActionType(action.ActionType)
	action.Title = trimTextRunes(strings.Join(strings.Fields(strings.TrimSpace(action.Title)), " "), 80)
	action.Summary = trimTextRunes(strings.Join(strings.Fields(strings.TrimSpace(action.Summary)), " "), 220)
	action.Body = strings.TrimSpace(action.Body)
	action.Target = trimTextRunes(strings.Join(strings.Fields(strings.TrimSpace(action.Target)), " "), 80)
	action.Status = normalizeFlowCandidateStatus(action.Status)
	action.Priority = normalizeFlowCandidatePriority(action.Priority)
	action.ConfirmationPolicy = firstNonEmpty(strings.TrimSpace(action.ConfirmationPolicy), "confirm")
	action.NotificationActions = normalizeFlowNotificationActions(action.NotificationActions)
	action.Payload = cleanFlowCandidatePayload(action.Payload)
	action.Evidence = cleanStrings(action.Evidence)
	action.DedupKey = strings.TrimSpace(strings.ToLower(action.DedupKey))
	action.Source = strings.TrimSpace(action.Source)
	action.DecisionActionID = strings.TrimSpace(action.DecisionActionID)
	action.DecisionReason = trimTextRunes(strings.Join(strings.Fields(strings.TrimSpace(action.DecisionReason)), " "), 160)
	if action.CreatedAt <= 0 {
		action.CreatedAt = now.Unix()
	}
	if action.UpdatedAt <= 0 {
		action.UpdatedAt = action.CreatedAt
	}
	if action.ExpiresAt <= 0 && action.Status == flowCandidateStatusPending {
		action.ExpiresAt = time.Unix(action.CreatedAt, 0).Add(8 * time.Hour).Unix()
	}
	if action.DedupKey == "" && action.ExtensionID != "" && action.ActionType != "" {
		action.DedupKey = strings.ToLower(action.ExtensionID + ":" + action.ActionType + ":" + shortHash(action.Title+strings.Join(action.Evidence, ":")))
	}
	if action.ID == "" && action.DedupKey != "" {
		action.ID = "flow-action-" + shortHash(action.DedupKey)
	}
	return action
}

func normalizeFlowCandidateActionType(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case flowCandidateActionPrepareReply, flowCandidateActionFollowUp, flowCandidateActionFactCheckWarning, flowCandidateActionTextPolishHint:
		return value
	default:
		if isFlowAutonomyToken(value) {
			return value
		}
		return ""
	}
}

func isFlowAutonomyToken(value string) bool {
	if value == "" || len(value) > 96 {
		return false
	}
	hasAlphaNumeric := false
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z':
			hasAlphaNumeric = true
		case char >= '0' && char <= '9':
			hasAlphaNumeric = true
		case char == '_' || char == '-' || char == '.':
			continue
		default:
			return false
		}
	}
	return hasAlphaNumeric
}

func normalizeFlowCandidateStatus(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case flowCandidateStatusAccepted,
		flowCandidateStatusSnoozed,
		flowCandidateStatusIgnored,
		flowCandidateStatusExpired,
		flowCandidateStatusDismissedByRule,
		flowCandidateStatusExecuted,
		flowCandidateStatusFailed:
		return strings.TrimSpace(strings.ToLower(value))
	default:
		return flowCandidateStatusPending
	}
}

func normalizeFlowCandidatePriority(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "low", "high", "urgent":
		return strings.TrimSpace(strings.ToLower(value))
	default:
		return "normal"
	}
}

func normalizeFlowNotificationActions(actions []FlowNotificationAction) []FlowNotificationAction {
	result := make([]FlowNotificationAction, 0, len(actions))
	seen := map[string]bool{}
	for _, action := range actions {
		action.ID = strings.TrimSpace(strings.ToLower(action.ID))
		action.Label = trimTextRunes(strings.Join(strings.Fields(strings.TrimSpace(action.Label)), " "), 16)
		action.Kind = strings.TrimSpace(strings.ToLower(action.Kind))
		if action.ID == "" || action.Label == "" || seen[action.ID] {
			continue
		}
		if action.Kind == "" {
			action.Kind = "secondary"
		}
		seen[action.ID] = true
		result = append(result, action)
	}
	if len(result) == 0 {
		return []FlowNotificationAction{{ID: "open_flow", Label: "打开心流", Kind: "secondary"}}
	}
	return result
}

func cleanFlowCandidatePayload(payload map[string]string) map[string]string {
	if len(payload) == 0 {
		return nil
	}
	result := map[string]string{}
	for key, value := range payload {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			result[key] = value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normalizeFlowCandidateDecision(decision string, actionID string) string {
	decision = strings.TrimSpace(strings.ToLower(decision))
	if decision != "" {
		switch normalizeFlowCandidateStatus(decision) {
		case flowCandidateStatusPending:
			return flowCandidateStatusAccepted
		default:
			return normalizeFlowCandidateStatus(decision)
		}
	}
	switch strings.TrimSpace(strings.ToLower(actionID)) {
	case "later", "snooze":
		return flowCandidateStatusSnoozed
	case "ignore", "dismiss":
		return flowCandidateStatusIgnored
	case "dismiss_rule":
		return flowCandidateStatusDismissedByRule
	case "failed":
		return flowCandidateStatusFailed
	default:
		return flowCandidateStatusAccepted
	}
}

func flowCandidateActionDecisionAllowed(status string) bool {
	switch normalizeFlowCandidateStatus(status) {
	case flowCandidateStatusPending, flowCandidateStatusSnoozed:
		return true
	default:
		return false
	}
}

func (s *Service) settleFlowCandidateActionsLocked(now time.Time) int {
	changed := 0
	nowUnix := now.Unix()
	for index := range s.flowCandidateActions {
		action := s.flowCandidateActions[index]
		switch normalizeFlowCandidateStatus(action.Status) {
		case flowCandidateStatusSnoozed:
			if action.ExpiresAt > 0 && action.ExpiresAt <= nowUnix {
				action.Status = flowCandidateStatusExpired
				action.UpdatedAt = nowUnix
				changed++
			} else if action.SnoozedUntil > 0 && action.SnoozedUntil <= nowUnix {
				action.Status = flowCandidateStatusPending
				action.SnoozedUntil = 0
				action.UpdatedAt = nowUnix
				changed++
			}
		case flowCandidateStatusPending:
			if action.ExpiresAt > 0 && action.ExpiresAt <= nowUnix {
				action.Status = flowCandidateStatusExpired
				action.UpdatedAt = nowUnix
				changed++
			}
		}
		s.flowCandidateActions[index] = normalizeFlowCandidateAction(action, now)
	}
	return changed
}

func activeFlowCandidateKeys(actions []FlowCandidateAction, now time.Time) map[string]bool {
	keys := map[string]bool{}
	nowUnix := now.Unix()
	for _, action := range actions {
		action = normalizeFlowCandidateAction(action, now)
		if action.DedupKey == "" {
			continue
		}
		switch action.Status {
		case flowCandidateStatusExpired, flowCandidateStatusFailed, flowCandidateStatusDismissedByRule:
			continue
		case flowCandidateStatusIgnored:
			if action.ExpiresAt > 0 && action.ExpiresAt <= nowUnix {
				continue
			}
		}
		keys[action.DedupKey] = true
	}
	return keys
}

func buildFlowCandidateActionListLocked(actions []FlowCandidateAction, request FlowCandidateActionListRequest, nowUnix int64) FlowCandidateActionList {
	statusFilter := normalizeFlowCandidateActionListStatus(request.Status)
	limit := request.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	result := FlowCandidateActionList{Items: []FlowCandidateAction{}, UpdatedAt: nowUnix}
	for _, action := range actions {
		action = normalizeFlowCandidateAction(action, time.Unix(nowUnix, 0))
		switch action.Status {
		case flowCandidateStatusPending:
			result.Pending++
		case flowCandidateStatusSnoozed:
			result.Snoozed++
		case flowCandidateStatusAccepted:
			result.Accepted++
		case flowCandidateStatusIgnored, flowCandidateStatusDismissedByRule:
			result.Ignored++
		case flowCandidateStatusExpired:
			result.Expired++
		case flowCandidateStatusExecuted:
			result.Executed++
		case flowCandidateStatusFailed:
			result.Failed++
		}
		if statusFilter != "" && action.Status != statusFilter {
			continue
		}
		if statusFilter == "" && action.Status != flowCandidateStatusPending {
			continue
		}
		if action.Status == flowCandidateStatusExpired && !request.IncludeExpired {
			continue
		}
		result.Items = append(result.Items, action)
	}
	sort.SliceStable(result.Items, func(i, j int) bool {
		if result.Items[i].Priority != result.Items[j].Priority {
			return flowCandidatePriorityRank(result.Items[i].Priority) < flowCandidatePriorityRank(result.Items[j].Priority)
		}
		return result.Items[i].UpdatedAt > result.Items[j].UpdatedAt
	})
	if len(result.Items) > limit {
		result.Items = result.Items[:limit]
	}
	return result
}

func normalizeFlowCandidateActionListStatus(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" || value == "active" {
		return ""
	}
	return normalizeFlowCandidateStatus(value)
}

func flowCandidatePriorityRank(priority string) int {
	switch normalizeFlowCandidatePriority(priority) {
	case "urgent":
		return 0
	case "high":
		return 1
	case "normal":
		return 2
	default:
		return 3
	}
}

func cloneFlowCandidateActions(source []FlowCandidateAction) []FlowCandidateAction {
	if len(source) == 0 {
		return nil
	}
	cloned := make([]FlowCandidateAction, len(source))
	for index, action := range source {
		cloned[index] = cloneFlowCandidateAction(action)
	}
	return cloned
}

func cloneFlowCandidateAction(action FlowCandidateAction) FlowCandidateAction {
	action.NotificationActions = append([]FlowNotificationAction(nil), action.NotificationActions...)
	action.Evidence = append([]string(nil), action.Evidence...)
	if len(action.Payload) > 0 {
		payload := map[string]string{}
		for key, value := range action.Payload {
			payload[key] = value
		}
		action.Payload = payload
	}
	return action
}
