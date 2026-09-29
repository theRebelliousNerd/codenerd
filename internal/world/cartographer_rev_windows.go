//go:build windows

package world

import (
	"cmp"
	"errors"
	"os"
	"slices"
	"strings"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// bulkFileRevs reads size, mtime and ChangeTime for every .go sibling in one
// directory query. Opening each file for FileBasicInfo made a warm pass over
// internal/ take 12s; ReadDir-style enumeration stays near the old stat cost.
// A failure falls back to the per-file reader (handled=false).
func bulkFileRevs(dir string) ([]goFileRev, bool, error) {
	revs, err := windowsDirRevs(dir)
	if err != nil {
		return nil, false, nil
	}
	return revs, true, nil
}

type fileIDExtdDirInfo struct {
	NextEntryOffset uint32
	FileIndex       uint32
	CreationTime    windows.Filetime
	LastAccessTime  windows.Filetime
	LastWriteTime   windows.Filetime
	ChangeTime      windows.Filetime
	EndOfFile       int64
	AllocationSize  int64
	FileAttributes  uint32
	FileNameLength  uint32
	EaSize          uint32
	ReparsePointTag uint32
	FileID          [16]byte
}

// dirContentGens is fileContentGen's clock for every non-directory child,
// from one FileIdExtdDirectoryInfo query. The world scan uses it so a no-op
// pass does not open each file; opening each file for ChangeTime made a warm
// pass over internal/ take 12s (2026-09-29).
func dirContentGens(dir string) (map[string]fileGen, bool) {
	entries, err := windowsDirEntries(dir)
	if err != nil {
		return nil, false
	}
	return entries, true
}

func windowsDirRevs(dir string) ([]goFileRev, error) {
	entries, err := windowsDirEntries(dir)
	if err != nil {
		return nil, err
	}
	out := make([]goFileRev, 0, len(entries))
	for name, g := range entries {
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		out = append(out, goFileRev{
			name:       name,
			size:       g.size,
			mtime:      g.mtime,
			gen:        g.gen,
			genOK:      g.genOK,
			genIsClock: g.genIsClock,
		})
	}
	slices.SortFunc(out, func(a, b goFileRev) int { return cmp.Compare(a.name, b.name) })
	return out, nil
}

func windowsDirEntries(dir string) (map[string]fileGen, error) {
	p16, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(
		p16,
		windows.FILE_LIST_DIRECTORY,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(h)

	header := int(unsafe.Sizeof(fileIDExtdDirInfo{}))
	buf := make([]byte, 64*1024)
	out := make(map[string]fileGen)
	for {
		err = windows.GetFileInformationByHandleEx(h, windows.FileIdExtdDirectoryInfo, &buf[0], uint32(len(buf)))
		if err != nil {
			if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
				break
			}
			if errors.Is(err, windows.ERROR_MORE_DATA) && len(buf) < 1<<20 {
				buf = make([]byte, len(buf)*2)
				continue
			}
			return nil, err
		}
		offset := 0
		for {
			if offset < 0 || offset+header > len(buf) {
				return nil, windows.ERROR_INVALID_DATA
			}
			info := (*fileIDExtdDirInfo)(unsafe.Pointer(&buf[offset]))
			nameLen := int(info.FileNameLength)
			nameAt := offset + header
			if nameLen < 0 || nameLen%2 != 0 || nameAt+nameLen > len(buf) {
				return nil, windows.ERROR_INVALID_DATA
			}
			name := string(utf16.Decode(unsafe.Slice((*uint16)(unsafe.Pointer(&buf[nameAt])), nameLen/2)))
			if name != "." && name != ".." && info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
				out[name] = fileGen{
					size:       info.EndOfFile,
					mtime:      info.LastWriteTime.Nanoseconds(),
					gen:        info.ChangeTime.Nanoseconds(),
					genOK:      true,
					genIsClock: true,
				}
			}
			if info.NextEntryOffset == 0 {
				break
			}
			next := offset + int(info.NextEntryOffset)
			if next <= offset {
				return nil, windows.ERROR_INVALID_DATA
			}
			offset = next
		}
	}
	return out, nil
}

// fileContentGen is the NTFS ChangeTime. os.Chtimes puts LastWriteTime back
// and leaves ChangeTime on the content write that preceded it (measured
// 2026-09-29: restored write times compared equal, change times did not).
// A same-tick rewrite that moves neither clock is rejected by
// symbolStampQuantum, not by this value.
func fileContentGen(path string, _ os.FileInfo) (int64, bool, bool) {
	p16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, false, true
	}
	h, err := windows.CreateFile(
		p16,
		windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return 0, false, true
	}
	defer windows.CloseHandle(h)
	var basic struct {
		CreationTime   windows.Filetime
		LastAccessTime windows.Filetime
		LastWriteTime  windows.Filetime
		ChangeTime     windows.Filetime
		FileAttributes uint32
		_              uint32
	}
	err = windows.GetFileInformationByHandleEx(
		h,
		windows.FileBasicInfo,
		(*byte)(unsafe.Pointer(&basic)),
		uint32(unsafe.Sizeof(basic)),
	)
	if err != nil {
		return 0, false, true
	}
	return basic.ChangeTime.Nanoseconds(), true, true
}
