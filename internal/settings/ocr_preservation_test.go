package settings

import (
	"path/filepath"
	"reflect"
	"testing"

	"ariadne/internal/appdb"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func TestMemoryRemovalPreservesOCRAndHistorySettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	service := NewServiceWithPaths(path, "")
	config := service.GetSettings()
	config.AI = AISettings{
		OCRModelEnabled: true, OCRProvider: "openai-compatible", OCRModel: "screenshot-model",
	}
	service.UpdateSettings(config)
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlitex.Execute(conn, `INSERT INTO settings2_values(scope,path,type,text_value,int_value,bool_value) VALUES
('config','ai.baseUrl','string','http://screenshot.test/v1',0,0),
('config','workMemory.retentionDays','int','',47,0)`, nil); err != nil {
		t.Fatal(err)
	}
	if err := sqlitex.Execute(conn, `DELETE FROM settings2_values WHERE scope='config' AND path='history.retentionDays'`, nil); err != nil {
		t.Fatal(err)
	}
	conn.Close()

	got := NewServiceWithPaths(path, "").GetSettings()
	if !got.AI.OCRModelEnabled || got.AI.OCRModel != config.AI.OCRModel || got.AI.OCRBaseURL != "http://screenshot.test/v1" {
		t.Fatalf("screenshot OCR configuration changed: %#v", got.AI)
	}
	if got.History.RetentionDays != 47 {
		t.Fatalf("history retention changed: %#v", got.History)
	}
	reloaded := NewServiceWithPaths(path, "")
	reloaded.UpdateSettings(got)
	if after := NewServiceWithPaths(path, "").GetSettings(); after.AI != got.AI || after.History != got.History {
		t.Fatalf("OCR or history settings did not survive saving: %#v", after)
	}
}

func TestSharedOCRMigrationKeepsScreenshotConfigurationAndRemovesPinnedSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	config := defaultSettings()
	config.Version = 22
	config.AI = AISettings{
		OCRModelEnabled: true, OCRProvider: "openai-compatible",
		OCRBaseURL: "http://screenshot.test/v1", OCRModel: "screenshot-model",
	}
	if err := saveSettingsToSQLite(path, config); err != nil {
		t.Fatal(err)
	}
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlitex.Execute(conn, `INSERT INTO settings2_values(scope,path,type,text_value,int_value,bool_value) VALUES
('config','ai.pinnedOcrModelEnabled','bool','',0,1),
('config','ai.pinnedOcrProvider','string','ollama-generate',0,0),
('config','ai.pinnedOcrBaseUrl','string','http://pinned.test',0,0),
('config','ai.pinnedOcrModel','string','pinned-model',0,0)`, nil); err != nil {
		t.Fatal(err)
	}
	conn.Close()

	for range 2 {
		got := NewServiceWithPaths(path, "").GetSettings()
		if got.Version != currentSettingsVersion || got.AI != config.AI || got.History != config.History || !reflect.DeepEqual(got.Screenshot, config.Screenshot) {
			t.Fatalf("shared OCR migration changed preserved settings: %#v", got)
		}
	}
	conn, err = appdb.OpenForPath(path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := sqlitex.Execute(conn, `SELECT count(*) FROM settings2_values WHERE scope='config' AND path LIKE 'ai.pinnedOcr%'`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			if count := stmt.ColumnInt(0); count != 0 {
				t.Fatalf("obsolete pinned settings remain: %d", count)
			}
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}
}
