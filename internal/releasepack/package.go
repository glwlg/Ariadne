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
	"strings"
	"time"
)

type Options struct {
	ProductName string
	Version     string
	ExePath     string
	IconPath    string
	OutputDir   string
	CreatedAt   time.Time
	SkipSetup   bool
}

type Manifest struct {
	ProductName       string        `json:"productName"`
	Version           string        `json:"version"`
	CreatedAt         int64         `json:"createdAt"`
	Platform          string        `json:"platform"`
	Arch              string        `json:"arch"`
	InstallDir        string        `json:"installDir"`
	PackageDir        string        `json:"packageDir"`
	ZipPath           string        `json:"zipPath"`
	SetupPath         string        `json:"setupPath,omitempty"`
	SetupFile         *PackageFile  `json:"setupFile,omitempty"`
	Files             []PackageFile `json:"files"`
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
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return Result{}, err
	}

	files := []PackageFile{}
	if file, err := copyPackageFile(options.ExePath, filepath.Join(appDir, strings.ToLower(options.ProductName)+".exe"), packageDir); err != nil {
		return Result{}, err
	} else {
		files = append(files, file)
	}
	runtimeFiles, err := copyRuntimeFiles(options.ExePath, appDir, packageDir)
	if err != nil {
		return Result{}, err
	}
	files = append(files, runtimeFiles...)
	if file, err := copyPackageFile(options.IconPath, filepath.Join(appDir, "logo.ico"), packageDir); err != nil {
		return Result{}, err
	} else {
		files = append(files, file)
	}

	if err := os.WriteFile(filepath.Join(packageDir, "README.txt"), []byte(readmeText(options)), 0o644); err != nil {
		return Result{}, err
	}

	manifest := Manifest{
		ProductName: options.ProductName,
		Version:     options.Version,
		CreatedAt:   options.CreatedAt.Unix(),
		Platform:    "windows",
		Arch:        runtime.GOARCH,
		InstallDir:  `%LOCALAPPDATA%\Programs\Ariadne`,
		PackageDir:  packageDir,
		ZipPath:     zipPath,
		Files:       files,
		RollbackNotes: []string{
			"The installer moves an existing Ariadne install directory to Ariadne.previous-<timestamp> before copying new files.",
			"User data under %APPDATA%\\Ariadne is preserved by default.",
			"Use Ariadne Settings > rollback checkpoint before risky release tests.",
			"Ariadne Settings can restore the latest rollback checkpoint after confirmation.",
		},
		VerificationNotes: []string{
			"Run the setup EXE from the release directory; it opens normally and requests administrator permission only when installing the search service.",
			"The setup EXE opens a wizard UI with user agreement, install directory, shortcuts, file index service, autostart, and launch options.",
			"The setup EXE installs app files and selected shortcuts without external command files.",
			"Run the installed Start Menu uninstaller shortcut to remove shortcuts and installed binaries.",
			"Ariadne file search uses the built-in Windows NTFS USN/MFT index path and the optional AriadneFileSearch Windows service.",
			"Local OCR uses an explicit Ariadne OCR runtime if configured.",
		},
	}
	if err := writeManifest(filepath.Join(packageDir, "manifest.json"), manifest); err != nil {
		return Result{}, err
	}
	if err := zipDirectory(options.OutputDir, packageDir, zipPath); err != nil {
		return Result{}, err
	}
	if !options.SkipSetup {
		setupFile, setupPath, err := buildSetupInstaller(options, packageName, zipPath)
		if err != nil {
			return Result{}, err
		}
		manifest.SetupPath = setupPath
		manifest.SetupFile = &setupFile
	}
	return Result{Manifest: manifest}, nil
}

func normalizeOptions(options Options) Options {
	if options.ProductName == "" {
		options.ProductName = "Ariadne"
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

func copyRuntimeFiles(exePath string, appDir string, packageDir string) ([]PackageFile, error) {
	files := []PackageFile{}
	for _, name := range []string{} {
		exeDir := filepath.Dir(exePath)
		source := filepath.Join(exeDir, name)
		info, err := os.Stat(source)
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("required runtime file not found: %s", source)
		}
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			continue
		}
		file, err := copyPackageFile(source, filepath.Join(appDir, name), packageDir)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
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

func readmeText(options Options) string {
	return strings.TrimSpace(fmt.Sprintf(`
Ariadne %s for Windows

Install:
  Run AriadneSetup-%s-windows-x64.exe, review the user agreement, and choose the install directory, shortcuts, autostart, and launch options.

Uninstall:
  Run "Uninstall Ariadne" from the Start Menu.

Install directory:
  %%LOCALAPPDATA%%\Programs\Ariadne

Package contents:
  app\ariadne.exe
  app\logo.ico

Data:
  User data under %%APPDATA%%\Ariadne is preserved by default.

OCR:
  Local OCR uses ARIADNE_OCR_PYTHON or an Ariadne-owned runtime under %%LOCALAPPDATA%%\Ariadne\ocr-python when configured.
`, options.Version, options.Version)) + "\n"
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
