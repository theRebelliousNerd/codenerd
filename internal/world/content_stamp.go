package world

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

// contentStamp is the one pre-check for incremental change detection,
// deep-fact reuse, and FileCache.Get. It is the same rule the package-symbol
// cache uses (cartographer.go revsAllowSkip / fileContentGen).
//
// Size and mtime matching is not a content identity: a same-size rewrite can
// land inside one filesystem tick, and os.Chtimes puts the mtime back while
// leaving the content generation on the write (NTFS ChangeTime, inode ctime
// elsewhere). Trust the stored hash only when size, mtime, and that
// generation are unchanged and older than symbolStampQuantum. Otherwise hash.
// The file changed only when the hash differs from the stored one, so a touch
// that rewrites identical bytes is not a change.
//
// Opening every file to read ChangeTime made a warm pass over internal/ take
// 12s (2026-09-29). dirContentGens is the directory query the symbol cache
// already uses; fileContentGen remains the per-file fallback. This is not a
// second clock.
type contentStamp struct {
	size       int64
	mtime      int64
	gen        int64
	genOK      bool
	genIsClock bool
	hash       string
}

// fileGen is one directory entry from dirContentGens.
type fileGen struct {
	size       int64
	mtime      int64
	gen        int64
	genOK      bool
	genIsClock bool
}

// dirGenSnapshot remembers one directory query per directory for the scan
// that owns it. It is not reused across scans: a later rewrite would still
// look like the generation captured here.
type dirGenSnapshot struct {
	mu sync.Mutex
	m  map[string]map[string]fileGen
}

func (s *dirGenSnapshot) of(dir string) map[string]fileGen {
	s.mu.Lock()
	if s.m == nil {
		s.m = make(map[string]map[string]fileGen)
	}
	if g, ok := s.m[dir]; ok {
		s.mu.Unlock()
		return g
	}
	s.mu.Unlock()

	loaded, have := dirContentGens(dir)
	if !have {
		// Negative cache: this OS has no bulk query, or the query failed.
		// Callers fall through to fileContentGen.
		loaded = map[string]fileGen{}
	}
	s.mu.Lock()
	if s.m[dir] == nil {
		s.m[dir] = loaded
	}
	g := s.m[dir]
	s.mu.Unlock()
	return g
}

func contentGeneration(path string, info os.FileInfo, bulk map[string]fileGen) (int64, bool, bool) {
	if g, ok := bulk[info.Name()]; ok {
		return g.gen, g.genOK, g.genIsClock
	}
	return fileContentGen(path, info)
}

func stampFromInfo(path string, info os.FileInfo, bulk map[string]fileGen) contentStamp {
	gen, ok, isClock := contentGeneration(path, info, bulk)
	return contentStamp{
		size:       info.Size(),
		mtime:      info.ModTime().UnixNano(),
		gen:        gen,
		genOK:      ok,
		genIsClock: isClock,
	}
}

func stampFromEntry(e CacheEntry) contentStamp {
	return contentStamp{
		size:       e.Size,
		mtime:      e.ModTime,
		gen:        e.Gen,
		genOK:      e.GenOK,
		genIsClock: e.GenIsClock,
		hash:       e.Hash,
	}
}

// trusts reports whether cur is still the observation stored was hashed
// from, and that observation can no longer be shared with a later write.
// A timestamp inside symbolStampQuantum is rejected: a same-tick rewrite
// can leave size, mtime, and even the generation unmoved.
func (stored contentStamp) trusts(cur contentStamp, now int64) bool {
	if stored.hash == "" || !stored.genOK || !cur.genOK {
		return false
	}
	if stored.size != cur.size || stored.mtime != cur.mtime || stored.gen != cur.gen || stored.genIsClock != cur.genIsClock {
		return false
	}
	limit := now - int64(symbolStampQuantum)
	if cur.mtime >= limit {
		return false
	}
	if cur.genIsClock && cur.gen >= limit {
		return false
	}
	return true
}

// resolveContent returns the file's hash and whether it differs from stored.
// A trusted stamp returns the stored hash without reading. Otherwise the
// file is hashed. changed is false when that hash equals the stored one,
// including a touch that only moved metadata. A read error is not a change:
// the caller must not retract rows it could not re-read.
func resolveContent(path string, cur, stored contentStamp, now int64) (hash string, changed bool, err error) {
	if stored.trusts(cur, now) {
		return stored.hash, false, nil
	}
	hash, err = calculateHash(path)
	if err != nil {
		return "", false, err
	}
	if stored.hash != "" && hash == stored.hash {
		return hash, false, nil
	}
	return hash, true, nil
}

// formatContentFingerprint is the deep-fact cache key. It carries the stamp
// and the content hash so the next scan can trust the bytes or, when the
// stamp moved, notice that the hash did not.
func formatContentFingerprint(st contentStamp, hash string) string {
	return fmt.Sprintf("c:%d:%d:%d:%d:%s", st.size, st.mtime, st.gen, stampFlags(st.genOK, st.genIsClock), hash)
}

func stampFlags(genOK, genIsClock bool) int {
	flags := 0
	if genOK {
		flags |= 1
	}
	if genIsClock {
		flags |= 2
	}
	return flags
}

func parseContentFingerprint(s string) (contentStamp, bool) {
	if !strings.HasPrefix(s, "c:") {
		return contentStamp{}, false
	}
	parts := strings.Split(s[2:], ":")
	if len(parts) != 5 {
		return contentStamp{}, false
	}
	size, err1 := strconv.ParseInt(parts[0], 10, 64)
	mtime, err2 := strconv.ParseInt(parts[1], 10, 64)
	gen, err3 := strconv.ParseInt(parts[2], 10, 64)
	flags, err4 := strconv.ParseInt(parts[3], 10, 64)
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return contentStamp{}, false
	}
	hash := parts[4]
	if len(hash) != 64 {
		return contentStamp{}, false
	}
	return contentStamp{
		size:       size,
		mtime:      mtime,
		gen:        gen,
		genOK:      flags&1 != 0,
		genIsClock: flags&2 != 0,
		hash:       hash,
	}, true
}
