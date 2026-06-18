package launchers

import (
	"sort"

	"ariadne/internal/appdb"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

type launcherState struct {
	Launchers []Launcher
	Removed   map[string]bool
}

func loadLauncherStateFromSQLite(path string) (launcherState, bool, error) {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return launcherState{}, false, err
	}
	defer conn.Close()
	if err := ensureLauncherSchema(conn); err != nil {
		return launcherState{}, false, err
	}
	state, ok, err := readLauncherState(conn)
	if err != nil || ok {
		return state, ok, err
	}
	var legacy struct {
		Version    int        `json:"version"`
		Launchers  []Launcher `json:"launchers"`
		RemovedIDs []string   `json:"removedIds,omitempty"`
	}
	if loaded, err := appdb.LegacyLoadJSON(path, "launchers", &legacy); err != nil || !loaded {
		return launcherState{}, false, err
	}
	state = launcherState{Launchers: legacy.Launchers, Removed: map[string]bool{}}
	for _, id := range legacy.RemovedIDs {
		if id != "" {
			state.Removed[id] = true
		}
	}
	if err := saveLauncherStateToSQLite(path, state); err != nil {
		return launcherState{}, false, err
	}
	_ = appdb.DropLegacyDocument(path, "launchers")
	return state, true, nil
}

func saveLauncherStateToSQLite(path string, state launcherState) error {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := ensureLauncherSchema(conn); err != nil {
		return err
	}
	return appdb.Immediate(conn, func() error {
		for _, stmt := range []string{
			`DELETE FROM launcher_keywords`,
			`DELETE FROM launcher_tags`,
			`DELETE FROM launcher_removed`,
			`DELETE FROM launchers`,
		} {
			if err := sqlitex.Execute(conn, stmt, nil); err != nil {
				return err
			}
		}
		for _, launcher := range state.Launchers {
			if err := insertLauncher(conn, launcher); err != nil {
				return err
			}
		}
		removedIDs := make([]string, 0, len(state.Removed))
		for id := range state.Removed {
			if id != "" {
				removedIDs = append(removedIDs, id)
			}
		}
		sort.Strings(removedIDs)
		for _, id := range removedIDs {
			if err := sqlitex.Execute(conn, `INSERT INTO launcher_removed(id) VALUES (?1)`, &sqlitex.ExecOptions{Args: []any{id}}); err != nil {
				return err
			}
		}
		return nil
	})
}

func ensureLauncherSchema(conn *sqlite.Conn) error {
	return sqlitex.ExecScript(conn, `
CREATE TABLE IF NOT EXISTS launchers(
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  kind TEXT NOT NULL,
  target TEXT NOT NULL,
  arguments TEXT NOT NULL DEFAULT '',
  working_dir TEXT NOT NULL DEFAULT '',
  enabled INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_launchers_name ON launchers(name);
CREATE TABLE IF NOT EXISTS launcher_keywords(
  launcher_id TEXT NOT NULL,
  position INTEGER NOT NULL,
  value TEXT NOT NULL,
  PRIMARY KEY(launcher_id, position),
  FOREIGN KEY(launcher_id) REFERENCES launchers(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS launcher_tags(
  launcher_id TEXT NOT NULL,
  position INTEGER NOT NULL,
  value TEXT NOT NULL,
  PRIMARY KEY(launcher_id, position),
  FOREIGN KEY(launcher_id) REFERENCES launchers(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS launcher_removed(
  id TEXT PRIMARY KEY
);
`)
}

func readLauncherState(conn *sqlite.Conn) (launcherState, bool, error) {
	count := 0
	if err := sqlitex.Execute(conn, `SELECT count(*) FROM launchers`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			count = stmt.ColumnInt(0)
			return nil
		},
	}); err != nil || count == 0 {
		return launcherState{}, false, err
	}
	state := launcherState{Launchers: make([]Launcher, 0, count), Removed: map[string]bool{}}
	err := sqlitex.Execute(conn, `SELECT id, name, kind, target, arguments, working_dir, enabled FROM launchers ORDER BY name`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			state.Launchers = append(state.Launchers, normalizeLauncher(Launcher{
				ID:         stmt.ColumnText(0),
				Name:       stmt.ColumnText(1),
				Kind:       LauncherKind(stmt.ColumnText(2)),
				Target:     stmt.ColumnText(3),
				Arguments:  stmt.ColumnText(4),
				WorkingDir: stmt.ColumnText(5),
				Enabled:    stmt.ColumnInt(6) != 0,
			}))
			return nil
		},
	})
	if err != nil {
		return launcherState{}, false, err
	}
	for index := range state.Launchers {
		values, err := readLauncherValues(conn, `launcher_keywords`, state.Launchers[index].ID)
		if err != nil {
			return launcherState{}, false, err
		}
		state.Launchers[index].Keywords = values
		values, err = readLauncherValues(conn, `launcher_tags`, state.Launchers[index].ID)
		if err != nil {
			return launcherState{}, false, err
		}
		state.Launchers[index].Tags = values
	}
	if err := sqlitex.Execute(conn, `SELECT id FROM launcher_removed`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			state.Removed[stmt.ColumnText(0)] = true
			return nil
		},
	}); err != nil {
		return launcherState{}, false, err
	}
	sortLaunchers(state.Launchers)
	return state, true, nil
}

func insertLauncher(conn *sqlite.Conn, launcher Launcher) error {
	launcher = normalizeLauncher(launcher)
	if launcher.ID == "" || launcher.Name == "" || launcher.Target == "" {
		return nil
	}
	if err := sqlitex.Execute(conn, `INSERT INTO launchers(id, name, kind, target, arguments, working_dir, enabled) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7)`, &sqlitex.ExecOptions{
		Args: []any{launcher.ID, launcher.Name, string(launcher.Kind), launcher.Target, launcher.Arguments, launcher.WorkingDir, appdb.BoolInt(launcher.Enabled)},
	}); err != nil {
		return err
	}
	if err := insertLauncherValues(conn, `launcher_keywords`, launcher.ID, launcher.Keywords); err != nil {
		return err
	}
	return insertLauncherValues(conn, `launcher_tags`, launcher.ID, launcher.Tags)
}

func insertLauncherValues(conn *sqlite.Conn, table string, launcherID string, values []string) error {
	for position, value := range values {
		if value == "" {
			continue
		}
		if err := sqlitex.Execute(conn, `INSERT INTO `+table+`(launcher_id, position, value) VALUES (?1, ?2, ?3)`, &sqlitex.ExecOptions{
			Args: []any{launcherID, position, value},
		}); err != nil {
			return err
		}
	}
	return nil
}

func readLauncherValues(conn *sqlite.Conn, table string, launcherID string) ([]string, error) {
	values := []string{}
	err := sqlitex.Execute(conn, `SELECT value FROM `+table+` WHERE launcher_id = ?1 ORDER BY position`, &sqlitex.ExecOptions{
		Args: []any{launcherID},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			values = append(values, stmt.ColumnText(0))
			return nil
		},
	})
	return values, err
}
