package brandmigration

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func importDatabase(source, target string, roots []root) (err error) {
	input, err := sqlite.OpenConn(source, sqlite.OpenReadOnly)
	if err != nil {
		return err
	}
	defer input.Close()
	input.SetBusyTimeout(5 * time.Second)
	output, err := sqlite.OpenConn(target, sqlite.OpenReadWrite|sqlite.OpenCreate)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, output.Close()) }()
	backup, err := sqlite.NewBackup(output, "main", input, "main")
	if err != nil {
		return err
	}
	_, err = backup.Step(-1)
	closeErr := backup.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := rewriteDatabasePaths(output, roots); err != nil {
		return err
	}
	return sqlitex.Execute(output, `PRAGMA quick_check`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			if result := stmt.ColumnText(0); result != "ok" {
				return fmt.Errorf("迁移数据库完整性检查失败：%s", result)
			}
			return nil
		},
	})
}

func rewriteDatabasePaths(conn *sqlite.Conn, roots []root) (err error) {
	tables := map[string]bool{}
	if err := sqlitex.Execute(conn, `SELECT name FROM sqlite_schema WHERE type = 'table'`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error { tables[stmt.ColumnText(0)] = true; return nil },
	}); err != nil {
		return err
	}
	end, err := sqlitex.ImmediateTransaction(conn)
	if err != nil {
		return err
	}
	defer end(&err)
	// Launcher children have foreign keys without ON UPDATE CASCADE.
	if err := sqlitex.Execute(conn, `PRAGMA defer_foreign_keys = ON`, nil); err != nil {
		return err
	}
	for table, columns := range map[string][]string{
		"capture_entries":        {"image_path", "thumbnail_path", "saved_path"},
		"clipboard_entries":      {"image_path", "thumbnail_path"},
		"image_index_entries":    {"image_path"},
		"launchers":              {"target", "working_dir"},
		"settings2_values":       {"text_value"},
		"settings2_string_lists": {"value"},
		"state_documents":        {"legacy_path"},
	} {
		if !tables[table] {
			continue
		}
		for _, column := range columns {
			type row struct {
				ID    int64
				Value string
			}
			var updates []row
			if err := sqlitex.Execute(conn, `SELECT rowid, `+column+` FROM `+table, &sqlitex.ExecOptions{
				ResultFunc: func(stmt *sqlite.Stmt) error {
					before := stmt.ColumnText(1)
					if after := rewritePath(before, roots); after != before {
						updates = append(updates, row{stmt.ColumnInt64(0), after})
					}
					return nil
				},
			}); err != nil {
				return err
			}
			for _, row := range updates {
				if err := sqlitex.Execute(conn, `UPDATE `+table+` SET `+column+` = ?1 WHERE rowid = ?2`, &sqlitex.ExecOptions{Args: []any{row.Value, row.ID}}); err != nil {
					return err
				}
			}
		}
	}
	if tables["settings2_values"] {
		if err := sqlitex.Execute(conn, `UPDATE settings2_values SET text_value = 'fancun_{date}_{time}'
WHERE path = 'screenshot.filenameTemplate' AND text_value = 'ariadne_{date}_{time}'`, nil); err != nil {
			return err
		}
	}
	if tables["settings2_string_lists"] {
		if err := sqlitex.Execute(conn, `UPDATE settings2_string_lists SET value = ?1
WHERE path = 'search.fileExcludePatterns' AND value = ?2`, &sqlitex.ExecOptions{
			Args: []any{rewriteExcludePattern(`(?i)(^|[\\/])ariadne[\\/](capture_images|capture_thumbnails)([\\/]|$)`),
				`(?i)(^|[\\/])ariadne[\\/](capture_images|capture_thumbnails)([\\/]|$)`},
		}); err != nil {
			return err
		}
	}
	for table, column := range map[string]string{
		"launchers": "id", "launcher_keywords": "launcher_id",
		"launcher_tags": "launcher_id", "launcher_removed": "id", "search_usage": "result_id",
	} {
		if !tables[table] {
			continue
		}
		before, after := "ariadne-config-dir", "fancun-config-dir"
		if table == "search_usage" {
			before, after = "launcher-"+before, "launcher-"+after
		}
		if err := sqlitex.Execute(conn, `UPDATE `+table+` SET `+column+` = ?1 WHERE `+column+` = ?2`,
			&sqlitex.ExecOptions{Args: []any{after, before}}); err != nil {
			return err
		}
	}
	if tables["launchers"] {
		if err := sqlitex.Execute(conn, `UPDATE launchers SET name = 'Fancun 配置目录'
WHERE id = 'fancun-config-dir' AND name = 'Ariadne 配置目录'`, nil); err != nil {
			return err
		}
	}
	if tables["launcher_keywords"] {
		if err := sqlitex.Execute(conn, `UPDATE launcher_keywords SET value = 'fancun'
WHERE launcher_id = 'fancun-config-dir' AND value = 'ariadne'`, nil); err != nil {
			return err
		}
	}
	if tables["state_documents"] {
		type document struct {
			ID      int64
			Payload []byte
		}
		var documents []document
		if err := sqlitex.Execute(conn, `SELECT rowid, payload FROM state_documents`, &sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				raw := []byte(stmt.ColumnText(1))
				if json.Valid(raw) {
					updated, err := rewriteJSON(raw, roots)
					if err != nil {
						return err
					}
					documents = append(documents, document{stmt.ColumnInt64(0), updated})
				}
				return nil
			},
		}); err != nil {
			return err
		}
		for _, document := range documents {
			if err := sqlitex.Execute(conn, `UPDATE state_documents SET payload = ?1 WHERE rowid = ?2`, &sqlitex.ExecOptions{Args: []any{document.Payload, document.ID}}); err != nil {
				return err
			}
		}
	}
	return nil
}
