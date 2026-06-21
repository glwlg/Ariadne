package workmemory

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	todoStatusOpen     = "open"
	todoStatusDoing    = "doing"
	todoStatusWaiting  = "waiting"
	todoStatusDone     = "done"
	todoStatusCanceled = "canceled"

	todoPriorityLow    = "low"
	todoPriorityNormal = "normal"
	todoPriorityHigh   = "high"
	todoPriorityUrgent = "urgent"
)

type TodoItem struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Note        string   `json:"note,omitempty"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority"`
	Scope       string   `json:"scope,omitempty"`
	Source      string   `json:"source"`
	Evidence    []string `json:"evidence,omitempty"`
	DueAt       int64    `json:"dueAt,omitempty"`
	RemindAt    int64    `json:"remindAt,omitempty"`
	CompletedAt int64    `json:"completedAt,omitempty"`
	CreatedAt   int64    `json:"createdAt"`
	UpdatedAt   int64    `json:"updatedAt"`
}

type TodoRequest struct {
	ID       string   `json:"id,omitempty"`
	Title    string   `json:"title"`
	Note     string   `json:"note,omitempty"`
	Status   string   `json:"status,omitempty"`
	Priority string   `json:"priority,omitempty"`
	Scope    string   `json:"scope,omitempty"`
	Source   string   `json:"source,omitempty"`
	Evidence []string `json:"evidence,omitempty"`
	DueAt    int64    `json:"dueAt,omitempty"`
	RemindAt int64    `json:"remindAt,omitempty"`
}

type TodoUpdateRequest struct {
	ID            string   `json:"id"`
	Title         string   `json:"title,omitempty"`
	Note          string   `json:"note,omitempty"`
	Status        string   `json:"status,omitempty"`
	Priority      string   `json:"priority,omitempty"`
	Scope         string   `json:"scope,omitempty"`
	Source        string   `json:"source,omitempty"`
	Evidence      []string `json:"evidence,omitempty"`
	DueAt         int64    `json:"dueAt,omitempty"`
	RemindAt      int64    `json:"remindAt,omitempty"`
	ClearDueAt    bool     `json:"clearDueAt,omitempty"`
	ClearRemindAt bool     `json:"clearRemindAt,omitempty"`
}

type TodoListRequest struct {
	Status      string `json:"status,omitempty"`
	Scope       string `json:"scope,omitempty"`
	Query       string `json:"query,omitempty"`
	IncludeDone bool   `json:"includeDone,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

type TodoList struct {
	Items     []TodoItem `json:"items"`
	Open      int        `json:"open"`
	Doing     int        `json:"doing"`
	Waiting   int        `json:"waiting"`
	Done      int        `json:"done"`
	Canceled  int        `json:"canceled"`
	UpdatedAt int64      `json:"updatedAt"`
}

func (s *Service) Todos(request TodoListRequest) TodoList {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return buildTodoListLocked(s.todoItems, request)
}

func (s *Service) UpsertTodo(request TodoRequest) TodoList {
	now := s.now().Unix()
	item := normalizeTodoItem(TodoItem{
		ID:        request.ID,
		Title:     request.Title,
		Note:      request.Note,
		Status:    request.Status,
		Priority:  request.Priority,
		Scope:     request.Scope,
		Source:    request.Source,
		Evidence:  request.Evidence,
		DueAt:     request.DueAt,
		RemindAt:  request.RemindAt,
		CreatedAt: now,
		UpdatedAt: now,
	}, now)
	if item.Title == "" {
		return s.Todos(TodoListRequest{IncludeDone: true})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if item.ID == "" {
		item.ID = fmt.Sprintf("todo-%d-%s", now, shortHash(item.Title+"\n"+item.Note+"\n"+item.Scope))
	}
	replaced := false
	for index, existing := range s.todoItems {
		if existing.ID != item.ID {
			continue
		}
		if item.CreatedAt <= 0 {
			item.CreatedAt = existing.CreatedAt
		}
		if item.CreatedAt <= 0 {
			item.CreatedAt = now
		}
		item.UpdatedAt = now
		s.todoItems[index] = normalizeTodoItem(item, now)
		replaced = true
		break
	}
	if !replaced {
		s.todoItems = append(s.todoItems, item)
	}
	sortTodoItems(s.todoItems)
	if err := s.saveLocked(); err != nil {
		s.saveError = err.Error()
	}
	return buildTodoListLocked(s.todoItems, TodoListRequest{IncludeDone: true})
}

func (s *Service) UpdateTodo(request TodoUpdateRequest) TodoList {
	id := strings.TrimSpace(request.ID)
	if id == "" {
		return s.Todos(TodoListRequest{IncludeDone: true})
	}
	now := s.now().Unix()
	s.mu.Lock()
	defer s.mu.Unlock()
	for index, item := range s.todoItems {
		if item.ID != id {
			continue
		}
		if strings.TrimSpace(request.Title) != "" {
			item.Title = request.Title
		}
		if request.Note != "" {
			item.Note = request.Note
		}
		if strings.TrimSpace(request.Status) != "" {
			item.Status = request.Status
		}
		if strings.TrimSpace(request.Priority) != "" {
			item.Priority = request.Priority
		}
		if request.Scope != "" {
			item.Scope = request.Scope
		}
		if request.Source != "" {
			item.Source = request.Source
		}
		if request.Evidence != nil {
			item.Evidence = request.Evidence
		}
		if request.ClearDueAt {
			item.DueAt = 0
		} else if request.DueAt > 0 {
			item.DueAt = request.DueAt
		}
		if request.ClearRemindAt {
			item.RemindAt = 0
		} else if request.RemindAt > 0 {
			item.RemindAt = request.RemindAt
		}
		item.UpdatedAt = now
		s.todoItems[index] = normalizeTodoItem(item, now)
		sortTodoItems(s.todoItems)
		if err := s.saveLocked(); err != nil {
			s.saveError = err.Error()
		}
		break
	}
	return buildTodoListLocked(s.todoItems, TodoListRequest{IncludeDone: true})
}

func (s *Service) DeleteTodo(id string) TodoList {
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	if id != "" {
		filtered := s.todoItems[:0]
		for _, item := range s.todoItems {
			if item.ID != id {
				filtered = append(filtered, item)
			}
		}
		s.todoItems = filtered
		if err := s.saveLocked(); err != nil {
			s.saveError = err.Error()
		}
	}
	return buildTodoListLocked(s.todoItems, TodoListRequest{IncludeDone: true})
}

func buildTodoListLocked(items []TodoItem, request TodoListRequest) TodoList {
	request.Status = normalizeTodoStatusFilter(request.Status)
	request.Scope = strings.TrimSpace(request.Scope)
	request.Query = strings.TrimSpace(strings.ToLower(request.Query))
	limit := request.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 300 {
		limit = 300
	}
	cloned := cloneTodoItems(items)
	sortTodoItems(cloned)
	result := TodoList{Items: []TodoItem{}}
	for _, item := range cloned {
		switch item.Status {
		case todoStatusDoing:
			result.Doing++
		case todoStatusWaiting:
			result.Waiting++
		case todoStatusDone:
			result.Done++
		case todoStatusCanceled:
			result.Canceled++
		default:
			result.Open++
		}
		if item.UpdatedAt > result.UpdatedAt {
			result.UpdatedAt = item.UpdatedAt
		}
		if request.Status != "" && item.Status != request.Status {
			continue
		}
		if request.Status == "" && !request.IncludeDone && isTodoClosed(item.Status) {
			continue
		}
		if request.Scope != "" && !strings.Contains(strings.ToLower(item.Scope), strings.ToLower(request.Scope)) {
			continue
		}
		if request.Query != "" && !strings.Contains(todoSearchText(item), request.Query) {
			continue
		}
		result.Items = append(result.Items, item)
		if len(result.Items) >= limit {
			break
		}
	}
	return result
}

func normalizeTodoItem(item TodoItem, now int64) TodoItem {
	item.ID = strings.TrimSpace(item.ID)
	item.Title = strings.TrimSpace(item.Title)
	item.Note = strings.TrimSpace(item.Note)
	item.Status = normalizeTodoStatus(item.Status)
	item.Priority = normalizeTodoPriority(item.Priority)
	item.Scope = strings.TrimSpace(item.Scope)
	item.Source = strings.TrimSpace(item.Source)
	if item.Source == "" {
		item.Source = "manual"
	}
	item.Evidence = cleanStrings(item.Evidence)
	if item.CreatedAt <= 0 {
		item.CreatedAt = now
	}
	if item.UpdatedAt <= 0 {
		item.UpdatedAt = item.CreatedAt
	}
	if item.Status == todoStatusDone {
		if item.CompletedAt <= 0 {
			item.CompletedAt = item.UpdatedAt
		}
	} else {
		item.CompletedAt = 0
	}
	return item
}

func normalizeTodoStatus(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case todoStatusDoing, todoStatusWaiting, todoStatusDone, todoStatusCanceled:
		return strings.TrimSpace(strings.ToLower(value))
	default:
		return todoStatusOpen
	}
}

func normalizeTodoStatusFilter(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case todoStatusOpen, todoStatusDoing, todoStatusWaiting, todoStatusDone, todoStatusCanceled:
		return value
	default:
		return ""
	}
}

func normalizeTodoPriority(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case todoPriorityLow, todoPriorityHigh, todoPriorityUrgent:
		return strings.TrimSpace(strings.ToLower(value))
	default:
		return todoPriorityNormal
	}
}

func isTodoClosed(status string) bool {
	status = normalizeTodoStatus(status)
	return status == todoStatusDone || status == todoStatusCanceled
}

func todoSearchText(item TodoItem) string {
	return strings.ToLower(strings.Join([]string{
		item.ID,
		item.Title,
		item.Note,
		item.Status,
		item.Priority,
		item.Scope,
		item.Source,
		strings.Join(item.Evidence, " "),
	}, "\n"))
}

func cloneTodoItems(items []TodoItem) []TodoItem {
	cloned := make([]TodoItem, 0, len(items))
	now := time.Now().Unix()
	for _, item := range items {
		item = normalizeTodoItem(item, now)
		item.Evidence = append([]string(nil), item.Evidence...)
		cloned = append(cloned, item)
	}
	return cloned
}

func sortTodoItems(items []TodoItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if rank := todoStatusRank(items[i].Status) - todoStatusRank(items[j].Status); rank != 0 {
			return rank < 0
		}
		if rank := todoPriorityRank(items[i].Priority) - todoPriorityRank(items[j].Priority); rank != 0 {
			return rank < 0
		}
		if items[i].DueAt != items[j].DueAt {
			if items[i].DueAt == 0 {
				return false
			}
			if items[j].DueAt == 0 {
				return true
			}
			return items[i].DueAt < items[j].DueAt
		}
		if items[i].UpdatedAt != items[j].UpdatedAt {
			return items[i].UpdatedAt > items[j].UpdatedAt
		}
		return items[i].Title < items[j].Title
	})
}

func todoStatusRank(status string) int {
	switch normalizeTodoStatus(status) {
	case todoStatusDoing:
		return 0
	case todoStatusOpen:
		return 1
	case todoStatusWaiting:
		return 2
	case todoStatusDone:
		return 3
	case todoStatusCanceled:
		return 4
	default:
		return 5
	}
}

func todoPriorityRank(priority string) int {
	switch normalizeTodoPriority(priority) {
	case todoPriorityUrgent:
		return 0
	case todoPriorityHigh:
		return 1
	case todoPriorityNormal:
		return 2
	default:
		return 3
	}
}
