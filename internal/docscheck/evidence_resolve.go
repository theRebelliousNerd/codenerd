package docscheck

import (
	"crypto/sha256"
	"fmt"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/docdecl"
)

// resolver resolves references against what the parse tree and the Mangle
// Decl statements declare. It reads tracked files once, on first use, and
// records everything it could not read as a deficit: a resolution that finds
// nothing is "unresolved" only when the domain that would hold the answer
// was fully observed, and "unobserved" otherwise.
type resolver struct {
	c        *Checker
	loaded   bool
	prod     map[string]*docdecl.Package // dir -> non-test declarations
	test     map[string]*docdecl.Package // dir -> _test.go declarations
	mg       map[string][]string         // predicate -> declaring files
	tracked  map[string]bool
	deficits []Deficit
	digests  []string
	parsed   int
	failed   int
	// goComplete / mgComplete: every tracked file of that kind was read and
	// (for Go) parsed without error.
	goComplete bool
	mgComplete bool
}

func newResolver(c *Checker) *resolver { return &resolver{c: c} }

type codeReport struct {
	digest   string
	parsed   int
	failed   int
	deficits []Deficit
}

// report finalises the code side of the manifest.
func (r *resolver) report() codeReport {
	if !r.loaded {
		return codeReport{}
	}
	sort.Strings(r.digests)
	h := sha256.New()
	for _, d := range r.digests {
		fmt.Fprintf(h, "%s\x00", d)
	}
	return codeReport{digest: fmt.Sprintf("%x", h.Sum(nil)), parsed: r.parsed, failed: r.failed, deficits: r.deficits}
}

// load indexes tracked Go and Mangle files. Paths under testdata, vendor,
// node_modules and dot directories are fixtures and crash dumps, not the
// corpus, and are skipped as docdecl skips them.
func (r *resolver) load() {
	if r.loaded {
		return
	}
	r.loaded = true
	r.prod = map[string]*docdecl.Package{}
	r.test = map[string]*docdecl.Package{}
	r.mg = map[string][]string{}
	r.tracked = map[string]bool{}
	files, err := r.c.loadTracked()
	if err != nil {
		r.deficits = append(r.deficits, Deficit{Code: "tracked_listing_failed", Detail: err.Error()})
		return
	}
	r.goComplete, r.mgComplete = true, true
	fset := token.NewFileSet()
	for _, rel := range files {
		r.tracked[rel] = true
		isGo, isMg := strings.HasSuffix(rel, ".go"), strings.HasSuffix(rel, ".mg")
		if (!isGo && !isMg) || inSkippedDir(rel) {
			continue
		}
		full, ok := workspacePath(r.c.root, rel)
		if !ok {
			continue
		}
		data, err := os.ReadFile(full)
		if err != nil {
			code := "go_unreadable"
			if isMg {
				code = "mangle_unreadable"
				r.mgComplete = false
			} else {
				r.goComplete = false
			}
			r.deficits = append(r.deficits, Deficit{Code: code, File: rel, Detail: err.Error()})
			continue
		}
		r.parsed++
		r.digests = append(r.digests, fmt.Sprintf("%s:%x", rel, sha256.Sum256(data)))
		if isMg {
			for name := range docdecl.MangleDecls(string(data)) {
				r.mg[name] = append(r.mg[name], rel)
			}
			continue
		}
		dir := path.Dir(rel)
		set := r.prod
		if strings.HasSuffix(rel, "_test.go") {
			set = r.test
		}
		pk := set[dir]
		if pk == nil {
			pk = docdecl.NewPackage(dir)
			set[dir] = pk
		}
		pk.AddFile(fset, full, rel, data)
		if msg, bad := pk.ParseErr[rel]; bad {
			r.failed++
			r.goComplete = false
			r.deficits = append(r.deficits, Deficit{Code: "go_parse_failed", File: rel, Detail: msg})
		}
	}
}

func inSkippedDir(rel string) bool {
	segs := strings.Split(rel, "/")
	for _, s := range segs[:len(segs)-1] {
		if docdecl.SkipCorpusDir(s) {
			return true
		}
	}
	return false
}

// resolve grades one reference.
func (r *resolver) resolve(ref Reference) Resolution {
	out := Resolution{Record: "resolution", RefID: ref.ID}
	switch ref.TargetKind {
	case "file":
		return r.resolveFile(out, ref.Spelling)
	case "path":
		return r.resolveFile(out, ref.Spelling)
	case "path_symbol":
		file, sym, _ := strings.Cut(ref.Spelling, "#")
		return r.resolvePathSymbol(out, file, sym)
	case "symbol":
		r.load()
		return r.resolveSymbol(out, ref.Spelling)
	case "test":
		r.load()
		return r.resolveTest(out, ref.Spelling)
	case "predicate":
		r.load()
		return r.resolvePredicate(out, ref.Spelling)
	}
	out.Grade, out.Source = GradeUnsupported, "none"
	out.Detail = "no resolver for this witness kind"
	return out
}

// absent is the grade for "nothing matched": unresolved only when the whole
// domain was observed.
func absent(complete bool) Grade {
	if complete {
		return GradeUnresolved
	}
	return GradeUnobserved
}

func (r *resolver) resolveFile(out Resolution, spelling string) Resolution {
	out.Source = "stat"
	full, ok := fileWitnessPath(r.c.root, stripLineSuffix(spelling))
	if !ok {
		out.Grade, out.Detail = GradeUnresolved, "path leaves the workspace or is not a workspace path"
		return out
	}
	st, err := os.Stat(full)
	if err != nil || st.IsDir() {
		out.Grade, out.Detail = GradeUnresolved, "no such file"
		return out
	}
	rel, _ := filepath.Rel(filepath.Clean(r.c.root), full)
	rel = filepath.ToSlash(rel)
	out.Grade, out.Entity = GradeFilePresent, rel
	out.Candidates = []Candidate{{File: rel}}
	if stripLineSuffix(spelling) != spelling {
		out.Detail = "cited lines are not read"
	}
	return out
}

// stripLineSuffix drops :N and :N-M from a cited path.
func stripLineSuffix(s string) string {
	i := strings.LastIndex(s, ":")
	if i < 0 {
		return s
	}
	tail := s[i+1:]
	lo, hi, _ := strings.Cut(tail, "-")
	if allDigits(lo) && (hi == "" || allDigits(hi)) {
		return s[:i]
	}
	return s
}

func (r *resolver) resolvePathSymbol(out Resolution, file, sym string) Resolution {
	file = stripLineSuffix(file)
	fr := r.resolveFile(Resolution{Record: out.Record, RefID: out.RefID}, file)
	if fr.Grade != GradeFilePresent {
		return fr
	}
	r.load()
	if strings.HasSuffix(file, ".mg") {
		out.Source = "mangle-decl"
		name, _, _ := strings.Cut(sym, "/")
		for _, f := range r.mg[name] {
			if f == fr.Entity {
				out.Grade, out.Entity = GradeDeclared, name
				out.Candidates = []Candidate{{File: f, Kind: "predicate"}}
				return out
			}
		}
		out.Grade = absent(r.mgComplete)
		out.Detail = "predicate has no Decl in the cited file"
		return out
	}
	out.Source = "go/parser"
	dir := path.Dir(fr.Entity)
	pk := r.prod[dir]
	if strings.HasSuffix(file, "_test.go") {
		pk = r.test[dir]
	}
	var sites []docdecl.Decl
	if pk != nil {
		sites = pk.Lookup(sym)
	}
	if len(sites) == 0 {
		out.Grade = absent(r.goComplete)
		out.Detail = "symbol is not declared in the cited file's package"
		return out
	}
	out.Candidates = declCandidates(dir, sites)
	for _, s := range sites {
		if s.File == fr.Entity {
			out.Grade, out.Entity, out.Constrained = GradeDeclared, dir+"."+sym, s.Constrained
			out.Candidates = declCandidates(dir, []docdecl.Decl{s})
			return out
		}
	}
	// Declared in the package but not in the cited file: the path is
	// wrong about where, so it does not satisfy the citation as written.
	out.Grade = GradeAmbiguous
	out.Detail = "symbol is declared in the package but not in the cited file"
	return out
}

func declCandidates(dir string, sites []docdecl.Decl) []Candidate {
	out := make([]Candidate, 0, len(sites))
	for _, s := range sites {
		out = append(out, Candidate{Package: dir, Name: s.Name, File: s.File, Line: s.Line, Kind: string(s.Kind)})
	}
	return out
}

func (r *resolver) resolveSymbol(out Resolution, sym string) Resolution {
	out.Source = "go/parser"
	var cands []Candidate
	var constrained bool
	// A spelling with a directory qualifier (internal/config.GuardedMerge)
	// names its package exactly; a bare or pkg-qualified one searches all.
	only, name := "", sym
	if i := strings.LastIndex(sym, "/"); i >= 0 {
		if j := strings.Index(sym[i:], "."); j > 0 {
			only, name = sym[:i+j], sym[i+j+1:]
		}
	}
	for _, dir := range sortedDirs(r.prod) {
		if only != "" && dir != only {
			continue
		}
		for _, s := range r.prod[dir].Lookup(name) {
			cands = append(cands, Candidate{Package: dir, Name: s.Name, File: s.File, Line: s.Line, Kind: string(s.Kind)})
			constrained = constrained || s.Constrained
		}
	}
	return r.finish(out, cands, constrained, GradeDeclared, func() (bool, error) {
		return r.c.witnessResolves("symbol:" + name)
	}, r.goComplete)
}

func (r *resolver) resolveTest(out Resolution, name string) Resolution {
	out.Source = "go/parser"
	var cands []Candidate
	var constrained bool
	for _, dir := range sortedDirs(r.test) {
		for _, s := range r.test[dir].Lookup(name) {
			if s.Kind != docdecl.KindTest {
				continue
			}
			cands = append(cands, Candidate{Package: dir, Name: s.Name, File: s.File, Line: s.Line, Kind: string(s.Kind)})
			constrained = constrained || s.Constrained
		}
	}
	res := r.finish(out, cands, constrained, GradeTestDeclared, func() (bool, error) {
		return r.c.witnessResolves("test:" + name)
	}, r.goComplete)
	if res.Grade == GradeOccurrenceOnly {
		res.Detail = "a func of this name is spelled in tracked source, but no _test.go function has a signature go test runs"
	}
	return res
}

func (r *resolver) resolvePredicate(out Resolution, pred string) Resolution {
	out.Source = "mangle-decl"
	pred, _, _ = strings.Cut(pred, "/")
	var cands []Candidate
	files := append([]string(nil), r.mg[pred]...)
	sort.Strings(files)
	for _, f := range files {
		cands = append(cands, Candidate{Name: pred, File: f, Kind: "predicate"})
	}
	return r.finish(out, cands, false, GradeDeclared, func() (bool, error) {
		return r.c.witnessResolves("predicate:" + pred)
	}, r.mgComplete)
}

// finish turns a candidate list into a grade. Zero candidates fall back to
// the lexical check only to tell "spelled somewhere" from "nowhere"; that
// answer is never better than occurrence_only.
func (r *resolver) finish(out Resolution, cands []Candidate, constrained bool, good Grade, lexical func() (bool, error), complete bool) Resolution {
	switch len(cands) {
	case 0:
		hit, err := lexical()
		switch {
		case err == nil && hit:
			out.Grade = GradeOccurrenceOnly
			out.Detail = "spelled as text in tracked source; no declaration found"
			out.Source = "text"
		case err != nil:
			out.Grade, out.Detail = GradeUnobserved, err.Error()
		default:
			out.Grade = absent(complete)
			if out.Grade == GradeUnobserved {
				out.Detail = "the tracked source was not fully observed"
			}
		}
	case 1:
		out.Grade, out.Candidates, out.Constrained = good, cands, constrained
		if cands[0].Package != "" {
			out.Entity = cands[0].Package + "." + cands[0].Name
		} else {
			out.Entity = cands[0].Name
		}
	default:
		out.Grade, out.Candidates, out.Constrained = GradeAmbiguous, cands, constrained
		out.Detail = fmt.Sprintf("%d candidates; none chosen", len(cands))
	}
	return out
}

func sortedDirs(m map[string]*docdecl.Package) []string {
	out := make([]string, 0, len(m))
	for d := range m {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}
