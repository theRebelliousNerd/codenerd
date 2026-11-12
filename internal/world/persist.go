package world

import (
	"fmt"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/store"
	"codenerd/internal/types"
	"os"
	"strings"
)

const globalWorldFactsPath = "__world_global__"

// PersistFastSnapshotToDB writes a full fast world snapshot into the LocalStore cache.
// This is used by explicit full scans (e.g., `nerd scan`) to keep DB and scan.mg in sync.
//
// Fact paths are canonical (workspace-relative), so opening them requires the
// workspace root. Callers that have it should use PersistFastSnapshotToDBInRoot;
// this spelling resolves against the process working directory, which is the
// behaviour every existing caller already depended on.
func PersistFastSnapshotToDB(db *store.LocalStore, facts []core.Fact) error {
	return PersistFastSnapshotToDBInRoot(db, "", facts)
}

// PersistFastSnapshotToDBInRoot is PersistFastSnapshotToDB with an explicit
// workspace root used to resolve canonical fact paths to real files.
func PersistFastSnapshotToDBInRoot(db *store.LocalStore, root string, facts []core.Fact) error {
	if db == nil || len(facts) == 0 {
		return nil
	}
	root = workspaceRootOrCwd(root)
	grouped := groupFactsByPath(facts)
	// One transaction for the whole snapshot. Two per file (an upsert of the
	// file row, a replace of its facts) took 30 s for codeNERD's 7k files.
	updates := make([]store.FileUpdates, 0, len(grouped))
	for path, fs := range grouped {
		lang := "unknown"
		for _, f := range fs {
			if f.Predicate == "file_topology" && len(f.Args) >= 3 {
				// See detectProjectLanguage: readback gives a plain string.
				if s := types.ExtractString(f.Args[2]); s != "" {
					lang = strings.TrimPrefix(s, "/")
				}
				break
			}
		}
		fp := path
		meta := store.WorldFileMeta{
			Path:        path,
			Lang:        lang,
			Hash:        extractHashFromFacts(fs),
			Fingerprint: path,
		}
		if path != globalWorldFactsPath {
			abs := types.ResolveWorkspacePath(root, path)
			info, statErr := os.Stat(abs)
			if statErr != nil {
				continue
			}
			meta.Size = info.Size()
			meta.ModTime = info.ModTime().UnixNano()
			// Size and mtime are not an identity: a same-size rewrite keeps
			// both when os.Chtimes puts the mtime back. The fingerprint is the
			// content stamp, and only when the snapshot hash is a digest
			// parseContentFingerprint will keep. Any other hash is stored
			// empty. That string does not parse, so the next reader reparses
			// and replaces the row. A size:mtime key already on disk fails
			// the same parse and takes the same path.
			if contentHashAccepted(meta.Hash) {
				fp = formatContentFingerprint(stampFromInfo(abs, info, nil), meta.Hash)
			} else {
				fp = ""
			}
			meta.Fingerprint = fp
		}
		inputs := make([]store.WorldFactInput, 0, len(fs))
		for _, f := range fs {
			inputs = append(inputs, store.WorldFactInput{Predicate: f.Predicate, Args: f.Args})
		}
		updates = append(updates, store.FileUpdates{Meta: meta, Facts: inputs})
	}
	if err := db.UpdateWorldFilesAndFacts("fast", updates); err != nil {
		logging.WorldWarn("PersistFastSnapshotToDB: failed to write %d files: %v", len(updates), err)
		return fmt.Errorf("persist fast world snapshot (%d files): %w", len(updates), err)
	}
	return nil
}
