package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"ariadne/internal/releasepack"
)

func main() {
	version := flag.String("version", "dev", "release version")
	exePath := flag.String("exe", "bin/ariadne.exe", "path to ariadne.exe")
	iconPath := flag.String("icon", "assets/logo.ico", "path to logo.ico")
	outputDir := flag.String("output", "dist/release", "output directory")
	flag.Parse()

	result, err := releasepack.Build(releasepack.Options{
		Version:   *version,
		ExePath:   *exePath,
		IconPath:  *iconPath,
		OutputDir: *outputDir,
	})
	if err != nil {
		log.Fatal(err)
	}

	raw, err := json.MarshalIndent(result.Manifest, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintln(os.Stdout, string(raw))
}
