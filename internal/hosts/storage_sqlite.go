package hosts

import (
	"fancun/internal/appdb"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

type hostsBackup struct {
	ID            int64
	CreatedAt     int64
	CreatedAtText string
	HostsPath     string
	Content       string
	Reason        string
}

func loadProfilesFromSQLite(path string) ([]Profile, bool, error) {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return nil, false, err
	}
	defer conn.Close()
	if err := ensureHostsSchema(conn); err != nil {
		return nil, false, err
	}
	profiles, ok, err := readProfiles(conn)
	if err != nil || ok {
		return profiles, ok, err
	}
	var legacy struct {
		Version  int       `json:"version"`
		Profiles []Profile `json:"profiles"`
	}
	if loaded, err := appdb.LegacyLoadJSON(path, "hosts_profiles", &legacy); err != nil || !loaded {
		return nil, false, err
	}
	profiles = normalizeProfiles(legacy.Profiles)
	if err := saveProfilesToSQLite(path, profiles); err != nil {
		return nil, false, err
	}
	_ = appdb.DropLegacyDocument(path, "hosts_profiles")
	return profiles, true, nil
}

func saveProfilesToSQLite(path string, profiles []Profile) error {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := ensureHostsSchema(conn); err != nil {
		return err
	}
	return appdb.Immediate(conn, func() error {
		if err := sqlitex.Execute(conn, `DELETE FROM hosts_profiles`, nil); err != nil {
			return err
		}
		for _, profile := range profiles {
			profile = normalizeProfile(profile)
			if profile.ID == "" || profile.System || profile.ID == systemProfileID {
				continue
			}
			if err := sqlitex.Execute(conn, `INSERT INTO hosts_profiles(id, title, content, enabled, type, url, updated_at) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7)`, &sqlitex.ExecOptions{
				Args: []any{profile.ID, profile.Title, profile.Content, appdb.BoolInt(profile.Enabled), profile.Type, profile.URL, profile.UpdatedAt},
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func ensureHostsSchema(conn *sqlite.Conn) error {
	return sqlitex.ExecScript(conn, `
CREATE TABLE IF NOT EXISTS hosts_profiles(
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  content TEXT NOT NULL DEFAULT '',
  enabled INTEGER NOT NULL DEFAULT 0,
  type TEXT NOT NULL DEFAULT 'local',
  url TEXT NOT NULL DEFAULT '',
  updated_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_hosts_profiles_enabled ON hosts_profiles(enabled);
CREATE TABLE IF NOT EXISTS hosts_backups(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  created_at INTEGER NOT NULL,
  created_at_text TEXT NOT NULL,
  hosts_path TEXT NOT NULL DEFAULT '',
  content TEXT NOT NULL DEFAULT '',
  reason TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_hosts_backups_created_at ON hosts_backups(id DESC);
`)
}

func saveHostsBackupToSQLite(path string, backup hostsBackup) (int64, error) {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	if err := ensureHostsSchema(conn); err != nil {
		return 0, err
	}
	var insertedID int64
	err = appdb.Immediate(conn, func() error {
		if err := sqlitex.Execute(conn, `INSERT INTO hosts_backups(created_at, created_at_text, hosts_path, content, reason) VALUES (?1, ?2, ?3, ?4, ?5)`, &sqlitex.ExecOptions{
			Args: []any{backup.CreatedAt, backup.CreatedAtText, backup.HostsPath, backup.Content, backup.Reason},
		}); err != nil {
			return err
		}
		insertedID = conn.LastInsertRowID()
		return sqlitex.Execute(conn, `DELETE FROM hosts_backups WHERE id NOT IN (SELECT id FROM hosts_backups ORDER BY id DESC LIMIT 20)`, nil)
	})
	return insertedID, err
}

func deleteHostsBackupFromSQLite(path string, id int64) error {
	if path == "" || id <= 0 {
		return nil
	}
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := ensureHostsSchema(conn); err != nil {
		return err
	}
	return appdb.Immediate(conn, func() error {
		return sqlitex.Execute(conn, `DELETE FROM hosts_backups WHERE id = ?1`, &sqlitex.ExecOptions{
			Args: []any{id},
		})
	})
}

func loadLatestHostsBackupFromSQLite(path string) (hostsBackup, bool, error) {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return hostsBackup{}, false, err
	}
	defer conn.Close()
	if err := ensureHostsSchema(conn); err != nil {
		return hostsBackup{}, false, err
	}
	var backup hostsBackup
	found := false
	err = sqlitex.Execute(conn, `SELECT id, created_at, created_at_text, hosts_path, content, reason FROM hosts_backups ORDER BY id DESC LIMIT 1`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			backup = hostsBackup{
				ID:            stmt.ColumnInt64(0),
				CreatedAt:     stmt.ColumnInt64(1),
				CreatedAtText: stmt.ColumnText(2),
				HostsPath:     stmt.ColumnText(3),
				Content:       stmt.ColumnText(4),
				Reason:        stmt.ColumnText(5),
			}
			found = true
			return nil
		},
	})
	if err != nil {
		return hostsBackup{}, false, err
	}
	return backup, found, nil
}

func readProfiles(conn *sqlite.Conn) ([]Profile, bool, error) {
	count := 0
	if err := sqlitex.Execute(conn, `SELECT count(*) FROM hosts_profiles`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error { count = stmt.ColumnInt(0); return nil },
	}); err != nil || count == 0 {
		return nil, false, err
	}
	profiles := make([]Profile, 0, count)
	err := sqlitex.Execute(conn, `SELECT id, title, content, enabled, type, url, updated_at FROM hosts_profiles`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			id := stmt.ColumnText(0)
			title := stmt.ColumnText(1)
			if title == "" {
				title = id
			}
			profiles = append(profiles, normalizeProfile(Profile{
				ID:        id,
				Title:     title,
				Content:   stmt.ColumnText(2),
				Enabled:   stmt.ColumnInt(3) != 0,
				Type:      stmt.ColumnText(4),
				URL:       stmt.ColumnText(5),
				UpdatedAt: stmt.ColumnInt64(6),
			}))
			return nil
		},
	})
	if err != nil {
		return nil, false, err
	}
	return normalizeProfiles(profiles), true, nil
}
