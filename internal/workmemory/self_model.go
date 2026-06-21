package workmemory

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	selfAssertionStatusConfirmed = "confirmed"
	selfAssertionStatusObserved  = "observed"
	selfAssertionStatusRejected  = "rejected"
	selfAssertionStatusEphemeral = "ephemeral"

	selfAssertionPrivacyAlways   = "always"
	selfAssertionPrivacyRelevant = "relevant"
	selfAssertionPrivacyNever    = "never"
)

type SelfAssertion struct {
	ID          string   `json:"id"`
	Category    string   `json:"category"`
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Value       string   `json:"value"`
	Status      string   `json:"status"`
	Privacy     string   `json:"privacy"`
	Scope       string   `json:"scope,omitempty"`
	Source      string   `json:"source"`
	Confidence  float64  `json:"confidence"`
	Evidence    []string `json:"evidence,omitempty"`
	PromptReady bool     `json:"promptReady"`
	CreatedAt   int64    `json:"createdAt"`
	UpdatedAt   int64    `json:"updatedAt"`
}

type SelfAssertionRequest struct {
	ID         string   `json:"id,omitempty"`
	Category   string   `json:"category"`
	Key        string   `json:"key"`
	Label      string   `json:"label"`
	Value      string   `json:"value"`
	Status     string   `json:"status,omitempty"`
	Privacy    string   `json:"privacy,omitempty"`
	Scope      string   `json:"scope,omitempty"`
	Source     string   `json:"source,omitempty"`
	Confidence float64  `json:"confidence,omitempty"`
	Evidence   []string `json:"evidence,omitempty"`
}

type SelfModelSummary struct {
	Prompt    string          `json:"prompt"`
	Included  []SelfAssertion `json:"included"`
	Excluded  int             `json:"excluded"`
	UpdatedAt int64           `json:"updatedAt"`
}

type SelfModel struct {
	Assertions []SelfAssertion  `json:"assertions"`
	Summary    SelfModelSummary `json:"summary"`
	UpdatedAt  int64            `json:"updatedAt"`
}

func (s *Service) SelfModel() SelfModel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return buildSelfModelLocked(s.selfAssertions)
}

func (s *Service) UpsertSelfAssertion(request SelfAssertionRequest) SelfModel {
	now := s.now().Unix()
	assertion := normalizeSelfAssertion(SelfAssertion{
		ID:         request.ID,
		Category:   request.Category,
		Key:        request.Key,
		Label:      request.Label,
		Value:      request.Value,
		Status:     request.Status,
		Privacy:    request.Privacy,
		Scope:      request.Scope,
		Source:     request.Source,
		Confidence: request.Confidence,
		Evidence:   request.Evidence,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, now)
	if assertion.Value == "" && assertion.Status != selfAssertionStatusRejected {
		return s.SelfModel()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if assertion.ID == "" {
		assertion.ID = fmt.Sprintf("self-%s-%d-%s", sanitizeSelfModelPart(assertion.Category), now, shortHash(assertion.Category+"\n"+assertion.Key+"\n"+assertion.Value))
	}
	replaced := false
	for index, existing := range s.selfAssertions {
		if existing.ID != assertion.ID {
			continue
		}
		if assertion.CreatedAt <= 0 {
			assertion.CreatedAt = existing.CreatedAt
		}
		if assertion.CreatedAt <= 0 {
			assertion.CreatedAt = now
		}
		assertion.UpdatedAt = now
		s.selfAssertions[index] = normalizeSelfAssertion(assertion, now)
		replaced = true
		break
	}
	if !replaced {
		s.selfAssertions = append(s.selfAssertions, assertion)
	}
	sortSelfAssertions(s.selfAssertions)
	if err := s.saveLocked(); err != nil {
		s.saveError = err.Error()
	}
	return buildSelfModelLocked(s.selfAssertions)
}

func (s *Service) DeleteSelfAssertion(id string) SelfModel {
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	if id != "" {
		filtered := s.selfAssertions[:0]
		for _, assertion := range s.selfAssertions {
			if assertion.ID != id {
				filtered = append(filtered, assertion)
			}
		}
		s.selfAssertions = filtered
		if err := s.saveLocked(); err != nil {
			s.saveError = err.Error()
		}
	}
	return buildSelfModelLocked(s.selfAssertions)
}

func buildSelfModelLocked(assertions []SelfAssertion) SelfModel {
	cloned := cloneSelfAssertions(assertions)
	sortSelfAssertions(cloned)
	summary := buildSelfModelSummary(cloned)
	updatedAt := summary.UpdatedAt
	if updatedAt <= 0 {
		for _, assertion := range cloned {
			if assertion.UpdatedAt > updatedAt {
				updatedAt = assertion.UpdatedAt
			}
		}
	}
	return SelfModel{
		Assertions: cloned,
		Summary:    summary,
		UpdatedAt:  updatedAt,
	}
}

func buildSelfModelSummary(assertions []SelfAssertion) SelfModelSummary {
	included := make([]SelfAssertion, 0, len(assertions))
	updatedAt := int64(0)
	for _, assertion := range assertions {
		assertion = normalizeSelfAssertion(assertion, time.Now().Unix())
		if assertion.UpdatedAt > updatedAt {
			updatedAt = assertion.UpdatedAt
		}
		if assertion.PromptReady {
			included = append(included, assertion)
		}
	}
	sortSelfAssertions(included)
	lines := []string{}
	byCategory := map[string][]SelfAssertion{}
	for _, assertion := range included {
		byCategory[assertion.Category] = append(byCategory[assertion.Category], assertion)
	}
	for _, category := range []string{"identity", "preference", "boundary"} {
		items := byCategory[category]
		if len(items) == 0 {
			continue
		}
		label := selfCategoryLabel(category)
		parts := []string{}
		for _, item := range items {
			parts = append(parts, fmt.Sprintf("%s: %s", firstNonEmpty(item.Label, item.Key), item.Value))
		}
		lines = append(lines, label+" - "+strings.Join(parts, "; "))
	}
	if len(lines) == 0 {
		lines = append(lines, "No confirmed low-risk Self Model assertions are available.")
	}
	return SelfModelSummary{
		Prompt:    strings.Join(lines, "\n"),
		Included:  included,
		Excluded:  len(assertions) - len(included),
		UpdatedAt: updatedAt,
	}
}

func normalizeSelfAssertion(assertion SelfAssertion, now int64) SelfAssertion {
	assertion.ID = strings.TrimSpace(assertion.ID)
	assertion.Category = normalizeSelfCategory(assertion.Category)
	assertion.Key = strings.TrimSpace(assertion.Key)
	assertion.Label = strings.TrimSpace(assertion.Label)
	assertion.Value = strings.TrimSpace(assertion.Value)
	assertion.Status = normalizeSelfStatus(assertion.Status)
	assertion.Privacy = normalizeSelfPrivacy(assertion.Privacy)
	assertion.Scope = strings.TrimSpace(assertion.Scope)
	assertion.Source = strings.TrimSpace(assertion.Source)
	if assertion.Source == "" {
		assertion.Source = "manual"
	}
	if assertion.Confidence <= 0 {
		if assertion.Status == selfAssertionStatusConfirmed {
			assertion.Confidence = 1
		} else if assertion.Status == selfAssertionStatusRejected {
			assertion.Confidence = 1
		} else {
			assertion.Confidence = 0.6
		}
	}
	if assertion.Confidence > 1 {
		assertion.Confidence = 1
	}
	assertion.Evidence = cleanStrings(assertion.Evidence)
	if assertion.CreatedAt <= 0 {
		assertion.CreatedAt = now
	}
	if assertion.UpdatedAt <= 0 {
		assertion.UpdatedAt = assertion.CreatedAt
	}
	assertion.PromptReady = assertion.Status == selfAssertionStatusConfirmed &&
		assertion.Privacy == selfAssertionPrivacyAlways &&
		(assertion.Category == "identity" || assertion.Category == "preference" || assertion.Category == "boundary") &&
		assertion.Value != ""
	return assertion
}

func normalizeSelfCategory(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "identity", "preference", "relationship", "boundary":
		return strings.TrimSpace(strings.ToLower(value))
	default:
		return "identity"
	}
}

func normalizeSelfStatus(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case selfAssertionStatusObserved, selfAssertionStatusRejected, selfAssertionStatusEphemeral:
		return strings.TrimSpace(strings.ToLower(value))
	default:
		return selfAssertionStatusConfirmed
	}
}

func normalizeSelfPrivacy(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case selfAssertionPrivacyRelevant, selfAssertionPrivacyNever:
		return strings.TrimSpace(strings.ToLower(value))
	default:
		return selfAssertionPrivacyAlways
	}
}

func sanitizeSelfModelPart(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "assertion"
	}
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			builder.WriteRune(r)
		}
	}
	if builder.Len() == 0 {
		return "assertion"
	}
	return builder.String()
}

func selfCategoryLabel(category string) string {
	switch category {
	case "identity":
		return "Identity"
	case "preference":
		return "Preferences"
	case "boundary":
		return "Boundaries"
	default:
		return "Self"
	}
}

func cloneSelfAssertions(assertions []SelfAssertion) []SelfAssertion {
	cloned := make([]SelfAssertion, 0, len(assertions))
	now := time.Now().Unix()
	for _, assertion := range assertions {
		assertion = normalizeSelfAssertion(assertion, now)
		assertion.Evidence = append([]string(nil), assertion.Evidence...)
		cloned = append(cloned, assertion)
	}
	return cloned
}

func sortSelfAssertions(assertions []SelfAssertion) {
	sort.SliceStable(assertions, func(i, j int) bool {
		if assertions[i].Category != assertions[j].Category {
			return assertions[i].Category < assertions[j].Category
		}
		if assertions[i].Status != assertions[j].Status {
			return assertions[i].Status < assertions[j].Status
		}
		if assertions[i].UpdatedAt != assertions[j].UpdatedAt {
			return assertions[i].UpdatedAt > assertions[j].UpdatedAt
		}
		return assertions[i].Label < assertions[j].Label
	})
}
