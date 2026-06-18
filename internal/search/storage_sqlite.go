package search

import (
	"ariadne/internal/appdb"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func loadUsageRecordsFromSQLite(path string) (map[string]UsageRecord, bool, error) {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return nil, false, err
	}
	defer conn.Close()
	if err := ensureSearchUsageSchema(conn); err != nil {
		return nil, false, err
	}
	records, ok, err := readUsageRecords(conn)
	if err != nil || ok {
		return records, ok, err
	}
	var legacy struct {
		Version int                    `json:"version"`
		Records map[string]UsageRecord `json:"records"`
	}
	if loaded, err := appdb.LegacyLoadJSON(path, "search_state", &legacy); err != nil || !loaded || legacy.Records == nil {
		return nil, false, err
	}
	records = map[string]UsageRecord{}
	for id, record := range legacy.Records {
		record = normalizeUsageRecord(id, record)
		if !isEmptyUsageRecord(record) {
			records[record.ResultID] = record
		}
	}
	if err := saveUsageRecordsToSQLite(path, records); err != nil {
		return nil, false, err
	}
	_ = appdb.DropLegacyDocument(path, "search_state")
	return records, true, nil
}

func saveUsageRecordsToSQLite(path string, records map[string]UsageRecord) error {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := ensureSearchUsageSchema(conn); err != nil {
		return err
	}
	return appdb.Immediate(conn, func() error {
		if err := sqlitex.Execute(conn, `DELETE FROM search_usage`, nil); err != nil {
			return err
		}
		for id, record := range records {
			record = normalizeUsageRecord(id, record)
			if isEmptyUsageRecord(record) {
				continue
			}
			if err := sqlitex.Execute(conn, `INSERT INTO search_usage(result_id, favorite, use_count, last_used_at) VALUES (?1, ?2, ?3, ?4)`, &sqlitex.ExecOptions{
				Args: []any{record.ResultID, appdb.BoolInt(record.Favorite), record.UseCount, record.LastUsedAt},
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func ensureSearchUsageSchema(conn *sqlite.Conn) error {
	return sqlitex.ExecScript(conn, `
CREATE TABLE IF NOT EXISTS search_usage(
  result_id TEXT PRIMARY KEY,
  favorite INTEGER NOT NULL DEFAULT 0,
  use_count INTEGER NOT NULL DEFAULT 0,
  last_used_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_search_usage_last_used_at ON search_usage(last_used_at DESC);
`)
}

func readUsageRecords(conn *sqlite.Conn) (map[string]UsageRecord, bool, error) {
	count := 0
	if err := sqlitex.Execute(conn, `SELECT count(*) FROM search_usage`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			count = stmt.ColumnInt(0)
			return nil
		},
	}); err != nil || count == 0 {
		return nil, false, err
	}
	records := map[string]UsageRecord{}
	err := sqlitex.Execute(conn, `SELECT result_id, favorite, use_count, last_used_at FROM search_usage`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			record := normalizeUsageRecord(stmt.ColumnText(0), UsageRecord{
				Favorite:   stmt.ColumnInt(1) != 0,
				UseCount:   stmt.ColumnInt(2),
				LastUsedAt: stmt.ColumnInt64(3),
			})
			if !isEmptyUsageRecord(record) {
				records[record.ResultID] = record
			}
			return nil
		},
	})
	return records, err == nil && len(records) > 0, err
}
