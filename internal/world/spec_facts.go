package world

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/projectdoc"
	"codenerd/internal/specs"
	"codenerd/internal/tools"
	"codenerd/internal/types"
	"codenerd/internal/world/codemodel"
)

// Spec pairing facts.
//
// A scan measures which Markdown section specifies which code element
// (internal/specs) and projects it as spec_* facts (schemas_spec.mg). The
// measurement follows the pattern the repository facts set (repo_facts.go): the
// scan attaches the complete current set to its result, and applying the result
// replaces the kernel's rows for exactly those predicates. It is not a per-file
// fact because a link's meaning crosses files: whether `Refresh` in a section
// names one element, or none, depends on every other file's declarations.
//
// What makes it incremental is the cache. Each document and each code file is
// read once per content fingerprint (size and modification time), so a scan
// after an edit rereads only the files that moved; resolving the cached scans
// against the structure index is in memory. When no file's fingerprint moved,
// the scan measures nothing and the kernel's rows stay as they are.
//
// Drift is a judgment about content (internal/specs/confirm.go): the scan
// attaches each side's fingerprint and the link's confirmation, which it reads
// from and writes back to .nerd/specs/links.json in the same step. The modified
// time of a file is not an input: a checkout, a touch or a reformat moves no link.

// SpecPredicates are the spec pairing predicates a scan replaces whole.
var SpecPredicates = specs.Predicates

// specMeasurer is a scanner's spec cache.
type specMeasurer struct {
	mu    sync.Mutex
	memo  string
	roots string
	// now is the scan clock in unix seconds; nil is the wall clock. Confirmations
	// are stamped with it, so a test that needs a pinned time sets it.
	now   func() int64
	docs  map[string]specDocEntry
	codes map[string]specCodeEntry
	files map[string]specFileEntry
}

type specDocEntry struct {
	fp   string
	scan *specs.DocScan
}

type specCodeEntry struct {
	fp   string
	scan *specs.CodeScan
}

type specFileEntry struct {
	fp   string
	file *specs.SpecFile
	err  error
}

// specCensus is what one walk of the workspace found.
type specCensus struct {
	docs  map[string]string // canonical path -> fingerprint
	codes map[string]string
	roots []string // top-level directories
	// abs maps a canonical path to its filesystem path.
	abs map[string]string
}

func fingerprintOf(info fs.FileInfo) string {
	return fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano())
}

// attachSpecFacts measures the spec pairing and attaches its facts to a scan
// result when it changed since this scanner last measured it. An error reading
// the workspace is not a failed scan: the world facts are sound and the next
// scan measures again.
func (s *Scanner) attachSpecFacts(ctx context.Context, root string, res *IncrementalResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	canonicalRoot, err := tools.CanonicalWorkspaceRoot(root)
	if err != nil {
		logging.WorldWarn("spec pairing not measured: %v", err)
		return nil
	}
	m := &s.specMeasure
	m.mu.Lock()
	defer m.mu.Unlock()

	census, err := s.specCensus(ctx, canonicalRoot)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		logging.WorldWarn("spec pairing not measured: %v", err)
		return nil
	}
	specFiles, warnings := m.readSpecFiles(canonicalRoot)
	key := m.censusKey(census, specFiles)
	if key == m.memo {
		return nil
	}

	input, err := m.input(ctx, root, census, specFiles)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		logging.WorldWarn("spec pairing not measured: %v", err)
		return nil
	}
	// The confirmations are read, the measurement judged against them, and the
	// advanced set written back as one step: the review tool shares the store.
	var result specs.Result
	_, err = specs.OpenStore(canonicalRoot).Update(func(prev specs.Confirmations) (specs.Confirmations, error) {
		input.Confirmed, input.Now = prev, m.clock()
		result = specs.Resolve(input)
		return result.Confirmed, nil
	})
	if err != nil {
		// Judging against a measurement that could not be recorded would lose
		// what a reviewer confirmed: say so, and measure again next scan.
		logging.WorldWarn("spec pairing not measured: %v", err)
		res.SpecWarnings = append(append([]string(nil), warnings...), err.Error())
		return nil
	}
	res.SpecMeasured = true
	res.SpecFacts = toWorldFacts(result.Facts())
	res.SpecWarnings = warnings
	res.SpecProblems = result.Problems
	m.memo = key
	logging.World("spec pairing measured: %d sections, %d elements, %d links, %d refused citations, %d requirements",
		len(result.Sections), len(result.Elements), len(result.Links), len(result.Problems), len(result.Requirements))
	for _, w := range warnings {
		logging.WorldWarn("spec file refused: %s", w)
	}
	return nil
}

func (m *specMeasurer) clock() int64 {
	if m.now != nil {
		return m.now()
	}
	return time.Now().Unix()
}

func toWorldFacts(in []types.Fact) []core.Fact {
	out := make([]core.Fact, len(in))
	for i, f := range in {
		out[i] = core.Fact(f)
	}
	return out
}

// specCensus walks the workspace for the files the pairing reads: Markdown, and
// the code files whose doc comments may name a spec.
func (s *Scanner) specCensus(ctx context.Context, root string) (*specCensus, error) {
	mem, err := s.membership(root)
	if err != nil {
		return nil, err
	}
	c := &specCensus{docs: map[string]string{}, codes: map[string]string{}, abs: map[string]string{}}
	topLevel := map[string]bool{}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		member, admitErr := mem.Admit(path, entry.IsDir())
		if admitErr != nil {
			return admitErr
		}
		if !member {
			return nil
		}
		canonical := canonicalScanPath(root, path)
		if entry.IsDir() {
			if !strings.Contains(canonical, "/") && canonical != "." && !strings.HasPrefix(canonical, ".") {
				topLevel[canonical] = true
			}
			return nil
		}
		isDoc := codemodel.IsMarkdownPath(path)
		if !isDoc && codemodel.LanguageOf(path) == "" {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		c.abs[canonical] = path
		if isDoc {
			c.docs[canonical] = fingerprintOf(info)
		} else {
			c.codes[canonical] = fingerprintOf(info)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// A nerd.md beside an agents.md carries that agents.md as its body
	// (projectdoc.AgentDocTwins): the same sections a second time, so it
	// would double every link and every drift row. The agents.md is the
	// source; its twin is not measured.
	for doc := range c.docs {
		if !strings.EqualFold(pathpkg.Base(doc), "agents.md") {
			continue
		}
		twin := pathpkg.Join(pathpkg.Dir(doc), projectdoc.FileName)
		if _, ok := c.docs[twin]; ok {
			delete(c.docs, twin)
			delete(c.abs, twin)
		}
	}
	for d := range topLevel {
		c.roots = append(c.roots, d)
	}
	sort.Strings(c.roots)
	return c, nil
}

// readSpecFiles parses the spec files under specs.SpecDirs. A file is parsed
// again only when its fingerprint moved. A refused file contributes nothing and
// is named in the warnings.
func (m *specMeasurer) readSpecFiles(root string) ([]*specs.SpecFile, []string) {
	if m.files == nil {
		m.files = map[string]specFileEntry{}
	}
	var files []*specs.SpecFile
	var warnings []string
	seen := map[string]bool{}
	for _, dir := range specs.SpecDirs {
		entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".mg") {
				continue
			}
			rel := dir + "/" + e.Name()
			seen[rel] = true
			info, err := e.Info()
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("%s: %v", rel, err))
				continue
			}
			fp := fingerprintOf(info)
			ent, ok := m.files[rel]
			if !ok || ent.fp != fp {
				ent = specFileEntry{fp: fp}
				data, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
				if readErr != nil {
					ent.err = readErr
				} else {
					ent.file, ent.err = specs.ParseSpecFile(rel, string(data))
				}
				m.files[rel] = ent
			}
			if ent.err != nil {
				warnings = append(warnings, ent.err.Error())
				continue
			}
			files = append(files, ent.file)
		}
	}
	for rel := range m.files {
		if !seen[rel] {
			delete(m.files, rel)
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, warnings
}

// censusKey identifies the state a measurement describes: every file the
// pairing reads, by fingerprint, and the roots paths may start at.
func (m *specMeasurer) censusKey(c *specCensus, files []*specs.SpecFile) string {
	h := sha256.New()
	write := func(kind string, set map[string]string) {
		paths := make([]string, 0, len(set))
		for p := range set {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, p := range paths {
			fmt.Fprintf(h, "%s\x00%s\x00%s\n", kind, p, set[p])
		}
	}
	write("doc", c.docs)
	write("code", c.codes)
	for _, f := range files {
		fmt.Fprintf(h, "spec\x00%s\x00%s\n", f.Path, m.files[f.Path].fp)
	}
	fmt.Fprintf(h, "roots\x00%s\n", strings.Join(c.roots, ","))
	return hex.EncodeToString(h.Sum(nil))
}

// input builds the resolver's input: rescans the files whose fingerprint
// moved, snapshots the structure index, and measures the requirement targets.
//
// root is the workspace as the caller names it, which keys the shared structure
// index: the tools and the campaign ask it by the same spelling, and a second
// spelling would build a second index.
func (m *specMeasurer) input(ctx context.Context, root string, c *specCensus, specFiles []*specs.SpecFile) (specs.Input, error) {
	roots := strings.Join(c.roots, ",")
	if roots != m.roots || m.docs == nil {
		m.docs = map[string]specDocEntry{}
		m.roots = roots
	}
	if m.codes == nil {
		m.codes = map[string]specCodeEntry{}
	}
	lex := specs.CodeLexer(c.roots)
	in := specs.Input{}
	// Only the files whose fingerprint moved are read and scanned, in parallel:
	// the first measurement of a workspace reads every document and code file.
	var staleDocs, staleCodes []string
	for path, fp := range c.docs {
		if ent, ok := m.docs[path]; !ok || ent.fp != fp {
			staleDocs = append(staleDocs, path)
		}
	}
	for path, fp := range c.codes {
		if ent, ok := m.codes[path]; !ok || ent.fp != fp {
			staleCodes = append(staleCodes, path)
		}
	}
	docScans := make([]specDocEntry, len(staleDocs))
	codeScans := make([]specCodeEntry, len(staleCodes))
	errs := make([]error, len(staleDocs)+len(staleCodes))
	var wg sync.WaitGroup
	gate := make(chan struct{}, max(runtime.NumCPU(), 2))
	run := func(slot int, path string, scan func(src string)) {
		gate <- struct{}{}
		wg.Go(func() {
			defer func() { <-gate }()
			data, err := os.ReadFile(c.abs[path])
			if err != nil {
				errs[slot] = fmt.Errorf("read %s: %w", path, err)
				return
			}
			scan(string(data))
		})
	}
	for i, path := range staleDocs {
		run(i, path, func(src string) {
			docScans[i] = specDocEntry{fp: c.docs[path], scan: specs.ScanDoc(path, src, lex)}
		})
	}
	for i, path := range staleCodes {
		run(len(staleDocs)+i, path, func(src string) {
			codeScans[i] = specCodeEntry{fp: c.codes[path], scan: specs.ScanCode(path, src)}
		})
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return in, err
	}
	for i, path := range staleDocs {
		m.docs[path] = docScans[i]
	}
	for i, path := range staleCodes {
		m.codes[path] = codeScans[i]
	}
	for path := range m.docs {
		if _, ok := c.docs[path]; !ok {
			delete(m.docs, path)
		}
	}
	for path := range m.codes {
		if _, ok := c.codes[path]; !ok {
			delete(m.codes, path)
		}
	}
	for path := range c.docs {
		in.Docs = append(in.Docs, m.docs[path].scan)
	}
	for path := range c.codes {
		if scan := m.codes[path].scan; len(scan.Cites) > 0 {
			in.Codes = append(in.Codes, scan)
		}
	}
	in.Specs = specFiles

	// Nothing to resolve means nothing to ask the structure index, whose first
	// build parses every code file in the workspace.
	if !anyCitation(in) {
		in.Code = specs.NewCodeIndex(nil, nil)
		return in, nil
	}
	var targets []string
	for _, f := range specFiles {
		for _, q := range f.Requires {
			targets = append(targets, q.CodeRef)
		}
		for _, q := range f.Calls {
			targets = append(targets, q.CodeRef)
		}
	}
	elems, files, measured, _, err := SharedStructureIndex(root).SpecSnapshot(ctx, targets)
	if err != nil {
		return in, err
	}
	for path := range c.docs {
		files = append(files, path)
	}
	in.Code = specs.NewCodeIndex(elems, files)
	in.Measure = func(e specs.Element) specs.Measurement { return measured[e.Ref] }
	return in, nil
}

func anyCitation(in specs.Input) bool {
	if len(in.Specs) > 0 || len(in.Codes) > 0 {
		return true
	}
	for _, d := range in.Docs {
		if len(d.Cites) > 0 {
			return true
		}
	}
	return false
}

// applySpecFacts replaces the kernel's spec rows with the scan's measurement,
// as the rows that left and the rows that arrived.
//
// A measurement is the complete set (about 75k rows on this repository) and it
// is retaken whenever any document or code file changes, so after every edit
// the model made. Retracting every spec predicate and loading the set again
// told the kernel 151k rows had changed when two had: a second or more of
// evaluation per edit (kernel log, "151107 changed rows, 2 inserted"). The
// kernel ends with exactly the measurement either way.
func applySpecFacts(kernel types.Kernel, res *IncrementalResult) error {
	if !res.SpecMeasured {
		return nil
	}
	measured := toTypesFacts(res.SpecFacts)
	held, ok := heldSpecFacts(kernel)
	if !ok {
		// The kernel could not say what it holds: replace whole, as before.
		for _, pred := range SpecPredicates {
			if err := kernel.Retract(pred); err != nil {
				return fmt.Errorf("retract %s: %w", pred, err)
			}
		}
		if len(measured) == 0 {
			return nil
		}
		return kernel.LoadFacts(measured)
	}
	// Identity is the fact's Mangle spelling, the form in which a row read
	// back from the kernel and the row a measurement builds agree.
	want := make(map[string]struct{}, len(measured))
	for _, f := range measured {
		want[f.String()] = struct{}{}
	}
	have := make(map[string]struct{}, len(held))
	var gone []types.Fact
	for _, f := range held {
		key := f.String()
		have[key] = struct{}{}
		if _, kept := want[key]; !kept {
			gone = append(gone, f)
		}
	}
	var arrived []types.Fact
	for _, f := range measured {
		if _, already := have[f.String()]; !already {
			arrived = append(arrived, f)
		}
	}
	if len(gone) > 0 {
		if err := kernel.RetractExactFactsBatch(gone); err != nil {
			return fmt.Errorf("retract %d spec rows: %w", len(gone), err)
		}
	}
	if len(arrived) == 0 {
		return nil
	}
	return kernel.LoadFacts(arrived)
}

// heldSpecFacts reads the kernel's current spec rows; false when any of the
// spec predicates could not be read.
func heldSpecFacts(kernel types.Kernel) ([]types.Fact, bool) {
	var held []types.Fact
	for _, pred := range SpecPredicates {
		rows, err := kernel.Query(pred)
		if err != nil {
			return nil, false
		}
		held = append(held, rows...)
	}
	return held, true
}
