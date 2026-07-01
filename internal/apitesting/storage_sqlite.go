package apitesting

import (
	"encoding/json"
	"strings"
	"time"

	"ariadne/internal/appdb"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

const stateKey = "api_testing"

func loadStateFromSQLite(path string) (state, bool, error) {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return state{}, false, err
	}
	defer conn.Close()
	if err := ensureAPITestingSchema(conn); err != nil {
		return state{}, false, err
	}
	var raw []byte
	ok := false
	err = sqlitex.Execute(conn, `SELECT payload FROM api_testing_state WHERE key = ?1`, &sqlitex.ExecOptions{
		Args: []any{stateKey},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			raw = make([]byte, stmt.ColumnLen(0))
			stmt.ColumnBytes(0, raw)
			ok = true
			return nil
		},
	})
	if err != nil || !ok {
		return state{}, ok, err
	}
	var loaded state
	if err := json.Unmarshal(raw, &loaded); err != nil {
		return state{}, false, err
	}
	return loaded, true, nil
}

func saveStateToSQLite(path string, next state) error {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := ensureAPITestingSchema(conn); err != nil {
		return err
	}
	raw, err := json.Marshal(next)
	if err != nil {
		return err
	}
	return appdb.Immediate(conn, func() error {
		return sqlitex.Execute(conn, `INSERT INTO api_testing_state(key, payload, updated_at) VALUES (?1, ?2, ?3) ON CONFLICT(key) DO UPDATE SET payload = excluded.payload, updated_at = excluded.updated_at`, &sqlitex.ExecOptions{Args: []any{stateKey, raw, time.Now().Unix()}})
	})
}

func ensureAPITestingSchema(conn *sqlite.Conn) error {
	return sqlitex.ExecScript(conn, `
CREATE TABLE IF NOT EXISTS api_testing_state(
  key TEXT PRIMARY KEY,
  payload BLOB NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_api_testing_state_updated_at ON api_testing_state(updated_at);
`)
}

func databasePath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	return appdb.DatabasePathForPath(path)
}
