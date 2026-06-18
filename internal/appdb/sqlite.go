package appdb

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func OpenForPath(path string) (*sqlite.Conn, error) {
	dbPath := DatabasePathForPath(path)
	if strings.TrimSpace(dbPath) == "" {
		return nil, errors.New("sqlite path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	conn, err := sqlite.OpenConn(dbPath, sqlite.OpenReadWrite, sqlite.OpenCreate, sqlite.OpenWAL)
	if err != nil {
		return nil, err
	}
	conn.SetBlockOnBusy()
	return conn, nil
}

func Immediate(conn *sqlite.Conn, fn func() error) (err error) {
	end, err := sqlitex.ImmediateTransaction(conn)
	if err != nil {
		return err
	}
	defer end(&err)
	return fn()
}

func DeleteByKey(conn *sqlite.Conn, table string, keyColumn string, key any) error {
	return sqlitex.Execute(conn, "DELETE FROM "+table+" WHERE "+keyColumn+" = ?1", &sqlitex.ExecOptions{
		Args: []any{key},
	})
}

func LegacyLoadJSON(path string, key string, target any) (bool, error) {
	if strings.TrimSpace(path) != "" {
		raw, ok, err := readLegacyRawFile(path)
		if err != nil {
			return false, err
		}
		if ok {
			if err := json.Unmarshal(raw, target); err != nil {
				return false, err
			}
			return true, nil
		}
	}
	raw, ok, err := readLegacyDocumentPayload(path, key)
	if err != nil || !ok {
		return ok, err
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return false, err
	}
	return true, nil
}

func DropLegacyDocument(path string, key string) error {
	conn, err := OpenForPath(path)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := ensureSchema(conn); err != nil {
		return err
	}
	key = strings.TrimSpace(strings.ToLower(key))
	if key == "" {
		key = documentKeyForPath(path)
	}
	return sqlitex.Execute(conn, `DELETE FROM state_documents WHERE key = ?1`, &sqlitex.ExecOptions{
		Args: []any{key},
	})
}

func readLegacyDocumentPayload(path string, key string) ([]byte, bool, error) {
	if strings.TrimSpace(path) == "" {
		return nil, false, nil
	}
	conn, err := OpenForPath(path)
	if err != nil {
		return nil, false, err
	}
	defer conn.Close()
	if err := ensureSchema(conn); err != nil {
		return nil, false, err
	}
	if strings.TrimSpace(key) != "" {
		key = strings.TrimSpace(strings.ToLower(key))
	} else {
		key = documentKeyForPath(path)
	}
	return readStateDocumentPayload(conn, key)
}

func BoolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func IntBool(value int) bool {
	return value != 0
}
