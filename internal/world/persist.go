package world

import (
	"codenerd/internal/core"
	"codenerd/internal/store"
	"os"
	"strings"
)

// PersistFastSnapshotToDB writes a full fast world snapshot into the LocalStore cache.
// This is used by explicit full scans (e.g., `nerd scan`) to keep DB and scan.mg in sync.
func PersistFastSnapshotToDB(db *store.LocalStore, facts []core.Fact) error {
	if db == nil || len(facts) == 0 {
		return nil
	}
	grouped := groupFactsByPath(facts)
	for path, fs := range grouped {
		info, statErr := os.Stat(path)
		if statErr != nil {
			continue
		}
		lang := "unknown"
		for _, f := range fs {
			if f.Predicate == "file_topology" && len(f.Args) >= 3 {
				if la, ok := f.Args[2].(core.MangleAtom); ok {
					lang = strings.TrimPrefix(string(la), "/")
				}
				break
			}
		}
		fp := fileFingerprint(info)
		_ = db.UpsertWorldFile(store.WorldFileMeta{
			Path:        path,
			Lang:        lang,
			Size:        info.Size(),
			ModTime:     info.ModTime().Unix(),
			Hash:        extractHashFromFacts(fs),
			Fingerprint: fp,
		})
		inputs := make([]store.WorldFactInput, 0, len(fs))
		for _, f := range fs {
			inputs = append(inputs, store.WorldFactInput{Predicate: f.Predicate, Args: f.Args})
		}
		_ = db.ReplaceWorldFactsForFile(path, "fast", fp, inputs)
	}
	return nil
}

