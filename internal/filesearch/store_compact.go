package filesearch

import (
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
	"unsafe"
)

type fileIndexNode struct {
	Ref         uint64
	Parent      uint64
	Name        string
	IsDirectory bool
}

type compactFileIndex struct {
	volumes   []string
	volumeIDs map[string]int
	nodes     []compactFileNode
	refs      []compactFileRef
	names     []byte
	finalized bool
}

type compactFileNode struct {
	ref         uint64
	parent      uint64
	nameOffset  uint32
	nameLength  uint16
	volume      uint16
	isDirectory bool
}

type compactFileRef struct {
	ref    uint64
	index  int
	volume uint16
}

type compactScoredNode struct {
	index int
	score float64
}

func newCompactFileIndex(volumes []string) *compactFileIndex {
	index := &compactFileIndex{
		volumes:   make([]string, 0, len(volumes)),
		volumeIDs: make(map[string]int, len(volumes)),
		nodes:     make([]compactFileNode, 0, 64*1024),
		names:     make([]byte, 0, 4*1024*1024),
	}
	for _, volume := range volumes {
		index.volumeID(volume)
	}
	return index
}

func (idx *compactFileIndex) AddNode(volume string, node fileIndexNode) error {
	if idx == nil {
		return nil
	}
	name := strings.TrimSpace(node.Name)
	if name == "" {
		return nil
	}
	volumeID := idx.volumeID(volume)
	if volumeID > int(^uint16(0)) {
		return nil
	}
	if len(name) > int(^uint16(0)) {
		name = name[:^uint16(0)]
	}
	nameOffset := len(idx.names)
	if nameOffset > int(^uint32(0)) {
		return nil
	}
	idx.names = append(idx.names, name...)
	next := compactFileNode{
		ref:         node.Ref,
		parent:      node.Parent,
		nameOffset:  uint32(nameOffset),
		nameLength:  uint16(len(name)),
		volume:      uint16(volumeID),
		isDirectory: node.IsDirectory,
	}
	idx.nodes = append(idx.nodes, next)
	idx.finalized = false
	return nil
}

func (idx *compactFileIndex) Search(query string, limit int) []rawResult {
	if idx == nil || len(idx.nodes) == 0 {
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
	idx.Finalize()
	matchQuery := normalized
	if base := strings.ToLower(strings.TrimSpace(filepath.Base(query))); base != "" && base != "." {
		matchQuery = base
	}
	candidates := idx.searchCandidates(matchQuery, normalized, limit*8)
	if len(candidates) == 0 {
		return nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return strings.ToLower(idx.nodeName(idx.nodes[candidates[i].index])) < strings.ToLower(idx.nodeName(idx.nodes[candidates[j].index]))
		}
		return candidates[i].score > candidates[j].score
	})
	pathLike := isPathLikeQuery(query)
	results := make([]rawResult, 0, limit)
	seen := map[string]bool{}
	for _, candidate := range candidates {
		node := idx.nodes[candidate.index]
		path := idx.resolvePath(node)
		if path == "" {
			continue
		}
		if pathLike && !strings.Contains(strings.ToLower(path), normalized) {
			continue
		}
		key := strings.ToLower(filepath.Clean(path))
		if seen[key] {
			continue
		}
		seen[key] = true
		results = append(results, rawResult{Name: idx.nodeName(node), Path: path, IsDirectory: node.isDirectory})
		if len(results) >= limit {
			break
		}
	}
	return results
}

func (idx *compactFileIndex) Count() int {
	if idx == nil {
		return 0
	}
	return len(idx.nodes)
}

func (idx *compactFileIndex) Volumes() []string {
	if idx == nil {
		return nil
	}
	return append([]string(nil), idx.volumes...)
}

func (idx *compactFileIndex) Close() {}

func (idx *compactFileIndex) Finalize() {
	if idx == nil || idx.finalized {
		return
	}
	idx.nodes = slices.Clip(idx.nodes)
	idx.names = slices.Clip(idx.names)
	idx.refs = make([]compactFileRef, 0, len(idx.nodes))
	for nodeIndex, node := range idx.nodes {
		idx.refs = append(idx.refs, compactFileRef{volume: node.volume, ref: node.ref, index: nodeIndex})
	}
	sort.Slice(idx.refs, func(i, j int) bool {
		if idx.refs[i].volume == idx.refs[j].volume {
			return idx.refs[i].ref < idx.refs[j].ref
		}
		return idx.refs[i].volume < idx.refs[j].volume
	})
	idx.volumeIDs = nil
	idx.finalized = true
}

func (idx *compactFileIndex) volumeID(volume string) int {
	volume = filepath.Clean(strings.TrimSpace(volume))
	if !strings.HasSuffix(volume, string(filepath.Separator)) && len(volume) == 2 && volume[1] == ':' {
		volume += string(filepath.Separator)
	}
	if idx.volumeIDs == nil {
		idx.volumeIDs = map[string]int{}
	}
	if id, ok := idx.volumeIDs[volume]; ok {
		return id
	}
	id := len(idx.volumes)
	idx.volumeIDs[volume] = id
	idx.volumes = append(idx.volumes, volume)
	return id
}

func (idx *compactFileIndex) searchCandidates(matchQuery string, normalized string, maxCandidates int) []compactScoredNode {
	if maxCandidates <= 0 {
		maxCandidates = int(defaultMaxResults)
	}
	candidates := make([]compactScoredNode, 0, maxCandidates)
	for nodeIndex, node := range idx.nodes {
		name := idx.nodeName(node)
		if !fileNameContains(name, matchQuery) {
			continue
		}
		score := fileNameScore(name, matchQuery)
		if fileNameEqual(name, normalized) {
			score += 6
		}
		if !node.isDirectory {
			score += 1
		}
		candidate := compactScoredNode{index: nodeIndex, score: score}
		if len(candidates) < maxCandidates {
			candidates = append(candidates, candidate)
			continue
		}
		worstIndex := 0
		worstScore := candidates[0].score
		for index := 1; index < len(candidates); index++ {
			if candidates[index].score < worstScore {
				worstIndex = index
				worstScore = candidates[index].score
			}
		}
		if score > worstScore {
			candidates[worstIndex] = candidate
		}
	}
	return candidates
}

func (idx *compactFileIndex) resolvePath(node compactFileNode) string {
	if idx == nil || int(node.volume) >= len(idx.volumes) || node.nameLength == 0 {
		return ""
	}
	parts := []string{idx.nodeName(node)}
	parent := node.parent
	for depth := 0; depth < 256 && parent != 0 && parent != node.ref; depth++ {
		parentIndex, ok := idx.nodeIndexByRef(node.volume, parent)
		if !ok {
			break
		}
		parentNode := idx.nodes[parentIndex]
		parentName := idx.nodeName(parentNode)
		if parentName != "" && parentName != "." {
			parts = append(parts, parentName)
		}
		if parentNode.parent == parent || parentNode.parent == 0 {
			break
		}
		parent = parentNode.parent
	}
	for left, right := 0, len(parts)-1; left < right; left, right = left+1, right-1 {
		parts[left], parts[right] = parts[right], parts[left]
	}
	pathParts := append([]string{idx.volumes[node.volume]}, parts...)
	return filepath.Clean(filepath.Join(pathParts...))
}

func (idx *compactFileIndex) nodeName(node compactFileNode) string {
	start := int(node.nameOffset)
	end := start + int(node.nameLength)
	if start < 0 || end > len(idx.names) || start >= end {
		return ""
	}
	return unsafe.String(unsafe.SliceData(idx.names[start:end]), end-start)
}

func (idx *compactFileIndex) nodeIndexByRef(volume uint16, ref uint64) (int, bool) {
	index := sort.Search(len(idx.refs), func(i int) bool {
		if idx.refs[i].volume == volume {
			return idx.refs[i].ref >= ref
		}
		return idx.refs[i].volume >= volume
	})
	if index >= len(idx.refs) || idx.refs[index].volume != volume || idx.refs[index].ref != ref {
		return 0, false
	}
	return idx.refs[index].index, true
}

func fileNameContains(name string, query string) bool {
	if query == "" {
		return true
	}
	if isASCIIString(query) {
		return containsASCIIFold(name, query)
	}
	return strings.Contains(name, query)
}

func fileNameEqual(name string, query string) bool {
	if isASCIIString(query) {
		return equalASCIIFold(name, query)
	}
	return name == query
}

func fileNameScore(name string, query string) float64 {
	if fileNameEqual(name, query) {
		return 95
	}
	if isASCIIString(query) {
		if hasPrefixASCIIFold(name, query) {
			return 88
		}
		if containsASCIIFold(name, query) {
			return 72
		}
		return 30
	}
	if strings.HasPrefix(name, query) {
		return 88
	}
	if strings.Contains(name, query) {
		return 72
	}
	return 30
}

func isASCIIString(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] > 0x7f {
			return false
		}
	}
	return true
}

func containsASCIIFold(name string, lowerQuery string) bool {
	if lowerQuery == "" {
		return true
	}
	if len(lowerQuery) > len(name) {
		return false
	}
	first := lowerASCII(lowerQuery[0])
	limit := len(name) - len(lowerQuery)
	for start := 0; start <= limit; start++ {
		if lowerASCII(name[start]) != first {
			continue
		}
		matched := true
		for offset := 1; offset < len(lowerQuery); offset++ {
			if lowerASCII(name[start+offset]) != lowerASCII(lowerQuery[offset]) {
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

func hasPrefixASCIIFold(name string, lowerQuery string) bool {
	if len(lowerQuery) > len(name) {
		return false
	}
	for index := 0; index < len(lowerQuery); index++ {
		if lowerASCII(name[index]) != lowerASCII(lowerQuery[index]) {
			return false
		}
	}
	return true
}

func equalASCIIFold(name string, lowerQuery string) bool {
	return len(name) == len(lowerQuery) && hasPrefixASCIIFold(name, lowerQuery)
}

func lowerASCII(value byte) byte {
	if value >= 'A' && value <= 'Z' {
		return value + ('a' - 'A')
	}
	return value
}

func releaseFileIndexBuildMemory() {
	runtime.GC()
	debug.FreeOSMemory()
}
