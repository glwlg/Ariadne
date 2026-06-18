package imageindex

import (
	"sort"

	"ariadne/internal/appdb"
	"ariadne/internal/ocr"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func loadImageIndexFromSQLite(path string) ([]Entry, bool, error) {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return nil, false, err
	}
	defer conn.Close()
	if err := ensureImageIndexSchema(conn); err != nil {
		return nil, false, err
	}
	entries, ok, err := readImageIndexEntries(conn)
	if err != nil || ok {
		return entries, ok, err
	}
	var legacy struct {
		Version int     `json:"version"`
		Entries []Entry `json:"entries"`
	}
	if loaded, err := appdb.LegacyLoadJSON(path, "image_index", &legacy); err != nil || !loaded {
		return nil, false, err
	}
	entries = normalizeEntries(legacy.Entries)
	if err := saveImageIndexToSQLite(path, entries); err != nil {
		return nil, false, err
	}
	_ = appdb.DropLegacyDocument(path, "image_index")
	return entries, true, nil
}

func saveImageIndexToSQLite(path string, entries []Entry) error {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := ensureImageIndexSchema(conn); err != nil {
		return err
	}
	return appdb.Immediate(conn, func() error {
		if err := sqlitex.Execute(conn, `DELETE FROM image_index_lines`, nil); err != nil {
			return err
		}
		if err := sqlitex.Execute(conn, `DELETE FROM image_index_entries`, nil); err != nil {
			return err
		}
		for _, entry := range entries {
			if err := insertImageIndexEntry(conn, entry); err != nil {
				return err
			}
		}
		return nil
	})
}

func ensureImageIndexSchema(conn *sqlite.Conn) error {
	return sqlitex.ExecScript(conn, `
CREATE TABLE IF NOT EXISTS image_index_entries(
  id TEXT PRIMARY KEY,
  source TEXT NOT NULL,
  source_id TEXT NOT NULL,
  image_path TEXT NOT NULL,
  text TEXT NOT NULL DEFAULT '',
  provider TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL DEFAULT 0,
  indexed_at INTEGER NOT NULL DEFAULT 0,
  width INTEGER NOT NULL DEFAULT 0,
  height INTEGER NOT NULL DEFAULT 0,
  ok INTEGER NOT NULL DEFAULT 0,
  sensitive INTEGER NOT NULL DEFAULT 0,
  redacted INTEGER NOT NULL DEFAULT 0,
  error TEXT NOT NULL DEFAULT '',
  recognized_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_image_index_source ON image_index_entries(source, source_id);
CREATE INDEX IF NOT EXISTS idx_image_index_indexed_at ON image_index_entries(indexed_at DESC);
CREATE TABLE IF NOT EXISTS image_index_lines(
  entry_id TEXT NOT NULL,
  position INTEGER NOT NULL,
  text TEXT NOT NULL,
  confidence REAL NOT NULL DEFAULT 0,
  x INTEGER NOT NULL DEFAULT 0,
  y INTEGER NOT NULL DEFAULT 0,
  width INTEGER NOT NULL DEFAULT 0,
  height INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY(entry_id, position),
  FOREIGN KEY(entry_id) REFERENCES image_index_entries(id) ON DELETE CASCADE
);
`)
}

func readImageIndexEntries(conn *sqlite.Conn) ([]Entry, bool, error) {
	count := 0
	if err := sqlitex.Execute(conn, `SELECT count(*) FROM image_index_entries`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error { count = stmt.ColumnInt(0); return nil },
	}); err != nil || count == 0 {
		return nil, false, err
	}
	entries := make([]Entry, 0, count)
	err := sqlitex.Execute(conn, `SELECT id, source, source_id, image_path, text, provider, created_at, indexed_at, width, height, ok, sensitive, redacted, error, recognized_at FROM image_index_entries`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			entries = append(entries, Entry{
				ID:           stmt.ColumnText(0),
				Source:       stmt.ColumnText(1),
				SourceID:     stmt.ColumnText(2),
				ImagePath:    stmt.ColumnText(3),
				Text:         stmt.ColumnText(4),
				Provider:     stmt.ColumnText(5),
				CreatedAt:    stmt.ColumnInt64(6),
				IndexedAt:    stmt.ColumnInt64(7),
				Width:        stmt.ColumnInt(8),
				Height:       stmt.ColumnInt(9),
				OK:           stmt.ColumnInt(10) != 0,
				Sensitive:    stmt.ColumnInt(11) != 0,
				Redacted:     stmt.ColumnInt(12) != 0,
				Error:        stmt.ColumnText(13),
				RecognizedAt: stmt.ColumnInt64(14),
			})
			return nil
		},
	})
	if err != nil {
		return nil, false, err
	}
	for index := range entries {
		lines, err := readImageIndexLines(conn, entries[index].ID)
		if err != nil {
			return nil, false, err
		}
		entries[index].Lines = lines
	}
	return normalizeEntries(entries), true, nil
}

func insertImageIndexEntry(conn *sqlite.Conn, entry Entry) error {
	entry = normalizeEntry(entry)
	if entry.ID == "" {
		return nil
	}
	if err := sqlitex.Execute(conn, `INSERT INTO image_index_entries(id, source, source_id, image_path, text, provider, created_at, indexed_at, width, height, ok, sensitive, redacted, error, recognized_at)
VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13, ?14, ?15)`, &sqlitex.ExecOptions{
		Args: []any{entry.ID, entry.Source, entry.SourceID, entry.ImagePath, entry.Text, entry.Provider, entry.CreatedAt, entry.IndexedAt, entry.Width, entry.Height, appdb.BoolInt(entry.OK), appdb.BoolInt(entry.Sensitive), appdb.BoolInt(entry.Redacted), entry.Error, entry.RecognizedAt},
	}); err != nil {
		return err
	}
	for position, line := range entry.Lines {
		if err := sqlitex.Execute(conn, `INSERT INTO image_index_lines(entry_id, position, text, confidence, x, y, width, height) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8)`, &sqlitex.ExecOptions{
			Args: []any{entry.ID, position, line.Text, line.Confidence, line.Rect.X, line.Rect.Y, line.Rect.Width, line.Rect.Height},
		}); err != nil {
			return err
		}
	}
	return nil
}

func readImageIndexLines(conn *sqlite.Conn, entryID string) ([]ocr.Line, error) {
	lines := []ocr.Line{}
	err := sqlitex.Execute(conn, `SELECT text, confidence, x, y, width, height FROM image_index_lines WHERE entry_id = ?1 ORDER BY position`, &sqlitex.ExecOptions{
		Args: []any{entryID},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			lines = append(lines, ocr.Line{
				Text:       stmt.ColumnText(0),
				Confidence: stmt.ColumnFloat(1),
				Rect: ocr.Rect{
					X:      stmt.ColumnInt(2),
					Y:      stmt.ColumnInt(3),
					Width:  stmt.ColumnInt(4),
					Height: stmt.ColumnInt(5),
				},
			})
			return nil
		},
	})
	return lines, err
}

func normalizeEntries(entries []Entry) []Entry {
	result := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		entry = normalizeEntry(entry)
		if entry.ID != "" {
			result = append(result, entry)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].IndexedAt > result[j].IndexedAt
	})
	return result
}
