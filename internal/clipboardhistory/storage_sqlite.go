package clipboardhistory

import (
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
	if err := ensureClipboardSchema(conn); err != nil {
		return nil, false, err
	}
	entries, ok, err := readClipboardEntries(conn)
	if err != nil || ok {
		return entries, ok, err
	}
	var legacy struct {
		Version int     `json:"version"`
		Entries []Entry `json:"entries"`
	}
	if loaded, err := appdb.LegacyLoadJSON(path, "clipboard_history", &legacy); err != nil || !loaded {
		return nil, false, err
	}
	entries = make([]Entry, 0, len(legacy.Entries))
	for _, entry := range legacy.Entries {
		entry = normalizeEntry(entry)
		if entryIsValid(entry) {
			entries = append(entries, entry)
		}
	}
	if err := saveEntriesToSQLite(path, entries); err != nil {
		return nil, false, err
	}
	_ = appdb.DropLegacyDocument(path, "clipboard_history")
	return entries, true, nil
}

func saveEntriesToSQLite(path string, entries []Entry) error {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := ensureClipboardSchema(conn); err != nil {
		return err
	}
	return appdb.Immediate(conn, func() error {
		if err := sqlitex.Execute(conn, `DELETE FROM clipboard_entry_tags`, nil); err != nil {
			return err
		}
		if err := sqlitex.Execute(conn, `DELETE FROM clipboard_entries`, nil); err != nil {
			return err
		}
		for _, entry := range entries {
			if err := insertClipboardEntry(conn, entry); err != nil {
				return err
			}
		}
		return nil
	})
}

func ensureClipboardSchema(conn *sqlite.Conn) error {
	return sqlitex.ExecScript(conn, `
CREATE TABLE IF NOT EXISTS clipboard_entries(
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL,
  text TEXT NOT NULL DEFAULT '',
  image_path TEXT NOT NULL DEFAULT '',
  thumbnail_path TEXT NOT NULL DEFAULT '',
  thumbnail_width INTEGER NOT NULL DEFAULT 0,
  thumbnail_height INTEGER NOT NULL DEFAULT 0,
  thumbnail_bytes INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL DEFAULT 0,
  pinned INTEGER NOT NULL DEFAULT 0,
  signature TEXT NOT NULL DEFAULT '',
  content_type TEXT NOT NULL DEFAULT '',
  source TEXT NOT NULL DEFAULT '',
  summary TEXT NOT NULL DEFAULT '',
  width INTEGER NOT NULL DEFAULT 0,
  height INTEGER NOT NULL DEFAULT 0,
  bytes INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_clipboard_entries_created_at ON clipboard_entries(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_clipboard_entries_type ON clipboard_entries(type);
CREATE TABLE IF NOT EXISTS clipboard_entry_tags(
  entry_id TEXT NOT NULL,
  position INTEGER NOT NULL,
  value TEXT NOT NULL,
  PRIMARY KEY(entry_id, position),
  FOREIGN KEY(entry_id) REFERENCES clipboard_entries(id) ON DELETE CASCADE
);
`)
}

func readClipboardEntries(conn *sqlite.Conn) ([]Entry, bool, error) {
	count := 0
	if err := sqlitex.Execute(conn, `SELECT count(*) FROM clipboard_entries`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			count = stmt.ColumnInt(0)
			return nil
		},
	}); err != nil || count == 0 {
		return nil, false, err
	}
	entries := make([]Entry, 0, count)
	err := sqlitex.Execute(conn, `SELECT id, type, text, image_path, thumbnail_path, thumbnail_width, thumbnail_height, thumbnail_bytes, created_at, pinned, signature, content_type, source, summary, width, height, bytes FROM clipboard_entries ORDER BY created_at DESC`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			entries = append(entries, normalizeEntry(Entry{
				ID:              stmt.ColumnText(0),
				Type:            EntryType(stmt.ColumnText(1)),
				Text:            stmt.ColumnText(2),
				ImagePath:       stmt.ColumnText(3),
				ThumbnailPath:   stmt.ColumnText(4),
				ThumbnailWidth:  stmt.ColumnInt(5),
				ThumbnailHeight: stmt.ColumnInt(6),
				ThumbnailBytes:  stmt.ColumnInt64(7),
				CreatedAt:       stmt.ColumnInt64(8),
				Pinned:          stmt.ColumnInt(9) != 0,
				Signature:       stmt.ColumnText(10),
				ContentType:     stmt.ColumnText(11),
				Source:          stmt.ColumnText(12),
				Summary:         stmt.ColumnText(13),
				Width:           stmt.ColumnInt(14),
				Height:          stmt.ColumnInt(15),
				Bytes:           stmt.ColumnInt64(16),
			}))
			return nil
		},
	})
	if err != nil {
		return nil, false, err
	}
	for index := range entries {
		tags, err := readClipboardTags(conn, entries[index].ID)
		if err != nil {
			return nil, false, err
		}
		entries[index].Tags = tags
	}
	sortEntries(entries)
	return entries, true, nil
}

func insertClipboardEntry(conn *sqlite.Conn, entry Entry) error {
	entry = normalizeEntry(entry)
	if !entryIsValid(entry) {
		return nil
	}
	if err := sqlitex.Execute(conn, `INSERT INTO clipboard_entries(id, type, text, image_path, thumbnail_path, thumbnail_width, thumbnail_height, thumbnail_bytes, created_at, pinned, signature, content_type, source, summary, width, height, bytes)
VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13, ?14, ?15, ?16, ?17)`, &sqlitex.ExecOptions{
		Args: []any{entry.ID, string(entry.Type), entry.Text, entry.ImagePath, entry.ThumbnailPath, entry.ThumbnailWidth, entry.ThumbnailHeight, entry.ThumbnailBytes, entry.CreatedAt, appdb.BoolInt(entry.Pinned), entry.Signature, entry.ContentType, entry.Source, entry.Summary, entry.Width, entry.Height, entry.Bytes},
	}); err != nil {
		return err
	}
	for position, tag := range entry.Tags {
		if tag == "" {
			continue
		}
		if err := sqlitex.Execute(conn, `INSERT INTO clipboard_entry_tags(entry_id, position, value) VALUES (?1, ?2, ?3)`, &sqlitex.ExecOptions{
			Args: []any{entry.ID, position, tag},
		}); err != nil {
			return err
		}
	}
	return nil
}

func readClipboardTags(conn *sqlite.Conn, entryID string) ([]string, error) {
	tags := []string{}
	err := sqlitex.Execute(conn, `SELECT value FROM clipboard_entry_tags WHERE entry_id = ?1 ORDER BY position`, &sqlitex.ExecOptions{
		Args: []any{entryID},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			tags = append(tags, stmt.ColumnText(0))
			return nil
		},
	})
	return tags, err
}
