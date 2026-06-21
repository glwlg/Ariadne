package workmemory

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"ariadne/internal/appdb"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

type memoryState struct {
	Entries              []Entry
	SelfAssertions       []SelfAssertion
	Todos                []TodoItem
	ExperienceDecisions  map[string]ExperienceDecision
	AutonomousArtifacts  []AutonomousArtifact
	AutonomousRejections map[string]AutonomousRejection
	LastAutonomousRunAt  int64
}

func loadMemoryStateFromSQLite(path string, now func() time.Time) (memoryState, bool, error) {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return memoryState{}, false, err
	}
	defer conn.Close()
	if err := ensureWorkMemorySchema(conn); err != nil {
		return memoryState{}, false, err
	}
	state, ok, err := readMemoryState(conn, now)
	if err != nil || ok {
		return state, ok, err
	}
	var legacy struct {
		Version              int                            `json:"version"`
		Entries              []Entry                        `json:"entries"`
		SelfAssertions       []SelfAssertion                `json:"selfAssertions,omitempty"`
		Todos                []TodoItem                     `json:"todos,omitempty"`
		ExperienceDecisions  map[string]ExperienceDecision  `json:"experienceDecisions,omitempty"`
		AutonomousArtifacts  []AutonomousArtifact           `json:"autonomousArtifacts,omitempty"`
		AutonomousRejections map[string]AutonomousRejection `json:"autonomousRejections,omitempty"`
		LastAutonomousRunAt  int64                          `json:"lastAutonomousRunAt,omitempty"`
	}
	if loaded, err := appdb.LegacyLoadJSON(path, "work_memory", &legacy); err != nil || !loaded {
		return memoryState{}, false, err
	}
	state = memoryState{
		Entries:              legacy.Entries,
		SelfAssertions:       legacy.SelfAssertions,
		Todos:                legacy.Todos,
		ExperienceDecisions:  legacy.ExperienceDecisions,
		AutonomousArtifacts:  legacy.AutonomousArtifacts,
		AutonomousRejections: legacy.AutonomousRejections,
		LastAutonomousRunAt:  legacy.LastAutonomousRunAt,
	}
	if err := saveMemoryStateToSQLite(path, state); err != nil {
		return memoryState{}, false, err
	}
	_ = appdb.DropLegacyDocument(path, "work_memory")
	return state, true, nil
}

func saveMemoryStateToSQLite(path string, state memoryState) error {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := ensureWorkMemorySchema(conn); err != nil {
		return err
	}
	return appdb.Immediate(conn, func() error {
		for _, stmt := range []string{
			`DELETE FROM work_memory_tags`,
			`DELETE FROM work_memory_frames`,
			`DELETE FROM work_memory_self_assertion_evidence`,
			`DELETE FROM work_memory_self_assertions`,
			`DELETE FROM work_memory_todo_evidence`,
			`DELETE FROM work_memory_todos`,
			`DELETE FROM work_memory_decisions`,
			`DELETE FROM work_memory_autonomous_artifact_evidence`,
			`DELETE FROM work_memory_autonomous_artifacts`,
			`DELETE FROM work_memory_autonomous_rejections`,
			`DELETE FROM work_memory_meta`,
			`DELETE FROM work_memory_entries`,
		} {
			if err := sqlitex.Execute(conn, stmt, nil); err != nil {
				return err
			}
		}
		for _, entry := range state.Entries {
			if err := insertMemoryEntry(conn, entry); err != nil {
				return err
			}
		}
		for _, assertion := range state.SelfAssertions {
			if err := insertSelfAssertion(conn, assertion); err != nil {
				return err
			}
		}
		for _, item := range state.Todos {
			if err := insertTodoItem(conn, item); err != nil {
				return err
			}
		}
		for key, decision := range state.ExperienceDecisions {
			if err := insertExperienceDecision(conn, key, decision); err != nil {
				return err
			}
		}
		for _, artifact := range state.AutonomousArtifacts {
			if err := insertAutonomousArtifact(conn, artifact); err != nil {
				return err
			}
		}
		for key, rejection := range state.AutonomousRejections {
			if err := insertAutonomousRejection(conn, key, rejection); err != nil {
				return err
			}
		}
		return sqlitex.Execute(conn, `INSERT INTO work_memory_meta(key, int_value, text_value) VALUES ('last_autonomous_run_at', ?1, '')`, &sqlitex.ExecOptions{
			Args: []any{state.LastAutonomousRunAt},
		})
	})
}

func ensureWorkMemorySchema(conn *sqlite.Conn) error {
	return sqlitex.ExecScript(conn, `
CREATE TABLE IF NOT EXISTS work_memory_entries(
  id TEXT PRIMARY KEY,
  source TEXT NOT NULL DEFAULT '',
  content_type TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL DEFAULT '',
  summary TEXT NOT NULL DEFAULT '',
  text TEXT NOT NULL DEFAULT '',
  ocr_text TEXT NOT NULL DEFAULT '',
  ocr_status TEXT NOT NULL DEFAULT '',
  quality_ocr_text TEXT NOT NULL DEFAULT '',
  quality_ocr_status TEXT NOT NULL DEFAULT '',
  window_title TEXT NOT NULL DEFAULT '',
  app_name TEXT NOT NULL DEFAULT '',
  capture_id TEXT NOT NULL DEFAULT '',
  image_path TEXT NOT NULL DEFAULT '',
  image_signature TEXT NOT NULL DEFAULT '',
  image_fingerprint TEXT NOT NULL DEFAULT '',
  frame_count INTEGER NOT NULL DEFAULT 0,
  quality_status TEXT NOT NULL DEFAULT '',
  quality_checked_at INTEGER NOT NULL DEFAULT 0,
  quality_reason TEXT NOT NULL DEFAULT '',
  width INTEGER NOT NULL DEFAULT 0,
  height INTEGER NOT NULL DEFAULT 0,
  bytes INTEGER NOT NULL DEFAULT 0,
  favorite INTEGER NOT NULL DEFAULT 0,
  sensitive INTEGER NOT NULL DEFAULT 0,
  merged_count INTEGER NOT NULL DEFAULT 0,
  last_merged_at INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_work_memory_entries_created_at ON work_memory_entries(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_work_memory_entries_source ON work_memory_entries(source);
CREATE INDEX IF NOT EXISTS idx_work_memory_entries_app ON work_memory_entries(app_name);
CREATE TABLE IF NOT EXISTS work_memory_tags(
  entry_id TEXT NOT NULL,
  position INTEGER NOT NULL,
  value TEXT NOT NULL,
  PRIMARY KEY(entry_id, position),
  FOREIGN KEY(entry_id) REFERENCES work_memory_entries(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS work_memory_frames(
  entry_id TEXT NOT NULL,
  position INTEGER NOT NULL,
  capture_id TEXT NOT NULL DEFAULT '',
  image_path TEXT NOT NULL DEFAULT '',
  image_signature TEXT NOT NULL DEFAULT '',
  image_fingerprint TEXT NOT NULL DEFAULT '',
  width INTEGER NOT NULL DEFAULT 0,
  height INTEGER NOT NULL DEFAULT 0,
  bytes INTEGER NOT NULL DEFAULT 0,
  window_title TEXT NOT NULL DEFAULT '',
  app_name TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY(entry_id, position),
  FOREIGN KEY(entry_id) REFERENCES work_memory_entries(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS work_memory_self_assertions(
  id TEXT PRIMARY KEY,
  category TEXT NOT NULL DEFAULT '',
  key TEXT NOT NULL DEFAULT '',
  label TEXT NOT NULL DEFAULT '',
  value TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT '',
  privacy TEXT NOT NULL DEFAULT '',
  scope TEXT NOT NULL DEFAULT '',
  source TEXT NOT NULL DEFAULT '',
  confidence REAL NOT NULL DEFAULT 0,
  prompt_ready INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL DEFAULT 0,
  updated_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_work_memory_self_assertions_category ON work_memory_self_assertions(category, status);
CREATE TABLE IF NOT EXISTS work_memory_self_assertion_evidence(
  assertion_id TEXT NOT NULL,
  position INTEGER NOT NULL,
  value TEXT NOT NULL,
  PRIMARY KEY(assertion_id, position),
  FOREIGN KEY(assertion_id) REFERENCES work_memory_self_assertions(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS work_memory_todos(
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL DEFAULT '',
  note TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT '',
  priority TEXT NOT NULL DEFAULT '',
  scope TEXT NOT NULL DEFAULT '',
  source TEXT NOT NULL DEFAULT '',
  due_at INTEGER NOT NULL DEFAULT 0,
  remind_at INTEGER NOT NULL DEFAULT 0,
  completed_at INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL DEFAULT 0,
  updated_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_work_memory_todos_status ON work_memory_todos(status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_work_memory_todos_scope ON work_memory_todos(scope);
CREATE TABLE IF NOT EXISTS work_memory_todo_evidence(
  todo_id TEXT NOT NULL,
  position INTEGER NOT NULL,
  value TEXT NOT NULL,
  PRIMARY KEY(todo_id, position),
  FOREIGN KEY(todo_id) REFERENCES work_memory_todos(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS work_memory_decisions(
  insight_id TEXT PRIMARY KEY,
  status TEXT NOT NULL,
  note TEXT NOT NULL DEFAULT '',
  task_package_id TEXT NOT NULL DEFAULT '',
  updated_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS work_memory_autonomous_artifacts(
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  title TEXT NOT NULL,
  summary TEXT NOT NULL DEFAULT '',
  body TEXT NOT NULL DEFAULT '',
  source_insight_id TEXT NOT NULL DEFAULT '',
  dedup_key TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT '',
  delete_reason TEXT NOT NULL DEFAULT '',
  confidence REAL NOT NULL DEFAULT 0,
  agent_executable INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL DEFAULT 0,
  updated_at INTEGER NOT NULL DEFAULT 0,
  deleted_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS work_memory_autonomous_artifact_evidence(
  artifact_id TEXT NOT NULL,
  position INTEGER NOT NULL,
  value TEXT NOT NULL,
  PRIMARY KEY(artifact_id, position),
  FOREIGN KEY(artifact_id) REFERENCES work_memory_autonomous_artifacts(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS work_memory_autonomous_rejections(
  key TEXT PRIMARY KEY,
  artifact_id TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL DEFAULT '',
  reason TEXT NOT NULL DEFAULT '',
  rejected_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS work_memory_meta(
  key TEXT PRIMARY KEY,
  int_value INTEGER NOT NULL DEFAULT 0,
  text_value TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS work_memory_embedding_records(
  entry_id TEXT PRIMARY KEY,
  indexed_at INTEGER NOT NULL DEFAULT 0,
  provider TEXT NOT NULL DEFAULT '',
  model TEXT NOT NULL DEFAULT '',
  vector_store_type TEXT NOT NULL DEFAULT '',
  vector_store_uri TEXT NOT NULL DEFAULT '',
  vector_collection TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS work_memory_embedding_vector_values(
  entry_id TEXT NOT NULL,
  position INTEGER NOT NULL,
  value REAL NOT NULL,
  PRIMARY KEY(entry_id, position),
  FOREIGN KEY(entry_id) REFERENCES work_memory_embedding_records(entry_id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS work_memory_embedding_meta(
  key TEXT PRIMARY KEY,
  int_value INTEGER NOT NULL DEFAULT 0,
  text_value TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS work_memory_flow_conversations(
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL DEFAULT 0,
  updated_at INTEGER NOT NULL DEFAULT 0,
  archived INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_work_memory_flow_conversations_updated_at ON work_memory_flow_conversations(updated_at DESC);
CREATE TABLE IF NOT EXISTS work_memory_flow_messages(
  id TEXT PRIMARY KEY,
  conversation_id TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT '',
  text TEXT NOT NULL DEFAULT '',
  question TEXT NOT NULL DEFAULT '',
  result_json TEXT NOT NULL DEFAULT '',
  error INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL DEFAULT 0,
  FOREIGN KEY(conversation_id) REFERENCES work_memory_flow_conversations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_work_memory_flow_messages_conversation_created ON work_memory_flow_messages(conversation_id, created_at ASC);
`)
}

func readMemoryState(conn *sqlite.Conn, now func() time.Time) (memoryState, bool, error) {
	hasRows, err := hasWorkMemoryRows(conn)
	if err != nil || !hasRows {
		return memoryState{}, false, err
	}
	state := memoryState{
		ExperienceDecisions:  map[string]ExperienceDecision{},
		AutonomousRejections: map[string]AutonomousRejection{},
	}
	if err := sqlitex.Execute(conn, `SELECT id, source, content_type, title, summary, text, ocr_text, ocr_status, quality_ocr_text, quality_ocr_status, window_title, app_name, capture_id, image_path, image_signature, image_fingerprint, frame_count, quality_status, quality_checked_at, quality_reason, width, height, bytes, favorite, sensitive, merged_count, last_merged_at, created_at FROM work_memory_entries ORDER BY created_at DESC`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			state.Entries = append(state.Entries, normalizeEntry(Entry{
				ID:               stmt.ColumnText(0),
				Source:           stmt.ColumnText(1),
				ContentType:      stmt.ColumnText(2),
				Title:            stmt.ColumnText(3),
				Summary:          stmt.ColumnText(4),
				Text:             stmt.ColumnText(5),
				OCRText:          stmt.ColumnText(6),
				OCRStatus:        stmt.ColumnText(7),
				QualityOCRText:   stmt.ColumnText(8),
				QualityOCRStatus: stmt.ColumnText(9),
				WindowTitle:      stmt.ColumnText(10),
				AppName:          stmt.ColumnText(11),
				CaptureID:        stmt.ColumnText(12),
				ImagePath:        stmt.ColumnText(13),
				ImageSignature:   stmt.ColumnText(14),
				ImageFingerprint: stmt.ColumnText(15),
				FrameCount:       stmt.ColumnInt(16),
				QualityStatus:    stmt.ColumnText(17),
				QualityCheckedAt: stmt.ColumnInt64(18),
				QualityReason:    stmt.ColumnText(19),
				Width:            stmt.ColumnInt(20),
				Height:           stmt.ColumnInt(21),
				Bytes:            stmt.ColumnInt64(22),
				Favorite:         appdb.IntBool(stmt.ColumnInt(23)),
				Sensitive:        appdb.IntBool(stmt.ColumnInt(24)),
				MergedCount:      stmt.ColumnInt(25),
				LastMergedAt:     stmt.ColumnInt64(26),
				CreatedAt:        stmt.ColumnInt64(27),
			}))
			return nil
		},
	}); err != nil {
		return memoryState{}, false, err
	}
	for index := range state.Entries {
		tags, err := readWorkMemoryValues(conn, `work_memory_tags`, `entry_id`, state.Entries[index].ID)
		if err != nil {
			return memoryState{}, false, err
		}
		state.Entries[index].Tags = tags
		frames, err := readCaptureFrames(conn, state.Entries[index].ID)
		if err != nil {
			return memoryState{}, false, err
		}
		state.Entries[index].Frames = frames
		state.Entries[index] = normalizeEntry(state.Entries[index])
	}
	assertions, err := readSelfAssertions(conn, now)
	if err != nil {
		return memoryState{}, false, err
	}
	state.SelfAssertions = assertions
	todos, err := readTodoItems(conn, now)
	if err != nil {
		return memoryState{}, false, err
	}
	state.Todos = todos
	if err := readExperienceDecisions(conn, state.ExperienceDecisions); err != nil {
		return memoryState{}, false, err
	}
	artifacts, err := readAutonomousArtifacts(conn, now)
	if err != nil {
		return memoryState{}, false, err
	}
	state.AutonomousArtifacts = artifacts
	if err := readAutonomousRejections(conn, state.AutonomousRejections); err != nil {
		return memoryState{}, false, err
	}
	if err := sqlitex.Execute(conn, `SELECT int_value FROM work_memory_meta WHERE key = 'last_autonomous_run_at'`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			state.LastAutonomousRunAt = stmt.ColumnInt64(0)
			return nil
		},
	}); err != nil {
		return memoryState{}, false, err
	}
	sortEntries(state.Entries)
	return state, true, nil
}

func hasWorkMemoryRows(conn *sqlite.Conn) (bool, error) {
	for _, table := range []string{
		"work_memory_entries",
		"work_memory_self_assertions",
		"work_memory_todos",
		"work_memory_decisions",
		"work_memory_autonomous_artifacts",
		"work_memory_autonomous_rejections",
		"work_memory_meta",
	} {
		count := 0
		if err := sqlitex.Execute(conn, `SELECT count(*) FROM `+table, &sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				count = stmt.ColumnInt(0)
				return nil
			},
		}); err != nil {
			return false, err
		}
		if count > 0 {
			return true, nil
		}
	}
	return false, nil
}

func listFlowConversationsFromSQLite(path string) ([]FlowConversation, error) {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if err := ensureWorkMemorySchema(conn); err != nil {
		return nil, err
	}
	conversations := []FlowConversation{}
	err = sqlitex.Execute(conn, `SELECT
  c.id,
  c.title,
  c.created_at,
  c.updated_at,
  (SELECT count(*) FROM work_memory_flow_messages m WHERE m.conversation_id = c.id),
  COALESCE((SELECT m.text FROM work_memory_flow_messages m WHERE m.conversation_id = c.id ORDER BY m.created_at DESC, m.id DESC LIMIT 1), '')
FROM work_memory_flow_conversations c
WHERE c.archived = 0
ORDER BY c.updated_at DESC, c.created_at DESC`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			conversations = append(conversations, FlowConversation{
				ID:           stmt.ColumnText(0),
				Title:        stmt.ColumnText(1),
				CreatedAt:    stmt.ColumnInt64(2),
				UpdatedAt:    stmt.ColumnInt64(3),
				MessageCount: stmt.ColumnInt(4),
				LastMessage:  stmt.ColumnText(5),
			})
			return nil
		},
	})
	return conversations, err
}

func readFlowConversationFromSQLite(path string, id string) (FlowConversation, bool, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return FlowConversation{}, false, nil
	}
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return FlowConversation{}, false, err
	}
	defer conn.Close()
	if err := ensureWorkMemorySchema(conn); err != nil {
		return FlowConversation{}, false, err
	}
	conversation := FlowConversation{}
	ok := false
	err = sqlitex.Execute(conn, `SELECT
  c.id,
  c.title,
  c.created_at,
  c.updated_at,
  (SELECT count(*) FROM work_memory_flow_messages m WHERE m.conversation_id = c.id),
  COALESCE((SELECT m.text FROM work_memory_flow_messages m WHERE m.conversation_id = c.id ORDER BY m.created_at DESC, m.id DESC LIMIT 1), '')
FROM work_memory_flow_conversations c
WHERE c.id = ?1 AND c.archived = 0`, &sqlitex.ExecOptions{
		Args: []any{id},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			conversation = FlowConversation{
				ID:           stmt.ColumnText(0),
				Title:        stmt.ColumnText(1),
				CreatedAt:    stmt.ColumnInt64(2),
				UpdatedAt:    stmt.ColumnInt64(3),
				MessageCount: stmt.ColumnInt(4),
				LastMessage:  stmt.ColumnText(5),
			}
			ok = true
			return nil
		},
	})
	return conversation, ok, err
}

func listFlowMessagesFromSQLite(path string, conversationID string) ([]FlowMessage, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return []FlowMessage{}, nil
	}
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if err := ensureWorkMemorySchema(conn); err != nil {
		return nil, err
	}
	messages := []FlowMessage{}
	err = sqlitex.Execute(conn, `SELECT id, conversation_id, role, text, question, result_json, error, created_at
FROM work_memory_flow_messages
WHERE conversation_id = ?1
ORDER BY created_at ASC, id ASC`, &sqlitex.ExecOptions{
		Args: []any{conversationID},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			message := FlowMessage{
				ID:             stmt.ColumnText(0),
				ConversationID: stmt.ColumnText(1),
				Role:           stmt.ColumnText(2),
				Text:           stmt.ColumnText(3),
				Question:       stmt.ColumnText(4),
				Error:          appdb.IntBool(stmt.ColumnInt(6)),
				CreatedAt:      stmt.ColumnInt64(7),
			}
			if raw := strings.TrimSpace(stmt.ColumnText(5)); raw != "" {
				var result FlowAskResponse
				if err := json.Unmarshal([]byte(raw), &result); err == nil {
					message.Result = &result
				}
			}
			messages = append(messages, message)
			return nil
		},
	})
	return messages, err
}

func createFlowConversationInSQLite(path string, conversation FlowConversation) (FlowConversation, error) {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return FlowConversation{}, err
	}
	defer conn.Close()
	if err := ensureWorkMemorySchema(conn); err != nil {
		return FlowConversation{}, err
	}
	conversation = normalizeFlowConversation(conversation)
	if conversation.ID == "" {
		return FlowConversation{}, nil
	}
	err = appdb.Immediate(conn, func() error {
		return sqlitex.Execute(conn, `INSERT INTO work_memory_flow_conversations(id, title, created_at, updated_at, archived)
VALUES (?1, ?2, ?3, ?4, 0)
ON CONFLICT(id) DO UPDATE SET title = excluded.title, updated_at = excluded.updated_at, archived = 0`, &sqlitex.ExecOptions{
			Args: []any{conversation.ID, conversation.Title, conversation.CreatedAt, conversation.UpdatedAt},
		})
	})
	if err != nil {
		return FlowConversation{}, err
	}
	saved, ok, err := readFlowConversationFromSQLite(path, conversation.ID)
	if err != nil || !ok {
		return conversation, err
	}
	return saved, nil
}

func deleteFlowConversationFromSQLite(path string, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := ensureWorkMemorySchema(conn); err != nil {
		return err
	}
	return appdb.Immediate(conn, func() error {
		if err := sqlitex.Execute(conn, `DELETE FROM work_memory_flow_messages WHERE conversation_id = ?1`, &sqlitex.ExecOptions{
			Args: []any{id},
		}); err != nil {
			return err
		}
		return sqlitex.Execute(conn, `DELETE FROM work_memory_flow_conversations WHERE id = ?1`, &sqlitex.ExecOptions{
			Args: []any{id},
		})
	})
}

func appendFlowConversationTurnToSQLite(path string, conversation FlowConversation, userMessage FlowMessage, assistantMessage FlowMessage) (FlowConversation, []FlowMessage, error) {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return FlowConversation{}, nil, err
	}
	defer conn.Close()
	if err := ensureWorkMemorySchema(conn); err != nil {
		return FlowConversation{}, nil, err
	}
	conversation = normalizeFlowConversation(conversation)
	userMessage = normalizeFlowMessage(userMessage)
	assistantMessage = normalizeFlowMessage(assistantMessage)
	if conversation.ID == "" {
		return FlowConversation{}, nil, nil
	}
	if userMessage.ConversationID == "" {
		userMessage.ConversationID = conversation.ID
	}
	if assistantMessage.ConversationID == "" {
		assistantMessage.ConversationID = conversation.ID
	}
	err = appdb.Immediate(conn, func() error {
		if err := sqlitex.Execute(conn, `INSERT INTO work_memory_flow_conversations(id, title, created_at, updated_at, archived)
VALUES (?1, ?2, ?3, ?4, 0)
ON CONFLICT(id) DO UPDATE SET title = CASE WHEN work_memory_flow_conversations.title = '' THEN excluded.title ELSE work_memory_flow_conversations.title END, updated_at = excluded.updated_at, archived = 0`, &sqlitex.ExecOptions{
			Args: []any{conversation.ID, conversation.Title, conversation.CreatedAt, conversation.UpdatedAt},
		}); err != nil {
			return err
		}
		if err := insertFlowMessage(conn, userMessage); err != nil {
			return err
		}
		return insertFlowMessage(conn, assistantMessage)
	})
	if err != nil {
		return FlowConversation{}, nil, err
	}
	saved, ok, err := readFlowConversationFromSQLite(path, conversation.ID)
	if err != nil || !ok {
		return conversation, nil, err
	}
	messages, err := listFlowMessagesFromSQLite(path, conversation.ID)
	return saved, messages, err
}

func insertFlowMessage(conn *sqlite.Conn, message FlowMessage) error {
	if message.ID == "" || message.ConversationID == "" || message.Role == "" {
		return nil
	}
	resultJSON := ""
	if message.Result != nil {
		raw, err := json.Marshal(message.Result)
		if err != nil {
			return err
		}
		resultJSON = string(raw)
	}
	return sqlitex.Execute(conn, `INSERT INTO work_memory_flow_messages(id, conversation_id, role, text, question, result_json, error, created_at)
VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8)`, &sqlitex.ExecOptions{
		Args: []any{message.ID, message.ConversationID, message.Role, message.Text, message.Question, resultJSON, appdb.BoolInt(message.Error), message.CreatedAt},
	})
}

func insertMemoryEntry(conn *sqlite.Conn, entry Entry) error {
	entry = normalizeEntry(entry)
	if entry.ID == "" {
		return nil
	}
	if err := sqlitex.Execute(conn, `INSERT INTO work_memory_entries(id, source, content_type, title, summary, text, ocr_text, ocr_status, quality_ocr_text, quality_ocr_status, window_title, app_name, capture_id, image_path, image_signature, image_fingerprint, frame_count, quality_status, quality_checked_at, quality_reason, width, height, bytes, favorite, sensitive, merged_count, last_merged_at, created_at)
VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13, ?14, ?15, ?16, ?17, ?18, ?19, ?20, ?21, ?22, ?23, ?24, ?25, ?26, ?27, ?28)`, &sqlitex.ExecOptions{
		Args: []any{entry.ID, entry.Source, entry.ContentType, entry.Title, entry.Summary, entry.Text, entry.OCRText, entry.OCRStatus, entry.QualityOCRText, entry.QualityOCRStatus, entry.WindowTitle, entry.AppName, entry.CaptureID, entry.ImagePath, entry.ImageSignature, entry.ImageFingerprint, entry.FrameCount, entry.QualityStatus, entry.QualityCheckedAt, entry.QualityReason, entry.Width, entry.Height, entry.Bytes, appdb.BoolInt(entry.Favorite), appdb.BoolInt(entry.Sensitive), entry.MergedCount, entry.LastMergedAt, entry.CreatedAt},
	}); err != nil {
		return err
	}
	if err := insertWorkMemoryValues(conn, `work_memory_tags`, `entry_id`, entry.ID, entry.Tags); err != nil {
		return err
	}
	return insertCaptureFrames(conn, entry.ID, entry.Frames)
}

func insertCaptureFrames(conn *sqlite.Conn, entryID string, frames []CaptureFrame) error {
	for position, frame := range normalizeCaptureFrames(frames) {
		if err := sqlitex.Execute(conn, `INSERT INTO work_memory_frames(entry_id, position, capture_id, image_path, image_signature, image_fingerprint, width, height, bytes, window_title, app_name, created_at)
VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12)`, &sqlitex.ExecOptions{
			Args: []any{entryID, position, frame.CaptureID, frame.ImagePath, frame.ImageSignature, frame.ImageFingerprint, frame.Width, frame.Height, frame.Bytes, frame.WindowTitle, frame.AppName, frame.CreatedAt},
		}); err != nil {
			return err
		}
	}
	return nil
}

func readCaptureFrames(conn *sqlite.Conn, entryID string) ([]CaptureFrame, error) {
	frames := []CaptureFrame{}
	err := sqlitex.Execute(conn, `SELECT capture_id, image_path, image_signature, image_fingerprint, width, height, bytes, window_title, app_name, created_at FROM work_memory_frames WHERE entry_id = ?1 ORDER BY position`, &sqlitex.ExecOptions{
		Args: []any{entryID},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			frames = append(frames, CaptureFrame{
				CaptureID:        stmt.ColumnText(0),
				ImagePath:        stmt.ColumnText(1),
				ImageSignature:   stmt.ColumnText(2),
				ImageFingerprint: stmt.ColumnText(3),
				Width:            stmt.ColumnInt(4),
				Height:           stmt.ColumnInt(5),
				Bytes:            stmt.ColumnInt64(6),
				WindowTitle:      stmt.ColumnText(7),
				AppName:          stmt.ColumnText(8),
				CreatedAt:        stmt.ColumnInt64(9),
			})
			return nil
		},
	})
	return normalizeCaptureFrames(frames), err
}

func insertSelfAssertion(conn *sqlite.Conn, assertion SelfAssertion) error {
	assertion = normalizeSelfAssertion(assertion, time.Now().Unix())
	if assertion.ID == "" {
		return nil
	}
	if err := sqlitex.Execute(conn, `INSERT INTO work_memory_self_assertions(id, category, key, label, value, status, privacy, scope, source, confidence, prompt_ready, created_at, updated_at)
VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13)`, &sqlitex.ExecOptions{
		Args: []any{assertion.ID, assertion.Category, assertion.Key, assertion.Label, assertion.Value, assertion.Status, assertion.Privacy, assertion.Scope, assertion.Source, assertion.Confidence, appdb.BoolInt(assertion.PromptReady), assertion.CreatedAt, assertion.UpdatedAt},
	}); err != nil {
		return err
	}
	return insertWorkMemoryValues(conn, `work_memory_self_assertion_evidence`, `assertion_id`, assertion.ID, assertion.Evidence)
}

func readSelfAssertions(conn *sqlite.Conn, now func() time.Time) ([]SelfAssertion, error) {
	assertions := []SelfAssertion{}
	err := sqlitex.Execute(conn, `SELECT id, category, key, label, value, status, privacy, scope, source, confidence, prompt_ready, created_at, updated_at FROM work_memory_self_assertions ORDER BY updated_at DESC`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			assertions = append(assertions, SelfAssertion{
				ID:          stmt.ColumnText(0),
				Category:    stmt.ColumnText(1),
				Key:         stmt.ColumnText(2),
				Label:       stmt.ColumnText(3),
				Value:       stmt.ColumnText(4),
				Status:      stmt.ColumnText(5),
				Privacy:     stmt.ColumnText(6),
				Scope:       stmt.ColumnText(7),
				Source:      stmt.ColumnText(8),
				Confidence:  stmt.ColumnFloat(9),
				PromptReady: appdb.IntBool(stmt.ColumnInt(10)),
				CreatedAt:   stmt.ColumnInt64(11),
				UpdatedAt:   stmt.ColumnInt64(12),
			})
			return nil
		},
	})
	if err != nil {
		return nil, err
	}
	for index := range assertions {
		evidence, err := readWorkMemoryValues(conn, `work_memory_self_assertion_evidence`, `assertion_id`, assertions[index].ID)
		if err != nil {
			return nil, err
		}
		assertions[index].Evidence = evidence
		assertions[index] = normalizeSelfAssertion(assertions[index], now().Unix())
	}
	sortSelfAssertions(assertions)
	return assertions, nil
}

func insertTodoItem(conn *sqlite.Conn, item TodoItem) error {
	item = normalizeTodoItem(item, time.Now().Unix())
	if item.ID == "" || item.Title == "" {
		return nil
	}
	if err := sqlitex.Execute(conn, `INSERT INTO work_memory_todos(id, title, note, status, priority, scope, source, due_at, remind_at, completed_at, created_at, updated_at)
VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12)`, &sqlitex.ExecOptions{
		Args: []any{item.ID, item.Title, item.Note, item.Status, item.Priority, item.Scope, item.Source, item.DueAt, item.RemindAt, item.CompletedAt, item.CreatedAt, item.UpdatedAt},
	}); err != nil {
		return err
	}
	return insertWorkMemoryValues(conn, `work_memory_todo_evidence`, `todo_id`, item.ID, item.Evidence)
}

func readTodoItems(conn *sqlite.Conn, now func() time.Time) ([]TodoItem, error) {
	items := []TodoItem{}
	err := sqlitex.Execute(conn, `SELECT id, title, note, status, priority, scope, source, due_at, remind_at, completed_at, created_at, updated_at FROM work_memory_todos ORDER BY updated_at DESC`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			items = append(items, TodoItem{
				ID:          stmt.ColumnText(0),
				Title:       stmt.ColumnText(1),
				Note:        stmt.ColumnText(2),
				Status:      stmt.ColumnText(3),
				Priority:    stmt.ColumnText(4),
				Scope:       stmt.ColumnText(5),
				Source:      stmt.ColumnText(6),
				DueAt:       stmt.ColumnInt64(7),
				RemindAt:    stmt.ColumnInt64(8),
				CompletedAt: stmt.ColumnInt64(9),
				CreatedAt:   stmt.ColumnInt64(10),
				UpdatedAt:   stmt.ColumnInt64(11),
			})
			return nil
		},
	})
	if err != nil {
		return nil, err
	}
	for index := range items {
		evidence, err := readWorkMemoryValues(conn, `work_memory_todo_evidence`, `todo_id`, items[index].ID)
		if err != nil {
			return nil, err
		}
		items[index].Evidence = evidence
		items[index] = normalizeTodoItem(items[index], now().Unix())
	}
	sortTodoItems(items)
	return items, nil
}

func insertExperienceDecision(conn *sqlite.Conn, key string, decision ExperienceDecision) error {
	decision = normalizeExperienceDecision(key, decision)
	if decision.InsightID == "" || decision.Status == "" {
		return nil
	}
	return sqlitex.Execute(conn, `INSERT INTO work_memory_decisions(insight_id, status, note, task_package_id, updated_at) VALUES (?1, ?2, ?3, ?4, ?5)`, &sqlitex.ExecOptions{
		Args: []any{decision.InsightID, decision.Status, decision.Note, decision.TaskPackageID, decision.UpdatedAt},
	})
}

func readExperienceDecisions(conn *sqlite.Conn, target map[string]ExperienceDecision) error {
	return sqlitex.Execute(conn, `SELECT insight_id, status, note, task_package_id, updated_at FROM work_memory_decisions`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			decision := normalizeExperienceDecision(stmt.ColumnText(0), ExperienceDecision{
				InsightID:     stmt.ColumnText(0),
				Status:        stmt.ColumnText(1),
				Note:          stmt.ColumnText(2),
				TaskPackageID: stmt.ColumnText(3),
				UpdatedAt:     stmt.ColumnInt64(4),
			})
			if decision.InsightID != "" && decision.Status != "" {
				target[decision.InsightID] = decision
			}
			return nil
		},
	})
}

func insertAutonomousArtifact(conn *sqlite.Conn, artifact AutonomousArtifact) error {
	artifact = normalizeAutonomousArtifact(artifact, time.Now())
	if artifact.ID == "" || artifact.Kind == "" {
		return nil
	}
	if err := sqlitex.Execute(conn, `INSERT INTO work_memory_autonomous_artifacts(id, kind, title, summary, body, source_insight_id, dedup_key, status, delete_reason, confidence, agent_executable, created_at, updated_at, deleted_at)
VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13, ?14)`, &sqlitex.ExecOptions{
		Args: []any{artifact.ID, artifact.Kind, artifact.Title, artifact.Summary, artifact.Body, artifact.SourceInsightID, artifact.DedupKey, artifact.Status, artifact.DeleteReason, artifact.Confidence, appdb.BoolInt(artifact.AgentExecutable), artifact.CreatedAt, artifact.UpdatedAt, artifact.DeletedAt},
	}); err != nil {
		return err
	}
	return insertWorkMemoryValues(conn, `work_memory_autonomous_artifact_evidence`, `artifact_id`, artifact.ID, artifact.Evidence)
}

func readAutonomousArtifacts(conn *sqlite.Conn, now func() time.Time) ([]AutonomousArtifact, error) {
	artifacts := []AutonomousArtifact{}
	err := sqlitex.Execute(conn, `SELECT id, kind, title, summary, body, source_insight_id, dedup_key, status, delete_reason, confidence, agent_executable, created_at, updated_at, deleted_at FROM work_memory_autonomous_artifacts ORDER BY created_at DESC`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			artifacts = append(artifacts, AutonomousArtifact{
				ID:              stmt.ColumnText(0),
				Kind:            stmt.ColumnText(1),
				Title:           stmt.ColumnText(2),
				Summary:         stmt.ColumnText(3),
				Body:            stmt.ColumnText(4),
				SourceInsightID: stmt.ColumnText(5),
				DedupKey:        stmt.ColumnText(6),
				Status:          stmt.ColumnText(7),
				DeleteReason:    stmt.ColumnText(8),
				Confidence:      stmt.ColumnFloat(9),
				AgentExecutable: appdb.IntBool(stmt.ColumnInt(10)),
				CreatedAt:       stmt.ColumnInt64(11),
				UpdatedAt:       stmt.ColumnInt64(12),
				DeletedAt:       stmt.ColumnInt64(13),
			})
			return nil
		},
	})
	if err != nil {
		return nil, err
	}
	for index := range artifacts {
		evidence, err := readWorkMemoryValues(conn, `work_memory_autonomous_artifact_evidence`, `artifact_id`, artifacts[index].ID)
		if err != nil {
			return nil, err
		}
		artifacts[index].Evidence = evidence
		artifacts[index] = normalizeAutonomousArtifact(artifacts[index], now())
	}
	return artifacts, nil
}

func insertAutonomousRejection(conn *sqlite.Conn, key string, rejection AutonomousRejection) error {
	key = strings.TrimSpace(strings.ToLower(firstNonEmpty(key, rejection.Key)))
	if key == "" {
		return nil
	}
	rejection.Key = key
	return sqlitex.Execute(conn, `INSERT INTO work_memory_autonomous_rejections(key, artifact_id, kind, title, reason, rejected_at) VALUES (?1, ?2, ?3, ?4, ?5, ?6)`, &sqlitex.ExecOptions{
		Args: []any{rejection.Key, rejection.ArtifactID, rejection.Kind, rejection.Title, rejection.Reason, rejection.RejectedAt},
	})
}

func readAutonomousRejections(conn *sqlite.Conn, target map[string]AutonomousRejection) error {
	return sqlitex.Execute(conn, `SELECT key, artifact_id, kind, title, reason, rejected_at FROM work_memory_autonomous_rejections`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			key := strings.TrimSpace(strings.ToLower(stmt.ColumnText(0)))
			if key != "" {
				target[key] = AutonomousRejection{
					Key:        key,
					ArtifactID: stmt.ColumnText(1),
					Kind:       stmt.ColumnText(2),
					Title:      stmt.ColumnText(3),
					Reason:     stmt.ColumnText(4),
					RejectedAt: stmt.ColumnInt64(5),
				}
			}
			return nil
		},
	})
}

func insertWorkMemoryValues(conn *sqlite.Conn, table string, idColumn string, id string, values []string) error {
	for position, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if err := sqlitex.Execute(conn, `INSERT INTO `+table+`(`+idColumn+`, position, value) VALUES (?1, ?2, ?3)`, &sqlitex.ExecOptions{
			Args: []any{id, position, value},
		}); err != nil {
			return err
		}
	}
	return nil
}

func readWorkMemoryValues(conn *sqlite.Conn, table string, idColumn string, id string) ([]string, error) {
	values := []string{}
	err := sqlitex.Execute(conn, `SELECT value FROM `+table+` WHERE `+idColumn+` = ?1 ORDER BY position`, &sqlitex.ExecOptions{
		Args: []any{id},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			values = append(values, stmt.ColumnText(0))
			return nil
		},
	})
	return values, err
}

func loadEmbeddingStateFromSQLite(path string) (embeddingStateFile, bool, error) {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return embeddingStateFile{}, false, err
	}
	defer conn.Close()
	if err := ensureWorkMemorySchema(conn); err != nil {
		return embeddingStateFile{}, false, err
	}
	payload, ok, err := readEmbeddingState(conn)
	if err != nil || ok {
		return payload, ok, err
	}
	if loaded, err := appdb.LegacyLoadJSON(path, "work_memory_vectors", &payload); err != nil || !loaded {
		return embeddingStateFile{}, false, err
	}
	if err := saveEmbeddingStateToSQLite(path, payload); err != nil {
		return embeddingStateFile{}, false, err
	}
	_ = appdb.DropLegacyDocument(path, "work_memory_vectors")
	return payload, true, nil
}

func saveEmbeddingStateToSQLite(path string, payload embeddingStateFile) error {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := ensureWorkMemorySchema(conn); err != nil {
		return err
	}
	return appdb.Immediate(conn, func() error {
		for _, stmt := range []string{
			`DELETE FROM work_memory_embedding_vector_values`,
			`DELETE FROM work_memory_embedding_records`,
			`DELETE FROM work_memory_embedding_meta`,
		} {
			if err := sqlitex.Execute(conn, stmt, nil); err != nil {
				return err
			}
		}
		if err := sqlitex.Execute(conn, `INSERT INTO work_memory_embedding_meta(key, int_value, text_value) VALUES ('last_indexed_at', ?1, '')`, &sqlitex.ExecOptions{
			Args: []any{payload.LastIndexedAt},
		}); err != nil {
			return err
		}
		for key, value := range map[string]string{
			"provider":          payload.Provider,
			"model":             payload.Model,
			"vector_store_type": payload.VectorStoreType,
			"vector_store_uri":  payload.VectorStoreURI,
			"vector_collection": payload.VectorCollection,
		} {
			if err := sqlitex.Execute(conn, `INSERT INTO work_memory_embedding_meta(key, int_value, text_value) VALUES (?1, 0, ?2)`, &sqlitex.ExecOptions{
				Args: []any{key, value},
			}); err != nil {
				return err
			}
		}
		records := append([]embeddingRecord(nil), payload.Records...)
		sort.SliceStable(records, func(i, j int) bool {
			return records[i].EntryID < records[j].EntryID
		})
		for _, record := range records {
			if err := insertEmbeddingRecord(conn, payload, record); err != nil {
				return err
			}
		}
		return nil
	})
}

func readEmbeddingState(conn *sqlite.Conn) (embeddingStateFile, bool, error) {
	hasRows := false
	for _, table := range []string{"work_memory_embedding_records", "work_memory_embedding_meta"} {
		count := 0
		if err := sqlitex.Execute(conn, `SELECT count(*) FROM `+table, &sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				count = stmt.ColumnInt(0)
				return nil
			},
		}); err != nil {
			return embeddingStateFile{}, false, err
		}
		if count > 0 {
			hasRows = true
		}
	}
	if !hasRows {
		return embeddingStateFile{}, false, nil
	}
	payload := embeddingStateFile{Version: 1}
	if err := sqlitex.Execute(conn, `SELECT key, int_value, text_value FROM work_memory_embedding_meta`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			switch stmt.ColumnText(0) {
			case "last_indexed_at":
				payload.LastIndexedAt = stmt.ColumnInt64(1)
			case "provider":
				payload.Provider = stmt.ColumnText(2)
			case "model":
				payload.Model = stmt.ColumnText(2)
			case "vector_store_type":
				payload.VectorStoreType = stmt.ColumnText(2)
			case "vector_store_uri":
				payload.VectorStoreURI = stmt.ColumnText(2)
			case "vector_collection":
				payload.VectorCollection = stmt.ColumnText(2)
			}
			return nil
		},
	}); err != nil {
		return embeddingStateFile{}, false, err
	}
	if err := sqlitex.Execute(conn, `SELECT entry_id, indexed_at, provider, model, vector_store_type, vector_store_uri, vector_collection FROM work_memory_embedding_records ORDER BY entry_id`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			payload.Records = append(payload.Records, embeddingRecord{
				EntryID:   stmt.ColumnText(0),
				IndexedAt: stmt.ColumnInt64(1),
			})
			if payload.Provider == "" {
				payload.Provider = stmt.ColumnText(2)
			}
			if payload.Model == "" {
				payload.Model = stmt.ColumnText(3)
			}
			if payload.VectorStoreType == "" {
				payload.VectorStoreType = stmt.ColumnText(4)
			}
			if payload.VectorStoreURI == "" {
				payload.VectorStoreURI = stmt.ColumnText(5)
			}
			if payload.VectorCollection == "" {
				payload.VectorCollection = stmt.ColumnText(6)
			}
			return nil
		},
	}); err != nil {
		return embeddingStateFile{}, false, err
	}
	for index := range payload.Records {
		vector, err := readEmbeddingVector(conn, payload.Records[index].EntryID)
		if err != nil {
			return embeddingStateFile{}, false, err
		}
		payload.Records[index].Vector = vector
	}
	return payload, true, nil
}

func insertEmbeddingRecord(conn *sqlite.Conn, payload embeddingStateFile, record embeddingRecord) error {
	record.EntryID = strings.TrimSpace(record.EntryID)
	if record.EntryID == "" {
		return nil
	}
	if err := sqlitex.Execute(conn, `INSERT INTO work_memory_embedding_records(entry_id, indexed_at, provider, model, vector_store_type, vector_store_uri, vector_collection)
VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7)`, &sqlitex.ExecOptions{
		Args: []any{record.EntryID, record.IndexedAt, payload.Provider, payload.Model, payload.VectorStoreType, payload.VectorStoreURI, payload.VectorCollection},
	}); err != nil {
		return err
	}
	for position, value := range record.Vector {
		if err := sqlitex.Execute(conn, `INSERT INTO work_memory_embedding_vector_values(entry_id, position, value) VALUES (?1, ?2, ?3)`, &sqlitex.ExecOptions{
			Args: []any{record.EntryID, position, value},
		}); err != nil {
			return err
		}
	}
	return nil
}

func readEmbeddingVector(conn *sqlite.Conn, entryID string) ([]float64, error) {
	vector := []float64{}
	err := sqlitex.Execute(conn, `SELECT value FROM work_memory_embedding_vector_values WHERE entry_id = ?1 ORDER BY position`, &sqlitex.ExecOptions{
		Args: []any{entryID},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			vector = append(vector, stmt.ColumnFloat(0))
			return nil
		},
	})
	return vector, err
}
