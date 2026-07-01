//go:build windows

package filesearch

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	fsctlEnumUsnData     = 0x000900b3
	fsctlQueryUsnJournal = 0x000900f4
	fileAttributeDir     = 0x00000010
)

var errFileIndexRequiresAdmin = errors.New("文件索引需要管理员权限")

type usnIndexBuilder struct {
	mu        sync.RWMutex
	policy    FileSearchPolicy
	indexPath string
	readPaths []string
}

type usnJournalDataV0 struct {
	UsnJournalID    uint64
	FirstUsn        int64
	NextUsn         int64
	LowestValidUsn  int64
	MaxUsn          int64
	MaximumSize     uint64
	AllocationDelta uint64
}

type mftEnumDataV0 struct {
	StartFileReferenceNumber uint64
	LowUsn                   int64
	HighUsn                  int64
}

func newDefaultIndexBuilder() indexBuilder {
	return &usnIndexBuilder{
		policy:    DefaultFileSearchPolicy(),
		indexPath: defaultLineIndexPath(),
		readPaths: defaultReadLineIndexPaths(),
	}
}

func newSharedIndexBuilder() indexBuilder {
	path := sharedLineIndexPath()
	return &usnIndexBuilder{
		policy:    DefaultFileSearchPolicy(),
		indexPath: path,
		readPaths: []string{path},
	}
}

func (b *usnIndexBuilder) ApplyPolicy(policy FileSearchPolicy) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.policy = NormalizeFileSearchPolicy(policy)
}

func (b *usnIndexBuilder) policySnapshot() FileSearchPolicy {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.policy
}

func (b *usnIndexBuilder) indexPathSnapshot() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if strings.TrimSpace(b.indexPath) == "" {
		return defaultLineIndexPath()
	}
	return b.indexPath
}

func (b *usnIndexBuilder) readPathSnapshot() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(b.readPaths) == 0 {
		return defaultReadLineIndexPaths()
	}
	return append([]string(nil), b.readPaths...)
}

func (b *usnIndexBuilder) CachedIndex(ctx context.Context) (IndexBuildResult, error) {
	if ctx.Err() != nil {
		return IndexBuildResult{}, ctx.Err()
	}
	var lastErr error
	for _, path := range b.readPathSnapshot() {
		index, err := openLineFileIndex(path)
		if err == nil {
			return IndexBuildResult{
				Index:        index,
				IndexedCount: index.Count(),
				Volumes:      index.Volumes(),
				Elevated:     isProcessElevated(),
			}, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return IndexBuildResult{}, lastErr
	}
	return IndexBuildResult{}, errors.New("file index cache is not available")
}

func (b *usnIndexBuilder) Build(ctx context.Context) (IndexBuildResult, error) {
	elevated := isProcessElevated()
	volumes := fixedNTFSVolumes()
	result := IndexBuildResult{
		Volumes:           volumes,
		RequiresElevation: true,
		Elevated:          elevated,
	}
	if !elevated {
		result.Errors = append(result.Errors, "需要以管理员身份运行 Ariadne 后才能读取 NTFS USN/MFT")
		return result, errFileIndexRequiresAdmin
	}
	index := newCompactFileIndex(volumes)
	for _, volume := range volumes {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		if err := enumerateVolumeMFT(ctx, volume, index); err != nil {
			result.Errors = append(result.Errors, volume+" "+err.Error())
			continue
		}
	}
	if index.Count() == 0 && len(result.Errors) > 0 {
		return result, errors.New(strings.Join(result.Errors, "；"))
	}
	lineIndex, err := writeLineFileIndex(b.indexPathSnapshot(), index, b.policySnapshot())
	index = nil
	releaseFileIndexBuildMemory()
	if err != nil {
		return result, err
	}
	result.Index = lineIndex
	result.IndexedCount = lineIndex.Count()
	return result, nil
}

func fixedNTFSVolumes() []string {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil
	}
	volumes := []string{}
	for index := 0; index < 26; index++ {
		if mask&(1<<uint(index)) == 0 {
			continue
		}
		root := string(rune('A'+index)) + `:\`
		rootPtr, err := windows.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		if windows.GetDriveType(rootPtr) != windows.DRIVE_FIXED {
			continue
		}
		if !isNTFS(rootPtr) {
			continue
		}
		volumes = append(volumes, root)
	}
	return volumes
}

func isNTFS(root *uint16) bool {
	var fsName [32]uint16
	err := windows.GetVolumeInformation(root, nil, 0, nil, nil, nil, &fsName[0], uint32(len(fsName)))
	return err == nil && strings.EqualFold(windows.UTF16ToString(fsName[:]), "NTFS")
}

func isProcessElevated() bool {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return false
	}
	defer token.Close()
	return token.IsElevated()
}

type fileIndexNodeWriter interface {
	AddNode(volume string, node fileIndexNode) error
}

func enumerateVolumeMFT(ctx context.Context, volumeRoot string, writer fileIndexNodeWriter) error {
	handle, err := openVolume(volumeRoot)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)

	journal, err := queryUSNJournal(handle)
	if err != nil {
		return err
	}
	return enumerateUSNRecords(ctx, handle, journal.NextUsn, volumeRoot, writer)
}

func openVolume(volumeRoot string) (windows.Handle, error) {
	volumeRoot = strings.TrimRight(volumeRoot, `\`)
	if len(volumeRoot) < 2 || volumeRoot[1] != ':' {
		return windows.InvalidHandle, fmt.Errorf("invalid volume %q", volumeRoot)
	}
	name := `\\.\` + volumeRoot[:2]
	ptr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return windows.InvalidHandle, err
	}
	handle, err := windows.CreateFile(
		ptr,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return windows.InvalidHandle, fmt.Errorf("open NTFS volume: %w", err)
	}
	return handle, nil
}

func queryUSNJournal(handle windows.Handle) (usnJournalDataV0, error) {
	var data usnJournalDataV0
	var returned uint32
	err := windows.DeviceIoControl(
		handle,
		fsctlQueryUsnJournal,
		nil,
		0,
		(*byte)(unsafe.Pointer(&data)),
		uint32(unsafe.Sizeof(data)),
		&returned,
		nil,
	)
	if err != nil {
		return usnJournalDataV0{}, fmt.Errorf("query USN journal: %w", err)
	}
	return data, nil
}

func enumerateUSNRecords(ctx context.Context, handle windows.Handle, highUSN int64, volumeRoot string, writer fileIndexNodeWriter) error {
	input := mftEnumDataV0{HighUsn: highUSN}
	buffer := make([]byte, 1024*1024)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var returned uint32
		err := windows.DeviceIoControl(
			handle,
			fsctlEnumUsnData,
			(*byte)(unsafe.Pointer(&input)),
			uint32(unsafe.Sizeof(input)),
			&buffer[0],
			uint32(len(buffer)),
			&returned,
			nil,
		)
		if err != nil {
			if errno, ok := err.(syscall.Errno); ok && errno == windows.ERROR_HANDLE_EOF {
				break
			}
			return fmt.Errorf("enumerate MFT: %w", err)
		}
		if returned <= 8 {
			break
		}
		input.StartFileReferenceNumber = binary.LittleEndian.Uint64(buffer[:8])
		if err := parseUSNRecords(buffer[8:returned], volumeRoot, writer); err != nil {
			return err
		}
	}
	return nil
}

func parseUSNRecords(buffer []byte, volumeRoot string, writer fileIndexNodeWriter) error {
	offset := 0
	for offset+60 <= len(buffer) {
		record := buffer[offset:]
		recordLength := int(binary.LittleEndian.Uint32(record[0:4]))
		if recordLength <= 0 || offset+recordLength > len(buffer) {
			return nil
		}
		major := binary.LittleEndian.Uint16(record[4:6])
		if major == 2 {
			nameLength := int(binary.LittleEndian.Uint16(record[56:58]))
			nameOffset := int(binary.LittleEndian.Uint16(record[58:60]))
			if nameLength > 0 && nameOffset+nameLength <= recordLength {
				ref := binary.LittleEndian.Uint64(record[8:16])
				name := utf16BytesToString(record[nameOffset : nameOffset+nameLength])
				if name != "" {
					attrs := binary.LittleEndian.Uint32(record[52:56])
					if err := writer.AddNode(volumeRoot, fileIndexNode{
						Ref:         ref,
						Parent:      binary.LittleEndian.Uint64(record[16:24]),
						Name:        name,
						IsDirectory: attrs&fileAttributeDir != 0,
					}); err != nil {
						return err
					}
				}
			}
		}
		offset += recordLength
	}
	return nil
}

func utf16BytesToString(raw []byte) string {
	units := make([]uint16, len(raw)/2)
	for index := range units {
		units[index] = binary.LittleEndian.Uint16(raw[index*2 : index*2+2])
	}
	return syscall.UTF16ToString(units)
}
