package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"ariadne/internal/perfcheck"
)

func main() {
	var options perfcheck.Options
	var outputPath string
	var pretty bool
	var realAppData bool
	flag.StringVar(&options.ExePath, "exe", filepath.Join("bin", "ariadne.exe"), "Path to ariadne.exe")
	flag.StringVar(&options.ReleaseZipPath, "release-zip", filepath.Join("dist", "release", "ariadne-dev-windows-x64.zip"), "Path to Ariadne release zip")
	flag.StringVar(&options.LegacyInstallerPath, "legacy-installer", filepath.Join("..", "..", "dist", "x-tools-setup.exe"), "Path to legacy x-tools installer")
	flag.StringVar(&options.LegacyDistPath, "legacy-dist", filepath.Join("..", "..", "dist", "x-tools"), "Path to legacy x-tools onedir output")
	flag.IntVar(&options.Iterations, "iterations", 3, "Number of startup samples to collect")
	flag.IntVar(&options.HotkeyIterations, "hotkey-iterations", 3, "Number of Alt+Q hotkey samples to collect")
	flag.Int64Var(&options.TimeoutMs, "timeout-ms", 8000, "Startup window timeout per sample")
	flag.StringVar(&outputPath, "output", "", "Optional JSON output path")
	flag.BoolVar(&pretty, "pretty", true, "Pretty-print JSON")
	flag.BoolVar(&realAppData, "real-appdata", false, "Use real APPDATA/LOCALAPPDATA instead of temporary directories")
	flag.Parse()

	options.UseTempAppData = !realAppData
	report := perfcheck.Run(options)
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
	if report.Startup.Count == 0 {
		os.Exit(2)
	}
}

func marshal(report perfcheck.Report, pretty bool) ([]byte, error) {
	if pretty {
		return json.MarshalIndent(report, "", "  ")
	}
	return json.Marshal(report)
}
