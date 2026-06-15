//go:build windows

package filesearch

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	everythingRequestFileName = 0x00000001
	everythingRequestPath     = 0x00000002
)

type everythingSDK struct {
	mu                sync.Mutex
	dll               *syscall.LazyDLL
	setSearch         *syscall.LazyProc
	setRequestFlags   *syscall.LazyProc
	setMax            *syscall.LazyProc
	query             *syscall.LazyProc
	getNumResults     *syscall.LazyProc
	getResultFileName *syscall.LazyProc
	getResultPath     *syscall.LazyProc
}

func newEverythingClient(dllPath string) (everythingClient, error) {
	if dllPath == "" {
		return nil, fmt.Errorf("Everything64.dll not found")
	}
	dll := syscall.NewLazyDLL(dllPath)
	if err := dll.Load(); err != nil {
		return nil, fmt.Errorf("load Everything SDK: %w", err)
	}
	return &everythingSDK{
		dll:               dll,
		setSearch:         dll.NewProc("Everything_SetSearchW"),
		setRequestFlags:   dll.NewProc("Everything_SetRequestFlags"),
		setMax:            dll.NewProc("Everything_SetMax"),
		query:             dll.NewProc("Everything_QueryW"),
		getNumResults:     dll.NewProc("Everything_GetNumResults"),
		getResultFileName: dll.NewProc("Everything_GetResultFileNameW"),
		getResultPath:     dll.NewProc("Everything_GetResultPathW"),
	}, nil
}

func (c *everythingSDK) Search(query string, maxResults uint32) ([]rawResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	queryPtr, err := windows.UTF16PtrFromString(query)
	if err != nil {
		return nil, err
	}
	c.setSearch.Call(uintptr(unsafe.Pointer(queryPtr)))
	c.setRequestFlags.Call(uintptr(everythingRequestFileName | everythingRequestPath))
	c.setMax.Call(uintptr(maxResults))
	ok, _, callErr := c.query.Call(1)
	if ok == 0 {
		if callErr != syscall.Errno(0) {
			return nil, callErr
		}
		return nil, fmt.Errorf("Everything query returned false")
	}
	count, _, _ := c.getNumResults.Call()
	results := make([]rawResult, 0, int(count))
	for index := uintptr(0); index < count; index++ {
		namePtr, _, _ := c.getResultFileName.Call(index)
		pathPtr, _, _ := c.getResultPath.Call(index)
		if namePtr == 0 {
			continue
		}
		name := windows.UTF16PtrToString((*uint16)(unsafe.Pointer(namePtr)))
		dir := ""
		if pathPtr != 0 {
			dir = windows.UTF16PtrToString((*uint16)(unsafe.Pointer(pathPtr)))
		}
		fullPath := name
		if dir != "" {
			fullPath = filepath.Join(dir, name)
		}
		results = append(results, rawResult{Name: name, Path: fullPath})
	}
	return results, nil
}

func (c *everythingSDK) SearchContext(ctx context.Context, query string, maxResults uint32) ([]rawResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	results, err := c.Search(query, maxResults)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	return results, err
}
