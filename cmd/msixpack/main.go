package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"ariadne/internal/msixpack"
)

func main() {
	productName := flag.String("product", "Ariadne", "product display name")
	packageName := flag.String("package-name", "Ariadne.CommandLauncher", "MSIX package identity name")
	publisher := flag.String("publisher", "CN=Ariadne", "MSIX publisher identity")
	version := flag.String("version", "dev", "MSIX package version")
	exePath := flag.String("exe", "bin/ariadne.exe", "path to Ariadne executable")
	logoPath := flag.String("logo", "assets/logo.png", "path to PNG logo")
	outputDir := flag.String("output", "dist/msix", "output directory")
	pack := flag.Bool("pack", false, "pack the layout with makeappx.exe")
	makeAppxPath := flag.String("makeappx", "makeappx.exe", "path to makeappx.exe")
	flag.Parse()

	result, err := msixpack.Build(msixpack.Options{
		ProductName:  *productName,
		PackageName:  *packageName,
		Publisher:    *publisher,
		Version:      *version,
		ExePath:      *exePath,
		LogoPath:     *logoPath,
		OutputDir:    *outputDir,
		Pack:         *pack,
		MakeAppxPath: *makeAppxPath,
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
