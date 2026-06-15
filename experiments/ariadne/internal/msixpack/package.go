package msixpack

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type Options struct {
	ProductName  string
	PackageName  string
	Publisher    string
	Version      string
	ExePath      string
	LogoPath     string
	OutputDir    string
	CreatedAt    time.Time
	Pack         bool
	MakeAppxPath string
}

type File struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	ProductName       string   `json:"productName"`
	PackageName       string   `json:"packageName"`
	Publisher         string   `json:"publisher"`
	Version           string   `json:"version"`
	CreatedAt         int64    `json:"createdAt"`
	Platform          string   `json:"platform"`
	Arch              string   `json:"arch"`
	LayoutDir         string   `json:"layoutDir"`
	AppxManifestPath  string   `json:"appxManifestPath"`
	CandidateMsixPath string   `json:"candidateMsixPath"`
	MsixPath          string   `json:"msixPath,omitempty"`
	MakeAppxPath      string   `json:"makeAppxPath,omitempty"`
	Packed            bool     `json:"packed"`
	Files             []File   `json:"files"`
	MsixFile          *File    `json:"msixFile,omitempty"`
	InstallNotes      []string `json:"installNotes"`
	SigningNotes      []string `json:"signingNotes"`
	VerificationNotes []string `json:"verificationNotes"`
}

type Result struct {
	Manifest Manifest `json:"manifest"`
}

func Build(options Options) (Result, error) {
	options = normalizeOptions(options)
	if err := validateOptions(options); err != nil {
		return Result{}, err
	}
	outputDir, err := filepath.Abs(options.OutputDir)
	if err != nil {
		return Result{}, err
	}
	options.OutputDir = outputDir

	layoutDir := filepath.Join(options.OutputDir, safePackageName(options.ProductName, options.Version)+"-msix")
	msixPath := filepath.Join(options.OutputDir, safePackageName(options.ProductName, options.Version)+".msix")
	if err := resetLayoutDir(options.OutputDir, layoutDir, msixPath); err != nil {
		return Result{}, err
	}

	assetsDir := filepath.Join(layoutDir, "Assets")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		return Result{}, err
	}

	files := []File{}
	if file, err := copyFile(options.ExePath, filepath.Join(layoutDir, "Ariadne.exe"), layoutDir); err != nil {
		return Result{}, err
	} else {
		files = append(files, file)
	}
	for _, name := range []string{"Square44x44Logo.png", "Square150x150Logo.png", "StoreLogo.png"} {
		file, err := copyFile(options.LogoPath, filepath.Join(assetsDir, name), layoutDir)
		if err != nil {
			return Result{}, err
		}
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	appxManifestPath := filepath.Join(layoutDir, "AppxManifest.xml")
	if err := os.WriteFile(appxManifestPath, []byte(appxManifestXML(options)), 0o644); err != nil {
		return Result{}, err
	}
	files = append(files, fileMetadata(appxManifestPath, layoutDir))

	readmePath := filepath.Join(layoutDir, "README-msix.txt")
	if err := os.WriteFile(readmePath, []byte(readmeText(options)), 0o644); err != nil {
		return Result{}, err
	}
	files = append(files, fileMetadata(readmePath, layoutDir))

	manifest := Manifest{
		ProductName:       options.ProductName,
		PackageName:       options.PackageName,
		Publisher:         options.Publisher,
		Version:           options.Version,
		CreatedAt:         options.CreatedAt.Unix(),
		Platform:          "windows",
		Arch:              runtime.GOARCH,
		LayoutDir:         layoutDir,
		AppxManifestPath:  appxManifestPath,
		CandidateMsixPath: msixPath,
		MakeAppxPath:      options.MakeAppxPath,
		Files:             files,
		InstallNotes: []string{
			"MSIX layout is generated for a full-trust Windows desktop app.",
			"Installable MSIX packages must be signed before Add-AppxPackage on ordinary Windows policies.",
			"User data under %APPDATA%\\Ariadne remains outside the MSIX package and is not removed by package generation.",
		},
		SigningNotes: []string{
			"Create or choose a code-signing certificate whose subject matches the manifest Publisher.",
			"Pack with makeappx.exe, then sign the .msix with signtool.exe before distribution.",
			"This generator never embeds API keys, work memory exports, screenshots, or legacy x-tools data.",
		},
		VerificationNotes: []string{
			"Inspect AppxManifest.xml before signing.",
			"Use Windows Settings > Apps or Remove-AppxPackage for MSIX uninstall smoke tests.",
			"Keep the existing user-level release zip as the fallback package until signed MSIX install is verified.",
		},
	}

	if options.Pack {
		if err := packWithMakeAppx(options.MakeAppxPath, layoutDir, msixPath); err != nil {
			return Result{}, err
		}
		manifest.Packed = true
		manifest.MsixPath = msixPath
		msixFile := fileMetadata(msixPath, options.OutputDir)
		manifest.MsixFile = &msixFile
	}
	if err := writeJSON(filepath.Join(layoutDir, "msix-manifest.json"), manifest); err != nil {
		return Result{}, err
	}
	return Result{Manifest: manifest}, nil
}

func normalizeOptions(options Options) Options {
	if options.ProductName == "" {
		options.ProductName = "Ariadne"
	}
	if options.PackageName == "" {
		options.PackageName = "Ariadne.CommandLauncher"
	}
	if options.Publisher == "" {
		options.Publisher = "CN=Ariadne"
	}
	if options.Version == "" {
		options.Version = "0.0.0.0"
	}
	options.Version = normalizeVersion(options.Version)
	if options.ExePath == "" {
		options.ExePath = filepath.Join("bin", "ariadne.exe")
	}
	if options.LogoPath == "" {
		options.LogoPath = filepath.Join("assets", "logo.png")
	}
	if options.OutputDir == "" {
		options.OutputDir = filepath.Join("dist", "msix")
	}
	if options.CreatedAt.IsZero() {
		options.CreatedAt = time.Now()
	}
	if options.MakeAppxPath == "" {
		options.MakeAppxPath = "makeappx.exe"
	}
	options.ExePath = filepath.Clean(options.ExePath)
	options.LogoPath = filepath.Clean(options.LogoPath)
	options.OutputDir = filepath.Clean(options.OutputDir)
	return options
}

func validateOptions(options Options) error {
	if options.ProductName == "" || options.PackageName == "" || options.Publisher == "" || options.Version == "" {
		return errors.New("product name, package name, publisher, and version are required")
	}
	if info, err := os.Stat(options.ExePath); err != nil || info.IsDir() {
		return fmt.Errorf("Ariadne executable not found at %s", options.ExePath)
	}
	if info, err := os.Stat(options.LogoPath); err != nil || info.IsDir() {
		return fmt.Errorf("Ariadne logo not found at %s", options.LogoPath)
	}
	if options.Pack {
		if _, err := exec.LookPath(options.MakeAppxPath); err != nil {
			return fmt.Errorf("makeappx not found at %s: %w", options.MakeAppxPath, err)
		}
	}
	return nil
}

func appxManifestXML(options Options) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<Package xmlns="http://schemas.microsoft.com/appx/manifest/foundation/windows10"
         xmlns:uap="http://schemas.microsoft.com/appx/manifest/uap/windows10"
         xmlns:rescap="http://schemas.microsoft.com/appx/manifest/foundation/windows10/restrictedcapabilities">
  <Identity Name="%s" Publisher="%s" Version="%s" ProcessorArchitecture="%s" />
  <Properties>
    <DisplayName>%s</DisplayName>
    <PublisherDisplayName>Ariadne</PublisherDisplayName>
    <Logo>Assets\StoreLogo.png</Logo>
  </Properties>
  <Dependencies>
    <TargetDeviceFamily Name="Windows.Desktop" MinVersion="10.0.17763.0" MaxVersionTested="10.0.22621.0" />
  </Dependencies>
  <Resources>
    <Resource Language="zh-CN" />
    <Resource Language="en-US" />
  </Resources>
  <Applications>
    <Application Id="Ariadne" Executable="Ariadne.exe" EntryPoint="Windows.FullTrustApplication">
      <uap:VisualElements DisplayName="%s"
                          Description="Ariadne command launcher and work memory center"
                          BackgroundColor="#0f766e"
                          Square44x44Logo="Assets\Square44x44Logo.png"
                          Square150x150Logo="Assets\Square150x150Logo.png"
                          AppListEntry="default" />
    </Application>
  </Applications>
  <Capabilities>
    <rescap:Capability Name="runFullTrust" />
  </Capabilities>
</Package>
`, xmlEscape(options.PackageName), xmlEscape(options.Publisher), xmlEscape(options.Version), msixArch(runtime.GOARCH), xmlEscape(options.ProductName), xmlEscape(options.ProductName))
}

func readmeText(options Options) string {
	return strings.TrimSpace(fmt.Sprintf(`%s MSIX layout

This directory contains an unsigned MSIX layout for the Ariadne full-trust desktop app.

Package identity: %s
Publisher: %s
Version: %s

Distribution checklist:
1. Inspect AppxManifest.xml.
2. Pack with makeappx.exe if the Windows SDK is installed.
3. Sign the resulting .msix with a certificate whose subject matches the Publisher.
4. Install with Add-AppxPackage and verify Alt+Q, tool windows, rollback, and credential access.

This layout intentionally does not include work memory exports, screenshots, clipboard images, API keys, or legacy x-tools data.
`, options.ProductName, options.PackageName, options.Publisher, options.Version)) + "\n"
}

func packWithMakeAppx(makeAppx string, layoutDir string, msixPath string) error {
	cmd := exec.Command(makeAppx, "pack", "/d", layoutDir, "/p", msixPath, "/overwrite")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("makeappx pack failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func resetLayoutDir(outputDir string, layoutDir string, msixPath string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	cleanOutput, err := filepath.Abs(outputDir)
	if err != nil {
		return err
	}
	cleanLayout, err := filepath.Abs(layoutDir)
	if err != nil {
		return err
	}
	if !sameOrInside(cleanLayout, cleanOutput) || strings.EqualFold(cleanLayout, cleanOutput) {
		return fmt.Errorf("refusing to reset MSIX layout outside output dir: %s", layoutDir)
	}
	if err := os.RemoveAll(layoutDir); err != nil {
		return err
	}
	if err := os.Remove(msixPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func copyFile(source string, target string, layoutDir string) (File, error) {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return File{}, err
	}
	input, err := os.Open(source)
	if err != nil {
		return File{}, err
	}
	defer input.Close()
	output, err := os.Create(target)
	if err != nil {
		return File{}, err
	}
	defer output.Close()
	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(output, hasher), input)
	if err != nil {
		return File{}, err
	}
	rel, err := filepath.Rel(layoutDir, target)
	if err != nil {
		return File{}, err
	}
	return File{Path: filepath.ToSlash(rel), Bytes: written, SHA256: hex.EncodeToString(hasher.Sum(nil))}, nil
}

func fileMetadata(path string, layoutDir string) File {
	raw, err := os.ReadFile(path)
	if err != nil {
		return File{Path: filepath.ToSlash(path)}
	}
	rel, err := filepath.Rel(layoutDir, path)
	if err != nil {
		rel = path
	}
	hash := sha256.Sum256(raw)
	return File{Path: filepath.ToSlash(rel), Bytes: int64(len(raw)), SHA256: hex.EncodeToString(hash[:])}
}

func writeJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" || strings.EqualFold(version, "dev") {
		return "0.0.0.0"
	}
	parts := strings.Split(version, ".")
	for len(parts) < 4 {
		parts = append(parts, "0")
	}
	if len(parts) > 4 {
		parts = parts[:4]
	}
	for index, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			part = "0"
		}
		for _, char := range part {
			if char < '0' || char > '9' {
				return "0.0.0.0"
			}
		}
		parts[index] = part
	}
	return strings.Join(parts, ".")
}

func safePackageName(product string, version string) string {
	clean := strings.ToLower(strings.TrimSpace(product))
	clean = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		default:
			return '-'
		}
	}, clean)
	clean = strings.Trim(clean, "-")
	if clean == "" {
		clean = "ariadne"
	}
	return clean + "-" + strings.ReplaceAll(normalizeVersion(version), ".", "-")
}

func msixArch(goarch string) string {
	switch goarch {
	case "amd64":
		return "x64"
	case "386":
		return "x86"
	case "arm64":
		return "arm64"
	default:
		return "neutral"
	}
}

func xmlEscape(value string) string {
	var buffer bytes.Buffer
	if err := xml.EscapeText(&buffer, []byte(value)); err != nil {
		return value
	}
	return buffer.String()
}

func sameOrInside(path string, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}
