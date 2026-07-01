//go:build windows

package filesearch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

const windowsFileNotifyHeaderSize = 12

func (b *usnIndexBuilder) WatchChanges(ctx context.Context, volumes []string, emit func(rawResult)) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if emit == nil {
		return nil
	}
	roots := normalizeWatchVolumes(volumes)
	if len(roots) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	errs := make(chan error, len(roots))
	for _, root := range roots {
		root := root
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := watchVolumeChanges(ctx, root, emit); err != nil && ctx.Err() == nil {
				errs <- err
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		<-done
		return ctx.Err()
	case <-done:
	}

	close(errs)
	collected := make([]error, 0, len(errs))
	for err := range errs {
		collected = append(collected, err)
	}
	if len(collected) > 0 {
		return errors.Join(collected...)
	}
	return nil
}

func normalizeWatchVolumes(volumes []string) []string {
	seen := map[string]struct{}{}
	roots := make([]string, 0, len(volumes))
	for _, volume := range volumes {
		root := filepath.Clean(strings.TrimSpace(volume))
		if root == "" || root == "." {
			continue
		}
		if vol := filepath.VolumeName(root); vol != "" && strings.EqualFold(root, vol) {
			root = vol + string(filepath.Separator)
		}
		if vol := filepath.VolumeName(root); vol != "" && len(root) == len(vol)+1 {
			root = vol + string(filepath.Separator)
		}
		key := strings.ToLower(root)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		roots = append(roots, root)
	}
	sort.Strings(roots)
	return roots
}

func watchVolumeChanges(ctx context.Context, root string, emit func(rawResult)) error {
	handle, err := openWatchDirectory(root)
	if err != nil {
		return err
	}

	var closeOnce sync.Once
	closeHandle := func() {
		_ = windows.CloseHandle(handle)
	}
	defer closeOnce.Do(closeHandle)
	go func() {
		<-ctx.Done()
		closeOnce.Do(closeHandle)
	}()

	buffer := make([]byte, 64*1024)
	mask := uint32(windows.FILE_NOTIFY_CHANGE_FILE_NAME | windows.FILE_NOTIFY_CHANGE_DIR_NAME | windows.FILE_NOTIFY_CHANGE_CREATION)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var returned uint32
		err := windows.ReadDirectoryChanges(handle, &buffer[0], uint32(len(buffer)), true, mask, &returned, nil, 0)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			if errors.Is(err, windows.ERROR_OPERATION_ABORTED) || errors.Is(err, windows.ERROR_INVALID_HANDLE) {
				return ctx.Err()
			}
			return fmt.Errorf("watch file changes on %s: %w", root, err)
		}
		if returned == 0 {
			continue
		}
		for _, entry := range parseWindowsNotifyBuffer(root, buffer[:returned]) {
			emit(entry)
		}
	}
}

func openWatchDirectory(root string) (windows.Handle, error) {
	ptr, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return windows.InvalidHandle, err
	}
	handle, err := windows.CreateFile(
		ptr,
		windows.FILE_LIST_DIRECTORY,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return windows.InvalidHandle, fmt.Errorf("open watch root %s: %w", root, err)
	}
	return handle, nil
}

func parseWindowsNotifyBuffer(root string, buffer []byte) []rawResult {
	results := []rawResult{}
	for offset := 0; offset+windowsFileNotifyHeaderSize <= len(buffer); {
		nextOffset := int(uint32(buffer[offset]) | uint32(buffer[offset+1])<<8 | uint32(buffer[offset+2])<<16 | uint32(buffer[offset+3])<<24)
		action := uint32(buffer[offset+4]) | uint32(buffer[offset+5])<<8 | uint32(buffer[offset+6])<<16 | uint32(buffer[offset+7])<<24
		nameLength := int(uint32(buffer[offset+8]) | uint32(buffer[offset+9])<<8 | uint32(buffer[offset+10])<<16 | uint32(buffer[offset+11])<<24)
		nameStart := offset + windowsFileNotifyHeaderSize
		nameEnd := nameStart + nameLength
		if nameLength > 0 && nameEnd <= len(buffer) && shouldIndexWindowsNotifyAction(action) {
			name := utf16BytesToString(buffer[nameStart:nameEnd])
			if raw, ok := rawResultForChangedPath(filepath.Join(root, name)); ok {
				results = append(results, raw)
			}
		}
		if nextOffset == 0 {
			break
		}
		offset += nextOffset
	}
	return results
}

func shouldIndexWindowsNotifyAction(action uint32) bool {
	return action == windows.FILE_ACTION_ADDED || action == windows.FILE_ACTION_RENAMED_NEW_NAME
}

func rawResultForChangedPath(path string) (rawResult, bool) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." {
		return rawResult{}, false
	}
	for attempt := 0; attempt < 4; attempt++ {
		info, err := os.Stat(path)
		if err == nil {
			return rawResult{Name: info.Name(), Path: path, IsDirectory: info.IsDir()}, true
		}
		if attempt < 3 {
			time.Sleep(30 * time.Millisecond)
		}
	}
	name := filepath.Base(path)
	if name == "" || name == "." {
		return rawResult{}, false
	}
	return rawResult{Name: name, Path: path}, true
}
