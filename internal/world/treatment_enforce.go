package world

import (
	"os"
	"path/filepath"
	"sort"

	"codenerd/internal/core"
	"codenerd/internal/workspace"
)

// Enforcement of orientation's semantic treatment before any body is read.
//
// A member file under an operator-confirmed /index_names_only or /exclude
// scope (workspace.TreatmentDecision.Enforced) is never hashed, read or
// parsed by a scan. It keeps its place in the world as metadata:
//
//   - /index_names_only: one file_metadata_only(Path, Language, LastModified,
//     Size, Treatment) row. file_topology is not emitted, because its hash
//     slot would have to be forged; file_exists is derived from both.
//   - /exclude: no per-file row. The directory fact stays.
//   - both: one treatment_scope(Dir, Treatment, Files, Bytes) row per
//     governing census directory, so "nothing found under Dir" is never read
//     as "nothing there".
//
// The rows are re-derived from the walk by every scan and never persisted. A
// file that becomes enforced drops out of the scan's file set, so the
// incremental scan retires its former hash, cache entry and body-derived
// facts through the same path as a deleted file; a file whose scope stops
// being enforced is new to the scan and is read again. An inferred treatment
// is only counted (TreatmentShadow), never enforced.

// enforcedScope is the coverage of one governing census directory.
type enforcedScope struct {
	treatment workspace.Treatment
	authority string
	files     int
	bytes     int64
}

// claim takes a member file out of the scan when its decision is enforced and
// records its metadata. It reports whether the scan must leave the file alone.
func (s *treatmentShadow) claim(root, path string, info os.FileInfo) bool {
	if s == nil || s.view == nil {
		return false
	}
	canonical := canonicalScanPath(root, path)
	d := s.view.Treatment(canonical)
	if !d.Enforced() {
		return false
	}
	sc := s.enforced[d.Dir]
	if sc == nil {
		sc = &enforcedScope{treatment: d.Treatment, authority: d.Authority}
		s.enforced[d.Dir] = sc
	}
	sc.files++
	sc.bytes += info.Size()
	if d.Treatment == workspace.TreatmentIndexNamesOnly {
		s.metadata = append(s.metadata, core.Fact{
			Predicate: "file_metadata_only",
			Args: []any{
				canonical,
				core.MangleAtom("/" + detectLanguage(filepath.Ext(path), path)),
				info.ModTime().UnixNano(),
				info.Size(),
				core.MangleAtom(string(d.Treatment)),
			},
		})
	}
	return true
}

// coverageFacts is the full replacement set of file_metadata_only and
// treatment_scope rows for the walk just made, in a stable order.
func (s *treatmentShadow) coverageFacts() []core.Fact {
	if s == nil {
		return nil
	}
	out := make([]core.Fact, 0, len(s.metadata)+len(s.enforced))
	out = append(out, s.metadata...)
	dirs := make([]string, 0, len(s.enforced))
	for dir := range s.enforced {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		sc := s.enforced[dir]
		out = append(out, core.Fact{
			Predicate: "treatment_scope",
			Args:      []any{dir, core.MangleAtom(string(sc.treatment)), int64(sc.files), sc.bytes},
		})
	}
	return out
}

// isTreatmentPredicate reports whether pred is one of TreatmentPredicates.
func isTreatmentPredicate(pred string) bool {
	for _, p := range TreatmentPredicates {
		if p == pred {
			return true
		}
	}
	return false
}
