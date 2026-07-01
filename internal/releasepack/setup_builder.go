package releasepack

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func buildSetupInstaller(options Options, packageName string, zipPath string) (PackageFile, string, error) {
	setupName := safeSetupName(options.ProductName, options.Version)
	setupPath := filepath.Join(options.OutputDir, setupName)
	if err := os.Remove(setupPath); err != nil && !os.IsNotExist(err) {
		return PackageFile{}, "", err
	}

	moduleRoot, err := findModuleRoot()
	if err != nil {
		return PackageFile{}, "", err
	}
	buildParent := filepath.Join(moduleRoot, "dist", "release", "setup-build")
	if err := os.MkdirAll(buildParent, 0o755); err != nil {
		return PackageFile{}, "", err
	}
	defer os.Remove(buildParent)
	buildDir, err := os.MkdirTemp(buildParent, packageName+"-")
	if err != nil {
		return PackageFile{}, "", err
	}
	defer os.RemoveAll(buildDir)

	if err := copyFile(zipPath, filepath.Join(buildDir, "payload.zip")); err != nil {
		return PackageFile{}, "", err
	}
	if err := os.WriteFile(filepath.Join(buildDir, "main.go"), []byte(setupMainSource(options)), 0o644); err != nil {
		return PackageFile{}, "", err
	}
	if err := buildSetupResources(options, moduleRoot, buildDir, setupName); err != nil {
		return PackageFile{}, "", err
	}

	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-H windowsgui", "-o", setupPath, ".")
	cmd.Dir = buildDir
	cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return PackageFile{}, "", fmt.Errorf("build setup installer: %w\n%s", err, strings.TrimSpace(string(output)))
	}

	file, err := describeFile(setupPath, options.OutputDir)
	if err != nil {
		return PackageFile{}, "", err
	}
	return file, setupPath, nil
}

func buildSetupResources(options Options, moduleRoot string, buildDir string, setupName string) error {
	iconSource := setupIconSource(options, moduleRoot)
	iconTarget := filepath.Join(buildDir, "setup-logo.png")
	if err := copyFile(iconSource, iconTarget); err != nil {
		return err
	}
	configPath := filepath.Join(buildDir, "winres.json")
	if err := os.WriteFile(configPath, []byte(setupWinresConfig(options, "setup-logo.png", setupName)), 0o644); err != nil {
		return err
	}
	cmd := exec.Command("go", "run", "github.com/tc-hib/go-winres@v0.3.3", "make", "--in", "winres.json", "--arch", "amd64", "--out", "setup_resource.syso", "--no-suffix")
	cmd.Dir = buildDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build setup resources: %w\n%s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func setupIconSource(options Options, moduleRoot string) string {
	iconPath := filepath.Join(moduleRoot, "assets", "logo.png")
	if info, err := os.Stat(iconPath); err != nil || info.IsDir() {
		iconPath = options.IconPath
		if !filepath.IsAbs(iconPath) {
			iconPath = filepath.Join(moduleRoot, iconPath)
		}
	}
	return iconPath
}

func setupWinresConfig(options Options, iconPath string, setupName string) string {
	return fmt.Sprintf(`{
  "RT_GROUP_ICON": {
    "APP": {
      "0000": [
        %s
      ]
    }
  },
  "RT_MANIFEST": {
    "#1": {
      "0409": {
        "identity": {
          "name": %s,
          "version": "0.0.0.0"
        },
        "description": %s,
        "minimum-os": "win10",
        "execution-level": "as invoker",
        "ui-access": false,
        "auto-elevate": false,
        "dpi-awareness": "permonitorv2",
        "long-path-aware": true,
        "use-common-controls-v6": true
      }
    }
  },
  "RT_VERSION": {
    "#1": {
      "0000": {
        "fixed": {
          "file_version": "0.0.0.0",
          "product_version": "0.0.0.0"
        },
        "info": {
          "0409": {
            "CompanyName": %s,
            "FileDescription": %s,
            "FileVersion": "0.0.0.0",
            "InternalName": "ariadne-setup",
            "LegalCopyright": "Ariadne contributors",
            "OriginalFilename": %s,
            "ProductName": %s,
            "ProductVersion": %s
          }
        }
      }
    }
  }
}
`, strconv.Quote(filepath.ToSlash(iconPath)),
		strconv.Quote(options.ProductName+".Setup"),
		strconv.Quote(options.ProductName+" installer"),
		strconv.Quote(options.ProductName),
		strconv.Quote(options.ProductName+" installer"),
		strconv.Quote(setupName),
		strconv.Quote(options.ProductName),
		strconv.Quote(options.Version))
}

func setupMainSource(options Options) string {
	return fmt.Sprintf(`package main

import (
	_ "embed"
	"fmt"
	"os"

	"ariadne/internal/setupstub"
)

//go:embed payload.zip
var payload []byte

func main() {
	parsed := setupstub.ParseArgs(os.Args[1:])
	options := setupstub.Options{
		ProductName: %s,
		Version: %s,
		Args: parsed.InstallArgs,
	}
	var result setupstub.Result
	var err error
	if !parsed.Quiet && len(parsed.InstallArgs) == 0 {
		result, err = setupstub.RunInteractive(payload, options)
	} else {
		result, err = setupstub.Run(payload, options)
	}
	if err != nil {
		if parsed.Quiet {
			fmt.Fprintln(os.Stderr, err)
		} else {
			setupstub.ShowError(%s, err.Error())
		}
		os.Exit(1)
	}
	if parsed.Quiet {
		return
	}
	switch result.Action {
	case setupstub.ActionCancelled:
		return
	case setupstub.ActionUninstall:
		setupstub.ShowInfo(%s, %s)
	case setupstub.ActionUninstallScheduled:
		setupstub.ShowInfo(%s, %s)
	default:
		setupstub.ShowInfo(%s, %s)
	}
}
`, strconv.Quote(options.ProductName), strconv.Quote(options.Version),
		strconv.Quote(options.ProductName+" 安装失败"),
		strconv.Quote(options.ProductName+" 卸载完成"),
		strconv.Quote(options.ProductName+" 已从当前用户目录卸载。"),
		strconv.Quote(options.ProductName+" 卸载已开始"),
		strconv.Quote("卸载程序正在后台移除 "+options.ProductName+"。"),
		strconv.Quote(options.ProductName+" 安装完成"),
		strconv.Quote("已安装 "+options.ProductName+"。可从开始菜单或桌面快捷方式启动应用。"))
}

func safeSetupName(product string, version string) string {
	name := strings.TrimSpace(product) + "Setup-" + version + "-windows-x64.exe"
	replacer := strings.NewReplacer(" ", "-", "\\", "-", "/", "-", ":", "-", "*", "-", "?", "-", "\"", "-", "<", "-", ">", "-", "|", "-")
	return replacer.Replace(name)
}

func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if info, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", dir)
		}
		dir = parent
	}
}

func copyFile(source string, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.Create(target)
	if err != nil {
		return err
	}
	defer output.Close()
	_, err = io.Copy(output, input)
	return err
}

func describeFile(path string, root string) (PackageFile, error) {
	input, err := os.Open(path)
	if err != nil {
		return PackageFile{}, err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return PackageFile{}, err
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, input); err != nil {
		return PackageFile{}, err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return PackageFile{}, err
	}
	return PackageFile{
		Path:   filepath.ToSlash(rel),
		Bytes:  info.Size(),
		SHA256: hex.EncodeToString(hasher.Sum(nil)),
	}, nil
}
