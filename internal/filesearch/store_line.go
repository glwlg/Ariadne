package filesearch

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const lineIndexFileName = "file-index.tsv"
const lineIndexScanBufferSize = 128 * 1024
const lineIndexPathEnv = "ARIADNE_FILE_INDEX_PATH"

type lineFileIndex struct {
	mu       sync.RWMutex
	path     string
	count    int
	volumes  []string
	appended map[string]struct{}
}

func defaultLineIndexPath() string {
	if configured := strings.TrimSpace(os.Getenv(lineIndexPathEnv)); configured != "" {
		return filepath.Clean(os.ExpandEnv(configured))
	}
	base := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if base == "" {
		if cacheDir, err := os.UserCacheDir(); err == nil {
			base = cacheDir
		}
	}
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "Ariadne", lineIndexFileName)
}

func sharedLineIndexPath() string {
	base := strings.TrimSpace(os.Getenv("PROGRAMDATA"))
	if base == "" {
		base = strings.TrimSpace(os.Getenv("ALLUSERSPROFILE"))
	}
	if base == "" {
		return defaultLineIndexPath()
	}
	return filepath.Join(base, "Ariadne", lineIndexFileName)
}

func defaultReadLineIndexPaths() []string {
	paths := []string{}
	if configured := strings.TrimSpace(os.Getenv(lineIndexPathEnv)); configured != "" {
		paths = append(paths, filepath.Clean(os.ExpandEnv(configured)))
	} else {
		paths = append(paths, sharedLineIndexPath(), defaultLineIndexPath())
	}
	seen := map[string]struct{}{}
	cleaned := make([]string, 0, len(paths))
	for _, path := range paths {
		path = filepath.Clean(strings.TrimSpace(path))
		if path == "" || path == "." {
			continue
		}
		key := strings.ToLower(path)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		cleaned = append(cleaned, path)
	}
	return cleaned
}

func openLineFileIndex(path string) (*lineFileIndex, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = defaultLineIndexPath()
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() || info.Size() == 0 {
		return nil, errors.New("file index cache is empty")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	count, volumes := scanLineFileIndexMetadata(file)
	if count == 0 {
		return nil, errors.New("file index cache has no entries")
	}
	return &lineFileIndex{path: path, count: count, volumes: volumes}, nil
}

func writeLineFileIndex(path string, source *compactFileIndex, policies ...FileSearchPolicy) (*lineFileIndex, error) {
	if source == nil {
		return nil, errors.New("file index source is empty")
	}
	source.Finalize()
	var filter fileSearchFilter
	if len(policies) > 0 {
		filter = newFileSearchFilter(policies[0])
	}
	path = strings.TrimSpace(path)
	if path == "" {
		path = defaultLineIndexPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	tempPath := path + ".tmp"
	file, err := os.Create(tempPath)
	if err != nil {
		return nil, err
	}
	writer := bufio.NewWriterSize(file, 1024*1024)
	count := 0
	writeErr := error(nil)
	for _, node := range source.nodes {
		raw := rawResult{Name: source.nodeName(node), Path: source.resolvePath(node), IsDirectory: node.isDirectory}
		cleaned, ok := normalizeLineFileRawResult(raw)
		if !ok {
			continue
		}
		if filter.Excludes(cleaned.Path) {
			continue
		}
		if writeErr = writeLineFileIndexEntry(writer, cleaned); writeErr != nil {
			break
		}
		count++
	}
	if writeErr == nil {
		writeErr = writer.Flush()
	}
	closeErr := file.Close()
	if writeErr != nil {
		_ = os.Remove(tempPath)
		return nil, writeErr
	}
	if closeErr != nil {
		_ = os.Remove(tempPath)
		return nil, closeErr
	}
	_ = os.Remove(path)
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return nil, err
	}
	return &lineFileIndex{path: path, count: count, volumes: source.Volumes()}, nil
}

func (idx *lineFileIndex) AppendRawResults(entries []rawResult) (int, error) {
	if idx == nil || idx.path == "" || len(entries) == 0 {
		return 0, nil
	}
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(idx.path), 0o755); err != nil {
		return 0, err
	}
	file, err := os.OpenFile(idx.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	writer := bufio.NewWriterSize(file, 32*1024)
	added := 0
	if idx.appended == nil {
		idx.appended = map[string]struct{}{}
	}
	for _, entry := range entries {
		cleaned, ok := normalizeLineFileRawResult(entry)
		if !ok {
			continue
		}
		key := strings.ToLower(filepath.Clean(cleaned.Path))
		if _, exists := idx.appended[key]; exists {
			continue
		}
		idx.appended[key] = struct{}{}
		if err := writeLineFileIndexEntry(writer, cleaned); err != nil {
			_ = file.Close()
			return added, err
		}
		added++
		idx.addVolumeLocked(cleaned.Path)
	}
	flushErr := writer.Flush()
	closeErr := file.Close()
	if flushErr != nil {
		return added, flushErr
	}
	if closeErr != nil {
		return added, closeErr
	}
	idx.count += added
	return added, nil
}

func normalizeLineFileRawResult(entry rawResult) (rawResult, bool) {
	entry.Path = filepath.Clean(strings.TrimSpace(entry.Path))
	if entry.Path == "" || entry.Path == "." {
		return rawResult{}, false
	}
	entry.Name = strings.TrimSpace(entry.Name)
	if entry.Name == "" {
		entry.Name = filepath.Base(entry.Path)
	}
	if entry.Name == "" || entry.Name == "." {
		return rawResult{}, false
	}
	return entry, true
}

func writeLineFileIndexEntry(writer *bufio.Writer, entry rawResult) error {
	entry, ok := normalizeLineFileRawResult(entry)
	if !ok {
		return nil
	}
	if _, err := writer.WriteString(strings.ToLower(entry.Name)); err != nil {
		return err
	}
	if err := writer.WriteByte('\t'); err != nil {
		return err
	}
	if entry.IsDirectory {
		if err := writer.WriteByte('1'); err != nil {
			return err
		}
	} else if err := writer.WriteByte('0'); err != nil {
		return err
	}
	if err := writer.WriteByte('\t'); err != nil {
		return err
	}
	if _, err := writer.WriteString(entry.Path); err != nil {
		return err
	}
	return writer.WriteByte('\n')
}

func (idx *lineFileIndex) addVolumeLocked(path string) {
	volume := strings.ToUpper(filepath.VolumeName(path))
	if volume == "" {
		return
	}
	volume += `\`
	for _, existing := range idx.volumes {
		if strings.EqualFold(existing, volume) {
			return
		}
	}
	idx.volumes = append(idx.volumes, volume)
	sort.Strings(idx.volumes)
}

func scanLineFileIndexMetadata(reader io.Reader) (int, []string) {
	buffered := bufio.NewReaderSize(reader, lineIndexScanBufferSize)
	count := 0
	seenVolumes := map[string]bool{}
	volumes := []string{}
	for {
		line, err := buffered.ReadSlice('\n')
		if len(line) > 0 {
			line = bytes.TrimRight(line, "\r\n")
			if volume, ok := lineFileIndexLineVolume(line); ok {
				count++
				if volume != "" && !seenVolumes[volume] {
					seenVolumes[volume] = true
					volumes = append(volumes, volume)
				}
			}
		}
		if err == nil {
			continue
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			_, _ = buffered.ReadBytes('\n')
			continue
		}
		break
	}
	sort.Strings(volumes)
	return count, volumes
}

func lineFileIndexLineVolume(line []byte) (string, bool) {
	firstTab := bytes.IndexByte(line, '\t')
	if firstTab <= 0 || firstTab+3 >= len(line) {
		return "", false
	}
	secondTab := bytes.IndexByte(line[firstTab+1:], '\t')
	if secondTab <= 0 {
		return "", false
	}
	secondTab += firstTab + 1
	path := string(line[secondTab+1:])
	volume := strings.ToUpper(filepath.VolumeName(path))
	if volume == "" {
		return "", true
	}
	return volume + `\`, true
}

func (idx *lineFileIndex) Search(query string, limit int) []rawResult {
	if idx == nil || idx.path == "" {
		return nil
	}
	query = strings.TrimSpace(query)
	normalized := strings.ToLower(query)
	if len([]rune(normalized)) < 2 {
		return nil
	}
	if limit <= 0 {
		limit = int(defaultMaxResults)
	}
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	file, err := os.Open(idx.path)
	if err != nil {
		return nil
	}
	defer file.Close()
	matchQuery := normalized
	if base := strings.ToLower(strings.TrimSpace(filepath.Base(query))); base != "" && base != "." {
		matchQuery = base
	}
	pathLike := isPathLikeQuery(query)
	candidates := scanLineFileIndex(file, []byte(matchQuery), normalized, pathLike, limit*8)
	if len(candidates) == 0 {
		return nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return strings.ToLower(candidates[i].raw.Path) < strings.ToLower(candidates[j].raw.Path)
		}
		return candidates[i].score > candidates[j].score
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	results := make([]rawResult, 0, len(candidates))
	for _, candidate := range candidates {
		results = append(results, candidate.raw)
	}
	return results
}

func (idx *lineFileIndex) Count() int {
	if idx == nil {
		return 0
	}
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.count
}

func (idx *lineFileIndex) Volumes() []string {
	if idx == nil {
		return nil
	}
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return append([]string(nil), idx.volumes...)
}

func (idx *lineFileIndex) Close() {}

func scanLineFileIndex(reader io.Reader, matchQuery []byte, normalized string, pathLike bool, maxCandidates int) []scoredRawResult {
	if maxCandidates <= 0 {
		maxCandidates = int(defaultMaxResults)
	}
	buffered := bufio.NewReaderSize(reader, lineIndexScanBufferSize)
	candidates := make([]scoredRawResult, 0, maxCandidates)
	seen := map[string]bool{}
	for {
		line, err := buffered.ReadSlice('\n')
		if len(line) > 0 {
			line = bytes.TrimRight(line, "\r\n")
			addLineFileCandidate(line, matchQuery, normalized, pathLike, maxCandidates, seen, &candidates)
		}
		if err == nil {
			continue
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			_, _ = buffered.ReadBytes('\n')
			continue
		}
		break
	}
	return candidates
}

func addLineFileCandidate(line []byte, matchQuery []byte, normalized string, pathLike bool, maxCandidates int, seen map[string]bool, candidates *[]scoredRawResult) {
	firstTab := bytes.IndexByte(line, '\t')
	if firstTab <= 0 || firstTab+3 >= len(line) {
		return
	}
	secondTab := bytes.IndexByte(line[firstTab+1:], '\t')
	if secondTab <= 0 {
		return
	}
	secondTab += firstTab + 1
	lowerNameBytes := line[:firstTab]
	isDirectory := len(line[firstTab+1:secondTab]) > 0 && line[firstTab+1] == '1'
	pathBytes := line[secondTab+1:]
	if len(pathBytes) == 0 {
		return
	}
	if pathLike {
		if !bytes.Contains(bytes.ToLower(pathBytes), []byte(normalized)) {
			return
		}
	} else if !bytes.Contains(lowerNameBytes, matchQuery) {
		return
	}
	path := string(pathBytes)
	key := strings.ToLower(filepath.Clean(path))
	if seen[key] {
		return
	}
	seen[key] = true
	lowerName := string(lowerNameBytes)
	score := fileScoreLower(lowerName, "", string(matchQuery))
	raw := rawResult{Name: filepath.Base(path), Path: path, IsDirectory: isDirectory}
	item := scoredRawResult{raw: raw, score: score}
	if len(*candidates) < maxCandidates {
		*candidates = append(*candidates, item)
		return
	}
	worstIndex := 0
	worstScore := (*candidates)[0].score
	for index := 1; index < len(*candidates); index++ {
		if (*candidates)[index].score < worstScore {
			worstIndex = index
			worstScore = (*candidates)[index].score
		}
	}
	if score > worstScore {
		(*candidates)[worstIndex] = item
	}
}
