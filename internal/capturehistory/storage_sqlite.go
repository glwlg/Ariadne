package capturehistory

import (
	"sort"

	"ariadne/internal/appdb"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func loadEntriesFromSQLite(path string) ([]Entry, bool, error) {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return nil, false, err
	}
	defer conn.Close()
	if err := ensureCaptureSchema(conn); err != nil {
		return nil, false, err
	}
	entries, ok, err := readCaptureEntries(conn)
	if err != nil || ok {
		return entries, ok, err
	}
	var legacy struct {
		Version int     `json:"version"`
		Entries []Entry `json:"entries"`
	}
	if loaded, err := appdb.LegacyLoadJSON(path, "capture_history", &legacy); err != nil || !loaded {
		return nil, false, err
	}
	entries = make([]Entry, 0, len(legacy.Entries))
	for _, entry := range legacy.Entries {
		entry = normalizeEntry(entry)
		if entry.ID != "" && entry.ImagePath != "" {
			entries = append(entries, entry)
		}
	}
	if err := saveEntriesToSQLite(path, entries); err != nil {
		return nil, false, err
	}
	_ = appdb.DropLegacyDocument(path, "capture_history")
	return entries, true, nil
}

func saveEntriesToSQLite(path string, entries []Entry) error {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := ensureCaptureSchema(conn); err != nil {
		return err
	}
	return appdb.Immediate(conn, func() error {
		for _, stmt := range []string{
			`DELETE FROM capture_entry_actions`,
			`DELETE FROM capture_entry_tags`,
			`DELETE FROM capture_entries`,
		} {
			if err := sqlitex.Execute(conn, stmt, nil); err != nil {
				return err
			}
		}
		for _, entry := range entries {
			if err := insertCaptureEntry(conn, entry); err != nil {
				return err
			}
		}
		return nil
	})
}

func ensureCaptureSchema(conn *sqlite.Conn) error {
	return sqlitex.ExecScript(conn, `
CREATE TABLE IF NOT EXISTS capture_entries(
  id TEXT PRIMARY KEY,
  image_path TEXT NOT NULL,
  thumbnail_path TEXT NOT NULL DEFAULT '',
  thumbnail_width INTEGER NOT NULL DEFAULT 0,
  thumbnail_height INTEGER NOT NULL DEFAULT 0,
  thumbnail_bytes INTEGER NOT NULL DEFAULT 0,
  saved_path TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL DEFAULT 0,
  source TEXT NOT NULL DEFAULT '',
  pinned INTEGER NOT NULL DEFAULT 0,
  width INTEGER NOT NULL DEFAULT 0,
  height INTEGER NOT NULL DEFAULT 0,
  bytes INTEGER NOT NULL DEFAULT 0,
  signature TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_capture_entries_created_at ON capture_entries(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_capture_entries_source ON capture_entries(source);
CREATE TABLE IF NOT EXISTS capture_entry_actions(
  entry_id TEXT NOT NULL,
  position INTEGER NOT NULL,
  value TEXT NOT NULL,
  PRIMARY KEY(entry_id, position),
  FOREIGN KEY(entry_id) REFERENCES capture_entries(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS capture_entry_tags(
  entry_id TEXT NOT NULL,
  position INTEGER NOT NULL,
  value TEXT NOT NULL,
  PRIMARY KEY(entry_id, position),
  FOREIGN KEY(entry_id) REFERENCES capture_entries(id) ON DELETE CASCADE
);
`)
}

func readCaptureEntries(conn *sqlite.Conn) ([]Entry, bool, error) {
	count := 0
	if err := sqlitex.Execute(conn, `SELECT count(*) FROM capture_entries`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			count = stmt.ColumnInt(0)
			return nil
		},
	}); err != nil || count == 0 {
		return nil, false, err
	}
	entries := make([]Entry, 0, count)
	err := sqlitex.Execute(conn, `SELECT id, image_path, thumbnail_path, thumbnail_width, thumbnail_height, thumbnail_bytes, saved_path, created_at, source, pinned, width, height, bytes, signature FROM capture_entries ORDER BY created_at DESC`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			entries = append(entries, normalizeEntry(Entry{
				ID:              stmt.ColumnText(0),
				ImagePath:       stmt.ColumnText(1),
				ThumbnailPath:   stmt.ColumnText(2),
				ThumbnailWidth:  stmt.ColumnInt(3),
				ThumbnailHeight: stmt.ColumnInt(4),
				ThumbnailBytes:  stmt.ColumnInt64(5),
				SavedPath:       stmt.ColumnText(6),
				CreatedAt:       stmt.ColumnInt64(7),
				Source:          stmt.ColumnText(8),
				Pinned:          stmt.ColumnInt(9) != 0,
				Width:           stmt.ColumnInt(10),
				Height:          stmt.ColumnInt(11),
				Bytes:           stmt.ColumnInt64(12),
				Signature:       stmt.ColumnText(13),
			}))
			return nil
		},
	})
	if err != nil {
		return nil, false, err
	}
	for index := range entries {
		values, err := readOrderedValues(conn, `capture_entry_actions`, entries[index].ID)
		if err != nil {
			return nil, false, err
		}
		entries[index].Actions = values
		values, err = readOrderedValues(conn, `capture_entry_tags`, entries[index].ID)
		if err != nil {
			return nil, false, err
		}
		entries[index].Tags = values
	}
	sortEntries(entries)
	return entries, true, nil
}

func insertCaptureEntry(conn *sqlite.Conn, entry Entry) error {
	entry = normalizeEntry(entry)
	if entry.ID == "" || entry.ImagePath == "" {
		return nil
	}
	if err := sqlitex.Execute(conn, `INSERT INTO capture_entries(id, image_path, thumbnail_path, thumbnail_width, thumbnail_height, thumbnail_bytes, saved_path, created_at, source, pinned, width, height, bytes, signature)
VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13, ?14)`, &sqlitex.ExecOptions{
		Args: []any{entry.ID, entry.ImagePath, entry.ThumbnailPath, entry.ThumbnailWidth, entry.ThumbnailHeight, entry.ThumbnailBytes, entry.SavedPath, entry.CreatedAt, entry.Source, appdb.BoolInt(entry.Pinned), entry.Width, entry.Height, entry.Bytes, entry.Signature},
	}); err != nil {
		return err
	}
	if err := insertOrderedValues(conn, `capture_entry_actions`, entry.ID, entry.Actions); err != nil {
		return err
	}
	return insertOrderedValues(conn, `capture_entry_tags`, entry.ID, entry.Tags)
}

func insertOrderedValues(conn *sqlite.Conn, table string, entryID string, values []string) error {
	for position, value := range values {
		if value == "" {
			continue
		}
		if err := sqlitex.Execute(conn, `INSERT INTO `+table+`(entry_id, position, value) VALUES (?1, ?2, ?3)`, &sqlitex.ExecOptions{
			Args: []any{entryID, position, value},
		}); err != nil {
			return err
		}
	}
	return nil
}

func readOrderedValues(conn *sqlite.Conn, table string, entryID string) ([]string, error) {
	values := []string{}
	err := sqlitex.Execute(conn, `SELECT value FROM `+table+` WHERE entry_id = ?1 ORDER BY position`, &sqlitex.ExecOptions{
		Args: []any{entryID},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			values = append(values, stmt.ColumnText(0))
			return nil
		},
	})
	return values, err
}

func sortedCaptureIDs(entries []Entry) []string {
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.ID != "" {
			ids = append(ids, entry.ID)
		}
	}
	sort.Strings(ids)
	return ids
}
