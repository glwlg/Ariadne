package filesearch

import (
	"bufio"
	"bytes"
	"context"
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
const lineIndexPathEnv = "FANCUN_FILE_INDEX_PATH"
const lineIndexAppendedCacheLimit = 16384
const lineIndexCancelCheckInterval = 2048

type lineFileIndex struct {
	mu            sync.RWMutex
	path          string
	count         int
	volumes       []string
	appended      map[string]struct{}
	appendedOrder []string
	appendedHead  int
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
	return filepath.Join(base, "Fancun", lineIndexFileName)
}

func sharedLineIndexPath() string {
	base := strings.TrimSpace(os.Getenv("PROGRAMDATA"))
	if base == "" {
		base = strings.TrimSpace(os.Getenv("ALLUSERSPROFILE"))
	}
	if base == "" {
		return defaultLineIndexPath()
	}
	return filepath.Join(base, "Fancun", lineIndexFileName)
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
	return openLineFileIndexContext(context.Background(), path)
}

func openLineFileIndexContext(ctx context.Context, path string) (*lineFileIndex, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
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
	count, volumes, err := scanLineFileIndexMetadataContext(ctx, file)
	if err != nil {
		return nil, err
	}
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
		key := strings.ToLower(cleaned.Path)
		if _, exists := idx.appended[key]; exists {
			continue
		}
		idx.recordAppendedLocked(key)
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

func (idx *lineFileIndex) recordAppendedLocked(key string) {
	if idx.appended == nil {
		idx.appended = map[string]struct{}{}
	}
	for len(idx.appended) >= lineIndexAppendedCacheLimit && idx.appendedHead < len(idx.appendedOrder) {
		oldest := idx.appendedOrder[idx.appendedHead]
		idx.appendedOrder[idx.appendedHead] = ""
		idx.appendedHead++
		delete(idx.appended, oldest)
	}
	if idx.appendedHead > 0 && idx.appendedHead >= len(idx.appendedOrder)/2 {
		remaining := copy(idx.appendedOrder, idx.appendedOrder[idx.appendedHead:])
		idx.appendedOrder = idx.appendedOrder[:remaining]
		idx.appendedHead = 0
	}
	idx.appended[key] = struct{}{}
	idx.appendedOrder = append(idx.appendedOrder, key)
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
	count, volumes, _ := scanLineFileIndexMetadataContext(context.Background(), reader)
	return count, volumes
}

func scanLineFileIndexMetadataContext(ctx context.Context, reader io.Reader) (int, []string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	buffered := bufio.NewReaderSize(reader, lineIndexScanBufferSize)
	count := 0
	seenVolumes := map[string]bool{}
	volumes := []string{}
	lines := 0
	for {
		if lines&(lineIndexCancelCheckInterval*2-1) == 0 {
			if err := ctx.Err(); err != nil {
				return 0, nil, err
			}
		}
		line, err := buffered.ReadSlice('\n')
		lines++
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
	return count, volumes, nil
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
	return idx.SearchContext(context.Background(), query, limit)
}

func (idx *lineFileIndex) SearchContext(ctx context.Context, query string, limit int) []rawResult {
	if idx == nil || idx.path == "" {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
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
	if ctx.Err() != nil {
		return nil
	}
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
	candidates := scanLineFileIndexContext(ctx, file, []byte(matchQuery), []byte(normalized), pathLike, limit*8)
	if ctx.Err() != nil || len(candidates) == 0 {
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
	return scanLineFileIndexContext(context.Background(), reader, matchQuery, []byte(normalized), pathLike, maxCandidates)
}

type lineCandidateSet struct {
	matchQuery      []byte
	normalizedBytes []byte
	normalizedASCII bool
	pathLike        bool
	maxCandidates   int
	seen            map[string]bool
	candidates      []scoredRawResult
	worstIndex      int
	worstScore      float64
}

func scanLineFileIndexContext(ctx context.Context, reader io.Reader, matchQuery []byte, normalizedBytes []byte, pathLike bool, maxCandidates int) []scoredRawResult {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return nil
	}
	if maxCandidates <= 0 {
		maxCandidates = int(defaultMaxResults)
	}
	buffered := bufio.NewReaderSize(reader, lineIndexScanBufferSize)
	tracker := lineCandidateSet{
		matchQuery:      matchQuery,
		normalizedBytes: normalizedBytes,
		normalizedASCII: isASCIIBytes(normalizedBytes),
		pathLike:        pathLike,
		maxCandidates:   maxCandidates,
		seen:            map[string]bool{},
		candidates:      make([]scoredRawResult, 0, maxCandidates),
	}
	lines := 0
	for {
		if lines&(lineIndexCancelCheckInterval-1) == 0 {
			if ctx.Err() != nil {
				return nil
			}
		}
		line, err := buffered.ReadSlice('\n')
		lines++
		if len(line) > 0 {
			line = bytes.TrimRight(line, "\r\n")
			tracker.add(line)
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
	if ctx.Err() != nil {
		return nil
	}
	return tracker.candidates
}

func (s *lineCandidateSet) add(line []byte) {
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
	if s.pathLike {
		if s.normalizedASCII {
			if !containsASCIIFoldBytes(pathBytes, s.normalizedBytes) {
				return
			}
		} else if !bytes.Contains(bytes.ToLower(pathBytes), s.normalizedBytes) {
			return
		}
	} else if !bytes.Contains(lowerNameBytes, s.matchQuery) {
		return
	}
	score := fileScoreLowerBytes(lowerNameBytes, s.matchQuery)
	if len(s.candidates) >= s.maxCandidates && score <= s.worstScore {
		return
	}
	path := string(pathBytes)
	key := strings.ToLower(path)
	if s.seen[key] {
		return
	}
	s.seen[key] = true
	raw := rawResult{Name: filepath.Base(path), Path: path, IsDirectory: isDirectory}
	item := scoredRawResult{raw: raw, score: score}
	if len(s.candidates) < s.maxCandidates {
		s.candidates = append(s.candidates, item)
		if len(s.candidates) == s.maxCandidates {
			s.recomputeWorst()
		}
		return
	}
	s.candidates[s.worstIndex] = item
	s.recomputeWorst()
}

func (s *lineCandidateSet) recomputeWorst() {
	if len(s.candidates) == 0 {
		s.worstIndex = 0
		s.worstScore = 0
		return
	}
	worstIndex := 0
	worstScore := s.candidates[0].score
	for index := 1; index < len(s.candidates); index++ {
		if s.candidates[index].score < worstScore {
			worstIndex = index
			worstScore = s.candidates[index].score
		}
	}
	s.worstIndex = worstIndex
	s.worstScore = worstScore
}

func fileScoreLowerBytes(name []byte, query []byte) float64 {
	if bytes.Equal(name, query) {
		return 95
	}
	if bytes.HasPrefix(name, query) {
		return 88
	}
	if bytes.Contains(name, query) {
		return 72
	}
	return 30
}

func isASCIIBytes(value []byte) bool {
	for _, b := range value {
		if b > 0x7f {
			return false
		}
	}
	return true
}

func containsASCIIFoldBytes(haystack []byte, lowerNeedle []byte) bool {
	if len(lowerNeedle) == 0 {
		return true
	}
	if len(lowerNeedle) > len(haystack) {
		return false
	}
	first := lowerASCII(lowerNeedle[0])
	limit := len(haystack) - len(lowerNeedle)
	for start := 0; start <= limit; start++ {
		if lowerASCII(haystack[start]) != first {
			continue
		}
		matched := true
		for offset := 1; offset < len(lowerNeedle); offset++ {
			if lowerASCII(haystack[start+offset]) != lowerASCII(lowerNeedle[offset]) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}
