package flownotify

import (
	"fmt"
	"hash/fnv"
	"log"
	"strings"
	"sync"
	"unicode"

	"ariadne/internal/workmemory"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

const (
	notificationDataCandidateID = "flowCandidateId"
	notificationDataActionType  = "flowActionType"
	notificationDataExtensionID = "flowExtensionId"
)

type notificationClient interface {
	OnNotificationResponse(func(notifications.NotificationResult))
	RegisterNotificationCategory(notifications.NotificationCategory) error
	SendNotificationWithActions(notifications.NotificationOptions) error
}

type Dispatcher struct {
	mu                   sync.Mutex
	workMemory           *workmemory.Service
	notifications        notificationClient
	openFlow             func()
	registeredCategories map[string]bool
}

func New(workMemory *workmemory.Service, notifications notificationClient, openFlow func()) *Dispatcher {
	dispatcher := &Dispatcher{
		workMemory:           workMemory,
		notifications:        notifications,
		openFlow:             openFlow,
		registeredCategories: map[string]bool{},
	}
	if notifications != nil {
		notifications.OnNotificationResponse(dispatcher.HandleNotificationResult)
	}
	return dispatcher
}

func (d *Dispatcher) HandleWorkMemoryEvent(event workmemory.ChangeEvent) {
	if d == nil || event.Kind != "flow_candidate_created" || event.FlowCandidateAction == nil {
		return
	}
	action := *event.FlowCandidateAction
	if strings.TrimSpace(action.ID) == "" || action.Status != "pending" {
		return
	}
	if err := d.SendCandidate(action); err != nil {
		log.Printf("flow notification: %v", err)
	}
}

func (d *Dispatcher) SendCandidate(action workmemory.FlowCandidateAction) error {
	if d == nil || d.notifications == nil {
		return nil
	}
	toastActions := toastActionsForCandidate(action)
	if len(toastActions) == 0 {
		return nil
	}
	categoryID := categoryIDForCandidate(action, toastActions)
	if err := d.ensureCategory(categoryID, toastActions); err != nil {
		return err
	}
	return d.notifications.SendNotificationWithActions(notifications.NotificationOptions{
		ID:         "flow-candidate-" + action.ID,
		Title:      firstNonEmpty(action.Title, "Ariadne 主动动作"),
		Body:       notificationBody(action),
		CategoryID: categoryID,
		Data: map[string]interface{}{
			notificationDataCandidateID: action.ID,
			notificationDataActionType:  action.ActionType,
			notificationDataExtensionID: action.ExtensionID,
		},
	})
}

func (d *Dispatcher) HandleNotificationResult(result notifications.NotificationResult) {
	if d == nil {
		return
	}
	if result.Error != nil {
		log.Printf("flow notification response: %v", result.Error)
		return
	}
	candidateID := stringFromUserInfo(result.Response.UserInfo, notificationDataCandidateID)
	actionID := strings.TrimSpace(result.Response.ActionIdentifier)
	if candidateID == "" {
		return
	}
	switch actionID {
	case "", notifications.DefaultActionIdentifier, "open", "view", "open_flow":
		d.openFlowWindow()
	case "add":
		d.decide(workmemory.FlowCandidateActionDecisionRequest{ID: candidateID, ActionID: actionID})
	case "later":
		d.decide(workmemory.FlowCandidateActionDecisionRequest{ID: candidateID, ActionID: actionID, Decision: "snoozed"})
	case "ignore":
		d.decide(workmemory.FlowCandidateActionDecisionRequest{ID: candidateID, ActionID: actionID, Decision: "ignored"})
	case "dismiss_rule":
		d.decide(workmemory.FlowCandidateActionDecisionRequest{ID: candidateID, ActionID: actionID, Decision: "dismissed_by_rule"})
	case "copy", "copy_revision":
		d.openFlowWindow()
	default:
		d.openFlowWindow()
	}
}

func (d *Dispatcher) ensureCategory(id string, actions []notifications.NotificationAction) error {
	d.mu.Lock()
	if d.registeredCategories[id] {
		d.mu.Unlock()
		return nil
	}
	d.mu.Unlock()

	if err := d.notifications.RegisterNotificationCategory(notifications.NotificationCategory{ID: id, Actions: actions}); err != nil {
		return fmt.Errorf("register notification category: %w", err)
	}

	d.mu.Lock()
	d.registeredCategories[id] = true
	d.mu.Unlock()
	return nil
}

func (d *Dispatcher) decide(request workmemory.FlowCandidateActionDecisionRequest) {
	if d.workMemory == nil {
		return
	}
	result := d.workMemory.DecideFlowCandidateAction(request)
	if !result.OK {
		log.Printf("flow notification decision failed: %s", result.Message)
	}
}

func (d *Dispatcher) openFlowWindow() {
	if d.openFlow != nil {
		d.openFlow()
	}
}

func toastActionsForCandidate(action workmemory.FlowCandidateAction) []notifications.NotificationAction {
	result := []notifications.NotificationAction{}
	seen := map[string]bool{}
	for _, notificationAction := range action.NotificationActions {
		id := strings.TrimSpace(notificationAction.ID)
		if id == "" || seen[id] || !toastActionSupported(id) {
			continue
		}
		seen[id] = true
		title := strings.TrimSpace(notificationAction.Label)
		if title == "" {
			title = toastActionLabel(id)
		}
		result = append(result, notifications.NotificationAction{ID: id, Title: title})
		if len(result) >= 3 {
			break
		}
	}
	if len(result) == 0 {
		result = append(result, notifications.NotificationAction{ID: "open_flow", Title: "打开心流"})
	}
	return result
}

func toastActionSupported(id string) bool {
	switch id {
	case "add", "later", "ignore", "dismiss_rule", "open", "view", "open_flow":
		return true
	default:
		return false
	}
}

func toastActionLabel(id string) string {
	switch id {
	case "add":
		return "添加"
	case "later":
		return "稍后"
	case "ignore":
		return "忽略"
	case "dismiss_rule":
		return "不再提示"
	case "open", "view", "open_flow":
		return "打开心流"
	default:
		return "处理"
	}
}

func categoryIDForCandidate(action workmemory.FlowCandidateAction, actions []notifications.NotificationAction) string {
	parts := []string{sanitizeCategoryToken(action.ActionType)}
	for _, notificationAction := range actions {
		parts = append(parts, sanitizeCategoryToken(notificationAction.ID))
	}
	key := strings.Join(parts, "_")
	if key == "" {
		key = "candidate"
	}
	return "ariadne_flow_" + key + "_" + shortHash(key)
}

func sanitizeCategoryToken(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var builder strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			continue
		}
		if builder.Len() > 0 {
			builder.WriteByte('_')
		}
	}
	return strings.Trim(builder.String(), "_")
}

func shortHash(value string) string {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(value))
	return fmt.Sprintf("%08x", hash.Sum32())
}

func notificationBody(action workmemory.FlowCandidateAction) string {
	body := firstNonEmpty(action.Summary, action.Body, action.Target)
	if len([]rune(body)) <= 180 {
		return body
	}
	runes := []rune(body)
	return string(runes[:177]) + "..."
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func stringFromUserInfo(info map[string]interface{}, key string) string {
	if len(info) == 0 {
		return ""
	}
	value, ok := info[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}
