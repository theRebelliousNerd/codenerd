package world

import (
	"sort"
	"sync"

	"codenerd/internal/logging"
	"codenerd/internal/workspace"
)

// TreatmentShadow is what orientation's semantic treatment would have kept
// a full scan from doing, had it been enforced. Treatment is shadow-only in
// this release: the scan still hashes and parses every admitted file, and
// nothing here changes a fact. Unclassified and conflicted paths keep
// current behavior and are not counted as skipped.
type TreatmentShadow struct {
	// Active is true when a treatment artifact was loaded for the scan.
	Active bool
	// Current is true when that artifact was derived for the member set the
	// scan saw; a stale artifact still reports, marked as such.
	Current    bool
	Generation string
	// Unavailable says why no artifact was used: empty when none exists
	// yet (before orientation), the refusal otherwise.
	Unavailable string
	// Files, Bytes, Hashes and BodyReads are the scan's work under
	// names-only or excluded scopes: admitted files, their bytes, content
	// hashes computed (cache misses), and body reads for parsing or headers.
	Files     int
	Bytes     int64
	Hashes    int
	BodyReads int
	// ConflictedFiles are under a scope whose claims conflict.
	ConflictedFiles int
	Dirs            []TreatmentShadowDir
}

// TreatmentShadowDir is one governing census directory's share.
type TreatmentShadowDir struct {
	Dir       string
	Treatment workspace.Treatment
	Authority string
	Files     int
	Bytes     int64
	Hashes    int
	BodyReads int
}

// treatmentShadow records shadow decisions from concurrent scan workers.
type treatmentShadow struct {
	view *workspace.TreatmentView

	mu      sync.Mutex
	summary TreatmentShadow
	dirs    map[string]*TreatmentShadowDir
}

// newTreatmentShadow loads mem's treatment view. A refused artifact is
// reported, never applied.
func newTreatmentShadow(mem *workspace.Membership) *treatmentShadow {
	s := &treatmentShadow{dirs: map[string]*TreatmentShadowDir{}}
	view, err := mem.TreatmentView()
	if err != nil {
		s.summary.Unavailable = err.Error()
		logging.Get(logging.CategoryWorld).Warn("Treatment shadow: artifact refused: %v", err)
		return s
	}
	if view == nil {
		return s
	}
	s.view = view
	s.summary.Active = true
	s.summary.Current = view.Current()
	s.summary.Generation = view.Generation()
	return s
}

// decide is the treatment of one workspace-relative path.
func (s *treatmentShadow) decide(rel string) workspace.TreatmentDecision {
	if s == nil || s.view == nil {
		return workspace.TreatmentDecision{}
	}
	return s.view.Treatment(rel)
}

// record notes the work a scan did for one file under its decision.
func (s *treatmentShadow) record(d workspace.TreatmentDecision, size int64, hashed bool, bodyReads int) {
	if s == nil || s.view == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if d.Conflict {
		s.summary.ConflictedFiles++
		return
	}
	if !d.Treatment.SkipsBodies() {
		return
	}
	dir := s.dirs[d.Dir]
	if dir == nil {
		dir = &TreatmentShadowDir{Dir: d.Dir, Treatment: d.Treatment, Authority: d.Authority}
		s.dirs[d.Dir] = dir
	}
	dir.Files++
	dir.Bytes += size
	s.summary.Files++
	s.summary.Bytes += size
	if hashed {
		dir.Hashes++
		s.summary.Hashes++
	}
	dir.BodyReads += bodyReads
	s.summary.BodyReads += bodyReads
}

// finish freezes the summary, logs it once, and returns it.
func (s *treatmentShadow) finish() *TreatmentShadow {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.summary
	out.Dirs = make([]TreatmentShadowDir, 0, len(s.dirs))
	for _, d := range s.dirs {
		out.Dirs = append(out.Dirs, *d)
	}
	sort.Slice(out.Dirs, func(i, j int) bool { return out.Dirs[i].Dir < out.Dirs[j].Dir })
	if out.Active {
		logging.World("Treatment shadow (generation %s, current=%t): would skip %d files, %d bytes, %d hashes, %d body reads across %d scopes; %d files under conflicted scopes kept",
			out.Generation, out.Current, out.Files, out.Bytes, out.Hashes, out.BodyReads, len(out.Dirs), out.ConflictedFiles)
	}
	return &out
}
