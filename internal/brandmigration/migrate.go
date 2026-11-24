package brandmigration

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"fancun/internal/securestore"
)

const receiptName = ".brand-migration.json"

type Options struct {
	RoamingBase        string
	RoamingSource      string
	LocalBase          string
	PicturesBase       string
	Credentials        securestore.Store
	CheckLegacyStopped func() error
}

type root struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

func MigrateDefault() error {
	return MigrateDefaultFrom("")
}

// MigrateDefaultFrom records an explicitly selected roaming profile in the import receipt.
// Subsequent launches use that receipt without merging other legacy profiles.
func MigrateDefaultFrom(source string) error {
	return MigrateFromPaths(source, "", "", "", true)
}

// MigrateFromPaths runs brand migration using explicit base directories when provided.
// Setup helpers can disable credential migration while elevated and run it unelevated as the invoking user.
func MigrateFromPaths(source, roamingBase, localBase, picturesBase string, withCredentials bool) error {
	roaming := strings.TrimSpace(roamingBase)
	if roaming == "" {
		var err error
		roaming, err = defaultRoamingBase()
		if err != nil {
			return err
		}
	}
	local := strings.TrimSpace(localBase)
	if local == "" {
		local = os.Getenv("LOCALAPPDATA")
	}
	pictures := strings.TrimSpace(picturesBase)
	if pictures == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		pictures = filepath.Join(home, "Pictures")
	}
	var credentials securestore.Store
	if withCredentials {
		credentials = securestore.DefaultStore{}
	}
	return Run(Options{
		RoamingBase:        roaming,
		RoamingSource:      source,
		LocalBase:          local,
		PicturesBase:       pictures,
		Credentials:        credentials,
		CheckLegacyStopped: checkLegacyStopped,
	})
}

func defaultRoamingBase() (string, error) {
	if roaming := os.Getenv("APPDATA"); roaming != "" {
		return roaming, nil
	}
	return os.UserConfigDir()
}

// Run imports into separate directories; the original data and credentials remain intact.
// A completed root can be resumed after another root or credential write fails.
func Run(options Options) error {
	var roots []root
	for _, base := range []string{options.RoamingBase, options.LocalBase, options.PicturesBase} {
		if strings.TrimSpace(base) == "" {
			continue
		}
		item := root{Source: filepath.Join(base, "Ariadne"), Target: filepath.Join(base, "Fancun")}
		duplicate := false
		for _, existing := range roots {
			duplicate = duplicate || strings.EqualFold(existing.Source, item.Source)
		}
		if !duplicate {
			roots = append(roots, item)
		}
	}
	if options.RoamingBase != "" && options.LocalBase != "" {
		matches, err := filepath.Glob(filepath.Join(options.LocalBase, "Packages", "*", "LocalCache", "Roaming", "Ariadne"))
		if err != nil {
			return err
		}
		for _, source := range matches {
			roots = append(roots, root{Source: source, Target: filepath.Join(options.RoamingBase, "Fancun")})
		}
	}
	selected := options.RoamingSource
	roamingAuthoritative := false
	roamingTarget := ""
	if options.RoamingBase != "" {
		roamingTarget = filepath.Join(options.RoamingBase, "Fancun")
		if info, err := os.Lstat(roamingTarget); err == nil {
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("Fancun 数据目录已存在，未覆盖其中内容；请先处理目录冲突：%s", roamingTarget)
			}
			raw, err := os.ReadFile(filepath.Join(roamingTarget, receiptName))
			if errors.Is(err, os.ErrNotExist) {
				// Preserve an existing Fancun roaming directory created without a migration receipt as authoritative.
				roamingAuthoritative = true
			} else {
				var receipt root
				if err != nil || json.Unmarshal(raw, &receipt) != nil || !strings.EqualFold(receipt.Target, roamingTarget) {
					return fmt.Errorf("Fancun 数据目录已存在，未覆盖其中内容；请先处理目录冲突：%s", roamingTarget)
				}
				if selected != "" && !strings.EqualFold(selected, receipt.Source) {
					return fmt.Errorf("选择的数据来源与已完成的迁移不一致：%s", receipt.Source)
				}
				selected = receipt.Source
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if selected != "" && !roamingAuthoritative {
			recognized := false
			for _, item := range roots {
				if strings.EqualFold(item.Target, roamingTarget) && strings.EqualFold(item.Source, selected) {
					recognized = true
				}
			}
			if !recognized {
				return fmt.Errorf("所选旧版数据目录不在可迁移来源中：%s", selected)
			}
			if options.RoamingSource != "" {
				if info, err := os.Lstat(selected); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
					return fmt.Errorf("所选旧版数据目录不存在或不是普通目录：%s", selected)
				}
			}
		}
	}
	var pending []root
	for _, item := range roots {
		if roamingTarget != "" && strings.EqualFold(item.Target, roamingTarget) {
			if roamingAuthoritative {
				continue
			}
			if selected != "" && !strings.EqualFold(item.Source, selected) {
				continue
			}
		}
		info, err := os.Lstat(item.Source)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("旧数据目录不是普通目录：%s", item.Source)
		}
		if _, err := os.Lstat(item.Target); err == nil {
			if err := checkReceipt(item); err != nil {
				return err
			}
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		for _, other := range pending {
			if strings.EqualFold(other.Target, item.Target) {
				return fmt.Errorf("检测到多个旧版数据目录，请先确认要迁移的数据：%s、%s", other.Source, item.Source)
			}
		}
		pending = append(pending, item)
	}
	if len(pending) > 0 && options.CheckLegacyStopped != nil {
		if err := options.CheckLegacyStopped(); err != nil {
			return err
		}
	}
	for _, item := range pending {
		if err := os.MkdirAll(filepath.Dir(item.Target), 0o700); err != nil {
			return err
		}
		if err := importRoot(item, roots); err != nil {
			return fmt.Errorf("迁移 %s：%w", item.Source, err)
		}
	}
	return importCredentials(options)
}

func checkReceipt(item root) error {
	raw, err := os.ReadFile(filepath.Join(item.Target, receiptName))
	var receipt root
	if err == nil && json.Unmarshal(raw, &receipt) == nil &&
		strings.EqualFold(receipt.Source, item.Source) && strings.EqualFold(receipt.Target, item.Target) {
		return nil
	}
	return fmt.Errorf("Fancun 数据目录已存在，未覆盖其中内容；请先处理目录冲突：%s", item.Target)
}

func importRoot(item root, roots []root) error {
	stage, err := os.MkdirTemp(filepath.Dir(item.Target), ".fancun-import-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	err = filepath.WalkDir(item.Source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(item.Source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("数据目录包含符号链接，请先处理该链接：%s", path)
		}
		if relative == "ariadne.sqlite-wal" || relative == "ariadne.sqlite-shm" {
			return nil // The SQLite backup API includes committed WAL pages.
		}
		targetRelative := relative
		if relative == "ariadne.sqlite" {
			targetRelative = "fancun.sqlite"
		} else if strings.HasPrefix(filepath.ToSlash(relative), "logs/ariadne.log") {
			targetRelative = strings.Replace(relative, "ariadne.log", "fancun.log", 1)
		}
		target := filepath.Join(stage, targetRelative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("不支持迁移特殊文件：%s", path)
		}
		if relative == "ariadne.sqlite" {
			return importDatabase(path, target, roots)
		}
		if err := copyFile(path, target, info.Mode().Perm()); err != nil {
			return err
		}
		switch relative {
		case "config.json", "capture_history.json", "clipboard_history.json", "image_index.json", "launchers.json", "search_state.json":
			raw, err := os.ReadFile(target)
			if err != nil {
				return err
			}
			updated, err := rewriteJSON(raw, roots)
			if err != nil {
				return fmt.Errorf("解析旧数据 %s：%w", path, err)
			}
			return os.WriteFile(target, updated, info.Mode().Perm())
		}
		return nil
	})
	if err != nil {
		return err
	}
	raw, err := json.Marshal(item)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(stage, receiptName), raw, 0o600); err != nil {
		return err
	}
	if err := os.Rename(stage, item.Target); err != nil {
		// Another Fancun instance may have completed the same import.
		if receiptErr := checkReceipt(item); receiptErr == nil {
			return nil
		}
		return err
	}
	return nil
}

// ImportSnapshot converts an extracted legacy checkpoint into a separate directory.
// The caller supplies the archive's original data roots and their current paths.
func ImportSnapshot(source, target string, mappings map[string]string) error {
	roots := make([]root, 0, len(mappings))
	for before, after := range mappings {
		roots = append(roots, root{Source: before, Target: after})
	}
	sort.Slice(roots, func(i, j int) bool { return len(roots[i].Source) > len(roots[j].Source) })
	if err := importRoot(root{Source: source, Target: target}, roots); err != nil {
		return err
	}
	// A temporary snapshot is not the user's first-run migration receipt.
	return os.Remove(filepath.Join(target, receiptName))
}

func copyFile(source, target string, mode fs.FileMode) (err error) {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, output.Close()) }()
	_, err = io.Copy(output, input)
	return err
}

func rewritePath(value string, roots []root) string {
	normalized := strings.ReplaceAll(value, `\`, "/")
	for _, item := range roots {
		source := strings.TrimRight(strings.ReplaceAll(item.Source, `\`, "/"), "/")
		if !strings.EqualFold(normalized, source) &&
			!(len(normalized) > len(source) && strings.EqualFold(normalized[:len(source)], source) && normalized[len(source)] == '/') {
			continue
		}
		suffix := strings.TrimPrefix(normalized[len(source):], "/")
		if strings.EqualFold(suffix, "ariadne.sqlite") {
			suffix = "fancun.sqlite"
		}
		if suffix == "" {
			return item.Target
		}
		return filepath.Join(item.Target, filepath.FromSlash(suffix))
	}
	return value
}

func rewriteJSON(raw []byte, roots []root) ([]byte, error) {
	if !json.Valid(raw) {
		return nil, fmt.Errorf("JSON 数据无效")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	rewriteJSONValue(value, "", roots)
	return json.MarshalIndent(value, "", "  ")
}

func rewriteJSONValue(value any, key string, roots []root) any {
	switch value := value.(type) {
	case map[string]any:
		if value["id"] == "ariadne-config-dir" {
			value["id"] = "fancun-config-dir"
			if value["name"] == "Ariadne 配置目录" {
				value["name"] = "Fancun 配置目录"
			}
			if keywords, ok := value["keywords"].([]any); ok {
				for index, keyword := range keywords {
					if keyword == "ariadne" {
						keywords[index] = "fancun"
					}
				}
			}
		}
		for childKey, child := range value {
			value[childKey] = rewriteJSONValue(child, childKey, roots)
		}
	case []any:
		for index, child := range value {
			value[index] = rewriteJSONValue(child, key, roots)
		}
	case string:
		switch key {
		case "imagePath", "thumbnailPath", "savedPath", "saveDir", "workingDir", "target", "fileExcludeFolders":
			return rewritePath(value, roots)
		case "filenameTemplate":
			if value == "ariadne_{date}_{time}" {
				return "fancun_{date}_{time}"
			}
		case "removedIds":
			if value == "ariadne-config-dir" {
				return "fancun-config-dir"
			}
		case "resultId":
			if value == "launcher-ariadne-config-dir" {
				return "launcher-fancun-config-dir"
			}
		case "fileExcludePatterns":
			return rewriteExcludePattern(value)
		}
	}
	return value
}

func rewriteExcludePattern(value string) string {
	const legacy = `(?i)(^|[\\/])ariadne[\\/](capture_images|capture_thumbnails)([\\/]|$)`
	if value == legacy {
		return `(?i)(^|[\\/])fancun[\\/](capture_images|capture_thumbnails)([\\/]|$)`
	}
	return value
}

func importCredentials(options Options) error {
	store := options.Credentials
	if store == nil || !store.Available() {
		return nil
	}
	if options.RoamingBase == "" {
		return fmt.Errorf("缺少用户数据目录，无法记录凭据迁移状态")
	}
	directory := filepath.Join(options.RoamingBase, "Fancun")
	marker := filepath.Join(directory, ".brand-credentials-migrated")
	if raw, err := os.ReadFile(marker); err == nil {
		if string(raw) != "Ariadne -> Fancun\n" {
			return fmt.Errorf("凭据迁移记录无效：%s", marker)
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, suffix := range []string{"OpenAI/APIKey", "Embedding/APIKey", "Milvus/Token"} {
		target := "Fancun/" + suffix
		if _, exists, err := store.Read(target); err != nil {
			return fmt.Errorf("读取新凭据 %s 失败", target)
		} else if exists {
			continue
		}
		secret, exists, err := store.Read("Ariadne/" + suffix)
		if err != nil {
			return fmt.Errorf("读取旧凭据 %s 失败", suffix)
		}
		if !exists || secret == "" {
			continue
		}
		if err := store.Write(target, secret); err != nil {
			return fmt.Errorf("迁移凭据 %s 失败", suffix)
		}
		verified, exists, err := store.Read(target)
		if err != nil || !exists || verified != secret {
			return fmt.Errorf("验证迁移凭据 %s 失败", suffix)
		}
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	return os.WriteFile(marker, []byte("Ariadne -> Fancun\n"), 0o600)
}
