package brandmigration

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

type credentialStore struct {
	values    map[string]string
	failWrite bool
}

func (s *credentialStore) Available() bool { return true }
func (s *credentialStore) Backend() string { return "test" }
func (s *credentialStore) Read(target string) (string, bool, error) {
	value, ok := s.values[target]
	return value, ok, nil
}
func (s *credentialStore) Write(target, secret string) error {
	if s.failWrite {
		return errors.New("sensitive detail that must not appear in errors")
	}
	s.values[target] = secret
	return nil
}
func (s *credentialStore) Delete(target string) error {
	delete(s.values, target)
	return nil
}

func TestImportKeepsOriginalAndRebasesDatabaseAndImagePaths(t *testing.T) {
	base := t.TempDir()
	roaming := filepath.Join(base, "roaming")
	local := filepath.Join(base, "local")
	pictures := filepath.Join(base, "Pictures")
	oldRoot := filepath.Join(roaming, "Ariadne")
	oldImage := filepath.Join(oldRoot, "capture_images", "截图.png")
	oldSaved := filepath.Join(pictures, "Ariadne", "saved.png")
	writeFile(t, oldImage, []byte("image bytes"))
	writeFile(t, oldSaved, []byte("saved image"))
	writeFile(t, filepath.Join(local, "Ariadne", "prefs.json"), []byte(`{"theme":"dark"}`))
	writeFile(t, filepath.Join(oldRoot, "logs", "ariadne.log"), []byte("original log"))
	rawConfig := `{"screenshot":{"saveDir":` + quote(oldSaved) + `,"filenameTemplate":"ariadne_{date}_{time}"},"note":"Ariadne","largeId":9007199254740993}`
	writeFile(t, filepath.Join(oldRoot, "config.json"), []byte(rawConfig))

	source, err := sqlite.OpenConn(filepath.Join(oldRoot, "ariadne.sqlite"), sqlite.OpenReadWrite|sqlite.OpenCreate|sqlite.OpenWAL)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close() // Keep committed rows in the WAL while the import takes its snapshot.
	script := `
PRAGMA wal_autocheckpoint = 0;
CREATE TABLE capture_entries(id TEXT PRIMARY KEY, image_path TEXT, thumbnail_path TEXT, saved_path TEXT);
CREATE TABLE clipboard_entries(id TEXT PRIMARY KEY, text TEXT, image_path TEXT, thumbnail_path TEXT);
CREATE TABLE settings2_values(scope TEXT, path TEXT, text_value TEXT);
CREATE TABLE state_documents(key TEXT PRIMARY KEY, legacy_path TEXT, payload BLOB);
`
	if err := sqlitex.ExecScript(source, script); err != nil {
		t.Fatal(err)
	}
	execute(t, source, `INSERT INTO capture_entries VALUES ('capture', ?1, '', ?2)`, oldImage, oldSaved)
	execute(t, source, `INSERT INTO clipboard_entries VALUES ('clipboard', 'Ariadne is user content', ?1, '')`, oldImage)
	execute(t, source, `INSERT INTO settings2_values VALUES ('config', 'screenshot.filenameTemplate', 'ariadne_{date}_{time}')`)
	execute(t, source, `INSERT INTO state_documents VALUES ('capture_history', ?1, ?2)`,
		filepath.Join(oldRoot, "capture_history.json"), []byte(`{"imagePath":`+quote(oldImage)+`,"text":"Ariadne"}`))
	store := &credentialStore{values: map[string]string{
		"Ariadne/OpenAI/APIKey":    "old-secret",
		"Fancun/Embedding/APIKey":  "keep-new-secret",
		"Ariadne/Embedding/APIKey": "old-embedding-secret",
	}}
	options := Options{RoamingBase: roaming, LocalBase: local, PicturesBase: pictures, Credentials: store}
	if err := Run(options); err != nil {
		t.Fatal(err)
	}
	newRoot := filepath.Join(roaming, "Fancun")
	newImage := filepath.Join(newRoot, "capture_images", "截图.png")
	for _, path := range []string{newImage, filepath.Join(pictures, "Fancun", "saved.png"), filepath.Join(local, "Fancun", "prefs.json"), filepath.Join(newRoot, "logs", "fancun.log")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing imported file %s: %v", path, err)
		}
	}
	imported, err := sqlite.OpenConn(filepath.Join(newRoot, "fancun.sqlite"), sqlite.OpenReadOnly)
	if err != nil {
		t.Fatal(err)
	}
	if value := scalar(t, imported, `SELECT image_path FROM capture_entries`); value != newImage {
		t.Fatalf("capture image not rebased: %s", value)
	}
	if value := scalar(t, imported, `SELECT saved_path FROM capture_entries`); value != filepath.Join(pictures, "Fancun", "saved.png") {
		t.Fatalf("saved image not rebased: %s", value)
	}
	if value := scalar(t, imported, `SELECT text FROM clipboard_entries`); value != "Ariadne is user content" {
		t.Fatalf("clipboard content changed: %s", value)
	}
	if value := scalar(t, imported, `SELECT text_value FROM settings2_values`); value != "fancun_{date}_{time}" {
		t.Fatalf("default filename template not migrated: %s", value)
	}
	payload := scalar(t, imported, `SELECT payload FROM state_documents`)
	if !strings.Contains(payload, `"text": "Ariadne"`) || !strings.Contains(payload, quote(newImage)) {
		t.Fatalf("legacy payload migration lost paths or content: %s", payload)
	}
	imported.Close()
	if scalar(t, source, `SELECT image_path FROM capture_entries`) != oldImage {
		t.Fatal("original database was modified")
	}
	raw, err := os.ReadFile(filepath.Join(oldRoot, "config.json"))
	if err != nil || string(raw) != rawConfig {
		t.Fatal("original JSON was modified")
	}
	raw, err = os.ReadFile(filepath.Join(newRoot, "config.json"))
	if err != nil || !strings.Contains(string(raw), "9007199254740993") {
		t.Fatal("JSON integer precision was lost")
	}
	if store.values["Fancun/OpenAI/APIKey"] != "old-secret" || store.values["Ariadne/OpenAI/APIKey"] != "old-secret" || store.values["Fancun/Embedding/APIKey"] != "keep-new-secret" {
		t.Fatal("credentials were not copied without overwriting the original or new values")
	}
	delete(store.values, "Fancun/OpenAI/APIKey")
	if err := os.WriteFile(newImage, []byte("new user change"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Run(options); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(newImage)
	if string(raw) != "new user change" {
		t.Fatal("repeated migration overwrote new user data")
	}
	if _, exists := store.values["Fancun/OpenAI/APIKey"]; exists {
		t.Fatal("repeated migration resurrected a deleted credential")
	}
}

func TestExistingDestinationStopsBeforeAnyImport(t *testing.T) {
	base := t.TempDir()
	roaming, local := filepath.Join(base, "roaming"), filepath.Join(base, "local")
	writeFile(t, filepath.Join(roaming, "Ariadne", "old.txt"), []byte("old"))
	writeFile(t, filepath.Join(local, "Ariadne", "old.txt"), []byte("old"))
	destination := filepath.Join(local, "Fancun", "keep.txt")
	writeFile(t, destination, []byte("keep"))
	if err := Run(Options{RoamingBase: roaming, LocalBase: local}); err == nil || !strings.Contains(err.Error(), "目录冲突") {
		t.Fatalf("existing destination must stop import: %v", err)
	}
	if _, err := os.Stat(filepath.Join(roaming, "Fancun")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("preflight conflict should not publish another root")
	}
	raw, _ := os.ReadFile(destination)
	if string(raw) != "keep" {
		t.Fatal("existing destination was changed")
	}
}

func TestImportRenamesBuiltinsWithoutChangingCustomLabels(t *testing.T) {
	base := t.TempDir()
	legacy := filepath.Join(base, "Ariadne")
	writeFile(t, filepath.Join(legacy, "launchers.json"), []byte(`{"launchers":[
{"id":"ariadne-config-dir","name":"我的 Ariadne 配置","keywords":["ariadne","custom"]}],
"removedIds":["ariadne-config-dir"]}`))
	writeFile(t, filepath.Join(legacy, "search_state.json"),
		[]byte(`{"records":[{"resultId":"launcher-ariadne-config-dir","favorite":true}]}`))
	conn, err := sqlite.OpenConn(filepath.Join(legacy, "ariadne.sqlite"), sqlite.OpenReadWrite|sqlite.OpenCreate)
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlitex.ExecScript(conn, `
PRAGMA foreign_keys = ON;
CREATE TABLE launchers(id TEXT PRIMARY KEY, name TEXT, target TEXT, working_dir TEXT);
CREATE TABLE launcher_keywords(launcher_id TEXT REFERENCES launchers(id), position INTEGER, value TEXT);
CREATE TABLE launcher_tags(launcher_id TEXT REFERENCES launchers(id), position INTEGER, value TEXT);
CREATE TABLE launcher_removed(id TEXT PRIMARY KEY);
CREATE TABLE search_usage(result_id TEXT PRIMARY KEY, favorite INTEGER);
CREATE TABLE settings2_string_lists(scope TEXT, path TEXT, value TEXT);
INSERT INTO launchers VALUES ('ariadne-config-dir','Ariadne 配置目录','','');
INSERT INTO launcher_keywords VALUES ('ariadne-config-dir',0,'ariadne');
INSERT INTO launcher_tags VALUES ('ariadne-config-dir',0,'自定义标签');
INSERT INTO launcher_removed VALUES ('ariadne-config-dir');
INSERT INTO search_usage VALUES ('launcher-ariadne-config-dir',1);
`); err != nil {
		t.Fatal(err)
	}
	oldPattern := `(?i)(^|[\\/])ariadne[\\/](capture_images|capture_thumbnails)([\\/]|$)`
	execute(t, conn, `INSERT INTO settings2_string_lists VALUES ('config', 'search.fileExcludePatterns', ?1)`, oldPattern)
	execute(t, conn, `INSERT INTO settings2_string_lists VALUES ('config', 'search.fileExcludePatterns', 'my-ariadne-pattern')`)
	conn.Close()
	if err := Run(Options{RoamingBase: base}); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(base, "Fancun")
	conn, err = sqlite.OpenConn(filepath.Join(target, "fancun.sqlite"), sqlite.OpenReadOnly)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for query, want := range map[string]string{
		`SELECT id FROM launchers`:                                 "fancun-config-dir",
		`SELECT name FROM launchers`:                               "Fancun 配置目录",
		`SELECT launcher_id FROM launcher_keywords`:                "fancun-config-dir",
		`SELECT value FROM launcher_keywords`:                      "fancun",
		`SELECT launcher_id FROM launcher_tags`:                    "fancun-config-dir",
		`SELECT value FROM launcher_tags`:                          "自定义标签",
		`SELECT id FROM launcher_removed`:                          "fancun-config-dir",
		`SELECT result_id FROM search_usage`:                       "launcher-fancun-config-dir",
		`SELECT value FROM settings2_string_lists WHERE rowid = 1`: rewriteExcludePattern(oldPattern),
		`SELECT value FROM settings2_string_lists WHERE rowid = 2`: "my-ariadne-pattern",
	} {
		if got := scalar(t, conn, query); got != want {
			t.Fatalf("%s: got %q, want %q", query, got, want)
		}
	}
	for _, name := range []string{"launchers.json", "search_state.json"} {
		raw, err := os.ReadFile(filepath.Join(target, name))
		if err != nil || strings.Contains(string(raw), "ariadne-config-dir") {
			t.Fatalf("builtin JSON identity not migrated: %s: %s: %v", name, raw, err)
		}
		if name == "launchers.json" && !strings.Contains(string(raw), "我的 Ariadne 配置") {
			t.Fatal("custom launcher label changed")
		}
	}
}

func TestCredentialFailureCanResumeWithoutLeakingSecrets(t *testing.T) {
	base := t.TempDir()
	writeFile(t, filepath.Join(base, "Ariadne", "history.txt"), []byte("history"))
	store := &credentialStore{values: map[string]string{"Ariadne/OpenAI/APIKey": "secret"}, failWrite: true}
	options := Options{RoamingBase: base, Credentials: store}
	if err := Run(options); err == nil || strings.Contains(err.Error(), "sensitive detail") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("credential error must be explicit and omit secret details: %v", err)
	}
	store.failWrite = false
	if err := Run(options); err != nil {
		t.Fatal(err)
	}
	if store.values["Fancun/OpenAI/APIKey"] != "secret" {
		t.Fatal("credential migration did not resume")
	}
}

func TestImportVirtualizedMSIXDataIntoCurrentRoamingDirectory(t *testing.T) {
	base := t.TempDir()
	roaming, local := filepath.Join(base, "roaming"), filepath.Join(base, "local")
	source := filepath.Join(local, "Packages", "glwlg.Ariadne_test", "LocalCache", "Roaming", "Ariadne")
	oldLogical := filepath.Join(roaming, "Ariadne", "capture_images", "old.png")
	writeFile(t, filepath.Join(source, "capture_history.json"), []byte(`{"imagePath":`+quote(oldLogical)+`}`))
	writeFile(t, filepath.Join(source, "capture_images", "old.png"), []byte("image"))
	options := Options{RoamingBase: roaming, LocalBase: local}
	if err := Run(options); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(roaming, "Fancun")
	raw, err := os.ReadFile(filepath.Join(target, "capture_history.json"))
	if err != nil || !strings.Contains(string(raw), quote(filepath.Join(target, "capture_images", "old.png"))) {
		t.Fatalf("virtualized data lost logical image paths: %s: %v", raw, err)
	}
	if err := Run(options); err != nil {
		t.Fatalf("virtualized import should be idempotent: %v", err)
	}
}

func TestMultipleLegacyProfilesStopBeforePublishing(t *testing.T) {
	base := t.TempDir()
	roaming, local := filepath.Join(base, "roaming"), filepath.Join(base, "local")
	writeFile(t, filepath.Join(roaming, "Ariadne", "config.json"), []byte(`{}`))
	writeFile(t, filepath.Join(local, "Packages", "old", "LocalCache", "Roaming", "Ariadne", "config.json"), []byte(`{}`))
	if err := Run(Options{RoamingBase: roaming, LocalBase: local}); err == nil || !strings.Contains(err.Error(), "多个旧版数据目录") {
		t.Fatalf("ambiguous legacy data should stop: %v", err)
	}
	if _, err := os.Stat(filepath.Join(roaming, "Fancun")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("ambiguous source created a destination")
	}
}

func TestExplicitProfileSelectionPersistsAcrossLaunches(t *testing.T) {
	for _, virtualized := range []bool{false, true} {
		t.Run(fmt.Sprintf("virtualized=%v", virtualized), func(t *testing.T) {
			base := t.TempDir()
			roaming, local := filepath.Join(base, "roaming"), filepath.Join(base, "local")
			normal := filepath.Join(roaming, "Ariadne")
			virtual := filepath.Join(local, "Packages", "old", "LocalCache", "Roaming", "Ariadne")
			writeFile(t, filepath.Join(normal, "profile.txt"), []byte("normal"))
			writeFile(t, filepath.Join(virtual, "profile.txt"), []byte("virtual"))
			writeFile(t, filepath.Join(local, "Ariadne", "cache.txt"), []byte("cache"))
			source, expected := normal, "normal"
			if virtualized {
				source, expected = virtual, "virtual"
			}
			options := Options{RoamingBase: roaming, LocalBase: local, RoamingSource: source}
			if err := Run(options); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(filepath.Join(roaming, "Fancun", "profile.txt"))
			if err != nil || string(raw) != expected {
				t.Fatalf("wrong profile imported: %q: %v", raw, err)
			}
			if _, err := os.Stat(filepath.Join(local, "Fancun", "cache.txt")); err != nil {
				t.Fatal("local data was not imported")
			}
			for path, expected := range map[string]string{normal: "normal", virtual: "virtual"} {
				raw, err := os.ReadFile(filepath.Join(path, "profile.txt"))
				if err != nil || string(raw) != expected {
					t.Fatalf("original profile changed: %s", path)
				}
			}
			options.RoamingSource = ""
			if err := Run(options); err != nil {
				t.Fatalf("normal startup must reuse recorded selection: %v", err)
			}
			options.RoamingSource = normal
			if !virtualized {
				options.RoamingSource = virtual
			}
			if err := Run(options); err == nil {
				t.Fatal("must not change the source of an existing import")
			}
		})
	}
}

func TestExplicitProfileSelectionRejectsUnknownOrMissingSource(t *testing.T) {
	base := t.TempDir()
	roaming := filepath.Join(base, "roaming")
	for _, source := range []string{filepath.Join(base, "unrelated"), filepath.Join(roaming, "Ariadne")} {
		err := Run(Options{RoamingBase: roaming, RoamingSource: source})
		if err == nil {
			t.Fatalf("accepted unavailable source: %s", source)
		}
		if _, err := os.Stat(filepath.Join(roaming, "Fancun")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("invalid selection created a destination")
		}
	}
}

func TestRunningLegacyStopsBeforeCopy(t *testing.T) {
	base := t.TempDir()
	writeFile(t, filepath.Join(base, "Ariadne", "history.txt"), []byte("history"))
	err := Run(Options{RoamingBase: base, CheckLegacyStopped: func() error { return errors.New("legacy still running") }})
	if err == nil {
		t.Fatal("running source must stop migration")
	}
	if _, err := os.Stat(filepath.Join(base, "Fancun")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("running source must not publish imported data")
	}
}

func TestInvalidJSONDoesNotPublishPartialDestination(t *testing.T) {
	base := t.TempDir()
	writeFile(t, filepath.Join(base, "Ariadne", "config.json"), []byte(`{"theme":`))
	if err := Run(Options{RoamingBase: base}); err == nil {
		t.Fatal("invalid source JSON must stop migration")
	}
	if _, err := os.Stat(filepath.Join(base, "Fancun")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("failed import left a published destination")
	}
}

func TestRewritePathRequiresDirectoryBoundaryAndPreservesUserText(t *testing.T) {
	roots := []root{{Source: filepath.Join("base", "Ariadne"), Target: filepath.Join("base", "Fancun")}}
	sibling := filepath.Join("base", "Ariadne-other", "file")
	if rewritePath(sibling, roots) != sibling {
		t.Fatal("similarly named sibling directory was rewritten")
	}
	raw := []byte(`{"text":"base/Ariadne/file","imagePath":"base/Ariadne/file","name":"Ariadne"}`)
	updated, err := rewriteJSON(raw, roots)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]string
	if err := json.Unmarshal(updated, &result); err != nil {
		t.Fatal(err)
	}
	if result["text"] != "base/Ariadne/file" || result["name"] != "Ariadne" || result["imagePath"] != filepath.Join("base", "Fancun", "file") {
		t.Fatalf("migration must only rewrite structured path fields: %s", updated)
	}
}

func writeFile(t *testing.T, path string, contents []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
}

func execute(t *testing.T, conn *sqlite.Conn, query string, args ...any) {
	t.Helper()
	if err := sqlitex.Execute(conn, query, &sqlitex.ExecOptions{Args: args}); err != nil {
		t.Fatal(err)
	}
}

func scalar(t *testing.T, conn *sqlite.Conn, query string) string {
	t.Helper()
	var result string
	if err := sqlitex.Execute(conn, query, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error { result = stmt.ColumnText(0); return nil },
	}); err != nil {
		t.Fatal(err)
	}
	return result
}

func quote(value string) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}
