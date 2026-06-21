package flownotify

import (
	"testing"

	"ariadne/internal/workmemory"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

type fakeNotificationClient struct {
	callback   func(notifications.NotificationResult)
	categories []notifications.NotificationCategory
	sent       []notifications.NotificationOptions
}

func (f *fakeNotificationClient) OnNotificationResponse(callback func(notifications.NotificationResult)) {
	f.callback = callback
}

func (f *fakeNotificationClient) RegisterNotificationCategory(category notifications.NotificationCategory) error {
	f.categories = append(f.categories, category)
	return nil
}

func (f *fakeNotificationClient) SendNotificationWithActions(options notifications.NotificationOptions) error {
	f.sent = append(f.sent, options)
	return nil
}

func TestDispatcherSendsFollowUpNotificationWithActions(t *testing.T) {
	service, action := flowCandidateForTest(t)
	fake := &fakeNotificationClient{}
	dispatcher := New(service, fake, nil)

	dispatcher.HandleWorkMemoryEvent(workmemory.ChangeEvent{
		Kind:                "flow_candidate_created",
		FlowCandidateID:     action.ID,
		FlowCandidateAction: &action,
	})

	if len(fake.sent) != 1 {
		t.Fatalf("expected one notification, got %#v", fake.sent)
	}
	if len(fake.categories) != 1 {
		t.Fatalf("expected one category, got %#v", fake.categories)
	}
	if got := actionIDs(fake.categories[0].Actions); !sameStrings(got, []string{"add", "later", "ignore"}) {
		t.Fatalf("expected add/later/ignore actions, got %#v", got)
	}
	if fake.sent[0].Data[notificationDataCandidateID] != action.ID {
		t.Fatalf("expected candidate id in notification data, got %#v", fake.sent[0].Data)
	}
}

func TestDispatcherNotificationAddExecutesCandidate(t *testing.T) {
	service, action := flowCandidateForTest(t)
	fake := &fakeNotificationClient{}
	New(service, fake, nil)
	if fake.callback == nil {
		t.Fatal("expected notification response callback")
	}

	fake.callback(notifications.NotificationResult{
		Response: notifications.NotificationResponse{
			ActionIdentifier: "add",
			UserInfo: map[string]interface{}{
				notificationDataCandidateID: action.ID,
			},
		},
	})

	actions := service.FlowCandidateActions(workmemory.FlowCandidateActionListRequest{
		Status:         "executed",
		IncludeExpired: true,
	})
	if len(actions.Items) != 1 || actions.Items[0].ID != action.ID {
		t.Fatalf("expected executed candidate, got %#v", actions)
	}
	todos := service.Todos(workmemory.TodoListRequest{IncludeDone: true})
	if len(todos.Items) != 1 || todos.Items[0].Source != "flow_autonomy" {
		t.Fatalf("expected flow autonomy todo, got %#v", todos.Items)
	}
}

func TestDispatcherDefaultActionOpensFlow(t *testing.T) {
	service, action := flowCandidateForTest(t)
	fake := &fakeNotificationClient{}
	opened := false
	New(service, fake, func() { opened = true })
	if fake.callback == nil {
		t.Fatal("expected notification response callback")
	}

	fake.callback(notifications.NotificationResult{
		Response: notifications.NotificationResponse{
			ActionIdentifier: notifications.DefaultActionIdentifier,
			UserInfo: map[string]interface{}{
				notificationDataCandidateID: action.ID,
			},
		},
	})

	if !opened {
		t.Fatal("expected default notification action to open flow")
	}
	actions := service.FlowCandidateActions(workmemory.FlowCandidateActionListRequest{})
	if actions.Pending != 1 {
		t.Fatalf("opening flow should not consume candidate, got %#v", actions)
	}
}

func flowCandidateForTest(t *testing.T) (*workmemory.Service, workmemory.FlowCandidateAction) {
	t.Helper()
	service := workmemory.NewServiceWithPath("", nil)
	t.Cleanup(service.Stop)
	service.ApplyFlowAutonomyPolicy(workmemory.FlowAutonomyPolicy{
		Enabled:                    true,
		CommunicationAssistEnabled: true,
		CandidateTTLHours:          8,
		CandidateCooldownMinutes:   1,
		DefaultSnoozeMinutes:       30,
	})
	service.AddNote(workmemory.NoteRequest{
		Text:      "我稍后把接口文档发你",
		Title:     "企业微信消息",
		Sensitive: false,
	})
	candidates := service.FlowCandidateActions(workmemory.FlowCandidateActionListRequest{})
	if len(candidates.Items) != 1 {
		t.Fatalf("expected one candidate, got %#v", candidates)
	}
	candidate := candidates.Items[0]
	return service, candidate
}

func actionIDs(actions []notifications.NotificationAction) []string {
	result := make([]string, 0, len(actions))
	for _, action := range actions {
		result = append(result, action.ID)
	}
	return result
}

func sameStrings(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
