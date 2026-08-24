package filesearch

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"ariadne/internal/contracts"
)

const fileIconPathPrefix = "/ariadne-file-icons/"

type fileIconResolver func(path string, size int) ([]byte, error)

type FileIconStore struct {
	mu         sync.RWMutex
	resolver   fileIconResolver
	byIdentity map[string]string
	assets     map[string][]byte
	order      []string
	maxEntries int
}

func NewFileIconStore() *FileIconStore {
	return newFileIconStoreWithResolver(resolveWindowsShellIconPNG)
}

func newFileIconStoreWithResolver(resolver fileIconResolver) *FileIconStore {
	return &FileIconStore{
		resolver:   resolver,
		byIdentity: make(map[string]string),
		assets:     make(map[string][]byte),
		maxEntries: 256,
	}
}

func (s *FileIconStore) Resolve(path string, isDirectory bool) *contracts.IconAsset {
	if s == nil || s.resolver == nil || strings.TrimSpace(path) == "" {
		return nil
	}
	identity := fileIconIdentity(path, isDirectory)
	s.mu.RLock()
	key := s.byIdentity[identity]
	_, found := s.assets[key]
	s.mu.RUnlock()
	if found {
		return fileIconAsset(key)
	}

	pngBytes, err := s.resolver(path, 48)
	if err != nil || len(pngBytes) == 0 {
		return nil
	}
	digest := sha256.Sum256(append([]byte(identity+"\x00"), pngBytes...))
	key = hex.EncodeToString(digest[:])

	s.mu.Lock()
	if existing := s.byIdentity[identity]; existing != "" {
		key = existing
	} else {
		s.byIdentity[identity] = key
		s.assets[key] = append([]byte(nil), pngBytes...)
		s.order = append(s.order, key)
		for len(s.order) > s.maxEntries {
			oldest := s.order[0]
			s.order = s.order[1:]
			delete(s.assets, oldest)
			for cachedIdentity, cachedKey := range s.byIdentity {
				if cachedKey == oldest {
					delete(s.byIdentity, cachedIdentity)
				}
			}
		}
	}
	s.mu.Unlock()
	return fileIconAsset(key)
}

func (s *FileIconStore) AssetHandler(next http.Handler) http.Handler {
	if next == nil {
		next = http.NotFoundHandler()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, fileIconPathPrefix) {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		key := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, fileIconPathPrefix), ".png")
		if !validFileIconKey(key) || r.URL.Path != fileIconPathPrefix+key+".png" {
			http.NotFound(w, r)
			return
		}
		s.mu.RLock()
		asset := append([]byte(nil), s.assets[key]...)
		s.mu.RUnlock()
		if len(asset) == 0 {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "private, max-age=86400, immutable")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method == http.MethodGet {
			_, _ = w.Write(asset)
		}
	})
}

func fileIconIdentity(path string, isDirectory bool) string {
	if isDirectory {
		return "directory"
	}
	extension := strings.ToLower(filepath.Ext(path))
	switch extension {
	case ".exe", ".lnk", ".ico":
		return "path:" + strings.ToLower(filepath.Clean(path))
	case "":
		return "file"
	default:
		return "extension:" + extension
	}
}

func fileIconAsset(key string) *contracts.IconAsset {
	return &contracts.IconAsset{URL: fileIconPathPrefix + key + ".png", CacheKey: key, Kind: "windows-shell"}
}

func validFileIconKey(key string) bool {
	if len(key) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(key)
	return err == nil
}
