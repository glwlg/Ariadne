package capturehistory

import (
	"slices"
	"sort"

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
	desired := make([]Entry, 0, len(entries))
	desiredByID := make(map[string]Entry, len(entries))
	for _, entry := range entries {
		entry = normalizeEntry(entry)
		if entry.ID == "" || entry.ImagePath == "" {
			continue
		}
		if _, exists := desiredByID[entry.ID]; exists {
			continue
		}
		desired = append(desired, entry)
		desiredByID[entry.ID] = entry
	}
	return appdb.Immediate(conn, func() error {
		existingByID, err := readRawCaptureEntryMap(conn)
		if err != nil {
			return err
		}
		for id := range existingByID {
			if _, keep := desiredByID[id]; keep {
				continue
			}
			if err := deleteCaptureEntryByID(conn, id); err != nil {
				return err
			}
		}
		for _, entry := range desired {
			prev, exists := existingByID[entry.ID]
			if !exists {
				if err := insertCaptureEntry(conn, entry); err != nil {
					return err
				}
				continue
			}
			if captureEntryPersistedEqual(prev, entry) {
				continue
			}
			if err := updateCaptureEntry(conn, prev, entry); err != nil {
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
	entries, err := readRawCaptureEntries(conn, true)
	if err != nil || len(entries) == 0 {
		return nil, false, err
	}
	sortEntries(entries)
	return entries, true, nil
}

func readRawCaptureEntryMap(conn *sqlite.Conn) (map[string]Entry, error) {
	entries, err := readRawCaptureEntries(conn, false)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]Entry, len(entries))
	for _, entry := range entries {
		byID[entry.ID] = entry
	}
	return byID, nil
}

func readRawCaptureEntries(conn *sqlite.Conn, normalize bool) ([]Entry, error) {
	entries := []Entry{}
	err := sqlitex.Execute(conn, `SELECT id, image_path, thumbnail_path, thumbnail_width, thumbnail_height, thumbnail_bytes, saved_path, created_at, source, pinned, width, height, bytes, signature FROM capture_entries ORDER BY created_at DESC`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			entry := Entry{
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
	actionsByEntry, err := readAllOrderedValues(conn, `capture_entry_actions`)
	if err != nil {
		return nil, err
	}
	tagsByEntry, err := readAllOrderedValues(conn, `capture_entry_tags`)
	if err != nil {
		return nil, err
	}
	for index := range entries {
		id := entries[index].ID
		if actions, ok := actionsByEntry[id]; ok {
			entries[index].Actions = actions
		} else {
			entries[index].Actions = []string{}
		}
		if tags, ok := tagsByEntry[id]; ok {
			entries[index].Tags = tags
		} else {
			entries[index].Tags = []string{}
		}
	}
	return entries, nil
}

func readAllOrderedValues(conn *sqlite.Conn, table string) (map[string][]string, error) {
	byEntry := make(map[string][]string)
	err := sqlitex.Execute(conn, `SELECT entry_id, value FROM `+table+` ORDER BY entry_id, position`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			id := stmt.ColumnText(0)
			byEntry[id] = append(byEntry[id], stmt.ColumnText(1))
			return nil
		},
	})
	return byEntry, err
}

func deleteCaptureEntryByID(conn *sqlite.Conn, entryID string) error {
	if err := sqlitex.Execute(conn, `DELETE FROM capture_entry_actions WHERE entry_id = ?1`, &sqlitex.ExecOptions{
		Args: []any{entryID},
	}); err != nil {
		return err
	}
	if err := sqlitex.Execute(conn, `DELETE FROM capture_entry_tags WHERE entry_id = ?1`, &sqlitex.ExecOptions{
		Args: []any{entryID},
	}); err != nil {
		return err
	}
	return sqlitex.Execute(conn, `DELETE FROM capture_entries WHERE id = ?1`, &sqlitex.ExecOptions{
		Args: []any{entryID},
	})
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

func updateCaptureEntry(conn *sqlite.Conn, prev Entry, entry Entry) error {
	if !captureEntryScalarEqual(prev, entry) {
		if err := sqlitex.Execute(conn, `UPDATE capture_entries
SET image_path = ?2, thumbnail_path = ?3, thumbnail_width = ?4, thumbnail_height = ?5, thumbnail_bytes = ?6, saved_path = ?7, created_at = ?8, source = ?9, pinned = ?10, width = ?11, height = ?12, bytes = ?13, signature = ?14
WHERE id = ?1`, &sqlitex.ExecOptions{
			Args: []any{entry.ID, entry.ImagePath, entry.ThumbnailPath, entry.ThumbnailWidth, entry.ThumbnailHeight, entry.ThumbnailBytes, entry.SavedPath, entry.CreatedAt, entry.Source, appdb.BoolInt(entry.Pinned), entry.Width, entry.Height, entry.Bytes, entry.Signature},
		}); err != nil {
			return err
		}
	}
	if !slices.Equal(nonEmptyValues(prev.Actions), nonEmptyValues(entry.Actions)) {
		if err := sqlitex.Execute(conn, `DELETE FROM capture_entry_actions WHERE entry_id = ?1`, &sqlitex.ExecOptions{
			Args: []any{entry.ID},
		}); err != nil {
			return err
		}
		if err := insertOrderedValues(conn, `capture_entry_actions`, entry.ID, entry.Actions); err != nil {
			return err
		}
	}
	if !slices.Equal(nonEmptyValues(prev.Tags), nonEmptyValues(entry.Tags)) {
		if err := sqlitex.Execute(conn, `DELETE FROM capture_entry_tags WHERE entry_id = ?1`, &sqlitex.ExecOptions{
			Args: []any{entry.ID},
		}); err != nil {
			return err
		}
		if err := insertOrderedValues(conn, `capture_entry_tags`, entry.ID, entry.Tags); err != nil {
			return err
		}
	}
	return nil
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

func captureEntryPersistedEqual(a Entry, b Entry) bool {
	return captureEntryScalarEqual(a, b) &&
		slices.Equal(nonEmptyValues(a.Actions), nonEmptyValues(b.Actions)) &&
		slices.Equal(nonEmptyValues(a.Tags), nonEmptyValues(b.Tags))
}

func captureEntryScalarEqual(a Entry, b Entry) bool {
	return a.ID == b.ID &&
		a.ImagePath == b.ImagePath &&
		a.ThumbnailPath == b.ThumbnailPath &&
		a.ThumbnailWidth == b.ThumbnailWidth &&
		a.ThumbnailHeight == b.ThumbnailHeight &&
		a.ThumbnailBytes == b.ThumbnailBytes &&
		a.SavedPath == b.SavedPath &&
		a.CreatedAt == b.CreatedAt &&
		a.Source == b.Source &&
		a.Pinned == b.Pinned &&
		a.Width == b.Width &&
		a.Height == b.Height &&
		a.Bytes == b.Bytes &&
		a.Signature == b.Signature
}

func nonEmptyValues(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
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
