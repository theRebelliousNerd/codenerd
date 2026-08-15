package world

import (
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
			info, statErr := os.Stat(ResolveWorkspacePath(root, path))
			if statErr != nil {
				continue
			}
			fp = fileFingerprint(info)
			meta.Size = info.Size()
			meta.ModTime = info.ModTime().UnixNano()
			meta.Fingerprint = fp
		}
		if err := db.UpsertWorldFile(meta); err != nil {
			logging.WorldWarn("PersistFastSnapshotToDB: failed to upsert world file %s: %v", path, err)
		}
		inputs := make([]store.WorldFactInput, 0, len(fs))
		for _, f := range fs {
			inputs = append(inputs, store.WorldFactInput{Predicate: f.Predicate, Args: f.Args})
		}
		if err := db.ReplaceWorldFactsForFile(path, "fast", fp, inputs); err != nil {
			logging.WorldWarn("PersistFastSnapshotToDB: failed to replace world facts for file %s: %v", path, err)
		}
	}
	return nil
}
