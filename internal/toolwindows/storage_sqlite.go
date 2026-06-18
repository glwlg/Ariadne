package toolwindows

import (
	"strings"

	"ariadne/internal/appdb"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func loadNetworkMiniConfigFromSQLite(path string, fallback networkMiniConfig) (networkMiniConfig, bool, error) {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return fallback, false, err
	}
	defer conn.Close()
	if err := ensureToolWindowSchema(conn); err != nil {
		return fallback, false, err
	}
	config, ok, err := readNetworkMiniConfig(conn, fallback)
	if err != nil || ok {
		return config, ok, err
	}
	var legacy struct {
		Anchor             string `json:"anchor"`
		ScreenMode         string `json:"screenMode,omitempty"`
		ScreenID           string `json:"screenId,omitempty"`
		AutoHideFullscreen *bool  `json:"autoHideFullscreen"`
		Visible            *bool  `json:"visible"`
	}
	if loaded, err := appdb.LegacyLoadJSON(path, "network_mini_window", &legacy); err != nil || !loaded {
		return fallback, false, err
	}
	config = fallback
	if anchor := normalizeNetworkMiniAnchor(legacy.Anchor); anchor != "" {
		config.Anchor = anchor
	}
	config.ScreenID = strings.TrimSpace(legacy.ScreenID)
	if strings.TrimSpace(legacy.ScreenMode) != "" {
		if mode := normalizeNetworkMiniScreenMode(legacy.ScreenMode); mode != "" {
			config.ScreenMode = mode
		}
	} else if strings.TrimSpace(legacy.ScreenID) != "" {
		config.ScreenMode = "screen"
	}
	if legacy.AutoHideFullscreen != nil {
		config.AutoHideFullscreen = *legacy.AutoHideFullscreen
	}
	if legacy.Visible != nil {
		config.Visible = *legacy.Visible
	}
	if err := saveNetworkMiniConfigToSQLite(path, config); err != nil {
		return fallback, false, err
	}
	_ = appdb.DropLegacyDocument(path, "network_mini_window")
	return config, true, nil
}

func saveNetworkMiniConfigToSQLite(path string, config networkMiniConfig) error {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := ensureToolWindowSchema(conn); err != nil {
		return err
	}
	return appdb.Immediate(conn, func() error {
		if err := sqlitex.Execute(conn, `DELETE FROM tool_window_network_mini`, nil); err != nil {
			return err
		}
		return sqlitex.Execute(conn, `INSERT INTO tool_window_network_mini(id, anchor, screen_mode, screen_id, auto_hide_fullscreen, visible) VALUES (1, ?1, ?2, ?3, ?4, ?5)`, &sqlitex.ExecOptions{
			Args: []any{config.Anchor, config.ScreenMode, config.ScreenID, appdb.BoolInt(config.AutoHideFullscreen), appdb.BoolInt(config.Visible)},
		})
	})
}

func ensureToolWindowSchema(conn *sqlite.Conn) error {
	return sqlitex.ExecScript(conn, `
CREATE TABLE IF NOT EXISTS tool_window_network_mini(
  id INTEGER PRIMARY KEY CHECK(id = 1),
  anchor TEXT NOT NULL,
  screen_mode TEXT NOT NULL DEFAULT '',
  screen_id TEXT NOT NULL DEFAULT '',
  auto_hide_fullscreen INTEGER NOT NULL DEFAULT 1,
  visible INTEGER NOT NULL DEFAULT 0
);
`)
}

func readNetworkMiniConfig(conn *sqlite.Conn, fallback networkMiniConfig) (networkMiniConfig, bool, error) {
	config := fallback
	ok := false
	err := sqlitex.Execute(conn, `SELECT anchor, screen_mode, screen_id, auto_hide_fullscreen, visible FROM tool_window_network_mini WHERE id = 1`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			ok = true
			if anchor := normalizeNetworkMiniAnchor(stmt.ColumnText(0)); anchor != "" {
				config.Anchor = anchor
			}
			if mode := normalizeNetworkMiniScreenMode(stmt.ColumnText(1)); mode != "" {
				config.ScreenMode = mode
			}
			config.ScreenID = strings.TrimSpace(stmt.ColumnText(2))
			config.AutoHideFullscreen = stmt.ColumnInt(3) != 0
			config.Visible = stmt.ColumnInt(4) != 0
			return nil
		},
	})
	return config, ok, err
}
