package clipboardhistory

import (
	"slices"

	"fancun/internal/appdb"

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
	desired := make([]Entry, 0, len(entries))
	desiredByID := make(map[string]Entry, len(entries))
	for _, entry := range entries {
		entry = normalizeEntry(entry)
		if !entryIsValid(entry) {
			continue
		}
		if _, exists := desiredByID[entry.ID]; exists {
			continue
		}
		desired = append(desired, entry)
		desiredByID[entry.ID] = entry
	}
	return appdb.Immediate(conn, func() error {
		existingByID, err := readRawClipboardEntryMap(conn)
		if err != nil {
			return err
		}
		for id := range existingByID {
			if _, keep := desiredByID[id]; keep {
				continue
			}
			if err := deleteClipboardEntryByID(conn, id); err != nil {
				return err
			}
		}
		for _, entry := range desired {
			prev, exists := existingByID[entry.ID]
			if !exists {
				if err := insertClipboardEntry(conn, entry); err != nil {
					return err
				}
				continue
			}
			if clipboardEntryPersistedEqual(prev, entry) {
				continue
			}
			if err := updateClipboardEntry(conn, prev, entry); err != nil {
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
	entries, err := readRawClipboardEntries(conn, true)
	if err != nil || len(entries) == 0 {
		return nil, false, err
	}
	sortEntries(entries)
	return entries, true, nil
}

func readRawClipboardEntryMap(conn *sqlite.Conn) (map[string]Entry, error) {
	entries, err := readRawClipboardEntries(conn, false)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]Entry, len(entries))
	for _, entry := range entries {
		byID[entry.ID] = entry
	}
	return byID, nil
}

func readRawClipboardEntries(conn *sqlite.Conn, normalize bool) ([]Entry, error) {
	entries := []Entry{}
	err := sqlitex.Execute(conn, `SELECT id, type, text, image_path, thumbnail_path, thumbnail_width, thumbnail_height, thumbnail_bytes, created_at, pinned, signature, content_type, source, summary, width, height, bytes FROM clipboard_entries ORDER BY created_at DESC`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			entry := Entry{
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
			}
			if normalize {
				entry = normalizeEntry(entry)
			}
			entries = append(entries, entry)
			return nil
		},
	})
	if err != nil || len(entries) == 0 {
		return entries, err
	}
	tagsByEntry, err := readAllClipboardTags(conn)
	if err != nil {
		return nil, err
	}
	for index := range entries {
		if tags, ok := tagsByEntry[entries[index].ID]; ok {
			entries[index].Tags = tags
		} else {
			entries[index].Tags = []string{}
		}
	}
	return entries, nil
}

func readAllClipboardTags(conn *sqlite.Conn) (map[string][]string, error) {
	tagsByEntry := make(map[string][]string)
	err := sqlitex.Execute(conn, `SELECT entry_id, value FROM clipboard_entry_tags ORDER BY entry_id, position`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			id := stmt.ColumnText(0)
			tagsByEntry[id] = append(tagsByEntry[id], stmt.ColumnText(1))
			return nil
		},
	})
	return tagsByEntry, err
}

func deleteClipboardEntryByID(conn *sqlite.Conn, entryID string) error {
	if err := sqlitex.Execute(conn, `DELETE FROM clipboard_entry_tags WHERE entry_id = ?1`, &sqlitex.ExecOptions{
		Args: []any{entryID},
	}); err != nil {
		return err
	}
	return sqlitex.Execute(conn, `DELETE FROM clipboard_entries WHERE id = ?1`, &sqlitex.ExecOptions{
		Args: []any{entryID},
	})
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
	return insertClipboardTags(conn, entry.ID, entry.Tags)
}

func updateClipboardEntry(conn *sqlite.Conn, prev Entry, entry Entry) error {
	if !clipboardEntryScalarEqual(prev, entry) {
		if err := sqlitex.Execute(conn, `UPDATE clipboard_entries
SET type = ?2, text = ?3, image_path = ?4, thumbnail_path = ?5, thumbnail_width = ?6, thumbnail_height = ?7, thumbnail_bytes = ?8, created_at = ?9, pinned = ?10, signature = ?11, content_type = ?12, source = ?13, summary = ?14, width = ?15, height = ?16, bytes = ?17
WHERE id = ?1`, &sqlitex.ExecOptions{
			Args: []any{entry.ID, string(entry.Type), entry.Text, entry.ImagePath, entry.ThumbnailPath, entry.ThumbnailWidth, entry.ThumbnailHeight, entry.ThumbnailBytes, entry.CreatedAt, appdb.BoolInt(entry.Pinned), entry.Signature, entry.ContentType, entry.Source, entry.Summary, entry.Width, entry.Height, entry.Bytes},
		}); err != nil {
			return err
		}
	}
	if !slices.Equal(nonEmptyTags(prev.Tags), nonEmptyTags(entry.Tags)) {
		if err := sqlitex.Execute(conn, `DELETE FROM clipboard_entry_tags WHERE entry_id = ?1`, &sqlitex.ExecOptions{
			Args: []any{entry.ID},
		}); err != nil {
			return err
		}
		if err := insertClipboardTags(conn, entry.ID, entry.Tags); err != nil {
			return err
		}
	}
	return nil
}

func insertClipboardTags(conn *sqlite.Conn, entryID string, tags []string) error {
	for position, tag := range tags {
		if tag == "" {
			continue
		}
		if err := sqlitex.Execute(conn, `INSERT INTO clipboard_entry_tags(entry_id, position, value) VALUES (?1, ?2, ?3)`, &sqlitex.ExecOptions{
			Args: []any{entryID, position, tag},
		}); err != nil {
			return err
		}
	}
	return nil
}

func clipboardEntryPersistedEqual(a Entry, b Entry) bool {
	return clipboardEntryScalarEqual(a, b) && slices.Equal(nonEmptyTags(a.Tags), nonEmptyTags(b.Tags))
}

func clipboardEntryScalarEqual(a Entry, b Entry) bool {
	return a.ID == b.ID &&
		a.Type == b.Type &&
		a.Text == b.Text &&
		a.ImagePath == b.ImagePath &&
		a.ThumbnailPath == b.ThumbnailPath &&
		a.ThumbnailWidth == b.ThumbnailWidth &&
		a.ThumbnailHeight == b.ThumbnailHeight &&
		a.ThumbnailBytes == b.ThumbnailBytes &&
		a.CreatedAt == b.CreatedAt &&
		a.Pinned == b.Pinned &&
		a.Signature == b.Signature &&
		a.ContentType == b.ContentType &&
		a.Source == b.Source &&
		a.Summary == b.Summary &&
		a.Width == b.Width &&
		a.Height == b.Height &&
		a.Bytes == b.Bytes
}

func nonEmptyTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		if tag != "" {
			out = append(out, tag)
		}
	}
	return out
}
