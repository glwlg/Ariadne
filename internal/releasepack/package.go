package releasepack

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type Options struct {
	ProductName string
	LegacyName  string
	Version     string
	ExePath     string
	IconPath    string
	OutputDir   string
	CreatedAt   time.Time
}

type Manifest struct {
	ProductName       string        `json:"productName"`
	LegacyName        string        `json:"legacyName"`
	Version           string        `json:"version"`
	CreatedAt         int64         `json:"createdAt"`
	Platform          string        `json:"platform"`
	Arch              string        `json:"arch"`
	InstallDir        string        `json:"installDir"`
	LegacyInstallDir  string        `json:"legacyInstallDir"`
	PackageDir        string        `json:"packageDir"`
	ZipPath           string        `json:"zipPath"`
	Files             []PackageFile `json:"files"`
	Scripts           []string      `json:"scripts"`
	CoexistenceNotes  []string      `json:"coexistenceNotes"`
	RollbackNotes     []string      `json:"rollbackNotes"`
	VerificationNotes []string      `json:"verificationNotes"`
}

type PackageFile struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type Result struct {
	Manifest Manifest
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

	packageName := safePackageName(options.ProductName, options.Version)
	packageDir := filepath.Join(options.OutputDir, packageName)
	zipPath := filepath.Join(options.OutputDir, packageName+".zip")
	if err := resetPackageDir(options.OutputDir, packageDir, zipPath); err != nil {
		return Result{}, err
	}

	appDir := filepath.Join(packageDir, "app")
	scriptDir := filepath.Join(packageDir, "scripts")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		return Result{}, err
	}

	files := []PackageFile{}
	if file, err := copyPackageFile(options.ExePath, filepath.Join(appDir, strings.ToLower(options.ProductName)+".exe"), packageDir); err != nil {
		return Result{}, err
	} else {
		files = append(files, file)
	}
	if file, err := copyPackageFile(options.IconPath, filepath.Join(appDir, "logo.ico"), packageDir); err != nil {
		return Result{}, err
	} else {
		files = append(files, file)
	}

	scripts := map[string]string{
		filepath.Join(scriptDir, "install.ps1"):   installScript(options),
		filepath.Join(scriptDir, "uninstall.ps1"): uninstallScript(options),
		filepath.Join(packageDir, "README.txt"):   readmeText(options),
	}
	scriptNames := []string{}
	for path, content := range scripts {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return Result{}, err
		}
		rel, _ := filepath.Rel(packageDir, path)
		scriptNames = append(scriptNames, filepath.ToSlash(rel))
	}
	sort.Strings(scriptNames)

	manifest := Manifest{
		ProductName:      options.ProductName,
		LegacyName:       options.LegacyName,
		Version:          options.Version,
		CreatedAt:        options.CreatedAt.Unix(),
		Platform:         "windows",
		Arch:             runtime.GOARCH,
		InstallDir:       `%LOCALAPPDATA%\Programs\Ariadne`,
		LegacyInstallDir: `%LOCALAPPDATA%\Programs\x-tools`,
		PackageDir:       packageDir,
		ZipPath:          zipPath,
		Files:            files,
		Scripts:          scriptNames,
		CoexistenceNotes: []string{
			"Ariadne installs beside legacy x-tools and does not remove legacy files.",
			"If legacy x-tools is running, it may own Alt+Q. Close x-tools before validating Ariadne hotkeys.",
			"After launch, Ariadne Settings > legacy x-tools can close legacy x-tools on confirmation and retry Alt+Q registration.",
			"Use the legacy uninstaller only after Ariadne migration is accepted.",
		},
		RollbackNotes: []string{
			"The installer moves an existing Ariadne install directory to Ariadne.previous-<timestamp> before copying new files.",
			"User data under %APPDATA%\\Ariadne is preserved by default.",
			"Use Ariadne Settings > rollback checkpoint before risky migration or release tests.",
			"Ariadne Settings can restore the latest rollback checkpoint on confirmation and creates a pre_restore checkpoint first.",
		},
		VerificationNotes: []string{
			"Run scripts\\install.ps1 from the extracted package to install per user.",
			"Run the installed uninstall.ps1 to remove shortcuts and installed binaries.",
			"The core package is independent of legacy x-tools, PyQt, and PyInstaller. Local OCR uses an explicit Ariadne OCR runtime if configured.",
		},
	}
	if err := writeManifest(filepath.Join(packageDir, "manifest.json"), manifest); err != nil {
		return Result{}, err
	}
	if err := zipDirectory(options.OutputDir, packageDir, zipPath); err != nil {
		return Result{}, err
	}
	return Result{Manifest: manifest}, nil
}

func normalizeOptions(options Options) Options {
	if options.ProductName == "" {
		options.ProductName = "Ariadne"
	}
	if options.LegacyName == "" {
		options.LegacyName = "x-tools"
	}
	if options.Version == "" {
		options.Version = "dev"
	}
	if options.ExePath == "" {
		options.ExePath = filepath.Join("bin", "ariadne.exe")
	}
	if options.IconPath == "" {
		options.IconPath = filepath.Join("assets", "logo.ico")
	}
	if options.OutputDir == "" {
		options.OutputDir = filepath.Join("dist", "release")
	}
	if options.CreatedAt.IsZero() {
		options.CreatedAt = time.Now()
	}
	options.ExePath = filepath.Clean(options.ExePath)
	options.IconPath = filepath.Clean(options.IconPath)
	options.OutputDir = filepath.Clean(options.OutputDir)
	return options
}

func validateOptions(options Options) error {
	if options.ProductName == "" || options.Version == "" {
		return errors.New("product name and version are required")
	}
	if info, err := os.Stat(options.ExePath); err != nil || info.IsDir() {
		return fmt.Errorf("Ariadne executable not found at %s", options.ExePath)
	}
	if info, err := os.Stat(options.IconPath); err != nil || info.IsDir() {
		return fmt.Errorf("Ariadne icon not found at %s", options.IconPath)
	}
	return nil
}

func resetPackageDir(outputDir string, packageDir string, zipPath string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	cleanOutput, err := filepath.Abs(outputDir)
	if err != nil {
		return err
	}
	cleanPackage, err := filepath.Abs(packageDir)
	if err != nil {
		return err
	}
	if !sameOrInside(cleanPackage, cleanOutput) || strings.EqualFold(cleanPackage, cleanOutput) {
		return fmt.Errorf("refusing to reset package directory outside output dir: %s", packageDir)
	}
	if err := os.RemoveAll(packageDir); err != nil {
		return err
	}
	if err := os.Remove(zipPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func copyPackageFile(source string, target string, packageDir string) (PackageFile, error) {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return PackageFile{}, err
	}
	input, err := os.Open(source)
	if err != nil {
		return PackageFile{}, err
	}
	defer input.Close()

	output, err := os.Create(target)
	if err != nil {
		return PackageFile{}, err
	}
	defer output.Close()

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(output, hasher), input)
	if err != nil {
		return PackageFile{}, err
	}
	rel, err := filepath.Rel(packageDir, target)
	if err != nil {
		return PackageFile{}, err
	}
	return PackageFile{
		Path:   filepath.ToSlash(rel),
		Bytes:  written,
		SHA256: hex.EncodeToString(hasher.Sum(nil)),
	}, nil
}

func writeManifest(path string, manifest Manifest) error {
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func zipDirectory(outputDir string, packageDir string, zipPath string) error {
	file, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	defer writer.Close()

	cleanOutput, err := filepath.Abs(outputDir)
	if err != nil {
		return err
	}
	return filepath.WalkDir(packageDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(cleanOutput, path)
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		header.Method = zip.Deflate
		item, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		source, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(item, source)
		closeErr := source.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func installScript(options Options) string {
	return strings.TrimSpace(fmt.Sprintf(`
param(
  [switch]$CreateDesktopShortcut,
  [switch]$NoShortcuts,
  [switch]$SkipLegacyCheck,
  [switch]$SkipProcessStop,
  [switch]$Force,
  [string]$InstallDir = "$env:LOCALAPPDATA\Programs\Ariadne",
  [string]$StartMenuDir = "",
  [string]$DesktopDir = ""
)

$ErrorActionPreference = "Stop"
$ProductName = "%s"
$LegacyName = "%s"
$Version = "%s"
$PackageRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$AppSource = Join-Path $PackageRoot "app"
$ExeSource = Join-Path $AppSource "ariadne.exe"
$IconSource = Join-Path $AppSource "logo.ico"

function New-Shortcut($TargetPath, $ShortcutPath, $IconPath, $Arguments = "") {
  $shell = New-Object -ComObject WScript.Shell
  $shortcut = $shell.CreateShortcut($ShortcutPath)
  $shortcut.TargetPath = $TargetPath
  $shortcut.WorkingDirectory = Split-Path -Parent $TargetPath
  if ($Arguments) { $shortcut.Arguments = $Arguments }
  if ($IconPath -and (Test-Path $IconPath)) { $shortcut.IconLocation = $IconPath }
  $shortcut.Save()
}

if (-not (Test-Path $ExeSource)) {
  throw "Package app payload is missing: $ExeSource"
}

$legacyExe = Join-Path $env:LOCALAPPDATA "Programs\x-tools\x-tools.exe"
$legacyRunning = Get-Process -Name "x-tools" -ErrorAction SilentlyContinue
if (-not $SkipLegacyCheck -and ((Test-Path $legacyExe) -or $legacyRunning)) {
  Write-Warning "Legacy x-tools is present. Ariadne installs side-by-side and will not remove legacy data."
  Write-Warning "Close x-tools before validating Alt+Q because the legacy app may own the hotkey."
  if ($legacyRunning -and -not $Force) {
    Write-Error "x-tools is running. Close it or rerun with -Force to install anyway."
  }
}

if (-not $SkipProcessStop) {
  Get-Process -Name "ariadne" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
  Start-Sleep -Milliseconds 300
}

if (Test-Path $InstallDir) {
  $stamp = Get-Date -Format "yyyyMMdd-HHmmss"
  $previous = Join-Path (Split-Path -Parent $InstallDir) "Ariadne.previous-$stamp"
  Move-Item -LiteralPath $InstallDir -Destination $previous
  Write-Host "Previous Ariadne install moved to $previous"
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Copy-Item -Path (Join-Path $AppSource "*") -Destination $InstallDir -Recurse -Force
Copy-Item -Path (Join-Path $PSScriptRoot "uninstall.ps1") -Destination (Join-Path $InstallDir "uninstall.ps1") -Force

$exePath = Join-Path $InstallDir "ariadne.exe"
$iconPath = Join-Path $InstallDir "logo.ico"
$startShortcutPath = $null
$uninstallShortcutPath = $null
$desktopShortcutPath = $null
if (-not $NoShortcuts) {
  $programsPath = $StartMenuDir
  if (-not $programsPath) { $programsPath = [Environment]::GetFolderPath("Programs") }
  New-Item -ItemType Directory -Force -Path $programsPath | Out-Null
  $startShortcutPath = Join-Path $programsPath "Ariadne.lnk"
  New-Shortcut $exePath $startShortcutPath $iconPath
  $uninstallArgs = '-ExecutionPolicy Bypass -File "' + (Join-Path $InstallDir "uninstall.ps1") + '"'
  $uninstallShortcutPath = Join-Path $programsPath "Uninstall Ariadne.lnk"
  New-Shortcut "$env:SystemRoot\System32\WindowsPowerShell\v1.0\powershell.exe" $uninstallShortcutPath $iconPath $uninstallArgs

  if ($CreateDesktopShortcut) {
    $desktopPath = $DesktopDir
    if (-not $desktopPath) { $desktopPath = [Environment]::GetFolderPath("Desktop") }
    New-Item -ItemType Directory -Force -Path $desktopPath | Out-Null
    $desktopShortcutPath = Join-Path $desktopPath "Ariadne.lnk"
    New-Shortcut $exePath $desktopShortcutPath $iconPath
  }
}

$receiptPath = Join-Path $InstallDir "install_receipt.json"
$receipt = [ordered]@{
  productName = $ProductName
  version = $Version
  installedAt = (Get-Date).ToUniversalTime().ToString("o")
  installDir = $InstallDir
  exePath = $exePath
  iconPath = $iconPath
  shortcuts = @($startShortcutPath, $uninstallShortcutPath, $desktopShortcutPath) | Where-Object { $_ }
}
$receipt | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath $receiptPath -Encoding UTF8

try {
  [void][Microsoft.Win32.Registry]::CurrentUser.OpenSubKey("Software\Microsoft\Windows\CurrentVersion\Run", $true).DeleteValue("Ariadne", $false)
} catch {}

if (-not $NoShortcuts) {
  try {
    Add-Type -Namespace Win32 -Name Shell -MemberDefinition '[System.Runtime.InteropServices.DllImport("shell32.dll")] public static extern void SHChangeNotify(int eventId, uint flags, System.IntPtr item1, System.IntPtr item2);'
    [Win32.Shell]::SHChangeNotify(0x08000000, 0, [IntPtr]::Zero, [IntPtr]::Zero)
  } catch {}
}

Write-Host "Ariadne installed to $InstallDir"
Write-Host "Run Ariadne from the Start Menu or $exePath"
`, options.ProductName, options.LegacyName, options.Version)) + "\n"
}

func uninstallScript(options Options) string {
	return strings.TrimSpace(fmt.Sprintf(`
param(
  [switch]$RemoveUserData,
  [switch]$NoShortcuts,
  [switch]$SkipProcessStop,
  [switch]$Synchronous,
  [switch]$Force,
  [string]$InstallDir = "",
  [string]$StartMenuDir = "",
  [string]$DesktopDir = ""
)

$ErrorActionPreference = "Stop"
$ProductName = "%s"

if (-not $InstallDir) {
  $InstallDir = Split-Path -Parent $MyInvocation.MyCommand.Path
}

if (-not $SkipProcessStop) {
  Get-Process -Name "ariadne" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
  Start-Sleep -Milliseconds 300
}

if (-not $NoShortcuts) {
  $programsPath = $StartMenuDir
  if (-not $programsPath) { $programsPath = [Environment]::GetFolderPath("Programs") }
  $desktopPath = $DesktopDir
  if (-not $desktopPath) { $desktopPath = [Environment]::GetFolderPath("Desktop") }

  $shortcuts = @(
    (Join-Path $programsPath "Ariadne.lnk"),
    (Join-Path $programsPath "Uninstall Ariadne.lnk"),
    (Join-Path $desktopPath "Ariadne.lnk")
  )
  $receiptPath = Join-Path $InstallDir "install_receipt.json"
  if (Test-Path $receiptPath) {
    try {
      $receipt = Get-Content -LiteralPath $receiptPath -Raw | ConvertFrom-Json
      if ($receipt.shortcuts) {
        $shortcuts += @($receipt.shortcuts)
      }
    } catch {}
  }
  $shortcuts | Where-Object { $_ } | Select-Object -Unique | ForEach-Object {
    Remove-Item -LiteralPath $_ -Force -ErrorAction SilentlyContinue
  }
}

try {
  [void][Microsoft.Win32.Registry]::CurrentUser.OpenSubKey("Software\Microsoft\Windows\CurrentVersion\Run", $true).DeleteValue("Ariadne", $false)
} catch {}

if ($RemoveUserData) {
  Remove-Item -LiteralPath (Join-Path $env:APPDATA "Ariadne") -Recurse -Force -ErrorAction SilentlyContinue
  Remove-Item -LiteralPath (Join-Path $env:LOCALAPPDATA "Ariadne") -Recurse -Force -ErrorAction SilentlyContinue
}

if ($Synchronous) {
  Remove-Item -LiteralPath $InstallDir -Recurse -Force -ErrorAction SilentlyContinue
  Write-Host "Ariadne uninstalled from $InstallDir. User data preserved unless -RemoveUserData was supplied."
  exit 0
}

$tempScript = Join-Path $env:TEMP ("ariadne-uninstall-" + [guid]::NewGuid().ToString("N") + ".ps1")
@"
Start-Sleep -Seconds 2
Remove-Item -LiteralPath '$InstallDir' -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath '$tempScript' -Force -ErrorAction SilentlyContinue
"@ | Set-Content -LiteralPath $tempScript -Encoding UTF8

$removeArgs = '-NoProfile -ExecutionPolicy Bypass -File "' + $tempScript + '"'
Start-Process -FilePath "$env:SystemRoot\System32\WindowsPowerShell\v1.0\powershell.exe" -ArgumentList $removeArgs -WindowStyle Hidden
Write-Host "Ariadne uninstall scheduled. User data preserved unless -RemoveUserData was supplied."
`, options.ProductName)) + "\n"
}

func readmeText(options Options) string {
	return strings.TrimSpace(fmt.Sprintf(`
Ariadne %s for Windows

Install:
  powershell -ExecutionPolicy Bypass -File .\scripts\install.ps1

Optional desktop shortcut:
  powershell -ExecutionPolicy Bypass -File .\scripts\install.ps1 -CreateDesktopShortcut

Temporary install smoke without touching the real Start Menu:
  powershell -ExecutionPolicy Bypass -File .\scripts\install.ps1 -InstallDir "$env:TEMP\AriadneSmoke\app" -StartMenuDir "$env:TEMP\AriadneSmoke\start-menu" -DesktopDir "$env:TEMP\AriadneSmoke\desktop" -CreateDesktopShortcut -SkipLegacyCheck -SkipProcessStop

Uninstall:
  Run "Uninstall Ariadne" from Start Menu, or:
  powershell -ExecutionPolicy Bypass -File "$env:LOCALAPPDATA\Programs\Ariadne\uninstall.ps1"

Coexistence:
  Ariadne installs to %%LOCALAPPDATA%%\Programs\Ariadne and does not remove legacy x-tools.
  Close legacy x-tools before validating Alt+Q. The legacy app may own that hotkey.
  Ariadne Settings > legacy x-tools can perform a confirmed handoff: close legacy x-tools and retry Alt+Q.

Rollback:
  Existing Ariadne install folders are moved to Ariadne.previous-<timestamp>.
  User data under %%APPDATA%%\Ariadne is preserved by default.
  Create an Ariadne rollback checkpoint before importing old x-tools history.
  Ariadne Settings can restore the latest checkpoint after confirmation and creates a pre_restore checkpoint first.

OCR:
  The core app does not bundle legacy x-tools Python or PyQt.
  Local OCR uses ARIADNE_OCR_PYTHON or an Ariadne-owned runtime under %%LOCALAPPDATA%%\Ariadne\ocr-python when configured.
`, options.Version)) + "\n"
}

func safePackageName(product string, version string) string {
	text := strings.ToLower(strings.TrimSpace(product + "-" + version + "-windows-x64"))
	replacer := strings.NewReplacer(" ", "-", "\\", "-", "/", "-", ":", "-", "*", "-", "?", "-", "\"", "-", "<", "-", ">", "-", "|", "-")
	return replacer.Replace(text)
}

func sameOrInside(path string, root string) bool {
	if strings.EqualFold(path, root) {
		return true
	}
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}
