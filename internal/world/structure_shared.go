package world

import (
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// SharedStructureIndex returns the process's structure index for a workspace
// root, creating it on first use. A consumer outside the structural tools --
// the campaign resolving what a task's brief names, to preload it -- asks here
// rather than building its own: one index over this repository is ~2,800
// files, ~40,000 declarations and ~70 MB of heap (measured 2026-09-23), and
// two instances also parse every changed file twice.
//
// The root is the key after cleaning and, on Windows, case folding: the same
// workspace spelled two ways is one index.
func SharedStructureIndex(root string) *StructureIndex {
	key := filepath.Clean(root)
	if abs, err := filepath.Abs(key); err == nil {
		key = abs
	}
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	sharedIndexesMu.Lock()
	defer sharedIndexesMu.Unlock()
	if idx, ok := sharedIndexes[key]; ok {
		return idx
	}
	idx := NewStructureIndex(root)
	sharedIndexes[key] = idx
	return idx
}

var (
	sharedIndexesMu sync.Mutex
	sharedIndexes   = map[string]*StructureIndex{}
)
