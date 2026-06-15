package release

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

var errUnsafeRoot = errors.New("refusing to restore unsafe data root")

type DataRootStatus struct {
	Kind        string `json:"kind"`
	ArchiveName string `json:"archiveName"`
	Path        string `json:"path"`
	Exists      bool   `json:"exists"`
	FileCount   int    `json:"fileCount"`
	Bytes       int64  `json:"bytes"`
}

type BackupStatus struct {
	DataRoots    []DataRootStatus `json:"dataRoots"`
	BackupDir    string           `json:"backupDir"`
	BackupCount  int              `json:"backupCount"`
	BackupBytes  int64            `json:"backupBytes"`
	LatestBackup string           `json:"latestBackup,omitempty"`
	Notes        []string         `json:"notes"`
}

type BackupRequest struct {
	Reason string `json:"reason,omitempty"`
}

type BackupResult struct {
	OK        bool             `json:"ok"`
	Message   string           `json:"message"`
	Path      string           `json:"path,omitempty"`
	Bytes     int64            `json:"bytes,omitempty"`
	FileCount int              `json:"fileCount"`
	Roots     []DataRootStatus `json:"roots"`
	CreatedAt int64            `json:"createdAt"`
}

type RestoreRequest struct {
	Path                   string `json:"path,omitempty"`
	Confirm                bool   `json:"confirm"`
	CreatePreRestoreBackup bool   `json:"createPreRestoreBackup"`
}

type RestoreRootResult struct {
	Kind          string `json:"kind"`
	ArchiveName   string `json:"archiveName"`
	Path          string `json:"path"`
	RestoredFiles int    `json:"restoredFiles"`
	RestoredBytes int64  `json:"restoredBytes"`
	SkippedFiles  int    `json:"skippedFiles"`
	Error         string `json:"error,omitempty"`
}

type RestoreResult struct {
	OK                   bool                `json:"ok"`
	Message              string              `json:"message"`
	Path                 string              `json:"path,omitempty"`
	PreRestoreBackupPath string              `json:"preRestoreBackupPath,omitempty"`
	FileCount            int                 `json:"fileCount"`
	Bytes                int64               `json:"bytes"`
	Roots                []RestoreRootResult `json:"roots"`
	RequiresConfirmation bool                `json:"requiresConfirmation"`
	RestoredAt           int64               `json:"restoredAt"`
}

type backupManifest struct {
	App          string           `json:"app"`
	Kind         string           `json:"kind"`
	CreatedAt    int64            `json:"createdAt"`
	Reason       string           `json:"reason,omitempty"`
	Roots        []DataRootStatus `json:"roots"`
	FileCount    int              `json:"fileCount"`
	Bytes        int64            `json:"bytes"`
	RestoreNotes []string         `json:"restoreNotes"`
}

type Service struct {
	roots     []dataRoot
	backupDir string
}

type dataRoot struct {
	Kind        string
	ArchiveName string
	Path        string
}

func NewService() *Service {
	return NewServiceWithRoots(defaultDataRoots(), defaultBackupDir())
}

func NewServiceWithRoots(roots []DataRootStatus, backupDir string) *Service {
	items := make([]dataRoot, 0, len(roots))
	for _, root := range roots {
		if strings.TrimSpace(root.Path) == "" {
			continue
		}
		kind := firstNonEmpty(root.Kind, "data")
		items = append(items, dataRoot{
			Kind:        kind,
			ArchiveName: firstNonEmpty(root.ArchiveName, safeZipName(kind)),
			Path:        filepath.Clean(root.Path),
		})
	}
	cleanBackupDir := strings.TrimSpace(backupDir)
	if cleanBackupDir != "" {
		cleanBackupDir = filepath.Clean(cleanBackupDir)
	}
	return &Service{roots: uniqueRoots(items), backupDir: cleanBackupDir}
}

func (s *Service) Status() BackupStatus {
	roots := s.rootStatuses()
	status := BackupStatus{
		DataRoots: roots,
		BackupDir: s.backupDir,
		Notes: []string{
			"回滚检查点只打包 Ariadne 当前本地数据，不删除、不覆盖旧版 x-tools 数据。",
			"恢复检查点需要用户二次确认；恢复前会先创建 pre_restore 检查点。",
			"恢复时只写回 Ariadne 当前声明的数据根，不会解压到旧版 x-tools 数据目录。",
		},
	}
	status.BackupCount, status.BackupBytes, status.LatestBackup = backupDirStatus(s.backupDir)
	return status
}

func (s *Service) CreateRollbackCheckpoint(request BackupRequest) BackupResult {
	createdAt := time.Now()
	roots := s.rootStatuses()
	result := BackupResult{
		OK:        true,
		CreatedAt: createdAt.Unix(),
		Roots:     roots,
	}
	if s.backupDir == "" {
		result.OK = false
		result.Message = "未能定位 Ariadne 备份目录"
		return result
	}
	if err := os.MkdirAll(s.backupDir, 0o755); err != nil {
		result.OK = false
		result.Message = "创建备份目录失败：" + err.Error()
		return result
	}
	archivePath := uniqueBackupPath(s.backupDir, "ariadne-rollback-"+createdAt.Format("20060102-150405")+".zip")
	file, err := os.Create(archivePath)
	if err != nil {
		result.OK = false
		result.Message = "创建回滚检查点失败：" + err.Error()
		return result
	}
	zipWriter := zip.NewWriter(file)
	manifest := backupManifest{
		App:       "Ariadne",
		Kind:      "rollback_checkpoint",
		CreatedAt: createdAt.Unix(),
		Reason:    strings.TrimSpace(request.Reason),
		Roots:     roots,
		RestoreNotes: []string{
			"退出 Ariadne 后再恢复数据，避免正在运行的服务覆盖文件。",
			"按 manifest 中的 root.path 将 data/<root.archiveName>/ 下文件解压回对应目录。",
			"不要把检查点解压到旧版 x-tools 数据目录；旧版数据由单独迁移流程管理。",
		},
	}
	for _, root := range roots {
		if !root.Exists {
			continue
		}
		count, bytes, walkErr := addRootToZip(zipWriter, root, s.backupDir)
		if walkErr != nil {
			result.OK = false
			result.Message = "写入回滚检查点失败：" + walkErr.Error()
			_ = zipWriter.Close()
			_ = file.Close()
			_ = os.Remove(archivePath)
			return result
		}
		manifest.FileCount += count
		manifest.Bytes += bytes
	}
	if err := addManifest(zipWriter, manifest); err != nil {
		result.OK = false
		result.Message = "写入回滚说明失败：" + err.Error()
		_ = zipWriter.Close()
		_ = file.Close()
		_ = os.Remove(archivePath)
		return result
	}
	if err := zipWriter.Close(); err != nil {
		result.OK = false
		result.Message = "关闭回滚检查点失败：" + err.Error()
		_ = file.Close()
		_ = os.Remove(archivePath)
		return result
	}
	if err := file.Close(); err != nil {
		result.OK = false
		result.Message = "保存回滚检查点失败：" + err.Error()
		_ = os.Remove(archivePath)
		return result
	}
	if info, err := os.Stat(archivePath); err == nil {
		result.Bytes = info.Size()
	}
	result.Path = archivePath
	result.FileCount = manifest.FileCount
	if result.FileCount == 0 {
		result.Message = "已创建空回滚检查点"
	} else {
		result.Message = "已创建 Ariadne 回滚检查点"
	}
	return result
}

func (s *Service) RestoreRollbackCheckpoint(request RestoreRequest) RestoreResult {
	restoredAt := time.Now()
	result := RestoreResult{RestoredAt: restoredAt.Unix()}
	if !request.Confirm {
		result.RequiresConfirmation = true
		result.Message = "需要确认后恢复 Ariadne 回滚检查点"
		return result
	}
	checkpointPath := strings.TrimSpace(request.Path)
	if checkpointPath == "" {
		_, _, checkpointPath = backupDirStatus(s.backupDir)
	}
	if checkpointPath == "" {
		result.Message = "未找到可恢复的回滚检查点"
		return result
	}
	checkpointPath = filepath.Clean(checkpointPath)
	if !sameOrInside(checkpointPath, s.backupDir) || !strings.HasSuffix(strings.ToLower(checkpointPath), ".zip") {
		result.Message = "拒绝恢复备份目录外的检查点：" + checkpointPath
		return result
	}
	result.Path = checkpointPath

	reader, err := zip.OpenReader(checkpointPath)
	if err != nil {
		result.Message = "打开回滚检查点失败：" + err.Error()
		return result
	}
	defer reader.Close()

	manifest, err := readBackupManifest(&reader.Reader)
	if err != nil {
		result.Message = "读取回滚检查点说明失败：" + err.Error()
		return result
	}
	if manifest.App != "Ariadne" || manifest.Kind != "rollback_checkpoint" {
		result.Message = "不是 Ariadne 回滚检查点"
		return result
	}

	if request.CreatePreRestoreBackup {
		preBackup := s.CreateRollbackCheckpoint(BackupRequest{Reason: "pre_restore:" + filepath.Base(checkpointPath)})
		if !preBackup.OK {
			result.Message = "恢复前检查点创建失败：" + preBackup.Message
			return result
		}
		result.PreRestoreBackupPath = preBackup.Path
	}

	rootsByArchive := map[string]dataRoot{}
	for _, root := range uniqueRoots(s.roots) {
		rootsByArchive[strings.ToLower(root.ArchiveName)] = root
	}
	seenRoots := map[string]bool{}
	for _, manifestRoot := range manifest.Roots {
		archiveName := safeZipName(firstNonEmpty(manifestRoot.ArchiveName, manifestRoot.Kind))
		root, ok := rootsByArchive[strings.ToLower(archiveName)]
		rootResult := RestoreRootResult{
			Kind:        manifestRoot.Kind,
			ArchiveName: archiveName,
			Path:        manifestRoot.Path,
		}
		if ok {
			rootResult.Kind = root.Kind
			rootResult.Path = root.Path
		}
		if !ok {
			rootResult.Error = "当前运行态未声明该数据根，已跳过"
			result.Roots = append(result.Roots, rootResult)
			continue
		}
		seenRoots[strings.ToLower(archiveName)] = true
		count, bytes, skipped, restoreErr := restoreRootFromZip(&reader.Reader, root, s.backupDir)
		rootResult.RestoredFiles = count
		rootResult.RestoredBytes = bytes
		rootResult.SkippedFiles = skipped
		if restoreErr != nil {
			rootResult.Error = restoreErr.Error()
		}
		result.Roots = append(result.Roots, rootResult)
		result.FileCount += count
		result.Bytes += bytes
	}
	for archiveName, root := range rootsByArchive {
		if seenRoots[archiveName] {
			continue
		}
		result.Roots = append(result.Roots, RestoreRootResult{
			Kind:        root.Kind,
			ArchiveName: root.ArchiveName,
			Path:        root.Path,
			Error:       "检查点未包含该数据根",
		})
	}
	for _, root := range result.Roots {
		if root.Error != "" {
			result.Message = "回滚恢复完成，但有数据根未恢复"
			return result
		}
	}
	result.OK = true
	result.Message = "Ariadne 回滚检查点已恢复"
	return result
}

func (s *Service) rootStatuses() []DataRootStatus {
	roots := uniqueRoots(s.roots)
	statuses := make([]DataRootStatus, 0, len(roots))
	for _, root := range roots {
		status := DataRootStatus{Kind: root.Kind, ArchiveName: root.ArchiveName, Path: root.Path}
		if info, err := os.Stat(root.Path); err == nil && info.IsDir() {
			status.Exists = true
			status.FileCount, status.Bytes = dirFileStats(root.Path, s.backupDir)
		}
		statuses = append(statuses, status)
	}
	return statuses
}

func defaultDataRoots() []DataRootStatus {
	logicalRoot := filepath.Join(defaultRoamingDir(), "Ariadne")
	roots := []DataRootStatus{{Kind: "roaming", ArchiveName: "roaming", Path: logicalRoot}}
	for index, path := range virtualizedRoots(logicalRoot) {
		archiveName := "virtualized"
		if index > 0 {
			archiveName = "virtualized_" + strconv.Itoa(index+1)
		}
		roots = append(roots, DataRootStatus{Kind: "virtualized", ArchiveName: archiveName, Path: path})
	}
	return roots
}

func defaultBackupDir() string {
	return filepath.Join(defaultRoamingDir(), "Ariadne", "backups")
}

func defaultRoamingDir() string {
	if base := os.Getenv("APPDATA"); strings.TrimSpace(base) != "" {
		return base
	}
	if dir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(dir) != "" {
		return dir
	}
	return "."
}

func virtualizedRoots(logicalRoot string) []string {
	appData := os.Getenv("APPDATA")
	localAppData := os.Getenv("LOCALAPPDATA")
	if logicalRoot == "" || appData == "" || localAppData == "" {
		return nil
	}
	relative, err := filepath.Rel(filepath.Clean(appData), filepath.Clean(logicalRoot))
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return nil
	}
	matches, err := filepath.Glob(filepath.Join(localAppData, "Packages", "*", "LocalCache", "Roaming", relative))
	if err != nil {
		return nil
	}
	roots := []string{}
	for _, match := range matches {
		if info, statErr := os.Stat(match); statErr == nil && info.IsDir() {
			roots = append(roots, match)
		}
	}
	sort.Strings(roots)
	return roots
}

func uniqueRoots(roots []dataRoot) []dataRoot {
	seen := map[string]bool{}
	seenArchiveNames := map[string]bool{}
	unique := []dataRoot{}
	for _, root := range roots {
		path := filepath.Clean(strings.TrimSpace(root.Path))
		if path == "." || path == "" {
			continue
		}
		key := strings.ToLower(path)
		if seen[key] {
			continue
		}
		seen[key] = true
		kind := firstNonEmpty(root.Kind, "data")
		archiveName := uniqueArchiveName(firstNonEmpty(root.ArchiveName, safeZipName(kind)), seenArchiveNames)
		unique = append(unique, dataRoot{
			Kind:        kind,
			ArchiveName: archiveName,
			Path:        path,
		})
	}
	sort.SliceStable(unique, func(i, j int) bool {
		if unique[i].Kind == unique[j].Kind {
			return unique[i].Path < unique[j].Path
		}
		return unique[i].Kind < unique[j].Kind
	})
	return unique
}

func uniqueArchiveName(value string, seen map[string]bool) string {
	base := safeZipName(firstNonEmpty(value, "data"))
	name := base
	for suffix := 2; seen[strings.ToLower(name)]; suffix++ {
		name = base + "_" + strconv.Itoa(suffix)
	}
	seen[strings.ToLower(name)] = true
	return name
}

func dirFileStats(root string, excludeDir string) (int, int64) {
	count := 0
	var bytes int64
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if sameOrInside(path, excludeDir) && filepath.Clean(path) != filepath.Clean(root) {
				return filepath.SkipDir
			}
			return nil
		}
		info, statErr := entry.Info()
		if statErr != nil {
			return nil
		}
		count++
		bytes += info.Size()
		return nil
	})
	return count, bytes
}

func addRootToZip(writer *zip.Writer, root DataRootStatus, excludeDir string) (int, int64, error) {
	count := 0
	var bytes int64
	cleanRoot := filepath.Clean(root.Path)
	err := filepath.WalkDir(cleanRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if sameOrInside(path, excludeDir) && filepath.Clean(path) != cleanRoot {
				return filepath.SkipDir
			}
			return nil
		}
		info, statErr := entry.Info()
		if statErr != nil {
			return statErr
		}
		relative, relErr := filepath.Rel(cleanRoot, path)
		if relErr != nil {
			return relErr
		}
		name := filepath.ToSlash(filepath.Join("data", safeZipName(firstNonEmpty(root.ArchiveName, root.Kind)), relative))
		header, headerErr := zip.FileInfoHeader(info)
		if headerErr != nil {
			return headerErr
		}
		header.Name = name
		header.Method = zip.Deflate
		item, createErr := writer.CreateHeader(header)
		if createErr != nil {
			return createErr
		}
		source, openErr := os.Open(path)
		if openErr != nil {
			return openErr
		}
		_, copyErr := io.Copy(item, source)
		closeErr := source.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		count++
		bytes += info.Size()
		return nil
	})
	return count, bytes, err
}

func addManifest(writer *zip.Writer, manifest backupManifest) error {
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	item, err := writer.Create("manifest.json")
	if err != nil {
		return err
	}
	_, err = item.Write(raw)
	return err
}

func readBackupManifest(reader *zip.Reader) (backupManifest, error) {
	for _, file := range reader.File {
		if file.Name != "manifest.json" {
			continue
		}
		item, err := file.Open()
		if err != nil {
			return backupManifest{}, err
		}
		defer item.Close()
		var manifest backupManifest
		if err := json.NewDecoder(item).Decode(&manifest); err != nil {
			return backupManifest{}, err
		}
		return manifest, nil
	}
	return backupManifest{}, os.ErrNotExist
}

func restoreRootFromZip(reader *zip.Reader, root dataRoot, backupDir string) (int, int64, int, error) {
	cleanRoot := filepath.Clean(root.Path)
	if strings.TrimSpace(cleanRoot) == "" || cleanRoot == "." {
		return 0, 0, 0, errUnsafeRoot
	}
	if err := os.MkdirAll(cleanRoot, 0o755); err != nil {
		return 0, 0, 0, err
	}
	if err := clearRootForRestore(cleanRoot, backupDir); err != nil {
		return 0, 0, 0, err
	}
	prefix := filepath.ToSlash(filepath.Join("data", safeZipName(firstNonEmpty(root.ArchiveName, root.Kind)))) + "/"
	count := 0
	skipped := 0
	var bytes int64
	for _, file := range reader.File {
		if file.FileInfo().IsDir() || !strings.HasPrefix(file.Name, prefix) {
			continue
		}
		relative := strings.TrimPrefix(file.Name, prefix)
		target := filepath.Join(cleanRoot, filepath.FromSlash(relative))
		if !sameOrInside(target, cleanRoot) {
			skipped++
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return count, bytes, skipped, err
		}
		source, err := file.Open()
		if err != nil {
			return count, bytes, skipped, err
		}
		output, err := os.Create(target)
		if err != nil {
			_ = source.Close()
			return count, bytes, skipped, err
		}
		written, copyErr := io.Copy(output, source)
		closeSourceErr := source.Close()
		closeOutputErr := output.Close()
		if copyErr != nil {
			return count, bytes, skipped, copyErr
		}
		if closeSourceErr != nil {
			return count, bytes, skipped, closeSourceErr
		}
		if closeOutputErr != nil {
			return count, bytes, skipped, closeOutputErr
		}
		count++
		bytes += written
	}
	return count, bytes, skipped, nil
}

func clearRootForRestore(root string, backupDir string) error {
	cleanRoot := filepath.Clean(root)
	if strings.TrimSpace(cleanRoot) == "" || cleanRoot == "." {
		return errUnsafeRoot
	}
	entries, err := os.ReadDir(cleanRoot)
	if os.IsNotExist(err) {
		return os.MkdirAll(cleanRoot, 0o755)
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(cleanRoot, entry.Name())
		if sameOrInside(path, backupDir) || sameOrInside(backupDir, path) {
			continue
		}
		if !sameOrInside(path, cleanRoot) {
			return errUnsafeRoot
		}
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

func backupDirStatus(path string) (int, int64, string) {
	if path == "" {
		return 0, 0, ""
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0, 0, ""
	}
	count := 0
	var bytes int64
	latest := ""
	var latestTime time.Time
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".zip") {
			continue
		}
		info, statErr := entry.Info()
		if statErr != nil {
			continue
		}
		count++
		bytes += info.Size()
		if latest == "" || info.ModTime().After(latestTime) {
			latest = filepath.Join(path, entry.Name())
			latestTime = info.ModTime()
		}
	}
	return count, bytes, latest
}

func uniqueBackupPath(dir string, filename string) string {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	ext := filepath.Ext(filename)
	candidate := filepath.Join(dir, filename)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate
	}
	for suffix := 2; ; suffix++ {
		candidate = filepath.Join(dir, base+"-"+strconv.Itoa(suffix)+ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

func sameOrInside(path string, root string) bool {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(root) == "" {
		return false
	}
	cleanPath := filepath.Clean(path)
	cleanRoot := filepath.Clean(root)
	if strings.EqualFold(cleanPath, cleanRoot) {
		return true
	}
	relative, err := filepath.Rel(cleanRoot, cleanPath)
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}

func safeZipName(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return "data"
	}
	replacer := strings.NewReplacer("\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_", " ", "_")
	return replacer.Replace(text)
}

func firstNonEmpty(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
