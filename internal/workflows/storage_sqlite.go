package workflows

import (
	"sort"

	"ariadne/internal/appdb"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

type workflowState struct {
	Workflows []Workflow
	Removed   map[string]bool
}

func loadWorkflowStateFromSQLite(path string) (workflowState, bool, error) {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return workflowState{}, false, err
	}
	defer conn.Close()
	if err := ensureWorkflowSchema(conn); err != nil {
		return workflowState{}, false, err
	}
	state, ok, err := readWorkflowState(conn)
	if err != nil || ok {
		return state, ok, err
	}
	var legacy struct {
		Version    int        `json:"version"`
		Workflows  []Workflow `json:"workflows"`
		RemovedIDs []string   `json:"removedIds,omitempty"`
	}
	if loaded, err := appdb.LegacyLoadJSON(path, "workflows", &legacy); err != nil || !loaded {
		return workflowState{}, false, err
	}
	state = workflowState{Workflows: legacy.Workflows, Removed: map[string]bool{}}
	for _, id := range legacy.RemovedIDs {
		if id != "" {
			state.Removed[id] = true
		}
	}
	if err := saveWorkflowStateToSQLite(path, state); err != nil {
		return workflowState{}, false, err
	}
	_ = appdb.DropLegacyDocument(path, "workflows")
	return state, true, nil
}

func saveWorkflowStateToSQLite(path string, state workflowState) error {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := ensureWorkflowSchema(conn); err != nil {
		return err
	}
	return appdb.Immediate(conn, func() error {
		for _, stmt := range []string{`DELETE FROM workflow_steps`, `DELETE FROM workflow_removed`, `DELETE FROM workflows`} {
			if err := sqlitex.Execute(conn, stmt, nil); err != nil {
				return err
			}
		}
		for _, workflow := range state.Workflows {
			if err := insertWorkflow(conn, workflow); err != nil {
				return err
			}
		}
		removed := make([]string, 0, len(state.Removed))
		for id := range state.Removed {
			if id != "" {
				removed = append(removed, id)
			}
		}
		sort.Strings(removed)
		for _, id := range removed {
			if err := sqlitex.Execute(conn, `INSERT INTO workflow_removed(id) VALUES (?1)`, &sqlitex.ExecOptions{Args: []any{id}}); err != nil {
				return err
			}
		}
		return nil
	})
}

func ensureWorkflowSchema(conn *sqlite.Conn) error {
	return sqlitex.ExecScript(conn, `
CREATE TABLE IF NOT EXISTS workflows(
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  updated_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS workflow_steps(
  workflow_id TEXT NOT NULL,
  position INTEGER NOT NULL,
  command TEXT NOT NULL,
  pick TEXT NOT NULL DEFAULT '',
  PRIMARY KEY(workflow_id, position),
  FOREIGN KEY(workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS workflow_removed(
  id TEXT PRIMARY KEY
);
`)
}

func readWorkflowState(conn *sqlite.Conn) (workflowState, bool, error) {
	count := 0
	if err := sqlitex.Execute(conn, `SELECT count(*) FROM workflows`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error { count = stmt.ColumnInt(0); return nil },
	}); err != nil || count == 0 {
		return workflowState{}, false, err
	}
	state := workflowState{Workflows: make([]Workflow, 0, count), Removed: map[string]bool{}}
	err := sqlitex.Execute(conn, `SELECT id, name, description, updated_at FROM workflows`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			state.Workflows = append(state.Workflows, Workflow{
				ID:          stmt.ColumnText(0),
				Name:        stmt.ColumnText(1),
				Description: stmt.ColumnText(2),
				UpdatedAt:   stmt.ColumnInt64(3),
			})
			return nil
		},
	})
	if err != nil {
		return workflowState{}, false, err
	}
	for index := range state.Workflows {
		steps, err := readWorkflowSteps(conn, state.Workflows[index].ID)
		if err != nil {
			return workflowState{}, false, err
		}
		state.Workflows[index].Steps = steps
	}
	if err := sqlitex.Execute(conn, `SELECT id FROM workflow_removed`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			state.Removed[stmt.ColumnText(0)] = true
			return nil
		},
	}); err != nil {
		return workflowState{}, false, err
	}
	sortWorkflows(state.Workflows)
	return state, true, nil
}

func insertWorkflow(conn *sqlite.Conn, workflow Workflow) error {
	workflow, ok := normalizeWorkflow(workflow)
	if !ok {
		return nil
	}
	if err := sqlitex.Execute(conn, `INSERT INTO workflows(id, name, description, updated_at) VALUES (?1, ?2, ?3, ?4)`, &sqlitex.ExecOptions{
		Args: []any{workflow.ID, workflow.Name, workflow.Description, workflow.UpdatedAt},
	}); err != nil {
		return err
	}
	for position, step := range workflow.Steps {
		if step.Command == "" {
			continue
		}
		if err := sqlitex.Execute(conn, `INSERT INTO workflow_steps(workflow_id, position, command, pick) VALUES (?1, ?2, ?3, ?4)`, &sqlitex.ExecOptions{
			Args: []any{workflow.ID, position, step.Command, step.Pick},
		}); err != nil {
			return err
		}
	}
	return nil
}

func readWorkflowSteps(conn *sqlite.Conn, workflowID string) ([]Step, error) {
	steps := []Step{}
	err := sqlitex.Execute(conn, `SELECT command, pick FROM workflow_steps WHERE workflow_id = ?1 ORDER BY position`, &sqlitex.ExecOptions{
		Args: []any{workflowID},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			steps = append(steps, Step{Command: stmt.ColumnText(0), Pick: stmt.ColumnText(1)})
			return nil
		},
	})
	return steps, err
}
