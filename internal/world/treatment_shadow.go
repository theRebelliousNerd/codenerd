package world

import (
	"sort"
	"sync"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/workspace"
)

// TreatmentShadow reports orientation's semantic treatment over one scan.
// Operator-confirmed scopes (workspace.TreatmentDecision.Enforced) are
// enforced: their files are neither hashed, read nor parsed, and the scan
// emits file_metadata_only and treatment_scope rows instead (see
// treatment_enforce.go). Every other body-skipping scope is inferred, so it
// stays shadow: the scan still does the work and the fields below only
// count what enforcing it would have saved. Unclassified, stale and
// conflicted paths keep current behavior.
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
	// EnforcedFiles and EnforcedBytes are the member files left unread
	// under operator-confirmed scopes. They are not part of Files.
	EnforcedFiles int
	EnforcedBytes int64
	Dirs          []TreatmentShadowDir
}

// TreatmentShadowDir is one governing census directory's share.
type TreatmentShadowDir struct {
	Dir       string
	Treatment workspace.Treatment
	Authority string
	// Enforced scopes were not read; the rest were read and only counted.
	Enforced  bool
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

	// enforced is the coverage of operator-confirmed scopes, keyed by the
	// governing census directory. Written only by the (serial) walk.
	enforced map[string]*enforcedScope
	metadata []core.Fact
}

// newTreatmentShadow loads mem's treatment view. A refused artifact is
// reported, never applied.
func newTreatmentShadow(mem *workspace.Membership) *treatmentShadow {
	s := &treatmentShadow{dirs: map[string]*TreatmentShadowDir{}, enforced: map[string]*enforcedScope{}}
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
	if !d.Treatment.SkipsBodies() || d.Enforced() {
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
	out.Dirs = make([]TreatmentShadowDir, 0, len(s.dirs)+len(s.enforced))
	for _, d := range s.dirs {
		out.Dirs = append(out.Dirs, *d)
	}
	for dir, sc := range s.enforced {
		out.Dirs = append(out.Dirs, TreatmentShadowDir{
			Dir: dir, Treatment: sc.treatment, Authority: sc.authority,
			Enforced: true, Files: sc.files, Bytes: sc.bytes,
		})
		out.EnforcedFiles += sc.files
		out.EnforcedBytes += sc.bytes
	}
	sort.Slice(out.Dirs, func(i, j int) bool { return out.Dirs[i].Dir < out.Dirs[j].Dir })
	if out.Active {
		logging.World("Treatment (generation %s, current=%t): enforced over %d files, %d bytes left unread; shadow would skip %d more files, %d bytes, %d hashes, %d body reads; %d files under conflicted scopes kept",
			out.Generation, out.Current, out.EnforcedFiles, out.EnforcedBytes, out.Files, out.Bytes, out.Hashes, out.BodyReads, out.ConflictedFiles)
	}
	return &out
}
