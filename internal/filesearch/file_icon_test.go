package filesearch

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestFileIconStoreUsesOpaqueControlledAssetURLAndCache(t *testing.T) {
	var calls atomic.Int32
	store := newFileIconStoreWithResolver(func(_ string, _ int) ([]byte, error) {
		calls.Add(1)
		return []byte("png-data"), nil
	})

	first := store.Resolve(`C:\docs\one.xlsx`, false)
	second := store.Resolve(`D:\other\two.xlsx`, false)
	if first == nil || second == nil {
		t.Fatal("expected icon assets")
	}
	if first.CacheKey != second.CacheKey || calls.Load() != 1 {
		t.Fatalf("same association should reuse one cached icon: %#v %#v calls=%d", first, second, calls.Load())
	}
	if first.Kind != "windows-shell" || !strings.HasPrefix(first.URL, fileIconPathPrefix) || strings.Contains(first.URL, "docs") || strings.Contains(first.URL, "xlsx") {
		t.Fatalf("asset URL must be controlled and opaque: %#v", first)
	}

	response := httptest.NewRecorder()
	store.AssetHandler(http.NotFoundHandler()).ServeHTTP(response, httptest.NewRequest(http.MethodGet, first.URL, nil))
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/png" || response.Body.String() != "png-data" {
		t.Fatalf("unexpected icon response: code=%d headers=%v body=%q", response.Code, response.Header(), response.Body.String())
	}
}

func TestFileIconAssetHandlerRejectsPathTraversalAndUnknownKeys(t *testing.T) {
	store := newFileIconStoreWithResolver(func(_ string, _ int) ([]byte, error) { return []byte("png"), nil })
	handler := store.AssetHandler(http.NotFoundHandler())
	for _, path := range []string{
		fileIconPathPrefix + "../../Windows/win.ini",
		fileIconPathPrefix + strings.Repeat("a", 64) + "/extra.png",
		fileIconPathPrefix + strings.Repeat("a", 64) + ".png",
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("unsafe or unknown path %q returned %d", path, response.Code)
		}
	}
}

func TestUnresolvableFileIconDoesNotBlockSearchResult(t *testing.T) {
	service := NewServiceWithIndex([]rawResult{{Name: "target.xlsx", Path: `P:\workspace\target.xlsx`}})
	service.fileIcons = newFileIconStoreWithResolver(func(_ string, _ int) ([]byte, error) { return nil, assertIconError{} })
	results := service.Search("target")
	if len(results) != 1 || results[0].Icon != "file" || results[0].IconAsset != nil {
		t.Fatalf("icon failure must preserve the semantic result: %#v", results)
	}
}

type assertIconError struct{}

func (assertIconError) Error() string { return "icon unavailable" }
