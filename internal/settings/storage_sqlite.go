package settings

import (
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"ariadne/internal/appdb"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func loadSettingsFromSQLite(path string) (AppSettings, bool, error) {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return AppSettings{}, false, err
	}
	defer conn.Close()
	if err := ensureSettingsSchema(conn); err != nil {
		return AppSettings{}, false, err
	}
	scope := settingsScope(path)
	if hasRows, err := hasSettingsRows(conn, scope); err != nil || hasRows {
		if err != nil {
			return AppSettings{}, false, err
		}
		loaded := defaultSettings()
		if err := readSettingsRows(conn, scope, &loaded); err != nil {
			return AppSettings{}, false, err
		}
		return loaded, true, nil
	}
	var legacy AppSettings
	if loaded, err := appdb.LegacyLoadJSON(path, "", &legacy); err != nil || !loaded {
		return AppSettings{}, false, err
	}
	if err := saveSettingsToSQLite(path, legacy); err != nil {
		return AppSettings{}, false, err
	}
	_ = appdb.DropLegacyDocument(path, "")
	return legacy, true, nil
}

func saveSettingsToSQLite(path string, value AppSettings) error {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := ensureSettingsSchema(conn); err != nil {
		return err
	}
	return appdb.Immediate(conn, func() error {
		for _, stmt := range []string{
			`DELETE FROM settings2_values WHERE scope = ?1`,
			`DELETE FROM settings2_string_lists WHERE scope = ?1`,
			`DELETE FROM settings2_app_capture_profiles WHERE scope = ?1`,
			`DELETE FROM settings2_plugins WHERE scope = ?1`,
		} {
			if err := sqlitex.Execute(conn, stmt, &sqlitex.ExecOptions{Args: []any{settingsScope(path)}}); err != nil {
				return err
			}
		}
		if err := writeSettingsScalars(conn, settingsScope(path), value); err != nil {
			return err
		}
		if err := writeSettingsStringLists(conn, settingsScope(path), value); err != nil {
			return err
		}
		if err := writeSettingsAppProfiles(conn, settingsScope(path), value.WorkMemory.AppCaptureProfiles); err != nil {
			return err
		}
		return writeSettingsPlugins(conn, settingsScope(path), value.Plugins.Enabled)
	})
}

func ensureSettingsSchema(conn *sqlite.Conn) error {
	return sqlitex.ExecScript(conn, `
CREATE TABLE IF NOT EXISTS settings2_values(
  scope TEXT NOT NULL,
  path TEXT NOT NULL,
  type TEXT NOT NULL,
  text_value TEXT NOT NULL DEFAULT '',
  int_value INTEGER NOT NULL DEFAULT 0,
  bool_value INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY(scope, path)
);
CREATE TABLE IF NOT EXISTS settings2_string_lists(
  scope TEXT NOT NULL,
  path TEXT NOT NULL,
  position INTEGER NOT NULL,
  value TEXT NOT NULL,
  PRIMARY KEY(scope, path, position)
);
CREATE TABLE IF NOT EXISTS settings2_app_capture_profiles(
  scope TEXT NOT NULL,
  id TEXT NOT NULL,
  display_name TEXT NOT NULL DEFAULT '',
  process_name TEXT NOT NULL DEFAULT '',
  icon TEXT NOT NULL DEFAULT '',
  enabled INTEGER NOT NULL DEFAULT 0,
  window_switch_delay_seconds INTEGER NOT NULL DEFAULT 0,
  active_interval_seconds INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY(scope, id)
);
CREATE TABLE IF NOT EXISTS settings2_plugins(
  scope TEXT NOT NULL,
  plugin_id TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY(scope, plugin_id)
);
`)
}

func hasSettingsRows(conn *sqlite.Conn, scope string) (bool, error) {
	for _, table := range []string{"settings2_values", "settings2_string_lists", "settings2_app_capture_profiles", "settings2_plugins"} {
		count := 0
		if err := sqlitex.Execute(conn, `SELECT count(*) FROM `+table+` WHERE scope = ?1`, &sqlitex.ExecOptions{
			Args: []any{scope},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				count = stmt.ColumnInt(0)
				return nil
			},
		}); err != nil {
			return false, err
		}
		if count > 0 {
			return true, nil
		}
	}
	return false, nil
}

func writeSettingsScalars(conn *sqlite.Conn, scope string, value AppSettings) error {
	return walkSettings(reflect.ValueOf(value), "", func(path string, value reflect.Value) error {
		switch value.Kind() {
		case reflect.String:
			return insertSettingValue(conn, scope, path, "string", value.String(), 0, false)
		case reflect.Bool:
			return insertSettingValue(conn, scope, path, "bool", "", 0, value.Bool())
		case reflect.Int:
			return insertSettingValue(conn, scope, path, "int", "", value.Int(), false)
		default:
			return nil
		}
	})
}

func writeSettingsStringLists(conn *sqlite.Conn, scope string, value AppSettings) error {
	lists := map[string][]string{
		"workMemory.excludeApps":            value.WorkMemory.ExcludeApps,
		"workMemory.excludeWindowKeywords":  value.WorkMemory.ExcludeWindowKeywords,
		"workMemory.excludePaths":           value.WorkMemory.ExcludePaths,
		"workMemory.excludeUrls":            value.WorkMemory.ExcludeURLs,
		"workMemory.excludeContentPatterns": value.WorkMemory.ExcludeContentPatterns,
	}
	paths := make([]string, 0, len(lists))
	for path := range lists {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		for position, item := range lists[path] {
			if err := sqlitex.Execute(conn, `INSERT INTO settings2_string_lists(scope, path, position, value) VALUES (?1, ?2, ?3, ?4)`, &sqlitex.ExecOptions{
				Args: []any{scope, path, position, item},
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeSettingsAppProfiles(conn *sqlite.Conn, scope string, profiles []WorkMemoryAppCaptureProfile) error {
	for _, profile := range profiles {
		if profile.ID == "" {
			continue
		}
		if err := sqlitex.Execute(conn, `INSERT INTO settings2_app_capture_profiles(scope, id, display_name, process_name, icon, enabled, window_switch_delay_seconds, active_interval_seconds)
VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8)`, &sqlitex.ExecOptions{
			Args: []any{scope, profile.ID, profile.DisplayName, profile.ProcessName, profile.Icon, appdb.BoolInt(profile.Enabled), profile.WindowSwitchDelaySeconds, profile.ActiveIntervalSeconds},
		}); err != nil {
			return err
		}
	}
	return nil
}

func writeSettingsPlugins(conn *sqlite.Conn, scope string, plugins map[string]bool) error {
	ids := make([]string, 0, len(plugins))
	for id := range plugins {
		if id != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := sqlitex.Execute(conn, `INSERT INTO settings2_plugins(scope, plugin_id, enabled) VALUES (?1, ?2, ?3)`, &sqlitex.ExecOptions{
			Args: []any{scope, id, appdb.BoolInt(plugins[id])},
		}); err != nil {
			return err
		}
	}
	return nil
}

func insertSettingValue(conn *sqlite.Conn, scope string, path string, kind string, text string, integer int64, boolean bool) error {
	return sqlitex.Execute(conn, `INSERT INTO settings2_values(scope, path, type, text_value, int_value, bool_value) VALUES (?1, ?2, ?3, ?4, ?5, ?6)`, &sqlitex.ExecOptions{
		Args: []any{scope, path, kind, text, integer, appdb.BoolInt(boolean)},
	})
}

func readSettingsRows(conn *sqlite.Conn, scope string, target *AppSettings) error {
	if err := sqlitex.Execute(conn, `SELECT path, type, text_value, int_value, bool_value FROM settings2_values WHERE scope = ?1`, &sqlitex.ExecOptions{
		Args: []any{scope},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			return setSettingScalar(target, stmt.ColumnText(0), stmt.ColumnText(1), stmt.ColumnText(2), stmt.ColumnInt64(3), stmt.ColumnInt(4) != 0)
		},
	}); err != nil {
		return err
	}
	lists := map[string][]string{}
	if err := sqlitex.Execute(conn, `SELECT path, value FROM settings2_string_lists WHERE scope = ?1 ORDER BY path, position`, &sqlitex.ExecOptions{
		Args: []any{scope},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			path := stmt.ColumnText(0)
			lists[path] = append(lists[path], stmt.ColumnText(1))
			return nil
		},
	}); err != nil {
		return err
	}
	target.WorkMemory.ExcludeApps = lists["workMemory.excludeApps"]
	target.WorkMemory.ExcludeWindowKeywords = lists["workMemory.excludeWindowKeywords"]
	target.WorkMemory.ExcludePaths = lists["workMemory.excludePaths"]
	target.WorkMemory.ExcludeURLs = lists["workMemory.excludeUrls"]
	target.WorkMemory.ExcludeContentPatterns = lists["workMemory.excludeContentPatterns"]
	target.WorkMemory.AppCaptureProfiles = nil
	if err := sqlitex.Execute(conn, `SELECT id, display_name, process_name, icon, enabled, window_switch_delay_seconds, active_interval_seconds FROM settings2_app_capture_profiles WHERE scope = ?1 ORDER BY id`, &sqlitex.ExecOptions{
		Args: []any{scope},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			target.WorkMemory.AppCaptureProfiles = append(target.WorkMemory.AppCaptureProfiles, WorkMemoryAppCaptureProfile{
				ID:                       stmt.ColumnText(0),
				DisplayName:              stmt.ColumnText(1),
				ProcessName:              stmt.ColumnText(2),
				Icon:                     stmt.ColumnText(3),
				Enabled:                  stmt.ColumnInt(4) != 0,
				WindowSwitchDelaySeconds: stmt.ColumnInt(5),
				ActiveIntervalSeconds:    stmt.ColumnInt(6),
			})
			return nil
		},
	}); err != nil {
		return err
	}
	target.Plugins.Enabled = map[string]bool{}
	return sqlitex.Execute(conn, `SELECT plugin_id, enabled FROM settings2_plugins WHERE scope = ?1`, &sqlitex.ExecOptions{
		Args: []any{scope},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			target.Plugins.Enabled[stmt.ColumnText(0)] = stmt.ColumnInt(1) != 0
			return nil
		},
	})
}

func walkSettings(value reflect.Value, prefix string, fn func(string, reflect.Value) error) error {
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return nil
	}
	valueType := value.Type()
	for index := 0; index < value.NumField(); index++ {
		field := valueType.Field(index)
		if !field.IsExported() {
			continue
		}
		name := jsonFieldName(field)
		if name == "" || name == "-" {
			continue
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		fieldValue := value.Field(index)
		switch fieldValue.Kind() {
		case reflect.Struct:
			if err := walkSettings(fieldValue, path, fn); err != nil {
				return err
			}
		case reflect.String, reflect.Bool, reflect.Int:
			if err := fn(path, fieldValue); err != nil {
				return err
			}
		}
	}
	return nil
}

func setSettingScalar(target *AppSettings, path string, kind string, text string, integer int64, boolean bool) error {
	field, ok := settingFieldByPath(reflect.ValueOf(target).Elem(), strings.Split(path, "."))
	if !ok || !field.CanSet() {
		return nil
	}
	switch field.Kind() {
	case reflect.String:
		if kind == "string" {
			field.SetString(text)
		}
	case reflect.Bool:
		if kind == "bool" {
			field.SetBool(boolean)
		}
	case reflect.Int:
		if kind == "int" {
			field.SetInt(integer)
		} else if parsed, err := strconv.ParseInt(text, 10, 64); err == nil {
			field.SetInt(parsed)
		}
	default:
		return fmt.Errorf("unsupported setting field %s", path)
	}
	return nil
}

func settingFieldByPath(value reflect.Value, parts []string) (reflect.Value, bool) {
	if len(parts) == 0 {
		return reflect.Value{}, false
	}
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return reflect.Value{}, false
	}
	valueType := value.Type()
	for index := 0; index < value.NumField(); index++ {
		fieldType := valueType.Field(index)
		if jsonFieldName(fieldType) != parts[0] {
			continue
		}
		field := value.Field(index)
		if len(parts) == 1 {
			return field, true
		}
		return settingFieldByPath(field, parts[1:])
	}
	return reflect.Value{}, false
}

func jsonFieldName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "" {
		return field.Name
	}
	name, _, _ := strings.Cut(tag, ",")
	return name
}

func settingsScope(path string) string {
	base := filepath.Base(strings.TrimSpace(path))
	ext := filepath.Ext(base)
	scope := strings.TrimSuffix(base, ext)
	scope = strings.TrimSpace(strings.ToLower(scope))
	if scope == "" {
		return "settings"
	}
	return scope
}
