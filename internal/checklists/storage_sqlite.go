package checklists

import (
	"ariadne/internal/appdb"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func loadChecklistsFromSQLite(path string) ([]Checklist, bool, error) {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return nil, false, err
	}
	defer conn.Close()
	if err := ensureChecklistSchema(conn); err != nil {
		return nil, false, err
	}
	checklists, ok, err := readChecklists(conn)
	if err != nil || ok {
		return checklists, ok, err
	}
	var payload stateFile
	if loaded, err := appdb.LegacyLoadJSON(path, "checklists", &payload); err == nil && loaded {
		checklists = normalizeChecklists(payload.Checklists)
		if err := saveChecklistsToSQLite(path, checklists); err != nil {
			return nil, false, err
		}
		_ = appdb.DropLegacyDocument(path, "checklists")
		return checklists, true, nil
	} else if err != nil {
		var legacy []Checklist
		if loaded, legacyErr := appdb.LegacyLoadJSON(path, "checklists", &legacy); legacyErr != nil || !loaded {
			return nil, false, err
		}
		checklists = normalizeChecklists(legacy)
		if err := saveChecklistsToSQLite(path, checklists); err != nil {
			return nil, false, err
		}
		_ = appdb.DropLegacyDocument(path, "checklists")
		return checklists, true, nil
	}
	return nil, false, nil
}

func saveChecklistsToSQLite(path string, checklists []Checklist) error {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := ensureChecklistSchema(conn); err != nil {
		return err
	}
	return appdb.Immediate(conn, func() error {
		for _, stmt := range []string{`DELETE FROM checklist_items`, `DELETE FROM checklist_evidence`, `DELETE FROM checklists`} {
			if err := sqlitex.Execute(conn, stmt, nil); err != nil {
				return err
			}
		}
		for _, checklist := range checklists {
			if err := insertChecklist(conn, checklist); err != nil {
				return err
			}
		}
		return nil
	})
}

func ensureChecklistSchema(conn *sqlite.Conn) error {
	return sqlitex.ExecScript(conn, `
CREATE TABLE IF NOT EXISTS checklists(
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  context TEXT NOT NULL DEFAULT '',
  source TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL DEFAULT 0,
  updated_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS checklist_items(
  checklist_id TEXT NOT NULL,
  position INTEGER NOT NULL,
  value TEXT NOT NULL,
  PRIMARY KEY(checklist_id, position),
  FOREIGN KEY(checklist_id) REFERENCES checklists(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS checklist_evidence(
  checklist_id TEXT NOT NULL,
  position INTEGER NOT NULL,
  value TEXT NOT NULL,
  PRIMARY KEY(checklist_id, position),
  FOREIGN KEY(checklist_id) REFERENCES checklists(id) ON DELETE CASCADE
);
`)
}

func readChecklists(conn *sqlite.Conn) ([]Checklist, bool, error) {
	count := 0
	if err := sqlitex.Execute(conn, `SELECT count(*) FROM checklists`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error { count = stmt.ColumnInt(0); return nil },
	}); err != nil || count == 0 {
		return nil, false, err
	}
	checklists := make([]Checklist, 0, count)
	err := sqlitex.Execute(conn, `SELECT id, title, context, source, created_at, updated_at FROM checklists`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			checklists = append(checklists, Checklist{
				ID:        stmt.ColumnText(0),
				Title:     stmt.ColumnText(1),
				Context:   stmt.ColumnText(2),
				Source:    stmt.ColumnText(3),
				CreatedAt: stmt.ColumnInt64(4),
				UpdatedAt: stmt.ColumnInt64(5),
			})
			return nil
		},
	})
	if err != nil {
		return nil, false, err
	}
	for index := range checklists {
		values, err := readChecklistValues(conn, `checklist_items`, checklists[index].ID)
		if err != nil {
			return nil, false, err
		}
		checklists[index].Items = values
		values, err = readChecklistValues(conn, `checklist_evidence`, checklists[index].ID)
		if err != nil {
			return nil, false, err
		}
		checklists[index].Evidence = values
	}
	return normalizeChecklists(checklists), true, nil
}

func insertChecklist(conn *sqlite.Conn, checklist Checklist) error {
	checklist, ok := normalizeChecklist(checklist)
	if !ok {
		return nil
	}
	if err := sqlitex.Execute(conn, `INSERT INTO checklists(id, title, context, source, created_at, updated_at) VALUES (?1, ?2, ?3, ?4, ?5, ?6)`, &sqlitex.ExecOptions{
		Args: []any{checklist.ID, checklist.Title, checklist.Context, checklist.Source, checklist.CreatedAt, checklist.UpdatedAt},
	}); err != nil {
		return err
	}
	if err := insertChecklistValues(conn, `checklist_items`, checklist.ID, checklist.Items); err != nil {
		return err
	}
	return insertChecklistValues(conn, `checklist_evidence`, checklist.ID, checklist.Evidence)
}

func insertChecklistValues(conn *sqlite.Conn, table string, checklistID string, values []string) error {
	for position, value := range values {
		if value == "" {
			continue
		}
		if err := sqlitex.Execute(conn, `INSERT INTO `+table+`(checklist_id, position, value) VALUES (?1, ?2, ?3)`, &sqlitex.ExecOptions{
			Args: []any{checklistID, position, value},
		}); err != nil {
			return err
		}
	}
	return nil
}

func readChecklistValues(conn *sqlite.Conn, table string, checklistID string) ([]string, error) {
	values := []string{}
	err := sqlitex.Execute(conn, `SELECT value FROM `+table+` WHERE checklist_id = ?1 ORDER BY position`, &sqlitex.ExecOptions{
		Args: []any{checklistID},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			values = append(values, stmt.ColumnText(0))
			return nil
		},
	})
	return values, err
}
