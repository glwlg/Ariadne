package main

import (
	"fmt"
	"os"

	"ariadne/internal/appdb"
	"ariadne/internal/capturehistory"
	"ariadne/internal/checklists"
	"ariadne/internal/clipboardhistory"
	"ariadne/internal/hosts"
	"ariadne/internal/imageindex"
	"ariadne/internal/launchers"
	"ariadne/internal/plugins"
	"ariadne/internal/search"
	"ariadne/internal/settings"
	"ariadne/internal/skills"
	"ariadne/internal/toolwindows"
	"ariadne/internal/workflows"
	"ariadne/internal/workmemory"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func main() {
	settingsService := settings.NewService()
	captureService := capturehistory.NewService()
	clipboardService := clipboardhistory.NewService(captureService)
	workMemoryService := workmemory.NewService(captureService)
	defer workMemoryService.Stop()

	searchService := search.NewService()
	pluginService := plugins.NewService()
	workflowService := workflows.NewService(pluginService)
	checklistService := checklists.NewService()
	skillService := skills.NewService()
	launcherService := launchers.NewService()
	hostsService := hosts.NewService()
	imageIndexService := imageindex.NewService(captureService, clipboardService, nil)
	toolWindowService := toolwindows.NewService()
	defer toolWindowService.Stop()

	storage := settingsService.StorageStatus()
	fmt.Fprintf(os.Stdout, "settings: %s (%d bytes, readback=%v)\n", storage.Path, storage.ReadBackBytes, storage.ReadBackOK)
	fmt.Fprintf(os.Stdout, "search: %s (%d records)\n", searchService.UsageStatus().Path, searchService.UsageStatus().Count)
	fmt.Fprintf(os.Stdout, "launchers: %s (%d items)\n", launcherService.Status().Path, launcherService.Status().Count)
	fmt.Fprintf(os.Stdout, "capture_history: %s (%d entries)\n", captureService.Status().Path, captureService.Status().Count)
	fmt.Fprintf(os.Stdout, "clipboard_history: %s (%d entries)\n", clipboardService.Status().Path, clipboardService.Status().Count)
	workMemoryStatus := workMemoryService.Status()
	fmt.Fprintf(os.Stdout, "work_memory: %s (%d entries)\n", workMemoryStatus.StoragePath, workMemoryStatus.EntryCount)
	fmt.Fprintf(os.Stdout, "hosts: %s (%d profiles)\n", hostsService.Status().ConfigPath, hostsService.Status().Count)
	fmt.Fprintf(os.Stdout, "workflows: %s (%d workflows)\n", workflowService.Status().Path, workflowService.Status().Count)
	fmt.Fprintf(os.Stdout, "checklists: %s (%d checklists)\n", checklistService.Status().Path, checklistService.Status().Count)
	fmt.Fprintf(os.Stdout, "skills: %s (%d skills)\n", skillService.Status().Path, skillService.Status().Count)
	fmt.Fprintf(os.Stdout, "image_index: %s (%d entries)\n", imageIndexService.Status().Path, imageIndexService.Status().Count)
	fmt.Fprintf(os.Stdout, "network_mini: %s\n", toolWindowService.NetworkMiniStatus().ConfigPath)
	if counts, err := sqliteCounts(storage.Path); err == nil {
		fmt.Fprintln(os.Stdout, "sqlite_counts:")
		for _, item := range counts {
			fmt.Fprintf(os.Stdout, "  %s: %d\n", item.Name, item.Count)
		}
	} else {
		fmt.Fprintf(os.Stdout, "sqlite_counts_error: %v\n", err)
	}
}

type tableCount struct {
	Name  string
	Count int
}

func sqliteCounts(path string) ([]tableCount, error) {
	tables := []string{
		"settings2_values",
		"settings2_string_lists",
		"settings2_app_capture_profiles",
		"settings2_plugins",
		"capture_entries",
		"clipboard_entries",
		"work_memory_entries",
		"work_memory_frames",
		"work_memory_decisions",
		"work_memory_autonomous_artifacts",
		"work_memory_embedding_records",
		"work_memory_embedding_vector_values",
		"search_usage",
		"launchers",
		"checklists",
		"skills",
		"workflows",
		"hosts_profiles",
		"image_index_entries",
		"tool_window_network_mini",
		"state_documents",
	}
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	counts := make([]tableCount, 0, len(tables))
	for _, table := range tables {
		exists, err := sqliteTableExists(conn, table)
		if err != nil {
			return nil, err
		}
		if !exists {
			counts = append(counts, tableCount{Name: table, Count: 0})
			continue
		}
		count, err := sqliteTableCount(conn, table)
		if err != nil {
			return nil, err
		}
		counts = append(counts, tableCount{Name: table, Count: count})
	}
	return counts, nil
}

func sqliteTableExists(conn *sqlite.Conn, table string) (bool, error) {
	exists := false
	err := sqlitex.Execute(conn, `SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?1`, &sqlitex.ExecOptions{
		Args: []any{table},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			exists = true
			return nil
		},
	})
	return exists, err
}

func sqliteTableCount(conn *sqlite.Conn, table string) (int, error) {
	count := 0
	err := sqlitex.Execute(conn, `SELECT count(*) FROM `+table, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			count = stmt.ColumnInt(0)
			return nil
		},
	})
	return count, err
}
