package world

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/store"
	"os"
	"strings"
)

const globalWorldFactsPath = "__world_global__"

// PersistFastSnapshotToDB writes a full fast world snapshot into the LocalStore cache.
// This is used by explicit full scans (e.g., `nerd scan`) to keep DB and scan.mg in sync.
func PersistFastSnapshotToDB(db *store.LocalStore, facts []core.Fact) error {
	if db == nil || len(facts) == 0 {
		return nil
	}
	grouped := groupFactsByPath(facts)
	for path, fs := range grouped {
		lang := "unknown"
		for _, f := range fs {
			if f.Predicate == "file_topology" && len(f.Args) >= 3 {
				if la, ok := f.Args[2].(core.MangleAtom); ok {
					lang = strings.TrimPrefix(string(la), "/")
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
			info, statErr := os.Stat(path)
			if statErr != nil {
				continue
			}
			fp = fileFingerprint(info)
			meta.Size = info.Size()
			meta.ModTime = info.ModTime().Unix()
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
