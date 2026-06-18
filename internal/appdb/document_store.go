package appdb

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

const databaseName = "ariadne.sqlite"

func DatabasePathForPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	dir := filepath.Dir(path)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		dir = path
	}
	if dir == "." || dir == "" {
		return databaseName
	}
	return filepath.Join(dir, databaseName)
}

func readStateDocumentPayload(conn *sqlite.Conn, key string) ([]byte, bool, error) {
	key = strings.TrimSpace(strings.ToLower(key))
	if key == "" {
		return nil, false, nil
	}
	var raw []byte
	ok := false
	err := sqlitex.Execute(conn, `SELECT payload FROM state_documents WHERE key = ?1;`, &sqlitex.ExecOptions{
		Args: []any{key},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			raw = make([]byte, stmt.ColumnLen(0))
			stmt.ColumnBytes(0, raw)
			ok = true
			return nil
		},
	})
	return raw, ok, err
}

func readLegacyRawFile(path string) ([]byte, bool, error) {
	if strings.TrimSpace(path) == "" {
		return nil, false, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if len(raw) == 0 {
		return nil, false, nil
	}
	if !json.Valid(raw) {
		return nil, false, fmt.Errorf("legacy JSON %s is invalid", path)
	}
	return raw, true, nil
}

func ensureSchema(conn *sqlite.Conn) error {
	return sqlitex.ExecScript(conn, `
CREATE TABLE IF NOT EXISTS state_documents(
  key TEXT PRIMARY KEY,
  legacy_path TEXT NOT NULL DEFAULT '',
  payload BLOB NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_state_documents_updated_at ON state_documents(updated_at);
`)
}

func documentKeyForPath(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	key := strings.TrimSuffix(base, ext)
	key = strings.TrimSpace(strings.ToLower(key))
	if key == "" {
		key = strings.TrimSpace(strings.ToLower(base))
	}
	return key
}
