package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"ariadne/internal/shell"
)

func main() {
	var exePath string
	var valueName string
	var outputPath string
	var pretty bool
	flag.StringVar(&exePath, "exe", "", "Ariadne executable path to validate in the temporary HKCU Run command")
	flag.StringVar(&valueName, "value-name", "", "Optional temporary HKCU Run value name")
	flag.StringVar(&outputPath, "output", "", "Optional JSON report path")
	flag.BoolVar(&pretty, "pretty", true, "Pretty-print JSON output")
	flag.Parse()

	report := shell.RunAutostartSmoke(shell.AutostartSmokeOptions{
		Executable: exePath,
		ValueName:  valueName,
	})
	raw, err := marshal(report, pretty)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal report: %v\n", err)
		os.Exit(2)
	}
	if outputPath != "" {
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "create report dir: %v\n", err)
			os.Exit(2)
		}
		if err := os.WriteFile(outputPath, raw, 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "write report: %v\n", err)
			os.Exit(2)
		}
	}
	fmt.Println(string(raw))
	if !report.OK {
		os.Exit(1)
	}
}

func marshal(report shell.AutostartSmokeReport, pretty bool) ([]byte, error) {
	if pretty {
		return json.MarshalIndent(report, "", "  ")
	}
	return json.Marshal(report)
}
