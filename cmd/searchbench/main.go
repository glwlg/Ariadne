package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ariadne/internal/searchbench"
)

func main() {
	var options searchbench.Options
	var outputPath string
	var pretty bool
	var realAppData bool
	var queryList string
	flag.IntVar(&options.Iterations, "iterations", 20, "Number of measured iterations over the query suite")
	flag.IntVar(&options.Warmup, "warmup", 2, "Number of warmup iterations over the query suite")
	flag.Int64Var(&options.TargetP95Ms, "target-p95-ms", 100, "Search p95 target in milliseconds")
	flag.IntVar(&options.SlowestLimit, "slowest", 10, "Number of slowest samples to keep in the report")
	flag.StringVar(&queryList, "queries", "", "Optional comma-separated query suite override")
	flag.StringVar(&outputPath, "output", "", "Optional JSON output path")
	flag.BoolVar(&pretty, "pretty", true, "Pretty-print JSON")
	flag.BoolVar(&realAppData, "real-appdata", false, "Use real APPDATA/LOCALAPPDATA instead of temporary directories")
	flag.Parse()

	options.UseTempAppData = !realAppData
	options.Queries = splitQueries(queryList)
	report := searchbench.Run(options)
	raw, err := marshal(report, pretty)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if outputPath != "" {
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := os.WriteFile(outputPath, raw, 0o600); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	fmt.Println(string(raw))
	if report.Summary.Count == 0 || !report.ActionValidation.OK {
		os.Exit(2)
	}
}

func splitQueries(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	queries := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			queries = append(queries, part)
		}
	}
	return queries
}

func marshal(report searchbench.Report, pretty bool) ([]byte, error) {
	if pretty {
		return json.MarshalIndent(report, "", "  ")
	}
	return json.Marshal(report)
}
